package fixture

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"
)

func fixture06Wait(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// fixture06ExaBGPRoute is the route both ExaBGP helpers announce. The two write
// it in the two ExaBGP forms, so the wire UPDATE each produces is the same and
// the two fixtures assert one hex string.
const fixture06ExaBGPRoute = "announce route 1.1.0.0/24 next-hop 101.1.101.1 origin igp local-preference 100"

// fixture06ExaBGPHelper writes the route in the form that names its neighbor.
func fixture06ExaBGPHelper(ctx context.Context, _ []string) error {
	return fixture06ExaBGPWrite(ctx, "neighbor 127.0.0.1 "+fixture06ExaBGPRoute)
}

// fixture06ExaBGPBareHelper writes the route in the bare form, which names no
// neighbor. ExaBGP sends such a line to every neighbor, so the bridge
// translates it with the wildcard peer selector.
func fixture06ExaBGPBareHelper(ctx context.Context, _ []string) error {
	return fixture06ExaBGPWrite(ctx, fixture06ExaBGPRoute)
}

// fixture06ExaBGPWrite writes one ExaBGP line to stdout, then holds the helper
// open until ze closes its stdin or the context ends. A helper that exits at
// once is a process the bridge sees fail.
func fixture06ExaBGPWrite(ctx context.Context, line string) error {
	fmt.Println(line)
	readDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, os.Stdin)
		readDone <- err
	}()
	select {
	case <-ctx.Done():
		return nil
	case err := <-readDone:
		return err
	}
}
