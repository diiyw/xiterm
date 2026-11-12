package ecs

import "cmp"

// minHeap keeps the next ready system available without rescanning the queue.
// Its backing slice is reserved by the caller and reused for every push/pop.
type minHeap[T cmp.Ordered] []T

func (heap minHeap[T]) init() {
	for index := len(heap)/2 - 1; index >= 0; index-- {
		heap.siftDown(index)
	}
}

func (heap *minHeap[T]) push(value T) {
	*heap = append(*heap, value)
	index := len(*heap) - 1
	for index > 0 {
		parent := (index - 1) / 2
		if !cmp.Less(value, (*heap)[parent]) {
			break
		}
		(*heap)[index] = (*heap)[parent]
		index = parent
	}
	(*heap)[index] = value
}

func (heap *minHeap[T]) pop() T {
	value := (*heap)[0]
	last := len(*heap) - 1
	(*heap)[0] = (*heap)[last]
	var zero T
	(*heap)[last] = zero
	*heap = (*heap)[:last]
	if last > 0 {
		heap.siftDown(0)
	}
	return value
}

func (heap minHeap[T]) siftDown(index int) {
	value := heap[index]
	for child := index*2 + 1; child < len(heap); child = index*2 + 1 {
		if child+1 < len(heap) && cmp.Less(heap[child+1], heap[child]) {
			child++
		}
		if !cmp.Less(heap[child], value) {
			break
		}
		heap[index] = heap[child]
		index = child
	}
	heap[index] = value
}
