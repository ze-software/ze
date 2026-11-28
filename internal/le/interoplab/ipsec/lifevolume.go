// Design: docs/architecture/ike/ipsec-13-rekey-wire.md -- volume-triggered Child SA rekey against strongSwan.
// Related: checkers.go -- scenarioCheckers registers child-rekey-life-bytes.
// RFC: rfc/short/rfc4301.md -- Section 4.4.2.1 simultaneous time and volume lifetimes, out-of-sync counters.

package ipsec

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// lifeBytesCeiling mirrors esp-group ESP-LIFE-BYTES in
	// test/interop-ipsec/scenarios/child-rekey-life-bytes/ze.conf.
	// volumeSoftThreshold places softBytes in [90%, 95%] = [117965, 124519].
	// Stopping traffic once the higher directional SA reaches 125500 bytes guarantees
	// softBytes has been crossed while keeping at least 4 KiB of headroom below the
	// 131072-byte hard ceiling.
	lifeBytesTargetStop uint64 = 125500
	lifeBytesFastMargin uint64 = 16000
	lifeBytesPingSize          = "1300"
	lifeBytesFastCount         = "10"
	lifeBytesMaxBatches        = 60
	lifeBytesRekeyWait         = 60 * time.Second
)

// checkChildRekeyLifeBytes proves that real bidirectional tunnel traffic increments Linux
// XFRM counters on Ze's Child SA, triggers Ze's volume-based CREATE_CHILD_SA rekey well
// before the 3600-second time lifetime expires (RFC 4301 Section 4.4.2.1), installs
// matching replacement SPIs on both Ze and strongSwan (which holds no matching byte limit,
// proving out-of-sync peer thresholds), deletes the superseded Child SA, and keeps tunnel
// traffic flowing afterward.
func checkChildRekeyLifeBytes(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	if _, err := lab.waitXFRM(ctx, zePeer); err != nil {
		return err
	}
	if _, err := lab.waitXFRM(ctx, swanPeer); err != nil {
		return err
	}
	if err := lab.checkXFRMCount(ctx, swanPeer, 2); err != nil {
		return err
	}

	beforeZe, beforeKernel, err := readbackSPIs(ctx, lab)
	if err != nil {
		return err
	}
	if err := requireSameSPISet(beforeZe, beforeKernel, "show vpn ipsec dataplane sa", "ze ip xfrm state"); err != nil {
		return err
	}
	beforeSwan, err := lab.espSPIValues(ctx, swanPeer)
	if err != nil {
		return err
	}
	if err := requireSameSPISet(beforeZe, beforeSwan, "show vpn ipsec dataplane sa", "strongSwan ip xfrm state"); err != nil {
		return err
	}

	if err := lab.verifyTunnelTraffic(ctx, "initial tunnel traffic failed before volume rekey"); err != nil {
		return err
	}

	if err := driveLifeBytesTraffic(ctx, lab, beforeZe); err != nil {
		return err
	}

	// 60s is far shorter than Ze's 3600s time lifetime, proving volume triggered the rekey.
	if err := waitRekeyAndDelete(ctx, lab, lifeBytesRekeyWait, false); err != nil {
		return err
	}

	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     xfrmWaitTimeout,
		Interval:    time.Second,
		Description: "volume-rekeyed SPIs installed on Ze and strongSwan and old SPIs retired",
	}, func(probe context.Context) (bool, error) {
		afterZe, afterKernel, readErr := readbackSPIs(probe, lab)
		if readErr != nil {
			return false, readErr
		}
		if err := requireSPISetChanged(beforeZe, afterKernel); err != nil {
			return false, err
		}
		if err := requireSPISetChanged(beforeZe, afterZe); err != nil {
			return false, err
		}
		if err := requireSameSPISet(afterZe, afterKernel, "Ze after volume rekey", "Ze kernel after volume rekey"); err != nil {
			return false, err
		}
		afterSwan, swanErr := lab.espSPIValues(probe, swanPeer)
		if swanErr != nil {
			return false, swanErr
		}
		if err := requireSameSPISet(afterZe, afterSwan, "Ze after volume rekey", "strongSwan kernel after volume rekey"); err != nil {
			return false, err
		}
		return true, nil
	}, func(ready bool) bool { return ready })
	if err != nil {
		return err
	}

	zeLogs, err := lab.logs(ctx, zePeer)
	if err != nil {
		return err
	}
	if strings.Contains(zeLogs, "hard lifetime expired") {
		return interoplab.WithSearchedOutput(
			fmt.Errorf("ze hit hard lifetime expiry instead of make-before-break soft volume rekey"),
			zeLogs,
		)
	}

	return lab.verifyTunnelTraffic(ctx, "no ESP traffic after volume-triggered Child SA rekey")
}

