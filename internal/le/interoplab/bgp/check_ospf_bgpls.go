// Design: docs/architecture/testing/interop.md -- native OSPF export at a BGP collector.
// Related: check_special.go -- existing OSPF TE scenarios retain their FRR assertions.
// RFC: rfc/short/rfc9552.md -- Sections 5.2.1 and 5.3.2.1.
package bgp

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/le/interoplab"
)

func checkOSPFTEBGPLS(ctx context.Context, check *interoplab.CheckContext) error {
	return checkOSPFRouterIDCollector(ctx, check, false)
}

func checkOSPFInterASBGPLS(ctx context.Context, check *interoplab.CheckContext) error {
	return checkOSPFRouterIDCollector(ctx, check, true)
}

func checkOSPFRouterIDCollector(ctx context.Context, check *interoplab.CheckContext, interAS bool) (resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = fmt.Errorf("%w%s", resultErr, ospfBGPLSFailureDiagnostics(ctx, check.Lab))
		}
	}()
	if !check.Network.IPv4.IsValid() {
		return errors.New("OSPF BGP-LS scenario has no selected IPv4 network")
	}
	if err := waitContains(ctx, check.Lab, peerFRR, []string{cmdVtysh, "-c", frrShowOSPFNeighbor}, 90*time.Second, ospfStateFull); err != nil {
		return err
	}
	// Preserve the ordinary TED and foreign LSDB assertions. Inter-AS evidence
	// binds a decoded link to its source LSA and the same LSA received by FRR.
	if interAS {
		if err := checkOSPFInterASDatabase(ctx, check); err != nil {
			return err
		}
	} else {
		operations := [2]operation{
			{kind: opRequireContains, peer: "ze", command: zeCommand("show ospf te-database"), contains: []string{frrLabAddress}},
			{kind: opRequireContains, peer: peerFRR, command: []string{cmdVtysh, "-c", frrShowOSPFDatabaseOpaqueArea}, contains: []string{zeLabAddress}},
		}
		for index := range operations {
			if err := runOperation(ctx, check.Network, check.Lab, &operations[index]); err != nil {
				return err
			}
		}
	}
	logs, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 120 * time.Second, Interval: time.Second,
		Description: "OSPF BGP-LS collector completed capture",
	}, func(probeCtx context.Context) (string, error) {
		result, err := check.Lab.Logs(probeCtx, peerSpeaker, 1000)
		if err != nil {
			return "", err
		}
		if !result.Available {
			return "", errors.New("BGP-LS collector logs unavailable")
		}
		return result.Text, nil
	}, func(output string) bool { return strings.Contains(output, "result:") })
	if err != nil {
		return err
	}
	return ospfBGPLSRouterIDVerdict(logs, networkHostAddress(check.Network, 2), networkHostAddress(check.Network, 3), interAS)
}

// Read the failing native adjacency before the lab tears its containers down.
// Diagnostic errors remain evidence beside the original failure, never a pass.
func ospfBGPLSFailureDiagnostics(ctx context.Context, lab interoplab.CheckerLab) string {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var output strings.Builder
	query := func(peer string, command []string) {
		answer, err := lab.Query(ctx, peer, command, queryEnvironment(peer, command))
		fmt.Fprintf(&output, "\n--- %s: %s ---\n%s\nquery error: %v\n", peer, strings.Join(command, " "), answer, err)
	}
	for _, command := range []string{"show ospf neighbor detail", "show ospf interface detail"} {
		query("ze", zeCommand(command))
	}
	command := zeCommand("show metrics values")
	answer, err := lab.Query(ctx, "ze", command, queryEnvironment("ze", command))
	fmt.Fprintln(&output, "\n--- ze: OSPF packet/drop and NSM counters ---")
	if err != nil {
		fmt.Fprintf(&output, "query error: %v\n", err)
	} else {
		var envelope struct {
			Metrics string `json:"metrics"`
		}
		if err := json.Unmarshal([]byte(answer), &envelope); err != nil {
			fmt.Fprintf(&output, "decode metrics error: %v\nraw answer: %s\n", err, answer)
		} else {
			for line := range strings.SplitSeq(envelope.Metrics, "\n") {
				if strings.HasPrefix(line, "ze_ospf_packets_") || strings.HasPrefix(line, "ze_ospf_nsm_events_total{") {
					fmt.Fprintln(&output, line)
				}
			}
		}
	}
	for _, command := range []string{"show ip ospf neighbor detail", "show ip ospf interface eth0"} {
		query(peerFRR, []string{cmdVtysh, "-c", command})
	}
	for _, peer := range []string{"ze", peerFRR} {
		query(peer, []string{"ip", "-j", ipObjectLink, ipActionShow, ipOptionDevice, containerInterface})
	}
	query(peerFRR, []string{cmdCat, frrLogPath})
	return output.String()
}

