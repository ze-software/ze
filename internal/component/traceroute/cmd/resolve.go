// Design: docs/architecture/resolve.md -- resolve traceroute command handler
// Related: traceroute.go -- router-side Go traceroute; this is the OS-tool resolve variant
//
// resolve.go implements `resolve traceroute` (ze-resolve:traceroute): run the OS
// traceroute tool from the router with an optional source binding, returning the
// captured output to the daemon caller. It is the daemon-RPC sibling of the
// router-side Go traceroute in traceroute.go; this one shells out to the system
// traceroute with validated argv (no shell).

package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

var errResolveTargetEmpty = errors.New("target must not be empty")

// handleResolveTraceroute runs `traceroute` against a target with an optional
// source binding, returning the tool's combined output.
func handleResolveTraceroute(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	target, errResp := requireResolveArg(args, "target")
	if errResp != nil {
		return errResp, nil
	}
	if err := validateResolveTarget(target); err != nil {
		return errResolveResponse(err.Error()), nil
	}

	cmdArgs := []string{"-n", "-w", "2", "-q", "1"}

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "source":
			if i+1 >= len(args) {
				return errResolveResponse("traceroute: \"source\" requires a value"), nil
			}
			i++
			if err := validateSourceIP(args[i]); err != nil {
				return errResolveResponse(err.Error()), nil
			}
			cmdArgs = append(cmdArgs, "-s", args[i])
		default:
			return errResolveResponse("traceroute: unknown option " + strconv.Quote(args[i])), nil
		}
	}
	cmdArgs = append(cmdArgs, target)

	reqCtx, cancel := context.WithTimeout(ctx.Context(), 30*time.Second)
	defer cancel()

	out, err := captureOSCommand(reqCtx, "traceroute", cmdArgs...)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data: plugin.Map{
				"target": target,
				"output": string(out),
				"error":  err.Error(),
			},
		}, nil
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"target": target,
			"output": string(out),
		},
	}, nil
}

// requireResolveArg returns args[0] or a usage error response.
func requireResolveArg(args []string, name string) (string, *plugin.Response) {
	if len(args) == 0 {
		return "", &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: resolve ... <" + name + ">",
		}
	}
	return args[0], nil
}

func errResolveResponse(msg string) *plugin.Response {
	return &plugin.Response{Status: plugin.StatusError, Error: msg}
}

// validateResolveTarget accepts an IP literal or a hostname of letters, digits,
// dot, and hyphen up to the RFC 1035 length ceiling. It rejects shell
// meta-characters by allowing only that character set.
func validateResolveTarget(s string) error {
	if s == "" {
		return errResolveTargetEmpty
	}
	if net.ParseIP(s) != nil {
		return nil
	}
	if len(s) > 253 {
		return fmt.Errorf("target %q: exceeds 253-character hostname limit", s)
	}
	for _, c := range s {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '.' {
			return fmt.Errorf("target %q: invalid character %q", s, string(c))
		}
	}
	return nil
}

func validateSourceIP(s string) error {
	if net.ParseIP(s) == nil {
		return fmt.Errorf("source %q: not a valid IP address", s)
	}
	return nil
}

func captureOSCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput() //nolint:gosec // fixed-allowlist name, no shell
}
