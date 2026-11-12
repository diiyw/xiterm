package window

func chainCallback[T any](previous, next func(T)) func(T) {
	if next == nil {
		return previous
	}
	if previous == nil {
		return next
	}
	return func(value T) {
		previous(value)
		next(value)
	}
}

func chainCallback2[A, B any](previous, next func(A, B)) func(A, B) {
	if next == nil {
		return previous
	}
	if previous == nil {
		return next
	}
	return func(first A, second B) {
		previous(first, second)
		next(first, second)
	}
}

func chainCallback3[A, B, C any](previous, next func(A, B, C)) func(A, B, C) {
	if next == nil {
		return previous
	}
	if previous == nil {
		return next
	}
	return func(first A, second B, third C) {
		previous(first, second, third)
		next(first, second, third)
	}
}
