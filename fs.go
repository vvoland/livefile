package livefile

import (
	"io"
	"io/fs"
	"os"
)

type WriteFS interface {
	Open(name string) (ReadSeekFile, error)
	OpenFile(name string, flag int, perm fs.FileMode) (WriteFile, error)
	MkdirAll(path string, perm fs.FileMode) error
	Remove(name string) error
}

type WriteFile interface {
	ReadSeekFile
	io.Writer
	io.WriterAt
	Truncate(size int64) error
	Sync() error
}

type ReadSeekFile interface {
	fs.File
	io.Seeker
}

// osFileSystem implements WriteFS using the os package
type osFileSystem struct{}

func (osfs osFileSystem) Open(name string) (ReadSeekFile, error) {
	return os.Open(name)
}

func (osfs osFileSystem) OpenFile(name string, flag int, perm fs.FileMode) (WriteFile, error) {
	return os.OpenFile(name, flag, perm)
}

func (osfs osFileSystem) OpenRead(name string) (ReadSeekFile, error) {
	return os.Open(name)
}

func (osfs osFileSystem) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (osfs osFileSystem) Remove(name string) error {
	return os.Remove(name)
}
