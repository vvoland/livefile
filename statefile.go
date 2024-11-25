package statefile

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

var BaseDir string

type StateFile[StateT any] struct {
	path string

	lastModTime time.Time
	cached      StateT
	mutex       sync.Mutex

	defaultFunc func() StateT
}

func New[T any](path string, def func() T) *StateFile[T] {
	if !filepath.IsAbs(path) && BaseDir != "" {
		path = filepath.Join(BaseDir, path)
	}
	return &StateFile[T]{
		path:        path,
		defaultFunc: def,
		cached:      def(),
	}
}

func (ps *StateFile[T]) Peek(ctx context.Context) T {
	ps.mutex.Lock()
	ps.ensure(ctx)
	c := ps.cached
	ps.mutex.Unlock()
	return c
}

func (ps *StateFile[T]) ensure(ctx context.Context) {
	file, err := os.Open(ps.path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			panic(err)
		}
	} else {
		ps.loadIfUpdated(ctx, file)
		file.Close()
	}
}

func (ps *StateFile[T]) View(ctx context.Context, f func(state *T)) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	ps.ensure(ctx)
	f(&ps.cached)
}

func (ps *StateFile[T]) loadIfUpdated(ctx context.Context, file *os.File) {
	stat, err := file.Stat()
	if err != nil {
		panic(err)
	}

	if stat.Size() == 0 {
		return
	}

	modTime := stat.ModTime()
	if modTime.After(ps.lastModTime) {
		ps.forceLoad(ctx, file)
		ps.lastModTime = modTime
	}
}

func (ps *StateFile[T]) forceLoad(ctx context.Context, file *os.File) {
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		panic(err)
	}

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&ps.cached)

	// File empty
	if err == io.EOF && decoder.InputOffset() == 0 {
		ps.cached = ps.defaultFunc()
		err = nil
	}
	log.Ctx(ctx).Debug().
		Err(err).
		Any("data", ps.cached).
		Str("path", ps.path).
		Time("lastMod", ps.lastModTime).
		Time("timestamp", time.Now()).
		Msg("loaded")

	if err != nil {
		panic(err)
	}
}

func (ps *StateFile[T]) Update(ctx context.Context, f func(state *T) error) error {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	ps.ensure(ctx)

	file, err := os.OpenFile(ps.path, os.O_RDWR|os.O_CREATE, 0o660)
	if errors.Is(err, os.ErrNotExist) {
		err = os.MkdirAll(path.Dir(ps.path), 0o770)
		if err != nil {
			return err
		}
		file, err = os.OpenFile(ps.path, os.O_RDWR|os.O_CREATE, 0o660)
	}
	if err != nil {
		return err
	}
	defer file.Close()

	ps.loadIfUpdated(ctx, file)
	err = f(&ps.cached)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("update failed, rolling back")
		ps.forceLoad(ctx, file)
		return err
	}

	err = file.Truncate(0)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")

	err = enc.Encode(ps.cached)
	if err != nil {
		return err
	}

	err = file.Sync()
	if err != nil {
		return err
	}

	stat, err := file.Stat()
	if err == nil {
		ps.lastModTime = stat.ModTime()
	}
	return err
}
