// Design: docs/labs/pppoe-interop.md -- pppoe-empty-service-name: PADO/PADS carry
// exactly one Service-Name tag when no service-name is configured.
// RFC: rfc/short/rfc2516.md -- Sections 5.1, 5.2, 5.4: an empty Service-Name is
// "any service", and the AC's PADO/PADS each carry exactly one Service-Name tag.
// Related: check_ac.go -- the general Ze access-concentrator checker this one
// specializes, sharing its dial, session-wait and LCP/auth/IPCP helpers.
// Related: check_helpers.go -- processRunning, the fail-closed peer queries.
package pppoe

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	discovery "github.com/ze-software/ze/internal/component/l2tp/pppoe"
	"github.com/ze-software/ze/internal/core/pcap"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// captureFile is the pcap path tcpdump writes inside the client container.
// captureExecutable names the process this checker starts and stops by name.
const (
	captureFile       = "/tmp/discovery.pcap"
	captureExecutable = "tcpdump"
)

// checkZeAccessConcentratorEmptyServiceName proves RFC 2516 Section 5.1's "any
// service is acceptable" end to end against a real pppd client, with no
// service-name configured on Ze's AC. A session coming up proves nothing on its
// own here: the fixed unit tests already pin BuildPADO/BuildPADS always writing
// one tag, so the wire-level assertion below -- read off a real capture of the
// frames Ze actually sent -- is what a scenario for this change needs to
// discriminate from a broken encoder (docs/architecture/testing/interop.md,
// "Prove a scenario discriminates").
func checkZeAccessConcentratorEmptyServiceName(
	ctx context.Context,
	check *interoplab.CheckContext,
) (err error) {
	if check == nil || check.Lab == nil {
		return errors.New("PPPoE empty-service-name checker has no lab")
	}
	defer func() {
		err = appendPPPDLog(ctx, check.Lab, err)
		err = appendDiagnostics(ctx, check.Lab, err, zeImageName, clientImageName)
	}()

	if err := waitLogsContain(
		ctx,
		check.Lab,
		zeImageName,
		"PPPoE interface configured",
		60*time.Second,
	); err != nil {
		return fmt.Errorf("ze PPPoE AC did not bind its access interface: %w", err)
	}
	if err := waitZeRESTReady(ctx, check.Lab, 60*time.Second); err != nil {
		return err
	}

	if err := startDiscoveryCapture(ctx, check.Lab); err != nil {
		return err
	}

	// The client dials with no requested service (RFC 2516 Section 5.1's "any
	// service is acceptable"), which is what an operator who configures no
	// service-name on the AC must accept from every client.
	if err := pppdDial(ctx, check.Lab, pppdUsername, pppdPassword, ""); err != nil {
		return err
	}
	sessions, err := waitZeSession(ctx, check.Lab, 45*time.Second)
	if err != nil {
		return err
	}
	if len(sessions) != 1 {
		return fmt.Errorf("ze allocated %d PPPoE sessions, expected exactly one", len(sessions))
	}
	if sessions[0].ServiceName != "" {
		return fmt.Errorf(
			"ze recorded Service-Name %q, expected the empty accepted service",
			sessions[0].ServiceName,
		)
	}

	if _, err := checkLCPAuthIPCP(ctx, check.Lab); err != nil {
		return err
	}

	capture, err := stopDiscoveryCapture(ctx, check.Lab)
	if err != nil {
		return err
	}
	return checkDiscoveryServiceNameTags(capture)
}

// startDiscoveryCapture starts tcpdump in the client container before the dial
// begins, so the capture carries PADO and PADS from the first attempt.
// startSessionCapture (check_ipv6cp.go) is the sibling for the PPPoE session
// (0x8864) ethertype; both call startCapture, which does the waiting.
func startDiscoveryCapture(ctx context.Context, lab interoplab.CheckerLab) error {
	return startCapture(ctx, lab, captureFile, "ether proto 0x8863")
}

// stopDiscoveryCapture signals tcpdump to flush and exit, waits for it to be
// gone so the file is complete, then reads captureFile back. stopSessionCapture
// (check_ipv6cp.go) is the sibling for sessionCaptureFile; both call stopCapture.
func stopDiscoveryCapture(ctx context.Context, lab interoplab.CheckerLab) ([]byte, error) {
	return stopCapture(ctx, lab, captureFile)
}

