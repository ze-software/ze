// Design: docs/architecture/testing/interop.md -- destination-connection LOCAL_PREF proof.
// Related: extended_relay.go -- the sole bounded session and frame owner.
package bgp

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	localPrefCapturePersonality = "local-pref-strip"
	localPrefSourceBoundary     = "local-pref-subjects-through-fifo-fence"
	localPrefReverseBoundary    = "keepalive-after-local-pref-fifo-fence"
)

// relayCaptureBoundary certifies only the declared original-connection interval,
// never the traffic forwarded after its terminal marker. Frames counts complete,
// successfully delivered frames preceding that marker.
type relayCaptureBoundary struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Frames   int    `json:"frames"`
	Boundary string `json:"boundary"`
}

// relayCaptureInterval belongs to one socket worker. runExtendedRelaySession MUST
// retain that worker until its result is joined. BeforeRead MUST run before the
// frame read: a reverse frame already being read when the fence arrives cannot
// serve as the subsequent KEEPALIVE barrier.
type relayCaptureInterval struct {
	BeforeRead func() bool
	Observe    func([]byte, bool) (bool, error)
	Boundary   relayCaptureBoundary
}

func localPrefRelayIntervals(source, target string) (*relayCaptureInterval, *relayCaptureInterval) {
	fence := make(chan struct{})
	outbound := &localPrefWireSequence{source: true}
	inbound := &localPrefWireSequence{}
	return &relayCaptureInterval{
		Boundary: relayCaptureBoundary{Source: source, Target: target, Boundary: localPrefSourceBoundary},
		Observe: func(frame []byte, _ bool) (bool, error) {
			if err := outbound.observe(frame); err != nil {
				return false, err
			}
			if outbound.fenced {
				// The sole source worker calls this only after the full fence
				// write and capture row succeeded. No shared mutable sequence.
				close(fence)
				return true, nil
			}
			return false, nil
		},
	}, &relayCaptureInterval{
		Boundary: relayCaptureBoundary{Source: target, Target: source, Boundary: localPrefReverseBoundary},
		BeforeRead: func() bool {
			select {
			case <-fence:
				return true
			default:
				return false
			}
		},
		Observe: func(frame []byte, afterFence bool) (bool, error) {
			if err := inbound.observe(frame); err != nil {
				return false, err
			}
			return afterFence && frame[18] == bgpKeepalive && inbound.keepalives >= 2, nil
		},
	}
}

// localPrefWireSequence validates the same fixture sequence in the capture owner
// and the checker. LOCAL_PREF presence is retained as a semantic failure, not a
// transport error: the deliberately leaked producer must still reach the fence.
type localPrefWireSequence struct {
	source     bool
	opened     bool
	keepalives int
	eor        bool
	subjects   uint8
	fenced     bool
	preference bool
}

