//go:build !darwin && !windows

package enrollmentstore

func openDirectory(string, bool, bool) (directory, error) { return nil, ErrStorage }
