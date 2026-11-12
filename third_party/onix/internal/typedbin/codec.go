// Package typedbin encodes values using their Go type as the schema.
package typedbin

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"reflect"
	"slices"
	"strconv"
	"sync"
	"unsafe"

	"github.com/onix-engine/onix/internal/reuse"
)

const (
	headerSize            = 16
	maxZeroSizeCollection = 1 << 20
	schemaReference       = '#'
)

var magic = [8]byte{'O', 'N', 'I', 'X', 'B', 'I', 'N', '1'}

type schemaResult struct {
	err         error
	fingerprint uint64
}

var schemaCache sync.Map

var nativeLittleEndian = func() bool {
	value := uint16(1)
	return *(*byte)(unsafe.Pointer(&value)) == 1
}()

// IsEncoded reports whether data starts with the typed binary format marker.
func IsEncoded(data []byte) bool {
	return len(data) >= len(magic) && bytes.Equal(data[:len(magic)], magic[:])
}

// Marshal encodes value using T as an implicit schema.
func Marshal[T any](value T) ([]byte, error) {
	typeOf := reflect.TypeFor[T]()
	fingerprint, err := schemaFingerprint(typeOf)
	if err != nil {
		return nil, err
	}
	encoder := valueEncoder{data: make([]byte, headerSize)}
	copy(encoder.data, magic[:])
	binary.LittleEndian.PutUint64(encoder.data[8:16], fingerprint)
	if err := encoder.encode(reflect.ValueOf(&value).Elem(), typeOf); err != nil {
		return nil, err
	}
	return encoder.data, nil
}

// Unmarshal decodes data directly into T.
func Unmarshal[T any](data []byte) (T, error) {
	var value T
	if len(data) < headerSize || !IsEncoded(data) {
		return value, errors.New("typedbin: invalid format marker")
	}
	typeOf := reflect.TypeFor[T]()
	fingerprint, err := schemaFingerprint(typeOf)
	if err != nil {
		return value, err
	}
	if encoded := binary.LittleEndian.Uint64(data[8:16]); encoded != fingerprint {
		return value, fmt.Errorf("typedbin: schema mismatch: encoded %016x, target %016x", encoded, fingerprint)
	}
	decoder := valueDecoder{data: data[headerSize:]}
	if err := decoder.decode(reflect.ValueOf(&value).Elem(), typeOf); err != nil {
		return value, err
	}
	if len(decoder.data) != 0 {
		return value, fmt.Errorf("typedbin: %d trailing bytes", len(decoder.data))
	}
	return value, nil
}

func schemaFingerprint(root reflect.Type) (uint64, error) {
	if cached, ok := schemaCache.Load(root); ok {
		result := cached.(schemaResult)
		return result.fingerprint, result.err
	}
	fingerprint, err := buildSchemaFingerprint(root)
	result, _ := schemaCache.LoadOrStore(root, schemaResult{fingerprint: fingerprint, err: err})
	cached := result.(schemaResult)
	return cached.fingerprint, cached.err
}

func buildSchemaFingerprint(root reflect.Type) (uint64, error) {
	hash := fnv.New64a()
	seen := make(map[reflect.Type]uint64)
	var appendType func(reflect.Type) error
	appendString := func(value string) {
		var length [binary.MaxVarintLen64]byte
		n := binary.PutUvarint(length[:], uint64(len(value)))
		_, _ = hash.Write(length[:n])
		_, _ = hash.Write([]byte(value))
	}
	appendType = func(typeOf reflect.Type) error {
		if id, ok := seen[typeOf]; ok {
			_, _ = hash.Write([]byte{schemaReference})
			appendString(strconv.FormatUint(id, 10))
			return nil
		}
		seen[typeOf] = uint64(len(seen))
		_, _ = hash.Write([]byte{byte(typeOf.Kind())})
		appendString(typeOf.PkgPath())
		appendString(typeOf.Name())
		switch typeOf.Kind() {
		case reflect.Bool,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64, reflect.String:
			return nil
		case reflect.Array:
			appendString(strconv.Itoa(typeOf.Len()))
			return appendType(typeOf.Elem())
		case reflect.Pointer, reflect.Slice:
			return appendType(typeOf.Elem())
		case reflect.Map:
			if err := appendType(typeOf.Key()); err != nil {
				return err
			}
			return appendType(typeOf.Elem())
		case reflect.Struct:
			for index := range typeOf.NumField() {
				field := typeOf.Field(index)
				if !field.IsExported() {
					return fmt.Errorf("typedbin: %s contains unexported field %s", typeOf, field.Name)
				}
				appendString(field.Name)
				if err := appendType(field.Type); err != nil {
					return err
				}
			}
			return nil
		default:
			return fmt.Errorf("typedbin: unsupported type %s", typeOf)
		}
	}
	if err := appendType(root); err != nil {
		return 0, err
	}
	return hash.Sum64(), nil
}

