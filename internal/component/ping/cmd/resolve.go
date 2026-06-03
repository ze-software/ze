// Design: docs/architecture/resolve.md -- resolve ping command handler
// Related: offline.go -- offline OS ping; this is the resolve-verb OS ping variant
//
// resolve.go implements `resolve ping` (ze-resolve:ping): run the OS ping tool
// from the router with an optional source binding, returning the captured
// output to the daemon caller. It is the daemon-RPC sibling of the offline
// `ze ping` root in offline.go; both shell out to the OS ping tool with
// validated argv (no shell).

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

// handleResolvePing runs `ping` against a target with optional source binding,
// count, and packet size, returning the tool's combined output.
func handleResolvePing(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	target, errResp := requireResolveArg(args, "target")
	if errResp != nil {
		return errResp, nil
	}
	if err := validateResolveTarget(target); err != nil {
		return errResolveResponse(err.Error()), nil
	}

	cmdArgs := []string{"-c", "4", "-W", "2"}

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "source":
			if i+1 >= len(args) {
				return errResolveResponse("ping: \"source\" requires a value"), nil
			}
			i++
			if err := validateSourceIP(args[i]); err != nil {
				return errResolveResponse(err.Error()), nil
			}
			cmdArgs = append(cmdArgs, "-I", args[i])
		case argCount:
			if i+1 >= len(args) {
				return errResolveResponse("ping: \"count\" requires a value"), nil
			}
			i++
			if err := validateUint(args[i], "count", 1, 100); err != nil {
				return errResolveResponse(err.Error()), nil
			}
			cmdArgs = append(cmdArgs, "-c", args[i])
		case "size":
			if i+1 >= len(args) {
				return errResolveResponse("ping: \"size\" requires a value"), nil
			}
			i++
			if err := validateUint(args[i], "size", 0, 65535); err != nil {
				return errResolveResponse(err.Error()), nil
			}
			cmdArgs = append(cmdArgs, "-s", args[i])
		default:
			return errResolveResponse("ping: unknown option " + strconv.Quote(args[i])), nil
		}
	}
	cmdArgs = append(cmdArgs, target)

	reqCtx, cancel := context.WithTimeout(ctx.Context(), 15*time.Second)
	defer cancel()

	out, err := captureOSCommand(reqCtx, "ping", cmdArgs...)
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

func validateUint(s, name string, lo, hi uint64) error {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("%s %q: not a valid number", name, s)
	}
	if n < lo || n > hi {
		return fmt.Errorf("%s %d: out of range %d..%d", name, n, lo, hi)
	}
	return nil
}

func captureOSCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput() //nolint:gosec // fixed-allowlist name, no shell
}
