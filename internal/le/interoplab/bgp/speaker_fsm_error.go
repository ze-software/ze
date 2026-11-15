// Design: docs/architecture/testing/interop.md -- RFC 6608 relay over actual FRR frames.
// Related: extended_relay.go -- existing dial, framing and bounded transparent workers.
package bgp

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

const (
	fsmZe               = "ze"
	fsmNative           = "native"
	fsmCaptureBase      = "/tmp/fsm-error"
	fsmWitnessPrefix    = "10.66.8.0/24"
	fsmCaptureFramesMax = 64
)

type fsmRelayFrame struct {
	Source    string `json:"source"`
	Frame     string `json:"frame"`
	Delivered bool   `json:"delivered"`
	Replay    bool   `json:"replay,omitempty"`
}

type fsmRelayReport struct {
	Personality    string          `json:"personality"`
	ZeEndpoint     string          `json:"ze-endpoint"`
	NativeEndpoint string          `json:"native-endpoint"`
	Frames         []fsmRelayFrame `json:"frames"`
	ZeEOF          bool            `json:"ze-eof"`
	NativeEOF      bool            `json:"native-eof"`
	Finished       bool            `json:"finished"`
	Problem        string          `json:"problem,omitempty"`
}

// fsmRelay owns exactly one original connection to each implementation. The
// sequential handshake has no timing window: every transition is witnessed by
// an actual complete frame before the captured native frame is delivered or
// replayed. It never constructs a protocol message.
type fsmRelay struct {
	ze, native net.Conn
	report     fsmRelayReport
}

func runFSMErrorRelay(options speakerOptions, _ io.Writer, test fsmRelayCase) (resultErr error) {
	ctx, cancel := context.WithTimeout(context.Background(), options.duration)
	defer cancel()
	relay := &fsmRelay{report: fsmRelayReport{Personality: test.personality}}
	defer func() {
		relay.report.Finished = true
		if resultErr != nil {
			relay.report.Problem = resultErr.Error()
		}
		resultErr = errors.Join(resultErr, writeJSON(options.result+"-result.json", relay.report))
		// A fatal connection must not make its evidence container disappear.
		<-ctx.Done()
	}()
	native, err := dialExtendedRelay(ctx, options.relayPeer)
	if err != nil {
		return err
	}
	relay.native = native
	defer func() { resultErr = errors.Join(resultErr, native.Close()) }()
	ze, err := dialExtendedRelay(ctx, options.connect)
	if err != nil {
		return err
	}
	relay.ze = ze
	defer func() { resultErr = errors.Join(resultErr, ze.Close()) }()
	relay.report.ZeEndpoint, relay.report.NativeEndpoint = ze.RemoteAddr().String(), native.RemoteAddr().String()
	deadline, ok := ctx.Deadline()
	if !ok {
		return errors.New("FSM relay has no deadline")
	}
	if err := errors.Join(ze.SetDeadline(deadline), native.SetDeadline(deadline)); err != nil {
		return err
	}
	if err := relay.handshake(test); err != nil {
		return err
	}
	if test.subcode != 0 {
		return judgeFSMRelay(relay.reportWithFinished(), test)
	}
	// The control's declared interval ends on the forwarded configured route.
	// Its sockets remain live for native state/RIB queries. Later forwarding is
	// outside the declared proof and is bounded by the same lifetime.
	relay.report.Finished = true
	if err := judgeFSMRelay(relay.report, test); err != nil {
		return err
	}
	if err := writeJSON(options.result+"-result.json", relay.report); err != nil {
		return err
	}
	results := make(chan error, 2)
	go forwardExtendedRelay(native, ze, relayCaptureOptions{transparent: true}, options.result+"-ze-tail.jsonl", results)
	go forwardExtendedRelay(ze, native, relayCaptureOptions{transparent: true}, options.result+"-native-tail.jsonl", results)
	first := <-results
	// A worker can finish without an error. Publish the lost forwarding
	// interval before stopping the other worker, so a native-query interval
	// cannot retain the earlier healthy report after either tail has ended.
	tailErr := relay.endControlTail(options.result, first)
	stopErr := errors.Join(ze.SetDeadline(time.Now()), native.SetDeadline(time.Now()))
	second := <-results
	return errors.Join(tailErr, second, stopErr)
}

func (relay *fsmRelay) endControlTail(path string, cause error) error {
	err := errors.Join(errors.New("FSM control relay forwarding ended"), cause)
	relay.report.Problem = err.Error()
	return errors.Join(err, writeJSON(path+"-result.json", relay.report))
}

func (relay *fsmRelay) reportWithFinished() fsmRelayReport {
	report := relay.report
	report.Finished = true
	return report
}

