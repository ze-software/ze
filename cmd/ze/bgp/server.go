package bgp

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"codeberg.org/thomas-mangin/ze/internal/config"
)

// pluginFlags collects multiple --plugin flag values.
type pluginFlags []string

func (p *pluginFlags) String() string {
	return strings.Join(*p, ",")
}

func (p *pluginFlags) Set(value string) error {
	*p = append(*p, value)
	return nil
}

func cmdServer(args []string) int {
	fs := flag.NewFlagSet("server", flag.ExitOnError)
	dryRun := fs.Bool("n", false, "dry-run (validate only, don't start)")
	var plugins pluginFlags
	fs.Var(&plugins, "plugin", "plugin to load (repeatable: ze.rib, ./path, \"cmd args\", auto)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze bgp [server] [options] <config-file>

Start ze BGP daemon.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Plugin formats:
  ze.rib              Internal plugin (built-in)
  ./myplugin          Fork local binary
  /path/to/plugin     Fork absolute path
  "ze bgp plugin rr"  Fork command with args
  auto                Auto-discover all plugins

Signals:
  SIGTERM, SIGINT   Graceful shutdown
  SIGHUP            Reload configuration
  SIGUSR1           Dump status

Examples:
  ze bgp config.conf
  ze bgp server config.conf
  ze bgp server -n config.conf                    # validate only
  ze bgp server --plugin ze.rib config.conf       # with RIB plugin
  ze bgp server --plugin ze.rib --plugin ze.gr config.conf
`)
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing config file\n")
		fs.Usage()
		return 1
	}

	configPath := fs.Arg(0)

	// Load config and create reactor (with CLI plugins if specified)
	reactor, err := config.LoadReactorFileWithPlugins(configPath, plugins)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if *dryRun {
		fmt.Printf("✓ %s: configuration valid\n", configPath)
		return 0
	}

	// Start reactor
	fmt.Printf("Starting ze BGP with config: %s\n", configPath)

	if err := reactor.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error starting reactor: %v\n", err)
		return 1
	}

	// Wait for signals or reactor self-stop (e.g., tcp.attempts reached)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	fmt.Println("Ze BGP running. Press Ctrl+C to stop.")

	// Wait for either signal or reactor to stop itself
	doneChan := make(chan struct{})
	go func() {
		_ = reactor.Wait(context.Background())
		close(doneChan)
	}()

	select {
	case <-sigChan:
		fmt.Println("\nShutting down...")
		reactor.Stop()
	case <-doneChan:
		fmt.Println("\nShutting down...")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*1e9)
	defer cancel()

	if err := reactor.Wait(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "warning: shutdown timeout: %v\n", err)
	}

	fmt.Println("Ze BGP stopped.")
	return 0
}
