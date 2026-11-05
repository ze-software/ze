//go:build ze_l2tp

// Design: docs/labs/pppoe-interop.md -- omitted peer MRU cannot bypass the AC transmit cap.
package pppoe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

func init() {
	wireCheckers["pppoe-omitted-mru-ze-ac"] = checkOmittedPeerMRU
}

// checkOmittedPeerMRU keeps the native pppd lifecycle and independently
// measures Ze's PPP interface. A client-side MTU alone cannot prove Ze's cap.
func checkOmittedPeerMRU(ctx context.Context, check *interoplab.CheckContext) (err error) {
	if check == nil || check.Lab == nil {
		return errors.New("omitted-MRU checker has no lab")
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
	// pppd 2.5.5 lcp_addci omits CI_MRU when neg_mru is false. The
	// rp-pppoe plugin otherwise lowers the requested MRU to 1492 itself,
	// even with no mru argument. default-mru deliberately clears neg_mru.
	if err := pppdStart(ctx, lab, pppdPassword, pppoeService, refusePAP, []string{"default-mru", "nopersist", "noipv6"}); err != nil {
		return err
	}
	sessions, err := waitZeSession(ctx, lab)
	if err != nil {
		return err
	}
	if err := checkDiscoverySession(sessions); err != nil {
		return err
	}
	clientLink, err := waitClientPPPLink(ctx, lab, 75*time.Second)
	if err != nil {
		return err
	}
	if _, err := waitClientAddress(ctx, lab, clientLink, 75*time.Second); err != nil {
		return err
	}
	if _, err := waitRoute(ctx, lab, clientImageName, clientLink, zeGateway, 20*time.Second); err != nil {
		return err
	}
	log, err := pppdLog(ctx, lab)
	if err != nil {
		return err
	}
	if err := omittedMRULCPEvidence(log); err != nil {
		return interoplab.WithSearchedOutput(err, log)
	}
	if err := chapAuthEvidence(log); err != nil {
		return err
	}
	zeLink, err := waitPPPLink(ctx, lab, zeImageName, 20*time.Second, time.Second)
	if err != nil {
		return err
	}
	if _, err := waitAddress(ctx, lab, zeImageName, zeLink, zeGateway, firstPoolAddr, 20*time.Second, time.Second); err != nil {
		return err
	}
	link, err := query(ctx, lab, zeImageName, []string{"ip", "-j", "link", "show", "dev", zeLink})
	if err != nil {
		return err
	}
	if err := omittedMRUZeLink(link, zeLink); err != nil {
		return interoplab.WithSearchedOutput(err, link)
	}
	if err := lifecyclePing(ctx, lab, clientImageName, clientLink, zeGateway); err != nil {
		return err
	}
	if err := lifecyclePing(ctx, lab, zeImageName, zeLink, firstPoolAddr); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(os.Stdout, "PPPoE omitted-MRU peer trace:\n"+log+"\nMeasured Ze PPP interface:\n"+link); err != nil {
		return fmt.Errorf("retain omitted-MRU proof: %w", err)
	}
	if err := checkTeardown(ctx, lab); err != nil {
		return err
	}
	return waitZeKernelRelease(ctx, lab, zeLink)
}

// omittedMRULCPEvidence correlates decoded requests and Acks in both
// directions. All client requests must omit MRU, not merely one early retry.
func omittedMRULCPEvidence(trace string) error {
	if strings.Count(trace, "Connect:") != 1 {
		return errors.New("omitted-MRU proof requires exactly one pppd connection attempt")
	}
	if strings.Contains(trace, "[PAP ") {
		return errors.New("omitted-MRU CHAP session used PAP")
	}
	clientRequests := make(map[uint8]string)
	zeRequests := make(map[uint8]string)
	clientAcked, zeAcked := false, false
	for line := range strings.SplitSeq(trace, "\n") {
		line = strings.TrimSpace(line)
		var packets map[uint8]string
		var prefix string
		var request, fromClient bool
		switch {
		case strings.HasPrefix(line, "sent [LCP ConfReq "):
			packets, prefix, request, fromClient = clientRequests, "sent [LCP ConfReq ", true, true
		case strings.HasPrefix(line, "rcvd [LCP ConfAck "):
			packets, prefix = clientRequests, "rcvd [LCP ConfAck "
		case strings.HasPrefix(line, "rcvd [LCP ConfReq "):
			packets, prefix, request = zeRequests, "rcvd [LCP ConfReq ", true
		case strings.HasPrefix(line, "sent [LCP ConfAck "):
			packets, prefix, fromClient = zeRequests, "sent [LCP ConfAck ", true
		default:
			continue
		}
		id, opts, err := omittedMRUPacket(line, prefix)
		if err != nil {
			return err
		}
		if request {
			if fromClient && strings.Contains(opts, "<mru ") {
				return errors.New("pppd transmitted an MRU option; omission was not exercised")
			}
			packets[id] = opts
			continue
		}
		want, present := packets[id]
		if !present || opts != want {
			return fmt.Errorf("LCP Ack %d has no preceding request with identical options", id)
		}
		if fromClient {
			zeAcked = true
		} else {
			clientAcked = true
		}
	}
	if !clientAcked || !zeAcked {
		return errors.New("omitted-MRU request and reciprocal LCP requests were not acknowledged")
	}
	return nil
}

func omittedMRUPacket(line, prefix string) (uint8, string, error) {
	body := strings.TrimPrefix(line, prefix)
	body, closed := strings.CutSuffix(body, "]")
	if !closed {
		return 0, "", fmt.Errorf("incomplete pppd packet: %s", line)
	}
	id, options, _ := strings.Cut(body, " ")
	id, named := strings.CutPrefix(id, "id=")
	if !named {
		return 0, "", fmt.Errorf("pppd packet lacks Identifier: %s", line)
	}
	n, err := strconv.ParseUint(id, 0, 8)
	if err != nil {
		return 0, "", fmt.Errorf("decode pppd Identifier %q: %w", id, err)
	}
	return uint8(n), options, nil
}

func omittedMRUZeLink(body, name string) error {
	var links []struct {
		Name string `json:"ifname"`
		MTU  int    `json:"mtu"`
	}
	if err := json.Unmarshal([]byte(body), &links); err != nil {
		return fmt.Errorf("decode measured Ze PPP interface: %w", err)
	}
	if len(links) != 1 || links[0].Name != name || links[0].MTU != 1492 {
		return fmt.Errorf("Ze PPP interface does not install the 1492 transmit cap: %+v", links)
	}
	return nil
}