func (relay *fsmRelay) read(source string) ([]byte, error) {
	if len(relay.report.Frames) >= fsmCaptureFramesMax {
		return nil, errors.New("FSM relay frame bound reached")
	}
	connection := relay.native
	if source == fsmZe {
		connection = relay.ze
	}
	var buffer [65535]byte
	n, err := readExtendedFrame(connection, buffer[:])
	if err != nil {
		return nil, err
	}
	frame := bytes.Clone(buffer[:n])
	if _, err := validateSpeakerMessageHeader(frame[:bgpHeaderLength]); err != nil {
		return nil, err
	}
	return frame, nil
}

// deliver records only after the complete write. A held original is recorded
// by hold; its later replay is an explicit second delivery of the same bytes.
func (relay *fsmRelay) deliver(source string, frame []byte, replay bool) error {
	if len(relay.report.Frames) >= fsmCaptureFramesMax {
		return errors.New("FSM relay frame bound reached")
	}
	target := relay.ze
	if source == fsmZe {
		target = relay.native
	}
	n, err := target.Write(frame)
	if err != nil {
		return err
	}
	if n != len(frame) {
		return io.ErrShortWrite
	}
	relay.report.Frames = append(relay.report.Frames, fsmRelayFrame{Source: source, Frame: hex.EncodeToString(frame), Delivered: true, Replay: replay})
	return nil
}

func (relay *fsmRelay) hold(source string, frame []byte) {
	relay.report.Frames = append(relay.report.Frames, fsmRelayFrame{Source: source, Frame: hex.EncodeToString(frame)})
}

func (relay *fsmRelay) expect(source string, kind byte) ([]byte, error) {
	frame, err := relay.read(source)
	if err != nil {
		return nil, err
	}
	if frame[18] != kind {
		return nil, fmt.Errorf("FSM relay %s type %d, want %d", source, frame[18], kind)
	}
	if kind == bgpOpen {
		if err := validateSpeakerOpen(frame[bgpHeaderLength:]); err != nil {
			return nil, err
		}
	}
	return frame, nil
}

func (relay *fsmRelay) handshake(test fsmRelayCase) error {
	nativeOpen, err := relay.expect(fsmNative, bgpOpen)
	if err != nil {
		return err
	}
	if test.subcode == 1 {
		relay.hold(fsmNative, nativeOpen)
	} else {
		if err := relay.deliver(fsmNative, nativeOpen, false); err != nil {
			return err
		}
	}
	zeOpen, err := relay.expect(fsmZe, bgpOpen)
	if err != nil {
		return err
	}
	if err := relay.deliver(fsmZe, zeOpen, false); err != nil {
		return err
	}
	if test.subcode != 1 {
		zeKeepalive, err := relay.expect(fsmZe, bgpKeepalive)
		if err != nil {
			return err
		}
		if err := relay.deliver(fsmZe, zeKeepalive, false); err != nil {
			return err
		}
	}
	nativeKeepalive, err := relay.expect(fsmNative, bgpKeepalive)
	if err != nil {
		return err
	}
	if test.subcode == 2 {
		relay.hold(fsmNative, nativeKeepalive)
	} else {
		if err := relay.deliver(fsmNative, nativeKeepalive, false); err != nil {
			return err
		}
	}
	if test.subcode == 0 || test.subcode == 3 {
		for {
			frame, err := relay.expect(fsmZe, bgpUpdate)
			if err != nil {
				return err
			}
			if err := relay.deliver(fsmZe, frame, false); err != nil {
				return err
			}
			if fsmRouteWitness(frame) {
				break
			}
		}
	}
	if test.subcode == 0 {
		return nil
	}
	if test.subcode != 1 {
		if err := relay.deliver(fsmNative, nativeOpen, true); err != nil {
			return err
		}
	}
	for {
		frame, err := relay.read(fsmZe)
		if err != nil {
			return err
		}
		if err := relay.deliver(fsmZe, frame, false); err != nil {
			return err
		}
		if frame[18] == bgpNotification {
			break
		}
		if frame[18] != bgpUpdate {
			return errors.New("unexpected Ze frame before FSM notification")
		}
	}
	// Neither side is force-closed to obtain these EOFs. The peer must read
	// Ze's notification and close its own connection. Drain an already queued
	// native EOR rather than mistaking a pending complete frame for non-closure.
	for _, source := range []string{fsmZe, fsmNative} {
		for {
			frame, err := relay.read(source)
			if errors.Is(err, io.EOF) {
				if source == fsmZe {
					relay.report.ZeEOF = true
				} else {
					relay.report.NativeEOF = true
				}
				break
			}
			if err != nil {
				return err
			}
			relay.hold(source, frame)
		}
	}
	return nil
}

func fsmRouteWitness(frame []byte) bool {
	if len(frame) < bgpHeaderLength || frame[18] != bgpUpdate {
		return false
	}
	update, err := parsedEmptyMPUpdate(frame[bgpHeaderLength:])
	if err != nil {
		return false
	}
	return bytes.Equal(update.nlri, []byte{24, 10, 66, 8})
}
