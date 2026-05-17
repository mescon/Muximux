//go:build windows

package discovery

import (
	"context"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

// dialNpipe opens a Windows named-pipe connection to the Docker
// engine. go-winio is the canonical client used by the Docker CLI
// itself, so the behaviour matches what operators get from
// `docker info` on Windows.
//
// The pipe path uses Windows form (`\\.\pipe\docker_engine`) -
// parseEndpoint() translates the `npipe:////./pipe/...` URI to that
// shape before this function runs.
func dialNpipe(ctx context.Context, pipe string) (net.Conn, error) {
	timeout := 10 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		if d := time.Until(deadline); d < timeout {
			timeout = d
		}
	}
	return winio.DialPipe(pipe, &timeout)
}
