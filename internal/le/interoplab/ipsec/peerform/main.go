// Design: docs/architecture/testing/interop.md -- a peer-local ESP form stimulus.
// Related: action.go -- portable action contract; socket_linux.go -- Linux boundary.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Stdout)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, err) //nolint:errcheck // The process reports the failure through its nonzero exit.
		os.Exit(1)
	}
}

func run(ctx context.Context, argv []string, output io.Writer) (resultErr error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	request, err := parseCommand(argv)
	if err != nil {
		return err
	}
	peer, err := openPeer(request)
	if err != nil {
		return err
	}
	// A successful openPeer caller MUST close the peer after its last action.
	defer func() { resultErr = errors.Join(resultErr, peer.close()) }()
	answer, err := runAction(ctx, request, peer)
	if err != nil {
		return err
	}
	return writeResult(output, answer)
}

func writeResult(output io.Writer, answer result) error {
	return json.NewEncoder(output).Encode(answer)
}