type visit struct {
	typeOf  reflect.Type
	pointer uintptr
}

type valueEncoder struct {
	pointers map[visit]struct{}
	data     []byte
}

func (e *valueEncoder) encode(value reflect.Value, typeOf reflect.Type) error {
	switch typeOf.Kind() {
	case reflect.Bool:
		e.data = append(e.data, byteFromBool(value.Bool()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		e.appendVarint(value.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		e.appendUvarint(value.Uint())
	case reflect.Float32:
		e.appendFixed(uint64(math.Float32bits(float32(value.Float()))), 4)
	case reflect.Float64:
		e.appendFixed(math.Float64bits(value.Float()), 8)
	case reflect.String:
		e.appendBytes([]byte(value.String()))
	case reflect.Array:
		if typeOf.Elem().Kind() == reflect.Uint32 && typeOf.Len() != 0 && value.CanAddr() {
			e.encodeUint32s(unsafe.Slice((*uint32)(value.Addr().UnsafePointer()), typeOf.Len()))
			return nil
		}
		if nativeLittleEndian && typeOf.Elem().Kind() == reflect.Float32 && typeOf.Len() != 0 && value.CanAddr() {
			e.data = append(e.data, unsafe.Slice((*byte)(value.Addr().UnsafePointer()), typeOf.Len()*4)...)
			return nil
		}
		for index := range typeOf.Len() {
			if err := e.encode(value.Index(index), typeOf.Elem()); err != nil {
				return atIndex(index, err)
			}
		}
	case reflect.Pointer:
		if value.IsNil() {
			e.data = append(e.data, 0)
			return nil
		}
		e.data = append(e.data, 1)
		current := visit{typeOf: typeOf, pointer: value.Pointer()}
		if _, cyclic := e.pointers[current]; cyclic {
			return fmt.Errorf("typedbin: cyclic pointer of type %s", typeOf)
		}
		if e.pointers == nil {
			e.pointers = make(map[visit]struct{})
		}
		e.pointers[current] = struct{}{}
		err := e.encode(value.Elem(), typeOf.Elem())
		delete(e.pointers, current)
		return err
	case reflect.Slice:
		if value.IsNil() {
			e.appendUvarint(0)
			return nil
		}
		e.appendUvarint(uint64(value.Len()) + 1)
		if typeOf.Elem().Kind() == reflect.Uint32 && value.Len() != 0 {
			e.encodeUint32s(unsafe.Slice((*uint32)(value.UnsafePointer()), value.Len()))
			return nil
		}
		if nativeLittleEndian && typeOf.Elem().Kind() == reflect.Float32 && value.Len() != 0 {
			e.data = append(e.data, unsafe.Slice((*byte)(value.UnsafePointer()), value.Len()*4)...)
			return nil
		}
		if nativeLittleEndian && value.Len() != 0 {
			if layout, ok := numericStructLayout(typeOf.Elem()); ok {
				e.encodeNumericStructs(value, layout)
				return nil
			}
		}
		for index := range value.Len() {
			if err := e.encode(value.Index(index), typeOf.Elem()); err != nil {
				return atIndex(index, err)
			}
		}
	case reflect.Map:
		return e.encodeMap(value, typeOf)
	case reflect.Struct:
		fields := typeOf.NumField()
		bitmapStart := len(e.data)
		e.data = append(e.data, make([]byte, (fields+7)/8)...)
		for index := range fields {
			field := value.Field(index)
			if field.IsZero() {
				continue
			}
			// Nested fields can grow data, so address the bitmap by offset.
			e.data[bitmapStart+index/8] |= 1 << (index % 8)
			if err := e.encode(field, typeOf.Field(index).Type); err != nil {
				return atField(typeOf.Field(index).Name, err)
			}
		}
	default:
		return fmt.Errorf("typedbin: unsupported type %s", typeOf)
	}
	return nil
}

type encodedMapEntry struct {
	value            reflect.Value
	keyStart, keyEnd int
}

type mapEncodeScratch struct {
	keys    valueEncoder
	entries []encodedMapEntry
}

const (
	maxCachedMapEntries  = 4096
	maxCachedMapKeyBytes = 256 << 10
)

var mapEncodePool = sync.Pool{New: func() any { return new(mapEncodeScratch) }}

func releaseMapEncodeScratch(scratch *mapEncodeScratch) {
	scratch.entries = reuse.Reset(scratch.entries, maxCachedMapEntries)
	if cap(scratch.keys.data) > maxCachedMapKeyBytes {
		scratch.keys.data = nil
	} else {
		scratch.keys.data = scratch.keys.data[:0]
	}
	scratch.keys.pointers = nil
	mapEncodePool.Put(scratch)
}

func (e *valueEncoder) encodeMap(value reflect.Value, typeOf reflect.Type) error {
	if value.IsNil() {
		e.appendUvarint(0)
		return nil
	}
	e.appendUvarint(uint64(value.Len()) + 1)
	if value.Len() == 0 {
		return nil
	}
	// Each active map owns its scratch until all values have been encoded;
	// nested maps therefore cannot overwrite their parent's sorted entries.
	scratch := mapEncodePool.Get().(*mapEncodeScratch)
	defer releaseMapEncodeScratch(scratch)
	scratch.entries = reuse.Resize(scratch.entries, value.Len())
	iterator := value.MapRange()
	for index := 0; iterator.Next(); index++ {
		start := len(scratch.keys.data)
		if err := scratch.keys.encode(iterator.Key(), typeOf.Key()); err != nil {
			return fmt.Errorf("typedbin: encode map key: %w", err)
		}
		scratch.entries[index] = encodedMapEntry{keyStart: start, keyEnd: len(scratch.keys.data), value: iterator.Value()}
	}
	keys := scratch.keys.data
	slices.SortFunc(scratch.entries, func(left, right encodedMapEntry) int {
		return bytes.Compare(keys[left.keyStart:left.keyEnd], keys[right.keyStart:right.keyEnd])
	})
	for index, entry := range scratch.entries {
		e.data = append(e.data, keys[entry.keyStart:entry.keyEnd]...)
		if err := e.encode(entry.value, typeOf.Elem()); err != nil {
			return atIndex(index, err)
		}
	}
	return nil
}

func (e *valueEncoder) appendVarint(value int64) {
	var encoded [binary.MaxVarintLen64]byte
	length := binary.PutVarint(encoded[:], value)
	e.data = append(e.data, encoded[:length]...)
}

func (e *valueEncoder) appendUvarint(value uint64) {
	var encoded [binary.MaxVarintLen64]byte
	length := binary.PutUvarint(encoded[:], value)
	e.data = append(e.data, encoded[:length]...)
}

func (e *valueEncoder) appendFixed(value uint64, size int) {
	for shift := range size {
		e.data = append(e.data, byte(value>>uint(shift*8)))
	}
}

func (e *valueEncoder) appendBytes(value []byte) {
	e.appendUvarint(uint64(len(value)))
	e.data = append(e.data, value...)
}

type valueDecoder struct{ data []byte }

func (d *valueDecoder) decode(value reflect.Value, typeOf reflect.Type) error {
	switch typeOf.Kind() {
	case reflect.Bool:
		encoded, err := d.takeByte()
		if err != nil {
			return err
		}
		if encoded > 1 {
			return fmt.Errorf("typedbin: invalid bool %d", encoded)
		}
		value.SetBool(encoded == 1)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		encoded, err := d.takeVarint()
		if err != nil {
			return err
		}
		if value.OverflowInt(encoded) {
			return fmt.Errorf("typedbin: %d overflows %s", encoded, typeOf)
		}
		value.SetInt(encoded)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		encoded, err := d.takeUvarint()
		if err != nil {
			return err
		}
		if value.OverflowUint(encoded) {
			return fmt.Errorf("typedbin: %d overflows %s", encoded, typeOf)
		}
		value.SetUint(encoded)
	case reflect.Float32:
		encoded, err := d.takeFixed(4)
		if err != nil {
			return err
		}
		value.SetFloat(float64(math.Float32frombits(uint32(encoded))))
	case reflect.Float64:
		encoded, err := d.takeFixed(8)
		if err != nil {
			return err
		}
		value.SetFloat(math.Float64frombits(encoded))
	case reflect.String:
		encoded, err := d.takeBytes()
		if err != nil {
			return err
		}
		value.SetString(string(encoded))
	case reflect.Array:
		if typeOf.Elem().Kind() == reflect.Uint32 && typeOf.Len() != 0 {
			return d.decodeUint32s(unsafe.Slice((*uint32)(value.Addr().UnsafePointer()), typeOf.Len()), typeOf.Elem())
		}
		if nativeLittleEndian && typeOf.Elem().Kind() == reflect.Float32 && typeOf.Len() != 0 {
			encoded, err := d.take(typeOf.Len() * 4)
			if err != nil {
				return err
			}
			copy(unsafe.Slice((*byte)(value.Addr().UnsafePointer()), typeOf.Len()*4), encoded)
			break
		}
		for index := range typeOf.Len() {
			if err := d.decode(value.Index(index), typeOf.Elem()); err != nil {
				return atIndex(index, err)
			}
		}
	case reflect.Pointer:
		present, err := d.takeByte()
		if err != nil {
			return err
		}
		if present > 1 {
			return fmt.Errorf("typedbin: invalid pointer marker %d", present)
		}
		if present == 1 {
			value.Set(reflect.New(typeOf.Elem()))
			return d.decode(value.Elem(), typeOf.Elem())
		}
	case reflect.Slice:
		length, isNil, err := d.takeCollectionLength()
		if err != nil {
			return err
		}
		if !isNil {
			if err := d.validateCollectionLength(length, minimumEncodedSize(typeOf.Elem())); err != nil {
				return err
			}
			value.Set(reflect.MakeSlice(typeOf, length, length))
			if typeOf.Elem().Kind() == reflect.Uint32 && length != 0 {
				return d.decodeUint32s(unsafe.Slice((*uint32)(value.UnsafePointer()), length), typeOf.Elem())
			}
			if nativeLittleEndian && typeOf.Elem().Kind() == reflect.Float32 && length != 0 {
				encoded, err := d.take(length * 4)
				if err != nil {
					return err
				}
				copy(unsafe.Slice((*byte)(value.UnsafePointer()), length*4), encoded)
				break
			}
			if nativeLittleEndian && length != 0 {
				if layout, ok := numericStructLayout(typeOf.Elem()); ok {
					return d.decodeNumericStructs(value, layout)
				}
			}
			for index := range length {
				if err := d.decode(value.Index(index), typeOf.Elem()); err != nil {
					return atIndex(index, err)
				}
			}
		}
	case reflect.Map:
		length, isNil, err := d.takeCollectionLength()
		if err != nil {
			return err
		}
		if !isNil {
			minimum := minimumEncodedSize(typeOf.Key()) + minimumEncodedSize(typeOf.Elem())
			if err := d.validateCollectionLength(length, minimum); err != nil {
				return err
			}
			value.Set(reflect.MakeMapWithSize(typeOf, length))
			if length == 0 {
				break
			}
			key := reflect.New(typeOf.Key()).Elem()
			item := reflect.New(typeOf.Elem()).Elem()
			for index := range length {
				key.SetZero()
				if err := d.decode(key, typeOf.Key()); err != nil {
					return fmt.Errorf("typedbin: decode map key %d: %w", index, err)
				}
				item.SetZero()
				if err := d.decode(item, typeOf.Elem()); err != nil {
					return atIndex(index, err)
				}
				value.SetMapIndex(key, item)
			}
		}
	case reflect.Struct:
		fields := typeOf.NumField()
		bitmap, err := d.take((fields + 7) / 8)
		if err != nil {
			return err
		}
		if fields%8 != 0 && bitmap[len(bitmap)-1]>>uint(fields%8) != 0 {
			return fmt.Errorf("typedbin: invalid field bitmap for %s", typeOf)
		}
		for index := range fields {
			if bitmap[index/8]&(1<<(index%8)) == 0 {
				continue
			}
			if err := d.decode(value.Field(index), typeOf.Field(index).Type); err != nil {
				return atField(typeOf.Field(index).Name, err)
			}
		}
	default:
		return fmt.Errorf("typedbin: unsupported type %s", typeOf)
	}
	return nil
}

func (d *valueDecoder) take(size int) ([]byte, error) {
	if size < 0 || size > len(d.data) {
		return nil, errors.New("typedbin: truncated data")
	}
	value := d.data[:size]
	d.data = d.data[size:]
	return value, nil
}

func (d *valueDecoder) takeByte() (byte, error) {
	value, err := d.take(1)
	if err != nil {
		return 0, err
	}
	return value[0], nil
}

func (d *valueDecoder) takeVarint() (int64, error) {
	value, length := binary.Varint(d.data)
	if length <= 0 {
		return 0, errors.New("typedbin: invalid signed integer")
	}
	d.data = d.data[length:]
	return value, nil
}

func (d *valueDecoder) takeUvarint() (uint64, error) {
	value, length := binary.Uvarint(d.data)
	if length <= 0 {
		return 0, errors.New("typedbin: invalid unsigned integer")
	}
	d.data = d.data[length:]
	return value, nil
}

func (d *valueDecoder) takeFixed(size int) (uint64, error) {
	encoded, err := d.take(size)
	if err != nil {
		return 0, err
	}
	var value uint64
	for shift, item := range encoded {
		value |= uint64(item) << uint(shift*8)
	}
	return value, nil
}

func (d *valueDecoder) takeBytes() ([]byte, error) {
	length, err := d.takeUvarint()
	if err != nil {
		return nil, err
	}
	if length > uint64(len(d.data)) {
		return nil, errors.New("typedbin: truncated bytes")
	}
	return d.take(int(length))
}

func (d *valueDecoder) takeCollectionLength() (length int, isNil bool, err error) {
	encoded, err := d.takeUvarint()
	if err != nil {
		return 0, false, err
	}
	if encoded == 0 {
		return 0, true, nil
	}
	encoded--
	if encoded > uint64(^uint(0)>>1) {
		return 0, false, fmt.Errorf("typedbin: collection length %d overflows int", encoded)
	}
	return int(encoded), false, nil
}

func (d *valueDecoder) validateCollectionLength(length, minimumSize int) error {
	if minimumSize == 0 {
		if length > maxZeroSizeCollection {
			return fmt.Errorf("typedbin: zero-size collection length %d exceeds limit", length)
		}
		return nil
	}
	if length > len(d.data)/minimumSize {
		return fmt.Errorf("typedbin: collection length %d exceeds remaining data", length)
	}
	return nil
}

func minimumEncodedSize(typeOf reflect.Type) int {
	switch typeOf.Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.String, reflect.Pointer, reflect.Slice, reflect.Map:
		return 1
	case reflect.Float32:
		return 4
	case reflect.Float64:
		return 8
	case reflect.Array:
		return typeOf.Len() * minimumEncodedSize(typeOf.Elem())
	case reflect.Struct:
		return (typeOf.NumField() + 7) / 8
	default:
		return 0
	}
}

func byteFromBool(value bool) byte {
	if value {
		return 1
	}
	return 0
}

func atField(field string, err error) error {
	return fmt.Errorf("typedbin: field %s: %w", field, err)
}

func atIndex(index int, err error) error {
	return fmt.Errorf("typedbin: index %d: %w", index, err)
}
