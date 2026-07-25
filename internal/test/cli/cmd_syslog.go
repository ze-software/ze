// Design: docs/architecture/testing/ci-format.md — syslog test server

package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/test/syslog"
)

func cmdSyslog(args []string) int {
	fs := flag.NewFlagSet("syslog", flag.ExitOnError)
	port := fs.Int("port", 0, "port to listen on (0 = dynamic)")
	pattern := fs.String("pattern", "", "regex pattern to match (exits on match)")
	timeout := fs.Duration("timeout", 30*time.Second, "timeout for pattern matching")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	srv := syslog.New(*port)
	if err := srv.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start: %v\n", err)
		return 1
	}
	defer func() { _ = srv.Close() }()

	fmt.Fprintf(os.Stdout, "listening on UDP port %d\n", srv.Port()) //nolint:errcheck // output

	if *pattern != "" {
		fmt.Fprintf(os.Stdout, "waiting for pattern: %s\n", *pattern) //nolint:errcheck // output
		deadline := time.Now().Add(*timeout)
		for time.Now().Before(deadline) {
			select {
			case <-ctx.Done():
				fmt.Fprintln(os.Stdout, "interrupted") //nolint:errcheck // terminal output
				return 1
			default:
			}

			if srv.Match(*pattern) {
				fmt.Fprintln(os.Stdout, "pattern matched") //nolint:errcheck // terminal output
				for _, msg := range srv.Messages() {
					fmt.Fprintln(os.Stdout, msg) //nolint:errcheck // report output
				}
				return 0
			}
			time.Sleep(100 * time.Millisecond)
		}
		fmt.Fprintln(os.Stdout, "timeout: pattern not matched") //nolint:errcheck // report output
		for _, msg := range srv.Messages() {
			fmt.Fprintln(os.Stdout, msg) //nolint:errcheck // report output
		}
		return 1
	}

	fmt.Fprintln(os.Stdout, "press Ctrl+C to stop") //nolint:errcheck // report output
	lastCount := 0
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(os.Stdout, "\nreceived messages:") //nolint:errcheck // report output
			for _, msg := range srv.Messages() {
				fmt.Fprintln(os.Stdout, msg) //nolint:errcheck // report output
			}
			return 0
		default:
		}

		msgs := srv.Messages()
		if len(msgs) > lastCount {
			for i := lastCount; i < len(msgs); i++ {
				fmt.Fprintf(os.Stdout, "[%d] %s\n", i+1, msgs[i]) //nolint:errcheck // output
			}
			lastCount = len(msgs)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
