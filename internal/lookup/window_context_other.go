//go:build !windows

package lookup

type WindowTracker struct{}

func (*WindowTracker) Context(string, string) (WindowContext, bool) { return WindowContext{}, false }
