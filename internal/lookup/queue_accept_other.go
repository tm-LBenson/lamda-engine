//go:build !windows

package lookup

import "context"

func RunQueueAccept(context.Context, string, string, func(string, ...any)) {}