type ospfBGPLSDescriptor struct {
	RouterID string `json:"router-id"`
	DR       string `json:"designated-router-id"`
	ASN      uint32 `json:"autonomous-system"`
}

type ospfBGPLSRoute struct {
	Kind     string                `json:"ls-nlri-type"`
	Protocol int                   `json:"protocol-id"`
	Node     []ospfBGPLSDescriptor `json:"node-descriptors"`
	Local    []ospfBGPLSDescriptor `json:"local-node-descriptors"`
	Remote   []ospfBGPLSDescriptor `json:"remote-node-descriptors"`
	IDs      []string
}

// RFC 9552 Section 5.2.1: "When configured, these auxiliary TE Router-IDs
// (TLV 1028/1029) MUST be included in the node attribute described in Section
// 5.3.1 and MAY be included in the link attribute described in Section 5.3.2."
// Section 5.3.2.1: "All auxiliary Router-IDs of both the local and the remote
// node MUST be included in the link attribute of each Link NLRI."
// Judge the final received inventory, not one matching historical advertisement.
func ospfBGPLSRouterIDVerdict(logs, local, remote string, interAS bool) error {
	if result, ok := logField(logs, "result"); !ok || result != speakerResultPass {
		return errors.New("BGP-LS collector did not complete successfully")
	}
	if established, ok := logField(logs, fieldEstablished); !ok || established != logValueYes {
		return errors.New("BGP-LS collector did not establish")
	}
	if complete, ok := logField(logs, "capture-complete"); !ok || complete != logValueYes {
		return errors.New("BGP-LS collector stopped before completing its capture")
	}
	if strings.Contains(logs, "received NOTIFICATION:") || strings.Contains(logs, "send failed, peer closed") {
		return errors.New("BGP-LS collector session terminated during capture")
	}
	inventory := make(map[string]ospfBGPLSRoute)
	for _, text := range speakerUpdateHexes(logs) {
		body, err := hex.DecodeString(text)
		if err != nil {
			return err
		}
		if err := ospfBGPLSApplyUpdate(inventory, body); err != nil {
			return err
		}
	}
	localID := ospfBGPLSWantID(1028, local)
	want := map[string][]string{"node/" + local + "/": {localID}}
	if interAS {
		want["link/"+local+"/203.0.113.9"] = []string{localID, ospfBGPLSWantID(1030, "203.0.113.9"), ospfBGPLSWantID(1031, "2001:db8::9")}
	} else {
		want["node/"+remote+"/"] = []string{ospfBGPLSWantID(1028, remote)}
		want["link/"+local+"/"+remote] = []string{localID, ospfBGPLSWantID(1030, remote)}
		want["link/"+remote+"/"+local] = []string{ospfBGPLSWantID(1028, remote), ospfBGPLSWantID(1030, local)}
	}
	seen := make(map[string]bool)
	for _, route := range inventory {
		if route.Protocol != int(ls.ProtoOSPFv2) {
			continue
		}
		localDesc := route.Local
		if route.Kind == "node" {
			localDesc = route.Node
		}
		localID, _ := ospfBGPLSDescriptorID(localDesc)
		remoteID, remoteAS := ospfBGPLSDescriptorID(route.Remote)
		key := route.Kind + "/" + localID + "/" + remoteID
		expected, ok := want[key]
		if !ok {
			continue
		}
		if interAS && route.Kind == "link" && remoteAS != 65001 {
			return fmt.Errorf("BGP-LS inter-AS remote descriptor has ASN %d, want 65001", remoteAS)
		}
		if !slices.Equal(route.IDs, expected) {
			return fmt.Errorf("BGP-LS %s router-ID TLVs %v, want exactly %v", key, route.IDs, expected)
		}
		seen[key] = true
	}
	for key := range want {
		if !seen[key] {
			return fmt.Errorf("BGP-LS collector lacks final %s", key)
		}
	}
	return nil
}

func ospfBGPLSWantID(code uint16, address string) string {
	return fmt.Sprintf("%d=%x", code, netip.MustParseAddr(address).AsSlice())
}

