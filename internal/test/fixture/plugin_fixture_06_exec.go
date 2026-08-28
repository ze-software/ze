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

func fixture06ExaBGPHelper(ctx context.Context, _ []string) error {
	fmt.Println("neighbor 127.0.0.1 announce route 1.1.0.0/24 next-hop 101.1.101.1 origin igp local-preference 100")
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
