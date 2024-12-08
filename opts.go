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
		s.errHandler = f
	}
}

// WithLoadedCallback sets the function that will be called when the file is
// reloaded from the filesystem.
// The function will be called with the context and a pointer to the new data.
// Any access to the data MUST happen inside the callback and MUST NOT be
// stored outside of it.
func WithLoadedCallback[T any](f func(context.Context, *T)) Opt[T] {
	return func(s *LiveFile[T]) {
		s.onLoaded = f
	}
}