func ospfBGPLSDescriptorID(descriptors []ospfBGPLSDescriptor) (string, uint32) {
	var router string
	var asn uint32
	for _, descriptor := range descriptors {
		if descriptor.DR != "" {
			return "", 0
		}
		if descriptor.RouterID != "" {
			router = descriptor.RouterID
		}
		if descriptor.ASN != 0 {
			asn = descriptor.ASN
		}
	}
	return router, asn
}

func ospfBGPLSApplyUpdate(inventory map[string]ospfBGPLSRoute, body []byte) error {
	update, err := message.UnpackUpdate(body)
	if err != nil {
		return err
	}
	var reach, unreach, attributes []byte
	iterator := attribute.NewAttrIterator(update.PathAttributes)
	for {
		code, flags, value, ok := iterator.Next()
		if !ok {
			break
		}
		//exhaustive:ignore // This collector extracts MP reachability and BGP-LS attributes, not other path attributes.
		switch code {
		case 14:
			if len(value) < 5 || 5+int(value[3]) > len(value) {
				return errors.New("truncated BGP-LS MP_REACH")
			}
			if binary.BigEndian.Uint16(value) == uint16(ls.AFIBGPLS) && value[2] == byte(ls.SAFIBGPLinkState) {
				reach = value[5+int(value[3]):]
			}
		case 15:
			if len(value) < 3 {
				return errors.New("truncated BGP-LS MP_UNREACH")
			}
			if binary.BigEndian.Uint16(value) == uint16(ls.AFIBGPLS) && value[2] == byte(ls.SAFIBGPLinkState) {
				unreach = value[3:]
			}
		case 29:
			if flags & ^attribute.FlagExtLength != attribute.FlagOptional {
				return fmt.Errorf("BGP-LS Attribute flags %02x are not optional non-transitive", flags)
			}
			attributes = value
		}
	}
	if iterator.Remaining() != 0 {
		return errors.New("truncated collector path attributes")
	}
	if _, err := nlrisplit.SplitBGPLS(unreach, false, func(nlri []byte) bool {
		delete(inventory, string(nlri))
		return true
	}); err != nil {
		return err
	}
	if len(reach) == 0 {
		return nil
	}
	ids, err := ospfBGPLSRouterIDTLVs(attributes)
	if err != nil {
		return err
	}
	decoder := registry.Lookup("bgp-nlri-ls")
	if decoder == nil || decoder.InProcessDecoder == nil {
		return errors.New("native BGP-LS decoder unavailable")
	}
	var decodeErr error
	_, err = nlrisplit.SplitBGPLS(reach, false, func(nlri []byte) bool {
		if decodeErr != nil {
			return true
		}
		var input, output bytes.Buffer
		fmt.Fprintf(&input, "decode nlri bgp-ls/bgp-ls %x\n", nlri)
		if decoder.InProcessDecoder(&input, &output) != 0 {
			decodeErr = errors.New("native BGP-LS NLRI decode failed")
			return true
		}
		text, ok := strings.CutPrefix(output.String(), "decoded json ")
		if !ok {
			decodeErr = fmt.Errorf("BGP-LS NLRI not decoded: %s", output.String())
			return true
		}
		var route ospfBGPLSRoute
		if decodeErr = json.Unmarshal([]byte(text), &route); decodeErr != nil {
			return true
		}
		if route.Kind == "" {
			decodeErr = errors.New("BGP-LS NLRI lacks decoded kind")
			return true
		}
		route.Kind = strings.TrimPrefix(route.Kind, "bgpls-")
		route.IDs = ids
		inventory[string(nlri)] = route
		return true
	})
	if err != nil {
		return err
	}
	return decodeErr
}

// RFC 9552 Section 5.3: attribute value is Type(2), Length(2), Value(Length).
// Preserve each raw router-ID TLV, including duplicates; JSON's convenient
// address arrays cannot distinguish a malformed extra TLV that decoding drops.
func ospfBGPLSRouterIDTLVs(data []byte) ([]string, error) {
	var ids []string
	for len(data) != 0 {
		if len(data) < 4 {
			return nil, errors.New("truncated BGP-LS TLV header")
		}
		code, length := binary.BigEndian.Uint16(data), int(binary.BigEndian.Uint16(data[2:]))
		if length > len(data)-4 {
			return nil, errors.New("truncated BGP-LS TLV value")
		}
		if code >= 1028 && code <= 1031 {
			ids = append(ids, fmt.Sprintf("%d=%x", code, data[4:4+length]))
		}
		data = data[4+length:]
	}
	slices.Sort(ids)
	return ids, nil
}
