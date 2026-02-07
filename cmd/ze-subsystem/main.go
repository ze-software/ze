// ze-subsystem is a forked process that handles a subset of commands.
// It communicates with ze engine via the YANG RPC protocol over dual socket pairs
// (ZE_ENGINE_FD=3, ZE_CALLBACK_FD=4).
//
// Usage:
//
//	ze-subsystem --mode=cache
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

const (
	statusDone  = "done"
	statusError = "error"
)

func main() {
	mode := flag.String("mode", "", "Subsystem mode: cache|route|session")
	flag.Parse()

	if *mode == "" {
		fmt.Fprintln(os.Stderr, "ze-subsystem: --mode is required")
		os.Exit(1)
	}

	p, err := sdk.NewFromEnv("subsystem-" + *mode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ze-subsystem: %v\n", err)
		os.Exit(1)
	}

	var reg sdk.Registration

	switch *mode {
	case "cache":
		reg = cacheRegistration()
		p.OnExecuteCommand(handleCacheCommand)
	case "route":
		reg = routeRegistration()
		p.OnExecuteCommand(handleRouteCommand)
	case "session":
		reg = sessionRegistration()
		p.OnExecuteCommand(handleSessionCommand)
	default:
		fmt.Fprintf(os.Stderr, "ze-subsystem: unknown mode: %s\n", *mode)
		os.Exit(1)
	}

	ctx := context.Background()
	if err := p.Run(ctx, reg); err != nil {
		fmt.Fprintf(os.Stderr, "ze-subsystem: %v\n", err)
		os.Exit(1)
	}
}

func cacheRegistration() sdk.Registration {
	return sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "bgp cache list", Description: "List cache entries"},
			{Name: "bgp cache retain", Description: "Retain cache entry"},
			{Name: "bgp cache release", Description: "Release cache entry"},
			{Name: "bgp cache expire", Description: "Expire cache entry"},
			{Name: "bgp cache forward", Description: "Forward cache entry"},
		},
	}
}

func routeRegistration() sdk.Registration {
	return sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "bgp route announce", Description: "Announce route"},
			{Name: "bgp route withdraw", Description: "Withdraw route"},
		},
	}
}

func sessionRegistration() sdk.Registration {
	return sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "bgp session ping", Description: "Ping session"},
			{Name: "bgp session bye", Description: "Session goodbye"},
			{Name: "bgp session ready", Description: "Session ready"},
		},
	}
}

func handleCacheCommand(_, command string, _ []string, _ string) (string, string, error) {
	switch command {
	case "bgp cache list":
		return statusDone, "[]", nil
	case "bgp cache retain", "bgp cache release", "bgp cache expire", "bgp cache forward":
		return statusDone, "", nil
	default:
		return statusError, fmt.Sprintf("unknown command: %s", command), nil
	}
}

func handleRouteCommand(_, command string, _ []string, _ string) (string, string, error) {
	switch command {
	case "bgp route announce", "bgp route withdraw":
		return statusDone, "", nil
	default:
		return statusError, fmt.Sprintf("unknown command: %s", command), nil
	}
}

func handleSessionCommand(_, command string, _ []string, _ string) (string, string, error) {
	switch command {
	case "bgp session ping":
		data, _ := json.Marshal(map[string]any{"pong": os.Getpid()})
		return statusDone, string(data), nil
	case "bgp session bye":
		data, _ := json.Marshal(map[string]any{"status": "goodbye"})
		return statusDone, string(data), nil
	case "bgp session ready":
		data, _ := json.Marshal(map[string]any{"api": "ready acknowledged"})
		return statusDone, string(data), nil
	default:
		return statusError, fmt.Sprintf("unknown command: %s", command), nil
	}
}
