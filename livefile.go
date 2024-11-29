package livefile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"
)

type LiveFile[StateT any] struct {
	path string

	lastModTime time.Time
	cached      StateT
	mutex       sync.Mutex

	defaultFunc func() StateT
	error       func(context.Context, error)
}

// The default error handler used for all LiveFile instances created without an
// explicit [WithDefault] option.
var DefaultErrorHandler = func(_ context.Context, err error) {
	panic(err)
}

// BaseDir is the base directory for the relative paths passed to the [New]
// function.
var BaseDir string

func New[T any](path string, opts ...Opt[T]) *LiveFile[T] {
	if !filepath.IsAbs(path) && BaseDir != "" {
		path = filepath.Join(BaseDir, path)
	}
	lf := &LiveFile[T]{
		path:  path,
		error: DefaultErrorHandler,
	}

	for _, opt := range opts {
		opt(lf)
	}
	if lf.defaultFunc == nil {
		lf.defaultFunc = func() T {
			var zero T
			return zero
		}
	}
	lf.cached = lf.defaultFunc()
	return lf
}

func (lf *LiveFile[T]) Peek(ctx context.Context) T {
	lf.mutex.Lock()
	lf.ensure(ctx)
	c := lf.cached
	lf.mutex.Unlock()
	return c
}

func (lf *LiveFile[T]) ensure(ctx context.Context) {
	file, err := os.Open(lf.path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			lf.error(ctx, err)
		}
	} else {
		lf.loadIfUpdated(ctx, file)
		file.Close()
	}
}

func (lf *LiveFile[T]) View(ctx context.Context, f func(state *T)) {
	lf.mutex.Lock()
	defer lf.mutex.Unlock()

	lf.ensure(ctx)
	f(&lf.cached)
}

func (lf *LiveFile[T]) loadIfUpdated(ctx context.Context, file *os.File) {
	stat, err := file.Stat()
	if err != nil {
		lf.error(ctx, fmt.Errorf("stat failed: %w", err))
	}

	if stat.Size() == 0 {
		return
	}

	modTime := stat.ModTime()
	if modTime.After(lf.lastModTime) {
		lf.forceLoad(ctx, file)
		lf.lastModTime = modTime
	}
}

func (lf *LiveFile[T]) forceLoad(ctx context.Context, file *os.File) {
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		lf.error(ctx, fmt.Errorf("failed to rewind file: %w", err))
	}

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&lf.cached)

	// File empty
	if err == io.EOF && decoder.InputOffset() == 0 {
		lf.cached = lf.defaultFunc()
		err = nil
	}
	if err != nil {
		lf.error(ctx, fmt.Errorf("invalid JSON: %w", err))
	}
}

func (lf *LiveFile[T]) Update(ctx context.Context, f func(state *T) error) error {
	lf.mutex.Lock()
	defer lf.mutex.Unlock()

	lf.ensure(ctx)

	file, err := os.OpenFile(lf.path, os.O_RDWR|os.O_CREATE, 0o660)
	if errors.Is(err, os.ErrNotExist) {
		err = os.MkdirAll(path.Dir(lf.path), 0o770)
		if err != nil {
			return err
		}
		file, err = os.OpenFile(lf.path, os.O_RDWR|os.O_CREATE, 0o660)
	}
	if err != nil {
		return err
	}
	defer file.Close()

	lf.loadIfUpdated(ctx, file)
	err = f(&lf.cached)
	if err != nil {
		// Update failed, rollback changes.
		lf.forceLoad(ctx, file)
		return err
	}

	err = file.Truncate(0)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")

	err = enc.Encode(lf.cached)
	if err != nil {
		return err
	}

	err = file.Sync()
	if err != nil {
		return err
	}

	stat, err := file.Stat()
	if err == nil {
		lf.lastModTime = stat.ModTime()
	}
	return err
}
