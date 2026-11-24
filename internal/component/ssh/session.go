// Design: docs/features/cli-commands.md -- daemon-owned interactive configuration.
// Overview: ssh.go -- SSH server lifecycle
// RFC 4254 Section 6.4: environment requests are decoded by charm.land/ssh.
// The selected names are application metadata, never process environment writes.
// Request offsets: byte 0 message; 1 channel; 5 request-name length; 9 "env";
// 12 reply flag; 13 name length N; 17 name; 17+N value length; 21+N value.

package ssh

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// createSessionModel builds the selected terminal in the owning daemon.
func (s *Server) createSessionModel(sessionCtx context.Context, username, remoteAddr string, authorizer plugin.Authorizer, request SessionRequest) (tea.Model, error) {
	s.mu.Lock()
	factory := s.sessionModelFactory
	s.mu.Unlock()

	if factory == nil {
		return nil, fmt.Errorf("interactive SSH is unavailable: no session model factory")
	}
	return factory(sessionCtx, username, remoteAddr, authorizer, request)
}

const (
	routeInteractiveSSH = "interactive SSH"
	routeSSHExec        = "SSH exec"
)

func parseSessionRequest(environ []string) (SessionRequest, error) {
	var request SessionRequest
	// The interactive session honors no exec-only session keys, but a value of
	// either key this daemon does not define is refused here as on the exec channel.
	if _, err := clientPresents(environ, routeInteractiveSSH); err != nil {
		return SessionRequest{}, err
	}
	if _, err := execAnswerFramed(environ, routeInteractiveSSH); err != nil {
		return SessionRequest{}, err
	}
	var configSeen, modeSeen bool
	for _, entry := range environ {
		key, value, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		switch key {
		case sshclient.EnvConfigName:
			if configSeen {
				return SessionRequest{}, fmt.Errorf("interactive SSH: duplicate configuration selection")
			}
			configSeen = true
			if !fs.ValidPath(value) || value == "." || strings.ContainsAny(value, "/\\\r\n\x00") {
				return SessionRequest{}, fmt.Errorf("interactive SSH: configuration %q must be one stored filename", value)
			}
			request.ConfigName = value
		case sshclient.EnvCLIMode:
			if modeSeen {
				return SessionRequest{}, fmt.Errorf("interactive SSH: duplicate mode selection")
			}
			modeSeen = true
			if value != sshclient.CLIModeEdit && value != sshclient.CLIModeCommand {
				return SessionRequest{}, fmt.Errorf("interactive SSH: unknown mode %q", value)
			}
			request.Mode = value
		}
	}
	return request, nil
}

// execStreamRendering answers the rendering the exec stream route states for
// a session: the client presents only when it set the rendering key
// (sshclient.EnvCLIRender) to its one value. It reads that key alone, so the
// exec channel still ignores the interactive keys (ZE_CONFIG_NAME,
// ZE_CLI_MODE) as it always did.
func execStreamRendering(environ []string) (command.StreamRendering, error) {
	presents, err := clientPresents(environ, routeSSHExec)
	if err != nil {
		return command.StreamRenderingUnspecified, err
	}
	if presents {
		return command.StreamRenderingClientPresentation, nil
	}
	return command.StreamRenderingTerminal, nil
}

// sessionEnvFlag describes a single-valued SSH session environment key that
// clients may set once to its one declared value or leave unset.
type sessionEnvFlag struct {
	key      string
	value    string
	kind     string
	dupWhy   string
	unsetFix string
}

// parseSessionEnvFlag reads a single-valued session key from environ: true
// when set once to flag.value, false when absent. Because the key is untrusted
// client input, a duplicate setting or any other value is refused by name.
func parseSessionEnvFlag(environ []string, route string, flag sessionEnvFlag) (bool, error) {
	seen := false
	for _, entry := range environ {
		key, value, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		if key != flag.key {
			continue
		}
		if seen {
			var problem textbuf.Buffer
			return false, opererr.New(
				problem.Str(route).Str(": the session sets ").Str(flag.key).Str(" more than once").String(),
				flag.dupWhy,
				"set "+flag.key+"="+flag.value+" once, or leave it unset")
		}
		seen = true
		if value != flag.value {
			var problem textbuf.Buffer
			return false, opererr.New(
				problem.Str(route).Str(": ").Str(flag.key).Str("=").Quoted(value).Str(" is not a ").Str(flag.kind).Str(" this daemon knows").String(),
				"the key's one value is "+flag.value+", and an unknown value is refused rather than guessed",
				"set "+flag.key+"="+flag.value+", or "+flag.unsetFix)
		}
	}
	return seen, nil
}

// clientPresents reads the rendering key from environ: true when it holds its
// one value, false when it is absent. The key is untrusted input from any
// client and hands over presentation only, so a second setting or any other
// value is refused by name; route names the channel in the refusal.
func clientPresents(environ []string, route string) (bool, error) {
	return parseSessionEnvFlag(environ, route, sessionEnvFlag{
		key:      sshclient.EnvCLIRender,
		value:    sshclient.CLIRenderClient,
		kind:     "rendering",
		dupWhy:   "the daemon cannot tell which rendering the client asked for, so it runs no command",
		unsetFix: "leave it unset so the daemon renders",
	})
}

// execAnswerFramed reports whether the session opted into the stderr answer
// frame (sshclient.EnvAnswerFrame=1). Absent means a plain SSH client that
// receives rendered output without frame lines on stderr. Because the key is
// untrusted client input, a duplicate setting or an unknown value is refused
// by name rather than silently ignored.
func execAnswerFramed(environ []string, route string) (bool, error) {
	return parseSessionEnvFlag(environ, route, sessionEnvFlag{
		key:      sshclient.EnvAnswerFrame,
		value:    sshclient.AnswerFrameEnabled,
		kind:     "framing value",
		dupWhy:   "the daemon cannot tell whether the client wants answer framing, so it runs no command",
		unsetFix: "leave it unset for plain SSH output",
	})
}
