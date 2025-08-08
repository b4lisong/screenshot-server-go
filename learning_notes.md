## `go mod init screenshot-server-go`
used to initialize a new Go module for your project.

A Go module is a collection of Go packages (your project code and dependencies)
that are versioned together as a unit. Modules are the standard
way to manage dependencies.

`go.mod` is created; this keeps track of:
- Project's (module) name (screenshot-server-go)
- Go version
- Dependencies imported

## Error handling
```
func Capture() (image.Image, error) {
	numDisplays := screenshot.NumActiveDisplays()
	if numDisplays == 0 {
		return nil, fmt.Errorf("no active displays found")
	}
```
- Count the number of active displays
- If this number is 0, then return a `nil` Image along with a
human-readable error message.
- Go encourages explicit error returns, rather than hidden logic
or exceptions.
- This makes the caller handle the issue (e.g. log it, or send an
HTTP 500).

```
img, err := screenshot.CaptureRect(bounds)
if err != nil {
    return nil, fmt.Errorf("failed to capture screen: %w", err)
}
```

Standard Go convention for calling a function/operation that might fail:
```
value, err := SomeFunction()
```

If the function succeeds, `err` will be `nil` and `img` will contain
the screenshot.

This is also a standard multiple assignment in Go; `img` will receive
the image result: an `image.Image` object (captured screenshot).

`err` will recieve any **error** returned by the function; e.g., if 
the system can't access the screen, or we're running headless, etc.

The `%w` feature wraps the original error inside a new error, with more context.

This allows for **error unwrapping** and structured error inspection
later on using `errors.Is()` or `errors.As()`.

This is idiomatic because it adds context (`"failed to capture screen"`)
without losing the original cause (permissions, unexpected hardware issues, etc.).

It also avoids over-handling; we don't try to guess what went wrong
inside the helper, we just pass it back.
