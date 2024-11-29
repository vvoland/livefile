package livefile

import "context"

type Opt[T any] func(s *LiveFile[T])

// WithDefault sets the function that will be called to get the default value of
// the file data.
// If not set, the default value will be the zero value of the type.
func WithDefault[T any](f func() T) Opt[T] {
	return func(s *LiveFile[T]) {
		s.defaultFunc = f
	}
}

// WithErrorHandler sets the function that will be called when an error occurs.
// If not set, the default error handler will panic.
func WithErrorHandler[T any](f func(context.Context, error)) Opt[T] {
	return func(s *LiveFile[T]) {
		s.error = f
	}
}
