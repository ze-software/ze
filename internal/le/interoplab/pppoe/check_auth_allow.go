// Design: docs/labs/pppoe-interop.md -- CHAP-only AC refusal of a PAP-only client.
// Related: check_lifecycle.go -- capture activation, packet reads and kernel cleanup.
// Related: check_pap.go -- PAP protocol identity and the independent PAP success scenario.
// Peer: https://github.com/ppp-project/ppp/blob/v2.5.5/pppd/lcp.c -- lcp_reqci
// Naks refused CHAP with PAP when EAP is also refused; fsm.c fsm_rtermreq
// acknowledges termination in Req-Sent/Ack-Sent without immediately exiting.

//go:build ze_l2tp

package pppoe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// checkCHAPOnlyPAPClient observes the complete refused dial, not merely an
// empty session table. Capture activation MUST precede the dial, and the final
// capture MUST follow peer exit and kernel cleanup so duplicate PADTs count.
// MUTATION: restore PAP in ResolveAuthAllow's default CHAP policy and rebuild
// the staged daemon/image; this same scenario must refuse the PAP offer/exchange.
func checkCHAPOnlyPAPClient(ctx context.Context, check *interoplab.CheckContext) (err error) {
	if check == nil {
		return errors.New("CHAP-only PAP-client checker has no context")
	}
	if check.Lab == nil {
		return errors.New("CHAP-only PAP-client checker has no lab")
	}
	lab := check.Lab
	defer func() {
		err = appendPPPDLog(ctx, lab, err)
		err = appendDiagnostics(ctx, lab, err, zeImageName, clientImageName)
	}()
	if err := waitZePPPoEConfigured(ctx, lab); err != nil {
		return err
	}
	if err := waitZeRESTReady(ctx, lab); err != nil {
		return err
	}
	local, err := startLifecycleCapture(ctx, lab)
	if err != nil {
		return err
	}
	if err := pppdDialRefusing(ctx, lab, pppdPassword, pppoeService, refuseCHAP); err != nil {
		return err
	}
	if err := waitLifecycleEvidence(ctx, lab, 30*time.Second, func(frames []lifecycleFrame) error {
		return chapOnlyRefusalEvidence(frames, local)
	}); err != nil {
		return err
	}
	// pppd keeps negotiating after an LCP TermReq before Opened. Ze's PADT,
	// not a timed stop or a signal from this checker, must end the dial.
	if err := waitPPPDExit(ctx, lab, 10*time.Second); err != nil {
		return err
	}
	if _, err := waitZeSessionsGone(ctx, lab, 10*time.Second); err != nil {
		return err
	}
	if err := waitZeKernelRelease(ctx, lab, "(refused dial)"); err != nil {
		return err
	}
	links, err := pppLinks(ctx, lab, clientImageName)
	if err != nil {
		return err
	}
	if len(links) != 0 {
		return fmt.Errorf("PAP-only client retained PPP links after refusal: %v", links)
	}
	if err := stopLifecycleCapture(ctx, lab); err != nil {
		return err
	}
	frames, err := readLifecycleCapture(ctx, lab)
	if err != nil {
		return err
	}
	if err := chapOnlyRefusalEvidence(frames, local); err != nil {
		return err
	}
	log, err := pppdLog(ctx, lab)
	if err != nil {
		return err
	}
	for _, marker := range []string{"rcvd [LCP ConfReq", "sent [LCP ConfNak", "<auth pap>", "rcvd [LCP TermReq"} {
		if !strings.Contains(log, marker) {
			return interoplab.WithSearchedOutput(fmt.Errorf("PAP-only pppd refusal trace lacks %s", marker), log)
		}
	}
	logs, err := lab.Logs(ctx, zeImageName, 200)
	if err != nil {
		return err
	}
	if !logs.Available {
		return errors.New("Ze authentication refusal log is unavailable")
	}
	return chapOnlyRefusalCause(logs.Text)
}

// chapOnlyRefusalCause requires one refusal record to name both the peer's
// suggestion and the effective allowed methods, rather than a generic failure.
func chapOnlyRefusalCause(log string) error {
	for line := range strings.SplitSeq(log, "\n") {
		if !strings.Contains(line, "auth-method-refused: peer suggested Auth-Protocol 0xc023 (pap)") {
			continue
		}
		if strings.Contains(line, "allowed [chap-md5]") {
			return nil
		}
	}
	return interoplab.WithSearchedOutput(errors.New("Ze refusal log does not name PAP and the CHAP-only allow-list"), log)
}

