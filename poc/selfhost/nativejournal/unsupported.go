//go:build (!darwin && !windows) || (darwin && !cgo)

package nativejournal

func openNative(string, bool) (keyValues, error) { return nil, ErrUnavailable }
