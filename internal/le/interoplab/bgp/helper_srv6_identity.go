// Design: docs/architecture/testing/interop.md -- actual registered raw export RPC.
// Related: check_srv6_identity.go -- independent FRR acceptance and exact wire oracle.
package bgp

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	srv6IdentityA = "2001:db8:9252::9"
	srv6IdentityB = "2001:db8:9252::77"
	// Label-Index Reserved/Flags normalize on transmission, unlike Service Reserved.
	srv6IdentityLabel      = "01000700000000000064"
	srv6IdentityDirtyLabel = "010007a1b2c300000064"
	// L3 End.DT46 carries three nonzero Reserved fields, unknown sub-TLV
	// f0 and unknown sub-sub-TLV ee. L2 Service belongs on an EVPN carrier,
	// so generic type-6 stripping remains a code-regression assertion.
	srv6IdentityServices = "05002da50100235a20010db8000000000000000000000001000014c3010006281810000000ee00027788f00003990011"
	srv6IdentityOther    = "c80003aabbcc0300080000003e800003e8"
)

// srv6IdentityCall is a receipt from the real SDK callback, not an oracle verdict.
type srv6IdentityCall struct {
	Peer   string           `json:"peer"`
	Input  string           `json:"input"`
	Output string           `json:"output"`
	Action sdk.FilterAction `json:"action"`
}

// srv6IdentityPolicyReceipt is a snapshot of all callback attempts. Attempts
// saturates at five (five or more); Calls holds three announcements and one
// exact IPv6 EOR. Failure and its first input are retained with fixed bounds.
type srv6IdentityPolicyReceipt struct {
	Attempts uint8                `json:"attempts"`
	Failure  string               `json:"failure"`
	Calls    []srv6IdentityCall   `json:"calls"`
	Rejected srv6IdentityRejected `json:"rejected"`
}

// srv6IdentityRejected retains the first failure's callback input, not a guessed
// interpretation. Input holds at most 4096 bytes encoded as hex; Octets records
// the original length. The empty value means no rejected callback.
type srv6IdentityRejected struct {
	Peer      string `json:"peer"`
	Direction string `json:"direction"`
	Filter    string `json:"filter"`
	Input     string `json:"input"`
	Octets    int    `json:"octets"`
}

const srv6IdentityEOR = "\x00\x00\x00\x06\x80\x0f\x03\x00\x02\x01"

// srv6IdentityPolicyLedger is safe for concurrent SDK filter/command callbacks.
// Its zero is ready for use. Snapshots own their slice, so RPC serialization
// cannot observe subsequent mutation. A failed attempt is never forgotten.
type srv6IdentityPolicyLedger struct {
	mutex   sync.Mutex
	receipt srv6IdentityPolicyReceipt
}

func (ledger *srv6IdentityPolicyLedger) filter(input *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
	ledger.mutex.Lock()
	defer ledger.mutex.Unlock()
	if ledger.receipt.Attempts < 5 {
		ledger.receipt.Attempts++
	}
	if ledger.receipt.Attempts == 5 {
		err := errors.New("SRv6 export callback exceeded three announcements and one IPv6 EOR")
		ledger.recordFailure(input, err)
		return nil, err
	}
	output, err := srv6IdentityPolicyOutput(input)
	if err != nil {
		ledger.recordFailure(input, err)
		return nil, err
	}
	action := sdk.FilterModify
	if string(input.Raw) == srv6IdentityEOR {
		if ledger.receipt.Attempts != 4 {
			err := errors.New("IPv6 EOR arrived before the three announcement callbacks")
			ledger.recordFailure(input, err)
			return nil, err
		}
		action = sdk.FilterAccept
	} else if ledger.receipt.Attempts == 4 {
		err := errors.New("fourth callback is not the exact source IPv6 EOR")
		ledger.recordFailure(input, err)
		return nil, err
	}
	ledger.receipt.Calls = append(ledger.receipt.Calls, srv6IdentityCall{
		Peer: input.Peer, Input: hex.EncodeToString(input.Raw),
		Output: hex.EncodeToString(output), Action: action,
	})
	if action == sdk.FilterAccept {
		return &sdk.FilterUpdateOutput{Action: action}, nil
	}
	return &sdk.FilterUpdateOutput{Action: action, Raw: output}, nil
}

// recordFailure MUST be called with the ledger mutex held.
func (ledger *srv6IdentityPolicyLedger) recordFailure(input *sdk.FilterUpdateInput, err error) {
	if ledger.receipt.Failure == "" {
		message := err.Error()
		ledger.receipt.Failure = message[:min(len(message), 512)]
		ledger.receipt.Rejected = srv6IdentityRejected{
			Peer:      input.Peer[:min(len(input.Peer), 256)],
			Direction: input.Direction[:min(len(input.Direction), 128)],
			Filter:    input.Filter[:min(len(input.Filter), 128)],
			Input:     hex.EncodeToString(input.Raw[:min(len(input.Raw), 4096)]),
			Octets:    len(input.Raw),
		}
	}
}

