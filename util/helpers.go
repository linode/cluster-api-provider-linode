package util

// Pointer returns the pointer of any type
func Pointer[T any](t T) *T {
	return &t
}
