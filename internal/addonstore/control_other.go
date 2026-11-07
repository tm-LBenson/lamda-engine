//go:build !windows

package addonstore

func acquireControl() (func(), error) { return func() {}, nil }
