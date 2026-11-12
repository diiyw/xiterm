package typedbin

import (
	"fmt"
	"reflect"
)

// Model indices and skin joints retain the ONIXBIN1 varint format, but avoid
// reflect.Value operations for each scalar.
func (e *valueEncoder) encodeUint32s(values []uint32) {
	for _, value := range values {
		e.appendUvarint(uint64(value))
	}
}

func (d *valueDecoder) decodeUint32s(values []uint32, typeOf reflect.Type) error {
	for index := range values {
		value, err := d.takeUvarint()
		if err != nil {
			return atIndex(index, err)
		}
		if value > uint64(^uint32(0)) {
			return atIndex(index, fmt.Errorf("typedbin: %d overflows %s", value, typeOf))
		}
		values[index] = uint32(value)
	}
	return nil
}