func (sequence *localPrefWireSequence) observe(frame []byte) error {
	if len(frame) < bgpHeaderLength {
		return errors.New("short LOCAL_PREF capture frame")
	}
	switch frame[18] {
	case bgpOpen:
		if sequence.opened {
			return errors.New("LOCAL_PREF destination session was replaced")
		}
		if err := requireLocalPrefOpen(frame, sequence.source); err != nil {
			return err
		}
		sequence.opened = true
		return nil
	case bgpKeepalive:
		if !sequence.opened || len(frame) != bgpHeaderLength {
			return errors.New("invalid LOCAL_PREF session KEEPALIVE")
		}
		sequence.keepalives++
		return nil
	case bgpUpdate:
		if !sequence.opened || sequence.keepalives == 0 {
			return errors.New("LOCAL_PREF UPDATE preceded original OPEN/KEEPALIVE")
		}
		update, err := parsedEmptyMPUpdate(frame[bgpHeaderLength:])
		if err != nil {
			return err
		}
		if len(update.withdrawn) == 0 && len(update.attributes) == 0 && len(update.nlri) == 0 {
			sequence.eor = true
			return nil
		}
		if !sequence.source || len(update.withdrawn) != 0 {
			return errors.New("unexpected route or withdrawal in LOCAL_PREF interval")
		}
		if err := sequence.attributes(update.attributes); err != nil {
			return err
		}
		// RFC 4271 Section 4.3: "An UPDATE message is used to advertise feasible
		// routes that share common path attributes to a peer, or to withdraw
		// multiple unfeasible routes from service (see 3.1)."
		// Routes in one UPDATE share one attribute set and arrive together, so
		// the fence is accepted in the UPDATE that completes the three subjects,
		// in either NLRI order inside it. The whole UPDATE is validated before
		// its subjects and fence are committed to the sequence. A fence in an
		// UPDATE that leaves a subject outstanding, a second fence, and a
		// subject in an UPDATE after the fence's are refused. The attribute
		// check above covers every NLRI here.
		before := sequence.subjects
		subjects := before
		fenced := false
		for nlri := update.nlri; len(nlri) != 0; nlri = nlri[4:] {
			if len(nlri) < 4 || !bytes.Equal(nlri[:3], []byte{24, 10, 54}) {
				return errors.New("unexpected or truncated LOCAL_PREF fixture NLRI")
			}
			subnet := nlri[3]
			if subnet == 255 {
				if sequence.fenced {
					return errors.New("second LOCAL_PREF FIFO fence in the declared interval")
				}
				if fenced {
					return errors.New("second LOCAL_PREF FIFO fence in the declared interval")
				}
				fenced = true
				continue
			}
			if subnet > 2 {
				return errors.New("unexpected or duplicate LOCAL_PREF subject")
			}
			if sequence.fenced {
				return errors.New("unexpected or duplicate LOCAL_PREF subject")
			}
			if subjects&(1<<subnet) != 0 {
				return errors.New("unexpected or duplicate LOCAL_PREF subject")
			}
			if subnet == 2 && (before != 3 || !sequence.eor) {
				return errors.New("third LOCAL_PREF subject preceded initial pair or EOR")
			}
			subjects |= 1 << subnet
		}
		if fenced && subjects != 7 {
			return errors.New("LOCAL_PREF FIFO fence arrived before the UPDATE completing all three subjects")
		}
		sequence.subjects = subjects
		sequence.fenced = fenced
		if len(update.nlri) == 0 {
			return errors.New("attribute-bearing LOCAL_PREF UPDATE has no subject")
		}
		return nil
	default:
		return fmt.Errorf("unexpected LOCAL_PREF session message %d (including NOTIFICATION)", frame[18])
	}
}

func requireLocalPrefOpen(frame []byte, source bool) error {
	if len(frame) < 29 || frame[19] != 4 || int(frame[28])+29 != len(frame) {
		return errors.New("invalid original LOCAL_PREF session OPEN")
	}
	asn, router := uint16(65003), []byte{172, 30, 0, 5}
	if source {
		asn, router = 65001, []byte{172, 30, 0, 2}
	}
	if binary.BigEndian.Uint16(frame[20:22]) != asn || !bytes.Equal(frame[24:28], router) {
		return errors.New("LOCAL_PREF capture belongs to the wrong speaker")
	}
	caps, err := capability.ParseFromOptionalParams(frame[29:], false)
	if err != nil {
		return err
	}
	asn4 := 0
	for _, advertised := range caps {
		switch entry := advertised.(type) {
		case *capability.ASN4:
			if entry.ASN != uint32(asn) {
				return errors.New("LOCAL_PREF OPEN has inconsistent ASN4 identity")
			}
			asn4++
		case *capability.AddPath:
			return errors.New("ADD-PATH confounds the LOCAL_PREF fixture encoding")
		}
	}
	if asn4 != 1 {
		return errors.New("LOCAL_PREF capture lacks exactly one ASN4 capability")
	}
	return nil
}

func (sequence *localPrefWireSequence) attributes(attributes []pathAttribute) error {
	var seen [256]bool
	for _, attr := range attributes {
		if seen[attr.code] {
			return fmt.Errorf("duplicate LOCAL_PREF fixture attribute %d", attr.code)
		}
		seen[attr.code] = true
		switch attr.code {
		case 1:
			if attr.flags&^byte(0x10) != 0x40 || !bytes.Equal(attr.value, []byte{0}) {
				return errors.New("LOCAL_PREF subject ORIGIN changed")
			}
		case 2:
			if attr.flags&^byte(0x10) != 0x40 || !bytes.Equal(attr.value, []byte{2, 2, 0, 0, 0xfd, 0xe9, 0, 0, 0xfd, 0xec}) {
				return errors.New("LOCAL_PREF subject AS_PATH is not 65001 65004")
			}
		case 3:
			if attr.flags&^byte(0x10) != 0x40 || !bytes.Equal(attr.value, []byte{172, 30, 0, 9}) {
				return errors.New("LOCAL_PREF subject NEXT_HOP is not the injector")
			}
		case 5:
			if attr.flags&^byte(0x10) != 0x40 || len(attr.value) != 4 {
				return errors.New("malformed LOCAL_PREF attribute on destination wire")
			}
			sequence.preference = true
		}
	}
	if !seen[1] || !seen[2] || !seen[3] {
		return errors.New("LOCAL_PREF subject lacks original source attributes")
	}
	return nil
}