// chapOnlyRefusalEvidence requires the same-session wire chain from Ze's PADS
// through the PAP Nak to Ze's LCP termination and exactly one outgoing PADT.
// A peer Configure-Ack would permit LCP Opened and is forbidden even if no PAP
// packet followed. Incoming PADT cannot substitute for Ze's refusal teardown.
// RFC 1661 Section 3.5: "Advancement from the Authentication phase to the
// Network-Layer Protocol phase MUST NOT occur until authentication has completed."
func chapOnlyRefusalEvidence(frames []lifecycleFrame, local [6]byte) error {
	var peer [6]byte
	var sid uint16
	var request *lifecycleFrame
	nak, terminated := false, false
	padts := 0
	for i := range frames {
		frame := &frames[i]
		if frame.protocol == 0 && frame.code == 0x65 {
			if frame.source != local {
				return errors.New("PADS did not come from Ze")
			}
			if frame.sid == 0 {
				return errors.New("Ze refused discovery instead of the authentication policy")
			}
			if sid != 0 {
				return errors.New("refused dial allocated another PPPoE session")
			}
			sid, peer = frame.sid, frame.target
			continue
		}
		if frame.sid == 0 && frame.protocol == 0 {
			continue // Initial PADI/PADO/PADR carry no session identity.
		}
		if sid == 0 {
			return errors.New("session traffic preceded Ze's PADS")
		}
		if frame.sid != sid {
			return errors.New("authentication refusal changed PPPoE session ID")
		}
		outgoing := frame.source == local && frame.target == peer
		incoming := frame.source == peer && frame.target == local
		if !outgoing && !incoming {
			return errors.New("authentication refusal changed Ethernet endpoints")
		}
		if frame.protocol == 0 {
			if frame.code != 0xa7 {
				return errors.New("unexpected discovery during authentication refusal")
			}
			if incoming {
				if padts == 0 {
					return errors.New("client PADT preceded Ze's policy teardown")
				}
				continue
			}
			if !terminated {
				return errors.New("Ze PADT preceded its LCP Terminate-Request")
			}
			padts++
			continue
		}
		if frame.protocol == pppProtocolPAP {
			return errors.New("PAP protocol traffic escaped the CHAP-only policy")
		}
		if frame.protocol != 0xc021 {
			return fmt.Errorf("authentication or network protocol 0x%04x appeared before LCP refusal", frame.protocol)
		}
		if incoming && frame.code == 2 {
			return errors.New("PAP-only client acknowledged Ze's LCP request: LCP could open")
		}
		if outgoing && frame.code == 1 {
			chap, err := chapOnlyAuthOption(frame.data, []byte{3, 5, 0xc2, 0x23, 5})
			if err != nil {
				return err
			}
			if !chap {
				return errors.New("Ze offered a non-CHAP-MD5 or absent authentication option")
			}
			request = frame
		}
		if incoming && frame.code == 3 {
			pap, err := chapOnlyAuthOption(frame.data, []byte{3, 4, 0xc0, 0x23})
			if err != nil {
				return err
			}
			if pap {
				if request == nil {
					return errors.New("PAP Nak preceded Ze's CHAP request")
				}
				if frame.id != request.id {
					return errors.New("PAP Nak did not answer Ze's CHAP request Identifier")
				}
				nak = true
			}
		}
		if outgoing && frame.code == 5 {
			if !nak {
				return errors.New("Ze terminated without the client's PAP Nak")
			}
			terminated = true
		}
	}
	if padts != 1 {
		return fmt.Errorf("captured %d Ze-originated PADTs after PAP refusal, want exactly one", padts)
	}
	return nil
}

// chapOnlyAuthOption checks every bounded LCP option, rejecting malformed or
// duplicate authentication options rather than interpreting them as absent.
func chapOnlyAuthOption(options, want []byte) (bool, error) {
	found, matches := false, false
	for len(options) > 0 {
		if len(options) < 2 {
			return false, errors.New("truncated LCP option in refusal capture")
		}
		length := int(options[1])
		if length < 2 {
			return false, errors.New("invalid LCP option length in refusal capture")
		}
		if length > len(options) {
			return false, errors.New("LCP option exceeds refusal capture payload")
		}
		if options[0] == 3 {
			if found {
				return false, errors.New("duplicate LCP authentication option in refusal capture")
			}
			found = true
			matches = bytes.Equal(options[:length], want)
		}
		options = options[length:]
	}
	return matches, nil
}
