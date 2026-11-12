package typedbin

import (
	"fmt"
	"reflect"
	"unsafe"
)

// Compiled vertices and skin weights use fixed numeric fields. The per-type
// layout is resolved once per slice; each row keeps its ONIXBIN1 field bitmap.
type numericFieldKind uint8

const (
	numericFloatArray numericFieldKind = iota
	numericUintArray
	numericBool
)

type numericField struct {
	elem   reflect.Type
	name   string
	offset uintptr
	length int
	kind   numericFieldKind
}

type numericLayout struct {
	fields [8]numericField
	count  int
	size   uintptr
}

func numericStructLayout(typeOf reflect.Type) (numericLayout, bool) {
	var layout numericLayout
	if typeOf.Kind() != reflect.Struct || typeOf.NumField() == 0 || typeOf.NumField() > len(layout.fields) {
		return layout, false
	}
	layout.count, layout.size = typeOf.NumField(), typeOf.Size()
	for index := range layout.count {
		field := typeOf.Field(index)
		if !field.IsExported() {
			return numericLayout{}, false
		}
		entry := numericField{name: field.Name, offset: field.Offset}
		switch field.Type.Kind() {
		case reflect.Array:
			entry.length = field.Type.Len()
			entry.elem = field.Type.Elem()
			switch field.Type.Elem().Kind() {
			case reflect.Float32:
				entry.kind = numericFloatArray
			case reflect.Uint32:
				entry.kind = numericUintArray
			default:
				return numericLayout{}, false
			}
		case reflect.Bool:
			entry.kind = numericBool
		default:
			return numericLayout{}, false
		}
		layout.fields[index] = entry
	}
	return layout, true
}

func (e *valueEncoder) encodeNumericStructs(value reflect.Value, layout numericLayout) {
	base := value.UnsafePointer()
	for index := range value.Len() {
		entry := unsafe.Add(base, uintptr(index)*layout.size)
		bitmapAt := len(e.data)
		e.data = append(e.data, 0)
		for fieldIndex := range layout.count {
			field := layout.fields[fieldIndex]
			pointer := unsafe.Add(entry, field.offset)
			zero := true
			switch field.kind {
			case numericFloatArray:
				for _, bits := range unsafe.Slice((*uint32)(pointer), field.length) {
					if bits&0x7fffffff != 0 {
						zero = false
						break
					}
				}
			case numericUintArray:
				for _, word := range unsafe.Slice((*uint32)(pointer), field.length) {
					if word != 0 {
						zero = false
						break
					}
				}
			case numericBool:
				zero = !*(*bool)(pointer)
			}
			if zero {
				continue
			}
			e.data[bitmapAt] |= 1 << fieldIndex
			switch field.kind {
			case numericFloatArray:
				e.data = append(e.data, unsafe.Slice((*byte)(pointer), field.length*4)...)
			case numericUintArray:
				e.encodeUint32s(unsafe.Slice((*uint32)(pointer), field.length))
			case numericBool:
				e.data = append(e.data, 1)
			}
		}
	}
}

func (d *valueDecoder) decodeNumericStructs(value reflect.Value, layout numericLayout) error {
	base := value.UnsafePointer()
	for index := range value.Len() {
		bitmap, err := d.takeByte()
		if err != nil {
			return atIndex(index, err)
		}
		if bitmap>>layout.count != 0 {
			return atIndex(index, fmt.Errorf("typedbin: invalid field bitmap for %s", value.Type().Elem()))
		}
		entry := unsafe.Add(base, uintptr(index)*layout.size)
		for fieldIndex := range layout.count {
			if bitmap&(1<<fieldIndex) == 0 {
				continue
			}
			field := layout.fields[fieldIndex]
			pointer := unsafe.Add(entry, field.offset)
			var err error
			switch field.kind {
			case numericFloatArray:
				var encoded []byte
				encoded, err = d.take(field.length * 4)
				if err == nil {
					copy(unsafe.Slice((*byte)(pointer), len(encoded)), encoded)
				}
			case numericUintArray:
				err = d.decodeUint32s(unsafe.Slice((*uint32)(pointer), field.length), field.elem)
			case numericBool:
				var encoded byte
				encoded, err = d.takeByte()
				if err == nil && encoded > 1 {
					err = fmt.Errorf("typedbin: invalid bool %d", encoded)
				}
				if err == nil {
					*(*bool)(pointer) = encoded == 1
				}
			}
			if err != nil {
				return atIndex(index, atField(field.name, err))
			}
		}
	}
	return nil
}
