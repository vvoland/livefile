package livefile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"
)

type LiveFile[StateT any] struct {
	path string
	fs   WriteFS

	lastModTime time.Time
	cached      StateT
	mutex       sync.Mutex

	defaultFunc func() StateT
	errHandler  func(context.Context, error)
	onLoaded    func(context.Context, *StateT)
}

// DefaultErrorHandler is the default error handler used for all [LiveFile]
// instances created without an explicit [WithDefault].
var DefaultErrorHandler = func(_ context.Context, err error) {
	panic(err)
}

// DefaultFileSystem is the default filesystem used for file operations in
// all [LiveFile] instances created without an explicit [WithFileSystem] option.
var DefaultFileSystem WriteFS = osFileSystem{}

// BaseDir is the base directory for the relative paths passed to the [New]
// function.
var BaseDir string

// New creates a new [LiveFile] instance with the given path and options.
// The path can be either absolute or relative. If it is relative,
// it will be joined with the [BaseDir].
func New[T any](path string, opts ...Opt[T]) *LiveFile[T] {
	if !filepath.IsAbs(path) && BaseDir != "" {
		path = filepath.Join(BaseDir, path)
	}
	lf := &LiveFile[T]{
		path:       path,
		fs:         DefaultFileSystem,
		errHandler: DefaultErrorHandler,
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

// View retrieves the current state of the file and passes it to the given
// function. The state pointer is only valid within the function call and
// must not be stored.
// The function must not modify the state or call other [LiveFile] methods.
func (lf *LiveFile[T]) View(ctx context.Context, f func(state *T)) {
	lf.mutex.Lock()
	defer lf.mutex.Unlock()

	lf.ensure(ctx)
	f(&lf.cached)
}

// Update calls the given function with a mutable reference to the current file
// state. If the function returns an error, the state is rolled back to the
// previous value.
// The function MUST NOT call other [LiveFile] methods.
func (lf *LiveFile[T]) Update(ctx context.Context, f func(state *T) error) error {
	lf.mutex.Lock()
	defer lf.mutex.Unlock()

	lf.ensure(ctx)

	file, err := lf.fs.OpenFile(lf.path, os.O_RDWR|os.O_CREATE, 0o660)
	if errors.Is(err, fs.ErrNotExist) {
		err = lf.fs.MkdirAll(path.Dir(lf.path), 0o770)
		if err != nil {
			return err
		}
		file, err = lf.fs.OpenFile(lf.path, os.O_RDWR|os.O_CREATE, 0o660)
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

// Peek retrieves the current state of the file and returns its copy.
func (lf *LiveFile[T]) Peek(ctx context.Context) T {
	lf.mutex.Lock()
	lf.ensure(ctx)
	c := lf.cached
	lf.mutex.Unlock()
	return c
}

func (lf *LiveFile[T]) ensure(ctx context.Context) {
	file, err := lf.fs.Open(lf.path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			lf.errHandler(ctx, err)
		}
	} else {
		lf.loadIfUpdated(ctx, file)
		file.Close()
	}
}

func (lf *LiveFile[T]) loadIfUpdated(ctx context.Context, file ReadSeekFile) {
	stat, err := file.Stat()
	if err != nil {
		lf.errHandler(ctx, fmt.Errorf("stat failed: %w", err))
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

func (lf *LiveFile[T]) forceLoad(ctx context.Context, file ReadSeekFile) {
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		lf.errHandler(ctx, fmt.Errorf("failed to rewind file: %w", err))
	}

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&lf.cached)

	// File empty
	if err == io.EOF && decoder.InputOffset() == 0 {
		lf.cached = lf.defaultFunc()
		err = nil
	}
	if err != nil {
		lf.errHandler(ctx, fmt.Errorf("invalid JSON: %w", err))
	} else {
		if lf.onLoaded != nil {
			lf.onLoaded(ctx, &lf.cached)
		}
	}
}