// localPrefWireSemanticError is returned only after that direction's complete
// interval passed every structural/source control. The checker MUST still wait
// for and validate the opposite interval before attributing this semantic red.
type localPrefWireSemanticError string

func (err localPrefWireSemanticError) Error() string { return string(err) }

func requireLocalPrefInterval(text string, source bool) error {
	if len(text) > 34*1024*1024 || !strings.HasSuffix(text, "\n") {
		return errors.New("missing, oversized or truncated LOCAL_PREF capture")
	}
	sequence := localPrefWireSequence{source: source}
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	frames, complete := 0, false
	var buffer [65535]byte
	for {
		var row extendedRelayFrame
		if err := decoder.Decode(&row); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if complete {
			return errors.New("LOCAL_PREF capture continues beyond its declared interval")
		}
		if row.Complete != nil {
			marker := row.Complete
			wantSource, wantTarget, boundary := "172.30.0.2:179", "172.30.0.5:179", localPrefSourceBoundary
			if !source {
				wantSource, wantTarget, boundary = wantTarget, wantSource, localPrefReverseBoundary
			}
			if row.Original != "" || row.Delivered != "" || marker.Frames != frames || marker.Source != wantSource || marker.Target != wantTarget || marker.Boundary != boundary {
				return errors.New("LOCAL_PREF completion has wrong connection, frame count or boundary")
			}
			if !sequence.opened || sequence.keepalives == 0 || (source && (!sequence.fenced || sequence.subjects != 7)) || (!source && (sequence.keepalives < 2 || buffer[18] != bgpKeepalive)) {
				return errors.New("LOCAL_PREF completion has no full original-session barrier")
			}
			complete = true
			continue
		}
		frames++
		if frames >= 256 || row.Delivered != "" {
			return errors.New("LOCAL_PREF relay exceeded bounds or altered a frame")
		}
		frame, err := hex.DecodeString(row.Original)
		if err != nil {
			return err
		}
		octets, err := readExtendedFrame(bytes.NewReader(frame), buffer[:])
		if err != nil {
			return err
		}
		if octets != len(frame) {
			return errors.New("LOCAL_PREF captured frame length mismatch")
		}
		if err := sequence.observe(frame); err != nil {
			return err
		}
	}
	if !complete {
		return errors.New("LOCAL_PREF capture has no complete declared interval")
	}
	if sequence.preference {
		return localPrefWireSemanticError("LOCAL_PREF appeared on the actual external destination wire")
	}
	return nil
}

func waitLocalPrefWire(ctx context.Context, lab interoplab.CheckerLab) error {
	var semanticErr error
	for _, direction := range []struct {
		suffix string
		source bool
	}{{"ze", true}, {peerFRR, false}} {
		var lastAnswer string
		text, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
			Timeout: 90 * time.Second, Interval: time.Second, Description: "complete original-connection LOCAL_PREF wire interval",
		}, func(probe context.Context) (string, error) {
			text, err := lab.Query(probe, peerSpeaker, []string{cmdCat, extendedCaptureBase + "-" + direction.suffix + ".jsonl"}, nil)
			lastAnswer = text
			return text, err
		}, func(text string) bool {
			if !strings.HasSuffix(text, "\n") {
				return false
			}
			last := strings.TrimSuffix(text, "\n")
			last = last[strings.LastIndex(last, "\n")+1:]
			var row extendedRelayFrame
			return json.Unmarshal([]byte(last), &row) == nil && row.Complete != nil
		})
		if err != nil {
			return interoplab.WithLastAnswer(err, lastAnswer)
		}
		if err := requireLocalPrefInterval(text, direction.source); err != nil {
			if _, semantic := errors.AsType[localPrefWireSemanticError](err); !semantic {
				return interoplab.WithSearchedOutput(err, text)
			}
			semanticErr = interoplab.WithSearchedOutput(err, text)
		}
	}
	return semanticErr
}
