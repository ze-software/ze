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

func parseSessionRequest(environ []string) (SessionRequest, error) {
	var request SessionRequest
	// The interactive session honors no rendering key, but a value of it
	// this daemon does not define is refused here as on the exec channel.
	if _, err := clientPresents(environ, "interactive SSH"); err != nil {
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
	presents, err := clientPresents(environ, "SSH exec")
	if err != nil {
		return command.StreamRenderingUnspecified, err
	}
	if presents {
		return command.StreamRenderingClientPresentation, nil
	}
	return command.StreamRenderingTerminal, nil
}

// clientPresents reads the rendering key from environ: true when it holds its
// one value, false when it is absent. The key is untrusted input from any
// client and hands over presentation only, so a second setting or any other
// value is refused by name; route names the channel in the refusal.
func clientPresents(environ []string, route string) (bool, error) {
	seen := false
	for _, entry := range environ {
		key, value, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		if key != sshclient.EnvCLIRender {
			continue
		}
		if seen {
			var problem textbuf.Buffer
			return false, opererr.New(
				problem.Str(route).Str(": the session sets ").Str(sshclient.EnvCLIRender).Str(" more than once").String(),
				"the daemon cannot tell which rendering the client asked for, so it runs no command",
				"set "+sshclient.EnvCLIRender+"="+sshclient.CLIRenderClient+" once, or leave it unset")
		}
		seen = true
		if value != sshclient.CLIRenderClient {
			var problem textbuf.Buffer
			return false, opererr.New(
				problem.Str(route).Str(": ").Str(sshclient.EnvCLIRender).Str("=").Quoted(value).Str(" is not a rendering this daemon knows").String(),
				"the key's one value is "+sshclient.CLIRenderClient+", and an unknown value is refused rather than guessed",
				"set "+sshclient.EnvCLIRender+"="+sshclient.CLIRenderClient+", or leave it unset so the daemon renders")
		}
	}
	return seen, nil
}
