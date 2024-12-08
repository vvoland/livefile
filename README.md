# livefile

## Overview
The `livefile` package provides an easy, read-write access to a live-reloadable struct instance backed by a JSON file with a transaction-like API.

## Features

- **Live Reload**: Automatically reloads the struct instance when the underlying JSON file changes.
- **Transaction-like API**: Provides a safe and consistent way to update the struct instance.

## Installation

To install `livefile`, use the following command:

```sh
go get woland.xyz/livefile
```

## Usage

1. Define your state struct.
2. Create an instance of `livefile`.
3. Use the `View` method to read the current state.
4. Use the `Update` method to modify the state.
5. Use the `Peek` method to get a copy of the current state (a simpler version of the `View` method).

### Example

```go
import (
    "context"
    "fmt"

    "woland.xyz/livefile"
)

type MyState struct {
    Value int
    Name  string
}

func main() {
    ctx := context.Background()
    lf := livefile.New[MyState]("state.json")

    // View the initial state
    lf.View(ctx, func(s *MyState) {
        fmt.Println("Updated State:", *s)
    })

    // Update the state
    err := lf.Update(ctx, func(s *MyState) error {
        s.Value = 42
        s.Name = "Updated"
        return nil
    })
    if err != nil {
        panic(err)
    }

    // View the modified state
    lf.View(ctx, func(s *MyState) {
        fmt.Println("Updated State:", *s)
    })

    // Perform external modification
    if err := os.WriteFile("state.json", []byte(`{"Name":"foo","Value":100}`), 0o777); err != nil {
        panic(err)
    }

    // Get a copy of the state
    s := lf.Peek(ctx)
    fmt.Println("Final state:", s)
}
```

