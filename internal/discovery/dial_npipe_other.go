//go:build !windows

package discovery

import (
	"context"
	"fmt"
	"net"
)

// dialNpipe on non-Windows builds returns a clear error so the
// Discovery test surfaces "npipe transport requires Windows"
// instead of a confusing low-level connect failure. Linux and
// macOS operators wanting to talk to a remote Windows Docker
// daemon should use the tcp:// + TLS path instead.
func dialNpipe(_ context.Context, pipe string) (net.Conn, error) {
	return nil, fmt.Errorf("npipe transport is Windows-only; configure endpoint as tcp://host:2376 to reach a remote Windows daemon (got pipe %q)", pipe)
}