// startCapture starts tcpdump in the client container filtering on filter,
// writing to file, and waits for the process to be running rather than
// assuming ExecDetached's acceptance means the capture already opened the
// interface.
func startCapture(ctx context.Context, lab interoplab.CheckerLab, file, filter string) error {
	shell := "rm -f " + file + "; exec " + captureExecutable + " -i eth0 -w " + file + " " + filter
	if err := lab.ExecDetached(ctx, clientImageName, []string{"sh", "-c", shell}, nil); err != nil {
		return fmt.Errorf("start capture: %w", err)
	}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     10 * time.Second,
		Interval:    200 * time.Millisecond,
		Description: "tcpdump capturing " + filter,
	}, func(probeCtx context.Context) (bool, error) {
		return processRunning(probeCtx, lab, clientImageName, captureExecutable)
	}, func(running bool) bool { return running })
	if err != nil {
		return fmt.Errorf("capture did not start: %w", err)
	}
	return nil
}

// stopCapture signals tcpdump to flush and exit, waits for it to be gone so
// file is complete, then reads it back as base64 (Docker exec output is
// captured as text, so the binary pcap crosses that boundary encoded).
func stopCapture(ctx context.Context, lab interoplab.CheckerLab, file string) ([]byte, error) {
	if _, err := exec(ctx, lab, clientImageName, []string{commandPkill, "-INT", "-x", captureExecutable}); err != nil {
		return nil, fmt.Errorf("stop capture: %w", err)
	}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     10 * time.Second,
		Interval:    200 * time.Millisecond,
		Description: "tcpdump exit",
	}, func(probeCtx context.Context) (bool, error) {
		return processRunning(probeCtx, lab, clientImageName, captureExecutable)
	}, func(running bool) bool { return !running })
	if err != nil {
		return nil, fmt.Errorf("capture did not stop: %w", err)
	}
	encoded, err := query(ctx, lab, clientImageName, []string{"sh", "-c", "base64 " + file})
	if err != nil {
		return nil, fmt.Errorf("read capture: %w", err)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, fmt.Errorf("decode capture: %w", err)
	}
	return raw, nil
}

// checkDiscoveryServiceNameTags reads every Ethernet frame tcpdump captured and
// requires exactly one PADO and one PADS, each carrying exactly one Service-Name
// tag. Absence of either frame is an error, never a passed assertion: the filter
// already narrowed the capture to PPPoE discovery, so a frame that fails to
// parse as one is a corrupt capture rather than noise to skip past.
func checkDiscoveryServiceNameTags(capture []byte) error {
	reader, err := pcap.NewReader(bytes.NewReader(capture))
	if err != nil {
		return fmt.Errorf("discovery capture: %w", err)
	}
	if reader.LinkType() != pcap.LinkTypeEthernet {
		return fmt.Errorf("discovery capture link type %d, expected Ethernet", reader.LinkType())
	}

	var padoSeen, padsSeen bool
	var record pcap.Record
	for {
		if err := reader.Next(&record); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("discovery capture: %w", err)
		}
		packet, parseErr := discovery.ParseDiscovery(record.Data)
		if parseErr != nil {
			return fmt.Errorf("discovery capture: frame matching the PPPoE filter did not parse: %w", parseErr)
		}
		switch packet.Code {
		case discovery.CodePADO:
			padoSeen = true
			if tags := len(packet.FindAllTags(discovery.TagServiceName)); tags != 1 {
				return fmt.Errorf("PADO carried %d Service-Name tags, expected exactly 1", tags)
			}
		case discovery.CodePADS:
			padsSeen = true
			if tags := len(packet.FindAllTags(discovery.TagServiceName)); tags != 1 {
				return fmt.Errorf("PADS carried %d Service-Name tags, expected exactly 1", tags)
			}
		}
	}
	if !padoSeen {
		return errors.New("discovery capture carries no PADO frame")
	}
	if !padsSeen {
		return errors.New("discovery capture carries no PADS frame")
	}
	return nil
}