func (ledger *srv6IdentityPolicyLedger) snapshot() srv6IdentityPolicyReceipt {
	ledger.mutex.Lock()
	defer ledger.mutex.Unlock()
	return srv6IdentityPolicyReceipt{
		Attempts: ledger.receipt.Attempts,
		Failure:  ledger.receipt.Failure,
		Calls:    append([]srv6IdentityCall(nil), ledger.receipt.Calls...),
		Rejected: ledger.receipt.Rejected,
	}
}

// runSRv6IdentityPolicy lives until the daemon closes its SDK transport. The
// checker owns teardown, after both independent recipients pass their fences.
func runSRv6IdentityPolicy(name string) error {
	registration := sdk.Registration{
		Filters: []sdk.FilterDecl{{Name: "change-hop", Direction: sdk.FilterExport, Raw: true, OnError: sdk.OnErrorReject}},
		Commands: []rpc.CommandDecl{
			{Name: srv6IdentityRelease, ShortHelp: "Release the received SRv6 fixture source"},
			{Name: srv6IdentityState, ShortHelp: "Read actual SRv6 export callback receipts"},
		},
	}
	var runErr error
	code := sdk.RunOrDeclare(registration, func() int {
		plugin, err := sdk.NewFromEnv(name)
		if err != nil {
			runErr = err
			return 1
		}
		defer plugin.Close() //nolint:errcheck // Run reports SDK transport failure.
		var ledger srv6IdentityPolicyLedger
		plugin.OnFilterUpdate(ledger.filter)
		plugin.OnExecuteCommand(func(_ string, command string, args []string, _ string) (string, any, error) {
			if command == srv6IdentityRelease {
				// This IPv4 readiness route goes ONLY to the injector. The three
				// subject/control IPv6 routes can originate only on its real socket.
				return releaseAIGPSource(plugin, args)
			}
			if command != srv6IdentityState {
				return rpc.StatusError, nil, errors.New("unexpected SRv6 fixture command")
			}
			return rpc.StatusDone, ledger.snapshot(), nil
		})
		runErr = plugin.Run(context.Background(), registration)
		if runErr != nil {
			return 1
		}
		return 0
	})
	if code != 0 && runErr == nil {
		return errors.New("SRv6 policy declaration failed")
	}
	return runErr
}

func srv6IdentityPolicyOutput(input *sdk.FilterUpdateInput) ([]byte, error) {
	if input.Direction != "export" {
		return nil, fmt.Errorf("unexpected filter direction %q", input.Direction)
	}
	if input.Filter != "change-hop" {
		return nil, fmt.Errorf("unexpected filter %q", input.Filter)
	}
	// The source's explicit MP_UNREACH EOR traverses the same received cached
	// forwarding/export path as its three announcements. No other withdrawal,
	// empty UPDATE, family, framing or extra attribute is admitted here.
	if string(input.Raw) == srv6IdentityEOR {
		return input.Raw, nil
	}
	update, err := message.UnpackUpdate(input.Raw)
	if err != nil {
		return nil, err
	}
	if len(update.WithdrawnRoutes) != 0 {
		return nil, errors.New("policy received legacy withdrawals")
	}
	if len(update.NLRI) != 0 {
		return nil, errors.New("policy received legacy announcements")
	}
	offset, flags, reach, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
	if !found {
		return nil, errors.New("policy received no MP_REACH")
	}
	if byte(flags) != 0x80 {
		return nil, fmt.Errorf("MP_REACH flags %x", flags)
	}
	if len(reach) != 30 {
		return nil, fmt.Errorf("MP_REACH length %d, want 30", len(reach))
	}
	if !bytes.Equal(reach[:4], []byte{0, 2, 1, 16}) {
		return nil, errors.New("policy requires IPv6 unicast with a native 16-octet next hop")
	}
	a := netip.MustParseAddr(srv6IdentityA).As16()
	if !bytes.Equal(reach[4:20], a[:]) {
		return nil, fmt.Errorf("policy input next hop %x, want A", reach[4:20])
	}
	if !bytes.Equal(reach[20:29], []byte{0, 64, 0x20, 1, 0x0d, 0xb8, 0x92, 0x52, 0}) {
		return nil, fmt.Errorf("unexpected source NLRI %x", reach[20:])
	}
	if reach[29] < 1 {
		return nil, errors.New("unexpected source prefix number zero")
	}
	if reach[29] > 3 {
		return nil, errors.New("unexpected source prefix number above three")
	}
	// The helper changes exactly sixteen address octets. It does NOT remove
	// attributes, normalize fields, or invoke any production forwarding helper.
	output := bytes.Clone(input.Raw)
	b := netip.MustParseAddr(srv6IdentityB).As16()
	copy(output[4+offset+3+4:4+offset+3+20], b[:])
	return output, nil
}
