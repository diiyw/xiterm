package render

type deviceValue[T any] struct {
	device *Device
	value  T
}

// DeviceValues keeps the common single-device residency inline. Additional
// devices allocate once on first use; warmed lookups and iteration allocate nothing.
type DeviceValues[T any] struct {
	first  deviceValue[T]
	others []deviceValue[T]
}

func (values *DeviceValues[T]) Get(device *Device) (T, bool) {
	if device != nil && device.Driver() != nil {
		if values.first.device == device {
			return values.first.value, true
		}
		for _, entry := range values.others {
			if entry.device == device {
				return entry.value, true
			}
		}
	}
	var zero T
	return zero, false
}

func (values *DeviceValues[T]) Set(device *Device, value T) {
	if values.first.device == device {
		values.first = deviceValue[T]{device, value}
		return
	}
	available := -1
	for index := range values.others {
		entry := &values.others[index]
		if entry.device == device {
			*entry = deviceValue[T]{device, value}
			return
		}
		if available < 0 && entry.device.Driver() == nil {
			available = index
		}
	}
	if values.first.device.Driver() == nil {
		values.first = deviceValue[T]{device, value}
		return
	}
	if available >= 0 {
		values.others[available] = deviceValue[T]{device, value}
		return
	}
	values.others = append(values.others, deviceValue[T]{device, value})
}

func (values *DeviceValues[T]) Each(visit func(*Device, T) error) error {
	if values.first.device.Driver() != nil {
		if err := visit(values.first.device, values.first.value); err != nil {
			return err
		}
	}
	for _, entry := range values.others {
		if entry.device.Driver() != nil {
			if err := visit(entry.device, entry.value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (values *DeviceValues[T]) Clear() {
	values.first = deviceValue[T]{}
	clear(values.others)
	values.others = values.others[:0]
}

// Remove transfers one device's value to the caller, including values whose
// device has already been destroyed. Other device residencies remain intact.
func (values *DeviceValues[T]) Remove(device *Device) (T, bool) {
	if values.first.device == device && device != nil {
		value := values.first.value
		values.first = deviceValue[T]{}
		return value, true
	}
	for index, entry := range values.others {
		if entry.device == device && device != nil {
			values.others[index] = deviceValue[T]{}
			return entry.value, true
		}
	}
	var zero T
	return zero, false
}

func (values *DeviceValues[T]) Release(dispose func(*Device, T) error) error {
	if values.first.device.Driver() != nil {
		if err := dispose(values.first.device, values.first.value); err != nil {
			return err
		}
	}
	values.first = deviceValue[T]{}
	for index := range values.others {
		entry := &values.others[index]
		if entry.device.Driver() != nil {
			if err := dispose(entry.device, entry.value); err != nil {
				return err
			}
		}
		*entry = deviceValue[T]{}
	}
	values.others = values.others[:0]
	return nil
}
