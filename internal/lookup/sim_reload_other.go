//go:build !windows

package lookup

import (
	"context"
	"fmt"
)

func SendSimReload(context.Context, uintptr, string, string, []byte, ...BridgePosition) error {
	return fmt.Errorf("reload key delivery requires Windows")
}
