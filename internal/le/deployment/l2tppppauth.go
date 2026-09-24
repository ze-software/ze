// Design: docs/architecture/testing/interop.md -- credentialed native L2TP PPP proof
// Overview: l2tpppp.go -- valid-secret traffic and restart precede this refusal
package deployment

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	pppCHAPAcceptedLine = "l2tp-auth-local: CHAP-MD5 accepted"
	pppCHAPRejectedLine = "l2tp-auth-local: CHAP-MD5 rejected"
)

// A timeout or a disconnected peer is not evidence of secret validation. The
// independent pppd must receive a Failure for the challenge it answered, and
// neither endpoint may have entered IPCP, even transiently.
func l2tpPPPCHAPRejected(log string) (bool, error) {
	for _, forbidden := range []string{" [IPCP ", " [CHAP Success ", "local  IP address ", "remote IP address "} {
		if strings.Contains(log, forbidden) {
			return false, errors.New("rejected peer reached the network phase: " + forbidden)
		}
	}
	var challenge, response string
	for line := range strings.SplitSeq(log, "\n") {
		for _, packet := range []struct{ prefix, kind string }{
			{"rcvd [CHAP Challenge ", "challenge"},
			{"sent [CHAP Response ", "response"},
			{"rcvd [CHAP Failure ", "failure"},
		} {
			at := strings.Index(line, packet.prefix)
			if at < 0 {
				continue
			}
			fields := strings.Fields(line[at+len(packet.prefix):])
			if len(fields) == 0 || !strings.HasPrefix(fields[0], "id=") {
				continue
			}
			id := strings.TrimSuffix(fields[0], "]")
			switch packet.kind {
			case "challenge":
				challenge, response = id, ""
			case "response":
				if id == challenge {
					response = id
				}
			case "failure":
				if id == response {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// The control tunnel may remain while xl2tpd is alive. PPP units, L2TP sessions,
// and subscriber routes MUST disappear before the proof stops that peer; forced
// peer cleanup cannot count as rejection by Ze.
func l2tpPPPRejectedState(baselines []pppBaseline) (bool, error) {
	for _, base := range baselines {
		links, err := pppLinks(base.ns)
		if err != nil {
			return false, err
		}
		if !maps.Equal(links, base.links) {
			return false, nil
		}
		state, err := readL2TPSnapshot(base.ns)
		if err != nil {
			return false, err
		}
		if state.session != base.l2tp.session {
			return false, nil
		}
		for _, address := range []string{L2TPPPPLocalAddr, L2TPPPPPeerAddr} {
			out, ok := nsText(base.ns, "ip", "-4", "route", "show", "table", "all", "exact", address+"/32")
			if !ok {
				return false, fmt.Errorf("%s rejected-peer route inspection failed: %s", base.ns, out)
			}
			if strings.TrimSpace(out) != "" {
				return false, nil
			}
		}
	}
	return true, nil
}

func (l *L2TPPPP) assertWrongSecret(seen *collector, ze *running, work string, baselines []pppBaseline) error {
	work = filepath.Join(work, "wrong-secret")
	if err := l.writeInputs(work); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(work, PeerOptionsFile), []byte(l.pppOptions(work, "wrong-secret")), secretsMode); err != nil {
		return err
	}
	mark := l2tpPPPObservation(seen)
	peer := nsCommand(l.LACNamespace, PeerName,
		"-D", "-c", filepath.Join(work, PeerConfigFile),
		"-s", filepath.Join(work, PeerSecretsFile),
		"-p", filepath.Join(work, "xl2tpd.pid"),
		"-C", filepath.Join(work, "l2tp-control"))
	said := newCollector()
	dialer, err := startWatched(peer, "wrong-secret xl2tpd> ", said, l.Progress)
	if err != nil {
		return err
	}
	// Every started peer MUST be stopped and its output collector drained.
	defer said.wait()
	defer dialer.stop()

	forbidden := append(slices.Clone(pppFatalLines), pppCHAPAcceptedLine, pppIPLine, pppRouteLine, pppUpLine)
	deadline := time.NewTimer(l.NCPWait)
	defer deadline.Stop()
	poll := time.NewTicker(pollInterval)
	defer poll.Stop()
	for {
		if ze.exited() {
			return errors.New("ze exited before completing wrong-secret rejection")
		}
		fresh := l2tpPPPSince(seen, mark)
		if fatal := fresh.firstSeen(forbidden); fatal != "" {
			return errors.New("wrong-secret session produced forbidden observation: " + fatal)
		}
		body, err := os.ReadFile(filepath.Join(work, "pppd.log"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		rejected, err := l2tpPPPCHAPRejected(string(body))
		if err != nil {
			return err
		}
		if rejected && fresh.sawAll([]string{pppSessionLine, pppCHAPRejectedLine, pppTeardownLine}) {
			settled, err := l2tpPPPRejectedState(baselines)
			if err != nil {
				return err
			}
			if settled {
				writeProgress(l.Progress, "wrong-secret pppd rejection:\n"+string(body))
				break
			}
		}
		select {
		case <-deadline.C:
			return errors.New("timed out awaiting CHAP Failure, local rejection, and removal of the rejected network session")
		case <-poll.C:
		}
	}
	dialer.stop()
	said.wait()
	if err := awaitTeardown(baselines, l.CleanupWait); err != nil {
		return err
	}
	if fatal := l2tpPPPSince(seen, mark).firstSeen(forbidden); fatal != "" {
		return errors.New("wrong-secret session produced forbidden observation: " + fatal)
	}
	body, err := os.ReadFile(filepath.Join(work, "pppd.log"))
	if err != nil {
		return err
	}
	rejected, err := l2tpPPPCHAPRejected(string(body))
	if err != nil {
		return err
	}
	if !rejected {
		return errors.New("final peer log lost the CHAP rejection exchange")
	}
	if ze.exited() {
		return errors.New("ze exited during wrong-secret cleanup")
	}
	writeProgress(l.Progress, "wrong-secret CHAP-MD5 rejected without IPCP, address assignment, subscriber route, or surviving PPP/L2TP session")
	return nil
}
