// Design: docs/architecture/testing/interop.md -- the FreeRADIUS subscriber suite, radius-subscriber-framed-mtu
// Related: checkers.go -- waitStart, the server's own record of the session
// Related: check_subscriber_dns.go -- overrideMount, the per-scenario users file
// RFC 2865 Section 5.12 -- see rfc/short/rfc2865.md.
package radiussubscriber

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// framedMTUWant is the Framed-MTU the scenario's users file returns.
	framedMTUWant = 1400
	// framedMTUSessionMRU is the receive MRU the LAC's pppd agrees in LCP:
	// the suite's ppp-options names no mru, so pppd keeps the default 1500.
	// It is also what ze's pppN would hold were the Framed-MTU ignored, so a
	// 1400 beside it can only come from RADIUS.
	framedMTUSessionMRU = 1500
	// framedMTUSubscriber is the user the scenario's users file names.
	framedMTUSubscriber = "alice"
	// restExecuteURL is ze's REST execute endpoint, the api-server block of
	// the scenario's ze.conf.
	restExecuteURL = "http://127.0.0.1:17012/api/v1/execute"
)

// configureFramedMTUServerPeer gives this scenario its own users file, whose
// Access-Accept carries Framed-MTU 1400 (FreeRADIUS 3.2.10
// share/dictionary.rfc2865: "ATTRIBUTE Framed-MTU 12 integer"; rlm_files'
// file_common moves a users entry's reply items into the reply).
func configureFramedMTUServerPeer(source interoplab.ScenarioSource, server *interoplab.PeerConfig) {
	overrideMount(server, serverUsers, filepath.Join(source.Directory, "users"))
}

// checkFramedMTU proves on ze's side that an Access-Accept's Framed-MTU sets
// the IP MTU of the subscriber's pppN: the kernel reports MTU 1400 on ze's one
// PPP interface, and `show l2tp sessions` reports mtu 1400 beside mru 1500.
// The mru clause is what makes the 1400 discriminate: LCP agreed 1500, so
// with the Framed-MTU ignored both the kernel and mtu read 1500.
//
// RFC 2865 Section 5.12: "This Attribute indicates the Maximum Transmission Unit to be
// configured for the user, when it is not negotiated by some other
// means (such as PPP)."
// MUTATION: return transmit unconditionally from pppSession.interfaceMTU
// (internal/component/l2tp/ppp/framed_mtu.go); the kernel then reports 1500
// and this checker fails.
func checkFramedMTU(ctx context.Context, lab *scenarioLab) error {
	if _, err := lab.waitStart(ctx, ""); err != nil {
		return err
	}
	if err := lab.waitKernelMTU(ctx, framedMTUWant); err != nil {
		return err
	}
	if err := lab.waitSessionMTU(ctx, framedMTUWant, framedMTUSessionMRU); err != nil {
		return err
	}
	return lab.subscriberPing(ctx)
}

// pppLink is one interface of `ip -j link show type ppp`.
type pppLink struct {
	Name string `json:"ifname"`
	MTU  int    `json:"mtu"`
}

// waitKernelMTU waits for ze's container to hold a PPP interface, then judges
// what the kernel reports. The poll waits only for the interface, so a wrong
// MTU fails at once with the value the kernel holds.
// verbShow is the read verb of the ip and tc commands the checkers run in
// ze's container.
const verbShow = "show"

func (s *scenarioLab) waitKernelMTU(ctx context.Context, want int) error {
	output, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: s.timeout, Interval: time.Second, Description: "ze's PPP interface",
	}, func(probeCtx context.Context) (string, error) {
		return s.lab.Query(probeCtx, peerZe, []string{"ip", "-j", "link", verbShow, "type", "ppp"}, nil)
	}, func(output string) bool {
		links, err := parsePPPLinks(output)
		return err == nil && len(links) > 0
	})
	if err != nil {
		return fmt.Errorf("ze's PPP interface never appeared: %w", err)
	}
	return judgeKernelMTU(output, want)
}

// parsePPPLinks decodes `ip -j link show type ppp`. Output that is not the
// JSON array iproute2 prints is an error, never an empty answer.
func parsePPPLinks(output string) ([]pppLink, error) {
	var links []pppLink
	if err := json.Unmarshal([]byte(output), &links); err != nil {
		return nil, fmt.Errorf("decode ze's ip -j link show type ppp: %w: %s", err, output)
	}
	return links, nil
}

