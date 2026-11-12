// Package smallsort provides compact in-place sorting for small collections.
package smallsort

const insertionLimit = 32

// InsertionSortFunc sorts values in place according to compare.
//
// It is intended for short, frequently sorted collections. The implementation
// does not allocate and avoids linking slices' type-specialized pdqsort body.
func InsertionSortFunc[T any](values []T, compare func(T, T) int) {
	for index := 1; index < len(values); index++ {
		value := values[index]
		position := index
		for position > 0 && compare(value, values[position-1]) < 0 {
			values[position] = values[position-1]
			position--
		}
		values[position] = value
	}
}

// SortFunc uses insertion sort for short slices and an in-place heap sort for
// larger ones. It does not allocate or link slices' type-specialized pdqsort.
func SortFunc[T any](values []T, compare func(T, T) int) {
	for index := 1; index < len(values); index++ {
		if compare(values[index-1], values[index]) > 0 {
			break
		}
		if index == len(values)-1 {
			return
		}
	}
	if len(values) <= insertionLimit {
		InsertionSortFunc(values, compare)
		return
	}
	for root := len(values)/2 - 1; root >= 0; root-- {
		siftDown(values, root, len(values), compare)
	}
	for end := len(values) - 1; end > 0; end-- {
		values[0], values[end] = values[end], values[0]
		siftDown(values, 0, end, compare)
	}
}

func siftDown[T any](values []T, root, end int, compare func(T, T) int) {
	for child := root*2 + 1; child < end; child = root*2 + 1 {
		if child+1 < end && compare(values[child], values[child+1]) < 0 {
			child++
		}
		if compare(values[root], values[child]) >= 0 {
			return
		}
		values[root], values[child] = values[child], values[root]
		root = child
	}
}
