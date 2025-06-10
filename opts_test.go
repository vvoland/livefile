package livefile

import (
	"context"
	"testing"

	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
)

func TestNewAPIWithTempFiles(t *testing.T) {
	ctx := context.TODO()

	t.Run("Default OS filesystem", func(t *testing.T) {
		// Test that New() uses OSFileSystem by default
		path := testFilePath(t)
		f := New(path, WithDefault(func() TestData {
			return TestData{Value: 123, Name: "default-os"}
		}))

		f.View(ctx, func(state *TestData) {
			assert.Check(t, cmp.Equal(state.Value, 123))
			assert.Check(t, cmp.Equal(state.Name, "default-os"))
		})

		err := f.Update(ctx, func(data *TestData) error {
			data.Value = 456
			return nil
		})
		assert.NilError(t, err)

		data := f.Peek(ctx)
		assert.Check(t, cmp.Equal(data.Value, 456))
	})

	t.Run("Custom filesystem via temporary files", func(t *testing.T) {
		path := testFilePath(t)
		f := New(path,
			WithDefault(func() TestData {
				return TestData{Value: 789, Name: "temp-file"}
			}))

		f.View(ctx, func(state *TestData) {
			assert.Check(t, cmp.Equal(state.Value, 789))
			assert.Check(t, cmp.Equal(state.Name, "temp-file"))
		})

		err := f.Update(ctx, func(data *TestData) error {
			data.Name = "updated-temp"
			return nil
		})
		assert.NilError(t, err)

		data := f.Peek(ctx)
		assert.Check(t, cmp.Equal(data.Value, 789))
		assert.Check(t, cmp.Equal(data.Name, "updated-temp"))
	})

	t.Run("Multiple temporary files work independently", func(t *testing.T) {
		path1 := testFilePath(t)
		path2 := testFilePath(t)

		// Test with different file paths
		f1 := New(path1,
			WithDefault(func() TestData {
				return TestData{Value: 111, Name: "temp1"}
			}))

		// Test with different file path
		f2 := New(path2,
			WithDefault(func() TestData {
				return TestData{Value: 222, Name: "temp2"}
			}))

		ctx := context.Background()

		// Both should work independently
		data1 := f1.Peek(ctx)
		data2 := f2.Peek(ctx)

		assert.Check(t, cmp.Equal(data1.Value, 111))
		assert.Check(t, cmp.Equal(data1.Name, "temp1"))
		assert.Check(t, cmp.Equal(data2.Value, 222))
		assert.Check(t, cmp.Equal(data2.Name, "temp2"))
	})
}