// judgeKernelMTU requires one PPP interface whose MTU is want. A second
// interface is a session the scenario never set up, and a nameless entry is
// an answer the kernel did not give.
func judgeKernelMTU(output string, want int) error {
	links, err := parsePPPLinks(output)
	if err != nil {
		return err
	}
	if len(links) != 1 {
		return fmt.Errorf("ze holds %d PPP interfaces, want one: %s", len(links), output)
	}
	if links[0].Name == "" {
		return fmt.Errorf("ze's PPP interface has no name: %s", output)
	}
	if links[0].MTU != want {
		return fmt.Errorf("ze's %s has kernel MTU %d, want the RADIUS Framed-MTU %d", links[0].Name, links[0].MTU, want)
	}
	return nil
}

// l2tpSession is the part of one `show l2tp sessions` entry this checker
// judges: mru is null before LCP opens, and mtu until the network lifetime
// is up.
type l2tpSession struct {
	Username string `json:"username"`
	MRU      *int   `json:"mru"`
	MTU      *int   `json:"mtu"`
}

// waitSessionMTU waits for ze's one session to report an mtu, then judges
// its mtu and mru.
func (s *scenarioLab) waitSessionMTU(ctx context.Context, wantMTU, wantMRU int) error {
	sessions, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: s.timeout, Interval: time.Second, Description: "ze's session reporting its interface MTU",
	}, s.showSessions, func(sessions []l2tpSession) bool {
		return len(sessions) == 1 && sessions[0].MTU != nil
	})
	if err != nil {
		return fmt.Errorf("show l2tp sessions never reported one session with an mtu: %w", err)
	}
	return judgeSession(sessions[0], wantMTU, wantMRU)
}

// judgeSession requires the scenario's subscriber, the MRU LCP agreed as mru,
// and the Framed-MTU as mtu.
func judgeSession(session l2tpSession, wantMTU, wantMRU int) error {
	if session.Username != framedMTUSubscriber {
		return fmt.Errorf("ze's session names user %q, want %s", session.Username, framedMTUSubscriber)
	}
	if session.MRU == nil {
		return fmt.Errorf("ze's session reports mru null, want %d: LCP is not open", wantMRU)
	}
	if *session.MRU != wantMRU {
		return fmt.Errorf("ze's session reports mru %d, want %d, the MRU LCP agreed", *session.MRU, wantMRU)
	}
	if session.MTU == nil {
		return fmt.Errorf("ze's session reports mtu null, want %d", wantMTU)
	}
	if *session.MTU != wantMTU {
		return fmt.Errorf("ze's session reports mtu %d, want the RADIUS Framed-MTU %d", *session.MTU, wantMTU)
	}
	return nil
}

// showSessions runs `show l2tp sessions` through ze's REST surface and decodes
// its data.
func (s *scenarioLab) showSessions(ctx context.Context) ([]l2tpSession, error) {
	answer, err := s.lab.Query(ctx, peerZe, []string{
		"wget", "-qO-",
		"--header=Authorization: Bearer secret",
		"--header=Content-Type: application/json",
		"--post-data={\"command\":\"show l2tp sessions\"}",
		restExecuteURL,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("show l2tp sessions: %w", err)
	}
	return parseSessions(answer)
}

// parseSessions decodes the REST answer to `show l2tp sessions`. A refused
// command or an undecodable answer is an error, never an empty list.
func parseSessions(answer string) ([]l2tpSession, error) {
	var response struct {
		Status string          `json:"status"`
		Error  string          `json:"error"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(answer), &response); err != nil {
		return nil, fmt.Errorf("decode ze's answer to show l2tp sessions: %w: %s", err, answer)
	}
	if response.Status != "done" {
		return nil, fmt.Errorf("ze refused show l2tp sessions (status %q): %s", response.Status, response.Error)
	}
	var sessions []l2tpSession
	if err := json.Unmarshal(response.Data, &sessions); err != nil {
		return nil, fmt.Errorf("decode ze's sessions: %w: %s", err, response.Data)
	}
	return sessions, nil
}
