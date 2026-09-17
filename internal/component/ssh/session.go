// Design: docs/features/cli-commands.md -- daemon-owned interactive configuration.
// Overview: ssh.go -- SSH server lifecycle
// RFC 4254 Section 6.4: environment requests are decoded by charm.land/ssh.
// The selected names are application metadata, never process environment writes.
// Request offsets: byte 0 message; 1 channel; 5 request-name length; 9 "env";
// 12 reply flag; 13 name length N; 17 name; 17+N value length; 21+N value.

package ssh

import (
	"fmt"
	"io/fs"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/ze-software/ze/internal/component/plugin"
	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
)

// createSessionModel builds the selected terminal in the owning daemon.
func (s *Server) createSessionModel(username, remoteAddr string, authorizer plugin.Authorizer, request SessionRequest) (tea.Model, error) {
	s.mu.Lock()
	factory := s.sessionModelFactory
	s.mu.Unlock()

	if factory == nil {
		return nil, fmt.Errorf("interactive SSH is unavailable: no session model factory")
	}
	return factory(username, remoteAddr, authorizer, request)
}

func parseSessionRequest(environ []string) (SessionRequest, error) {
	var request SessionRequest
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
