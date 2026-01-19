// Command test-syslog is a UDP syslog server for testing ZeBGP logging.
//
// Usage:
//
//	test-syslog --port 1514
//	test-syslog --port 1514 --pattern "subsystem=server"
//
// The server listens for UDP syslog messages and prints them to stdout.
// If --pattern is specified, it exits with code 0 when a matching message
// is received, or code 1 on timeout.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/zebgp/pkg/testsyslog"
)

func main() {
	os.Exit(run())
}

func run() int {
	port := flag.Int("port", 0, "port to listen on (0 = dynamic)")
	pattern := flag.String("pattern", "", "regex pattern to match (exits on match)")
	timeout := flag.Duration("timeout", 30*time.Second, "timeout for pattern matching")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	srv := testsyslog.New(*port)
	if err := srv.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start: %v\n", err)
		return 1
	}
	defer func() { _ = srv.Close() }()

	fmt.Printf("listening on UDP port %d\n", srv.Port())

	if *pattern != "" {
		// Pattern matching mode
		fmt.Printf("waiting for pattern: %s\n", *pattern)
		deadline := time.Now().Add(*timeout)
		for time.Now().Before(deadline) {
			select {
			case <-ctx.Done():
				fmt.Println("interrupted")
				return 1
			default:
			}

			if srv.Match(*pattern) {
				fmt.Println("pattern matched")
				for _, msg := range srv.Messages() {
					fmt.Println(msg)
				}
				return 0
			}
			time.Sleep(100 * time.Millisecond)
		}
		fmt.Println("timeout: pattern not matched")
		for _, msg := range srv.Messages() {
			fmt.Println(msg)
		}
		return 1
	}

	// Interactive mode - print messages until interrupted
	fmt.Println("press Ctrl+C to stop")
	lastCount := 0
	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nreceived messages:")
			for _, msg := range srv.Messages() {
				fmt.Println(msg)
			}
			return 0
		default:
		}

		msgs := srv.Messages()
		if len(msgs) > lastCount {
			for i := lastCount; i < len(msgs); i++ {
				fmt.Printf("[%d] %s\n", i+1, msgs[i])
			}
			lastCount = len(msgs)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