// driveLifeBytesTraffic sends sized ICMP echo batches across the tunnel until either a
// replacement SPI appears or the higher directional byte counter on Ze's initial Child SA
// enters the guaranteed-soft band [125500, 131072).
func driveLifeBytesTraffic(ctx context.Context, lab *scenarioLab, initialSPIs map[uint32]struct{}) error {
	startCounters, err := lab.xfrmCounters(ctx, zePeer)
	if err != nil {
		return err
	}
	currentMax, _, err := lifeBytesProgress(startCounters, initialSPIs)
	if err != nil {
		return err
	}

	for batch := range lifeBytesMaxBatches {
		if currentMax >= lifeBytesTargetStop {
			return nil
		}
		cmd := lifeBytesPingCommand(currentMax)
		if _, execErr := lab.exec(ctx, zePeer, cmd...); execErr != nil {
			return fmt.Errorf("volume traffic batch %d failed: %w", batch+1, execErr)
		}
		counters, readErr := lab.xfrmCounters(ctx, zePeer)
		if readErr != nil {
			return readErr
		}
		nextMax, rotated, parseErr := lifeBytesProgress(counters, initialSPIs)
		if parseErr != nil {
			return parseErr
		}
		if rotated {
			return nil
		}
		if batch == 0 {
			if nextMax <= currentMax {
				return fmt.Errorf("volume traffic batch 1 did not advance Ze XFRM byte counters (before=%d after=%d)",
					currentMax, nextMax)
			}
		}
		currentMax = nextMax
	}
	return fmt.Errorf("volume traffic did not reach %d bytes after %d batches (max observed=%d)",
		lifeBytesTargetStop, lifeBytesMaxBatches, currentMax)
}

// lifeBytesPingCommand picks a 10-packet fast burst while far below the soft band and
// single 1300-byte echo requests once within lifeBytesFastMargin of the stop target so
// counters never overshoot the 131072-byte hard ceiling.
func lifeBytesPingCommand(currentBytes uint64) []string {
	if currentBytes+lifeBytesFastMargin < lifeBytesTargetStop {
		return []string{cmdPing, "-c", lifeBytesFastCount, "-i", "0.01", "-s", lifeBytesPingSize, "-W", "2", swanIP}
	}
	return []string{cmdPing, "-c", "1", "-s", lifeBytesPingSize, "-W", "2", swanIP}
}

// lifeBytesProgress returns the maximum byte counter across initialSPIs and reports
// whether any fresh SPI not present in initialSPIs has already been installed.
func lifeBytesProgress(counters map[saKey]uint64, initialSPIs map[uint32]struct{}) (maxInitialBytes uint64, rotated bool, err error) {
	for key, bytes := range counters {
		spi, parseErr := strconv.ParseUint(key.spi, 0, 32)
		if parseErr != nil {
			return 0, false, fmt.Errorf("parse SPI %q: %w", key.spi, parseErr)
		}
		if _, isInitial := initialSPIs[uint32(spi)]; !isInitial {
			rotated = true
			continue
		}
		maxInitialBytes = max(maxInitialBytes, bytes)
	}
	return maxInitialBytes, rotated, nil
}
