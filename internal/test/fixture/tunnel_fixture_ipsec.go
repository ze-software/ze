package fixture

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func init() {
	Register("ipsec/child-rekey-xfrm-narrowing", tunnelIPsecRekeyNarrowing)
	Register("ipsec/child-rekey-xfrm", tunnelIPsecRekey)
	Register("ipsec/error-notify-no-loop", tunnelIPsecErrorNotify)
	Register("ipsec/peer-reload-applies-selectors", tunnelIPsecReloadSelectors)
	Register("ipsec/peer-reload-leaves-tunnel-alone", tunnelIPsecReloadQuiet)
	Register("ipsec/teardown-leaves-nothing/cycle", tunnelIPsecTeardownCycle)
	Register("ipsec/teardown-leaves-nothing/residue", tunnelIPsecTeardownResidue)
}

func tunnelIPsecCommand(ctx context.Context, args ...string) (string, error) {
	command := exec.CommandContext(ctx, args[0], args[1:]...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s failed: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(output)), err)
	}
	return string(output), nil
}

func tunnelIPsecSPIs(ctx context.Context, endpoints map[string]bool) ([]string, error) {
	output, err := tunnelIPsecCommand(ctx, "ip", "xfrm", "state")
	if err != nil {
		return nil, err
	}
	found := make(map[string]struct{})
	include := endpoints == nil
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "src" && len(fields) >= 4 {
			include = endpoints == nil || (endpoints[fields[1]] && endpoints[fields[3]])
			continue
		}
		if !include {
			continue
		}
		for index, field := range fields {
			if field == "spi" && index+1 < len(fields) {
				found[fields[index+1]] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(found))
	for spi := range found {
		result = append(result, spi)
	}
	sort.Strings(result)
	return result, nil
}

func tunnelIPsecPolicies(ctx context.Context, networks ...string) ([]string, error) {
	output, err := tunnelIPsecCommand(ctx, "ip", "xfrm", "policy")
	if err != nil {
		return nil, err
	}
	found := make(map[string]struct{})
	selector := ""
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "src" {
			selector = ""
			matches := true
			for _, network := range networks {
				present := false
				for _, field := range fields {
					present = present || field == network
				}
				matches = matches && present
			}
			if matches {
				selector = strings.Join(fields, " ")
			}
		} else if selector != "" && fields[0] == "dir" && len(fields) > 1 {
			found[selector+" | "+fields[1]] = struct{}{}
			selector = ""
		}
	}
	result := make([]string, 0, len(found))
	for policy := range found {
		result = append(result, policy)
	}
	sort.Strings(result)
	return result, nil
}

func tunnelSameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func tunnelIPsecWaitDelay(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func tunnelIPsecWait(ctx context.Context, attempts int, delay time.Duration, read func() ([]string, error), condition func([]string) bool) ([]string, error) {
	var latest []string
	var latestErr error
	ok := Poll(ctx, attempts, delay, func() bool {
		latest, latestErr = read()
		return latestErr == nil && condition(latest)
	})
	if !ok {
		if latestErr != nil {
			return nil, latestErr
		}
		return latest, errors.New("condition did not become true")
	}
	return latest, nil
}

func tunnelIPsecRekey(ctx context.Context, _ []string) error {
	readSPIs := func() ([]string, error) { return tunnelIPsecSPIs(ctx, nil) }
	readPolicies := func() ([]string, error) { return tunnelIPsecPolicies(ctx, "192.0.2.0/25", "192.0.2.128/25") }
	firstSPIs, err := tunnelIPsecWait(ctx, 400, 250*time.Millisecond, readSPIs, func(values []string) bool { return len(values) >= 2 })
	if err != nil {
		return fmt.Errorf("tunnel never reached kernel: %w", err)
	}
	firstPolicies, err := tunnelIPsecWait(ctx, 400, 250*time.Millisecond, readPolicies, func(values []string) bool { return len(values) >= 2 })
	if err != nil {
		return fmt.Errorf("tunnel policies never reached kernel: %w", err)
	}
	fmt.Println("INSTALLED:", strings.Join(firstSPIs, " "))
	fmt.Println("POLICIES:", strings.Join(firstPolicies, " | "))
	firstSet := make(map[string]bool, len(firstSPIs))
	for _, spi := range firstSPIs {
		firstSet[spi] = true
	}
	var replacement []string
	_, err = tunnelIPsecWait(ctx, 400, 250*time.Millisecond, readSPIs, func(values []string) bool {
		replacement = replacement[:0]
		for _, spi := range values {
			if !firstSet[spi] {
				replacement = append(replacement, spi)
			}
		}
		return len(replacement) >= 2
	})
	if err != nil {
		return fmt.Errorf("no replacement states arrived: %w", err)
	}
	fmt.Println("REKEYED:", strings.Join(replacement, " "))
	_, err = tunnelIPsecWait(ctx, 400, 250*time.Millisecond, readSPIs, func(values []string) bool {
		for _, spi := range values {
			if firstSet[spi] {
				return false
			}
		}
		return true
	})
	if err != nil {
		return fmt.Errorf("retired states stayed in kernel: %v", firstSPIs)
	}
	fmt.Println("RETIRED:", strings.Join(firstSPIs, " "))
	policies, err := readPolicies()
	if err != nil {
		return err
	}
	if !tunnelSameStrings(policies, firstPolicies) {
		return fmt.Errorf("POLICY-MOVED: before=%v after=%v", firstPolicies, policies)
	}
	fmt.Println("POLICY-STABLE:", strings.Join(policies, " | "))
	return nil
}

func tunnelIPsecRekeyNarrowing(ctx context.Context, _ []string) error {
	endpoints := map[string]bool{"127.0.0.1": true, "127.0.0.2": true}
	readSPIs := func() ([]string, error) { return tunnelIPsecSPIs(ctx, endpoints) }
	readPolicies := func() ([]string, error) { return tunnelIPsecPolicies(ctx, "192.0.2.0/26") }
	firstSPIs, err := tunnelIPsecWait(ctx, 400, 250*time.Millisecond, readSPIs, func(values []string) bool { return len(values) >= 2 })
	if err != nil {
		return fmt.Errorf("tunnel never reached kernel: %w", err)
	}
	firstPolicies, err := tunnelIPsecWait(ctx, 400, 250*time.Millisecond, readPolicies, func(values []string) bool { return len(values) >= 2 })
	if err != nil {
		return err
	}
	fmt.Println("INSTALLED:", strings.Join(firstSPIs, " "))
	fmt.Println("POLICIES:", strings.Join(firstPolicies, " | "))
	var changedSPIs, changedPolicies []string
	statesChanged := false
	Poll(ctx, 80, 250*time.Millisecond, func() bool {
		spis, spiErr := readSPIs()
		if spiErr == nil && !tunnelSameStrings(spis, firstSPIs) {
			changedSPIs = spis
			statesChanged = true
			return true
		}
		policies, policyErr := readPolicies()
		if policyErr == nil && !tunnelSameStrings(policies, firstPolicies) {
			changedPolicies = policies
			return true
		}
		return false
	})
	if changedPolicies != nil {
		return fmt.Errorf("POLICY-MOVED: refused rekey changed selectors: before=%v after=%v", firstPolicies, changedPolicies)
	}
	if statesChanged {
		if len(changedSPIs) > 0 {
			return fmt.Errorf("REKEY-ACCEPTED: refused narrowed rekey changed states: before=%v after=%v", firstSPIs, changedSPIs)
		}
		fmt.Println("RETIRED:", strings.Join(firstSPIs, " "))
	}
	fmt.Println("POLICY-STABLE:", strings.Join(firstPolicies, " | "))
	return nil
}

func tunnelIPsecReload() error {
	pidBytes, err := os.ReadFile("daemon.pid")
	if err != nil {
		return fmt.Errorf("daemon.pid never appeared: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		return fmt.Errorf("parse daemon.pid: %w", err)
	}
	config, err := os.ReadFile("config2.conf")
	if err != nil {
		return err
	}
	if err := os.WriteFile("ze-bgp.conf", config, 0o600); err != nil {
		return err
	}
	if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
		return err
	}
	fmt.Println("RELOADED: sent SIGHUP to pid", pid)
	return nil
}

func tunnelIPsecReloadSelectors(ctx context.Context, _ []string) error {
	readSPIs := func() ([]string, error) { return tunnelIPsecSPIs(ctx, nil) }
	wide := func() ([]string, error) { return tunnelIPsecPolicies(ctx, "198.51.100.0/25", "198.51.100.128/25") }
	narrow := func() ([]string, error) { return tunnelIPsecPolicies(ctx, "198.51.100.0/26", "198.51.100.128/25") }
	wideSPIs, err := tunnelIPsecWait(ctx, 400, 250*time.Millisecond, readSPIs, func(values []string) bool { return len(values) >= 2 })
	if err != nil {
		return err
	}
	widePolicies, err := tunnelIPsecWait(ctx, 400, 250*time.Millisecond, wide, func(values []string) bool { return len(values) >= 2 })
	if err != nil {
		return err
	}
	fmt.Println("INSTALLED-WIDE:", strings.Join(widePolicies, " | "))
	fmt.Println("INSTALLED-SPIS:", strings.Join(wideSPIs, " "))
	if _, err := tunnelIPsecWait(ctx, 200, 50*time.Millisecond, func() ([]string, error) {
		if _, statErr := os.Stat("daemon.pid"); statErr != nil {
			return nil, statErr
		}
		return []string{"ready"}, nil
	}, func(values []string) bool { return len(values) == 1 }); err != nil {
		return err
	}
	if err := tunnelIPsecReload(); err != nil {
		return err
	}
	narrowPolicies, err := tunnelIPsecWait(ctx, 400, 250*time.Millisecond, narrow, func(values []string) bool {
		wideNow, wideErr := wide()
		return wideErr == nil && len(values) >= 2 && len(wideNow) == 0
	})
	if err != nil {
		return fmt.Errorf("WIDE-SURVIVED: reload did not reach kernel: %w", err)
	}
	fmt.Println("NARROWED:", strings.Join(narrowPolicies, " | "))
	currentSPIs, err := readSPIs()
	if err != nil {
		return err
	}
	wideSet := make(map[string]bool, len(wideSPIs))
	for _, spi := range wideSPIs {
		wideSet[spi] = true
	}
	for _, spi := range currentSPIs {
		if wideSet[spi] {
			return fmt.Errorf("SPIS-SURVIVED: original state %s outlived restart", spi)
		}
	}
	fmt.Println("RESTARTED:", strings.Join(currentSPIs, " "))
	return nil
}

func tunnelIPsecReloadQuiet(ctx context.Context, _ []string) error {
	readSPIs := func() ([]string, error) { return tunnelIPsecSPIs(ctx, nil) }
	readPolicies := func() ([]string, error) { return tunnelIPsecPolicies(ctx, "203.0.113.0/25") }
	beforeSPIs, err := tunnelIPsecWait(ctx, 400, 250*time.Millisecond, readSPIs, func(values []string) bool { return len(values) >= 2 })
	if err != nil {
		return err
	}
	beforePolicies, err := tunnelIPsecWait(ctx, 400, 250*time.Millisecond, readPolicies, func(values []string) bool { return len(values) >= 2 })
	if err != nil {
		return err
	}
	fmt.Println("INSTALLED:", strings.Join(beforeSPIs, " "))
	fmt.Println("POLICIES:", strings.Join(beforePolicies, " | "))
	if _, err := tunnelIPsecWait(ctx, 200, 50*time.Millisecond, func() ([]string, error) {
		if _, statErr := os.Stat("daemon.pid"); statErr != nil {
			return nil, statErr
		}
		return []string{"ready"}, nil
	}, func(values []string) bool { return len(values) == 1 }); err != nil {
		return err
	}
	if err := tunnelIPsecReload(); err != nil {
		return err
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		spis, spiErr := readSPIs()
		if spiErr != nil {
			return spiErr
		}
		if !tunnelSameStrings(spis, beforeSPIs) {
			return fmt.Errorf("BOUNCED: ESP states moved: before=%v after=%v", beforeSPIs, spis)
		}
		policies, policyErr := readPolicies()
		if policyErr != nil {
			return policyErr
		}
		if !tunnelSameStrings(policies, beforePolicies) {
			return fmt.Errorf("BOUNCED: policies moved: before=%v after=%v", beforePolicies, policies)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	fmt.Println("STABLE:", strings.Join(beforeSPIs, " "))
	fmt.Println("POLICY-STABLE:", strings.Join(beforePolicies, " | "))
	return nil
}

func tunnelIPsecTeardownCycle(ctx context.Context, _ []string) error {
	read := func() ([]string, error) { return tunnelIPsecSPIs(ctx, nil) }
	installed, err := tunnelIPsecWait(ctx, 240, 250*time.Millisecond, read, func(values []string) bool { return len(values) >= 2 })
	if err != nil {
		return fmt.Errorf("tunnel never reached kernel: %w", err)
	}
	fmt.Println("INSTALLED:", strings.Join(installed, " "))
	installedSet := make(map[string]bool, len(installed))
	for _, spi := range installed {
		installedSet[spi] = true
	}
	_, err = tunnelIPsecWait(ctx, 240, 250*time.Millisecond, read, func(values []string) bool {
		for _, spi := range values {
			if installedSet[spi] {
				return false
			}
		}
		return true
	})
	if err != nil {
		return fmt.Errorf("operator clear left installed SA in kernel: %v", installed)
	}
	fmt.Println("TORN-DOWN:", strings.Join(installed, " "))
	return nil
}

func tunnelIPsecTeardownResidue(ctx context.Context, _ []string) error {
	statesOutput, err := tunnelIPsecCommand(ctx, "ip", "xfrm", "state")
	if err != nil {
		return err
	}
	policiesOutput, err := tunnelIPsecCommand(ctx, "ip", "xfrm", "policy")
	if err != nil {
		return err
	}
	states, policies := 0, 0
	for _, line := range strings.Split(statesOutput, "\n") {
		if strings.HasPrefix(line, "src") {
			states++
		}
	}
	for _, line := range strings.Split(policiesOutput, "\n") {
		if strings.HasPrefix(line, "src") {
			policies++
		}
	}
	rawContents, err := os.ReadFile("/proc/net/raw")
	if err != nil {
		return err
	}
	raw := make([]string, 0)
	for index, line := range strings.Split(string(rawContents), "\n") {
		if index > 0 && strings.Contains(line, ":0032 ") {
			raw = append(raw, line)
		}
	}
	if states != 0 || policies != 0 || len(raw) != 0 {
		fmt.Printf("RESIDUE: states=%d policies=%d esp-sockets=%d\n%s%s%s\n", states, policies, len(raw), statesOutput, policiesOutput, strings.Join(raw, "\n"))
		return errors.New("IPsec teardown left kernel residue")
	}
	fmt.Println("RESIDUE: none")
	return nil
}

func tunnelIPsecIKEHeader(ispi, rspi []byte, exchange, flags byte, messageID uint32) []byte {
	packet := make([]byte, 28)
	copy(packet[0:8], ispi)
	copy(packet[8:16], rspi)
	packet[17], packet[18], packet[19] = 0x20, exchange, flags
	binary.BigEndian.PutUint32(packet[20:24], messageID)
	binary.BigEndian.PutUint32(packet[24:28], 28)
	return packet
}

func tunnelIPsecErrorNotify(ctx context.Context, _ []string) error {
	port, err := strconv.Atoi(os.Getenv("ze_test_ike_port"))
	if err != nil || port < 1 {
		return errors.New("ze_test_ike_port is not set")
	}
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		return err
	}
	defer conn.Close()
	read := func(timeout time.Duration) ([]byte, error) {
		buffer := make([]byte, 4096)
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
		n, _, readErr := conn.ReadFromUDP(buffer)
		return append([]byte(nil), buffer[:n]...), readErr
	}
	ispi := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88}
	rspi := []byte{0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x01}
	request := tunnelIPsecIKEHeader(ispi, rspi, 37, 0, 7)
	var answer []byte
	for range 50 {
		if _, err := conn.Write(request); err != nil {
			return err
		}
		answer, err = read(100 * time.Millisecond)
		if err == nil {
			break
		}
		if err := tunnelIPsecWaitDelay(ctx, 100*time.Millisecond); err != nil {
			return err
		}
	}
	if err != nil {
		return errors.New("unknown IKE SA drew no answer")
	}
	if len(answer) < 36 || !bytes.Equal(answer[:8], ispi) || !bytes.Equal(answer[8:16], rspi) {
		return errors.New("INVALID_IKE_SPI answer did not copy SPIs or was too short")
	}
	if answer[16] != 41 || answer[17]>>4 != 2 || answer[18] != 37 || answer[19]&0x20 == 0 || binary.BigEndian.Uint32(answer[20:24]) != 7 {
		return errors.New("INVALID_IKE_SPI answer header fields are incorrect")
	}
	body := answer[32:]
	if len(body) != 4 || body[1] != 0 || binary.BigEndian.Uint16(body[2:4]) != 4 {
		return errors.New("Notify payload is not empty INVALID_IKE_SPI")
	}
	if _, err := conn.Write(answer); err != nil {
		return fmt.Errorf("feed INVALID_IKE_SPI answer back: %w", err)
	}
	if _, err := conn.Write(tunnelIPsecIKEHeader(ispi, rspi, 37, 0x20, 9)); err != nil {
		return fmt.Errorf("send response-marked probe: %w", err)
	}
	markerISPI := []byte{0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7}
	if _, err := conn.Write(tunnelIPsecIKEHeader(markerISPI, rspi, 37, 0, 11)); err != nil {
		return fmt.Errorf("send ordering marker: %w", err)
	}
	marker, err := read(10 * time.Second)
	if err != nil || len(marker) < 24 || !bytes.Equal(marker[:8], markerISPI) || binary.BigEndian.Uint32(marker[20:24]) != 11 {
		return errors.New("ze replied to its own output or a response-marked message")
	}
	if _, err := conn.Write(tunnelIPsecIKEHeader(ispi, make([]byte, 8), 34, 0, 0)); err != nil {
		return fmt.Errorf("send unknown IKE_SA_INIT: %w", err)
	}
	marker2ISPI := []byte{0xb0, 0xb1, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7}
	if _, err := conn.Write(tunnelIPsecIKEHeader(marker2ISPI, rspi, 37, 0, 13)); err != nil {
		return fmt.Errorf("send second ordering marker: %w", err)
	}
	marker2, err := read(10 * time.Second)
	if err != nil || len(marker2) < 8 || !bytes.Equal(marker2[:8], marker2ISPI) {
		return errors.New("IKE_SA_INIT drew an answer before ordering marker")
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	fmt.Println("probe: all out-of-SA notification assertions passed")
	return nil
}
