package statefile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
)

func testFilePath(t *testing.T) string {
	dir := t.TempDir()
	assert.NilError(t, os.MkdirAll(dir, 0o700))
	return filepath.Join(dir, "testfile.json")
}

type TestData struct {
	Value int
	Name  string
}

func TestSimple(t *testing.T) {
	path := testFilePath(t)
	ctx := context.Background()
	f := New(path, func() TestData {
		return TestData{Value: 42, Name: "test"}
	})

	t.Run("View", func(t *testing.T) {
		f.View(ctx, func(state *TestData) {
			assert.Check(t, cmp.Equal(state.Value, 42))
			assert.Check(t, cmp.Equal(state.Name, "test"))
		})
	})

	t.Run("Peek", func(t *testing.T) {
		data := f.Peek(ctx)
		assert.Check(t, cmp.Equal(data.Value, 42))
		assert.Check(t, cmp.Equal(data.Name, "test"))
	})

	t.Run("Update", func(t *testing.T) {
		err := f.Update(ctx, func(data *TestData) error {
			data.Value = 100
			data.Name = "updated"
			return nil
		})
		assert.NilError(t, err)

		data := f.Peek(ctx)
		assert.Check(t, cmp.Equal(data.Value, 100))
		assert.Check(t, cmp.Equal(data.Name, "updated"))
	})
}

func TestFileIsntCreatedBeforeFirstUpdate(t *testing.T) {
	path := testFilePath(t)
	ctx := context.Background()
	f := New(path, func() TestData {
		return TestData{Value: 42, Name: "test"}
	})

	_, err := os.Stat(path)
	assert.Check(t, errors.Is(err, os.ErrNotExist))

	data := f.Peek(ctx)
	assert.Check(t, cmp.Equal(data.Value, 42))
	assert.Check(t, cmp.Equal(data.Name, "test"))

	_, err = os.Stat(path)
	assert.Check(t, errors.Is(err, os.ErrNotExist))

	err = f.Update(ctx, func(data *TestData) error {
		data.Value = 100
		return nil
	})
	assert.NilError(t, err)

	_, err = os.Stat(path)
	assert.NilError(t, err)
}

func TestUpdateErrorWillRollbackChanges(t *testing.T) {
	path := testFilePath(t)
	ctx := context.Background()
	f := New(path, func() TestData {
		return TestData{Value: 42, Name: "test"}
	})

	err := f.Update(ctx, func(data *TestData) error {
		data.Name = "updated"
		return errors.New("something failed")
	})

	assert.Check(t, err != nil)
	data := f.Peek(ctx)
	// Name should not be updated
	assert.Check(t, cmp.Equal(data.Name, "test"))

	t.Run("existing", func(t *testing.T) {
		// Update to force file creation
		err = f.Update(ctx, func(data *TestData) error {
			data.Name = "asdf"
			return nil
		})
		assert.NilError(t, err)

		err := f.Update(ctx, func(data *TestData) error {
			data.Name = "failure"
			return errors.New("something failed")
		})

		assert.Check(t, err != nil)
		data = f.Peek(ctx)
		// Name should not be updated
		assert.Check(t, cmp.Equal(data.Name, "asdf"))
	})

}

func TestFileExists(t *testing.T) {
	path := testFilePath(t)
	ctx := context.Background()

	assert.NilError(t, os.WriteFile(path, []byte(`{"Value": 1337, "Name": "foobar"}`), 0o600))
	f := New(path, func() TestData {
		return TestData{Value: 42, Name: "test"}
	})

	data := f.Peek(ctx)
	assert.Check(t, cmp.Equal(data.Value, 1337))
	assert.Check(t, cmp.Equal(data.Name, "foobar"))
}

func TestFileExternalChange(t *testing.T) {
	path := testFilePath(t)
	ctx := context.Background()

	f := New(path, func() TestData {
		return TestData{Value: 42, Name: "test"}
	})

	data := f.Peek(ctx)
	assert.Check(t, cmp.Equal(data.Value, 42))
	assert.Check(t, cmp.Equal(data.Name, "test"))

	assert.NilError(t, os.WriteFile(path, []byte(`{"Value": 1337, "Name": "foobar"}`), 0o600))

	data = f.Peek(ctx)
	assert.Check(t, cmp.Equal(data.Value, 1337))
	assert.Check(t, cmp.Equal(data.Name, "foobar"))
}

func TestFileExternalChangeDuringView(t *testing.T) {
	path := testFilePath(t)
	ctx := context.Background()

	f := New(path, func() TestData {
		return TestData{Value: 42, Name: "test"}
	})

	doWrite := make(chan struct{})
	doRead := make(chan struct{})
	go func() {
		<-doWrite
		assert.NilError(t, os.WriteFile(path, []byte(`{"Value": 1337, "Name": "foobar"}`), 0o600))
		close(doRead)
	}()

	f.View(ctx, func(state *TestData) {
		close(doWrite)
		<-doRead
		assert.Check(t, cmp.Equal(state.Value, 42))
		assert.Check(t, cmp.Equal(state.Name, "test"))
	})
}

func TestFileExternalChangeDuringUpdate(t *testing.T) {
	path := testFilePath(t)
	ctx := context.Background()

	f := New(path, func() TestData {
		return TestData{Value: 42, Name: "test"}
	})

	doWrite := make(chan struct{})
	doRead := make(chan struct{})
	go func() {
		<-doWrite
		assert.NilError(t, os.WriteFile(path, []byte(`{"Value": 1337, "Name": "foobar"}`), 0o600))
		close(doRead)
	}()

	err := f.Update(ctx, func(state *TestData) error {
		close(doWrite)
		state.Value = 100
		<-doRead
		return nil
	})

	// External change will be overriden
	assert.NilError(t, err)
	data := f.Peek(ctx)
	assert.Check(t, cmp.Equal(data.Value, 100))
}
