// Design: docs/architecture/testing/interop.md -- real FreeRADIUS subscriber authorization.
// Related: checkers.go -- xl2tpd redial and FreeRADIUS accounting observations.
// RFC 2865 Section 1.1 -- see rfc/short/rfc2865.md.
package radiussubscriber

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	serverUsers     = "/etc/raddb/mods-config/files/authorize"
	framedPPPUsers  = "/run/ze-users-ppp"
	framedSLIPUsers = "/run/ze-users-slip"
	framedRefusal   = "Access-Accept names an unsupported Framed-Protocol; rejecting"
	chapRefusal     = "CHAP authentication failed"
)

// configureFramedServerPeer gives only this scenario a writable server-owned users
// file. Both answers still come from FreeRADIUS, with its authenticators intact.
func configureFramedServerPeer(source interoplab.ScenarioSource, peer *interoplab.PeerConfig) {
	for i := range peer.Mounts {
		if peer.Mounts[i].Target == serverUsers {
			peer.Mounts[i].Source = filepath.Join(source.Directory, "users-ppp")
			peer.Mounts[i].Target = framedPPPUsers
		}
	}
	peer.Mounts = append(peer.Mounts, interoplab.Mount{
		Source: filepath.Join(source.Directory, "users-slip"), Target: framedSLIPUsers, ReadOnly: true,
	})
	peer.Command = []string{"sh", "-c", "cp " + framedPPPUsers + " " + serverUsers + " && exec /docker-entrypoint.sh radiusd -f -X"}
}

// checkFramedReject brackets an unavailable SLIP authorization with two PPP
// authorizations for the same username/password. The server accepts all three;
// only the LNS's interpretation of Framed-Protocol changes.
// MUTATION: remove doRADIUS's AcceptedFramedProtocol guard; pppd then receives
// CHAP Success and the refusal/state assertions fail.
func checkFramedReject(ctx context.Context, lab *scenarioLab) error {
	first, err := lab.waitRecord(ctx, "Start", "")
	if err != nil {
		return err
	}
	if err := lab.subscriberPing(ctx); err != nil {
		return err
	}
	if err := lab.clearSessions(ctx); err != nil {
		return err
	}
	if _, err := lab.waitStop(ctx, first.sessionID); err != nil {
		return err
	}
	if err := lab.setFramedProtocol(ctx, framedSLIPUsers, "SLIP"); err != nil {
		return err
	}
	before, err := lab.records(ctx)
	if err != nil {
		return err
	}
	refusals, err := lab.peerLogCount(ctx, peerLAC, chapRefusal)
	if err != nil {
		return err
	}
	if err := lab.redial(ctx); err != nil {
		return err
	}
	// RFC 2865 Section 1.1: "A NAS MUST treat a RADIUS access-accept
	// authorizing an unavailable service as an access-reject instead."
	if err := lab.waitPeerLogCount(ctx, peerZe, framedRefusal, 1); err != nil {
		return err
	}
	if err := lab.waitPeerLogCount(ctx, peerLAC, chapRefusal, refusals+1); err != nil {
		return err
	}
	after, err := lab.records(ctx)
	if err != nil {
		return err
	}
	if len(after) != len(before) {
		return fmt.Errorf("SLIP authorization changed accounting from %d to %d records; rejected authentication must not open a session", len(before), len(after))
	}
	if err := lab.setFramedProtocol(ctx, framedPPPUsers, "PPP"); err != nil {
		return err
	}
	if err := lab.redial(ctx); err != nil {
		return err
	}
	if _, err := lab.waitRecord(ctx, "Start", first.sessionID); err != nil {
		return err
	}
	return lab.subscriberPing(ctx)
}

// setFramedProtocol waits for the server's own radclient to observe the new
// Access-Accept. That is the reload barrier, not a timer or HUP's exit status.
func (s *scenarioLab) setFramedProtocol(ctx context.Context, source, protocol string) error {
	result, err := s.lab.Exec(ctx, peerServer, []string{"cp", source, serverUsers}, nil)
	if err != nil {
		return fmt.Errorf("select FreeRADIUS %s authorization: %w: %s", protocol, err, result.Stderr)
	}
	// PAP is only the reload probe. The real subscriber still authenticates
	// with CHAP from xl2tpd/pppd through the same files entry.
	probe := "printf '%s\\n' 'User-Name = alice' 'User-Password = s3cr3t' 'Message-Authenticator = 0x00' | radclient -x -r 1 -t 2 172.26.0.3:1812 auth " + sharedSecret
	// FreeRADIUS 3.2.10's handle_signal_self ignores HUP less than five
	// seconds after the preceding accepted HUP. Retry the signal, not only
	// the query, because the preceding scenario step can have just reloaded.
	// Six seconds also covers the server's whole-second time(NULL) clock.
	// Upstream: https://github.com/FreeRADIUS/freeradius-server/blob/release_3_2_10/src/main/process.c
	nextReload := time.Now()
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: s.timeout, Interval: time.Second, Description: "FreeRADIUS Access-Accept with Framed-Protocol " + protocol,
	}, func(probeCtx context.Context) (string, error) {
		if !time.Now().Before(nextReload) {
			if err := s.lab.Signal(probeCtx, peerServer, "HUP"); err != nil {
				return "", fmt.Errorf("reload FreeRADIUS users: %w", err)
			}
			nextReload = time.Now().Add(6 * time.Second)
		}
		return s.lab.Query(probeCtx, peerServer, []string{"sh", "-c", probe}, nil)
	}, func(output string) bool {
		_, received, found := strings.Cut(output, "Received Access-Accept ")
		if !found {
			return false
		}
		for line := range strings.SplitSeq(received, "\n") {
			if strings.TrimSpace(line) == "Framed-Protocol = "+protocol {
				return true
			}
		}
		return false
	})
	return err
}

func (s *scenarioLab) peerLogCount(ctx context.Context, peer, line string) (int, error) {
	logs, err := s.lab.Logs(ctx, peer, 5000)
	if err != nil {
		return 0, err
	}
	if !logs.Available {
		return 0, errors.New("peer logs unavailable: " + peer)
	}
	return strings.Count(logs.Text, line), nil
}

func (s *scenarioLab) waitPeerLogCount(ctx context.Context, peer, line string, want int) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: s.timeout, Interval: time.Second, Description: peer + " records " + line,
	}, func(probeCtx context.Context) (int, error) {
		return s.peerLogCount(probeCtx, peer, line)
	}, func(count int) bool { return count >= want })
	return err
}
