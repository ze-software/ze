//go:build ze_bgp

// Design: docs/architecture/testing/interop.md -- deliberately unsafe SDK policy answers.
// Related: check_atomic_policy.go -- the independent FRR delivery oracle.
// RFC 4271 Sections 4.3 and 5.1.6 -- see rfc/short/rfc4271.md.
package bgp

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const atomicPolicyExportDirection = "export"

// runAtomicPolicy declares without opening a transport in query mode. Its
// runtime lasts until the owning daemon closes the SDK connection.
func runAtomicPolicy(name string) error {
	registration := sdk.Registration{
		Filters: []sdk.FilterDecl{{Name: "rewrite", Direction: sdk.FilterExport,
			Reads: []string{sdk.FilterFieldRaw}, OnError: sdk.OnErrorReject}},
		Commands: []rpc.CommandDecl{{Name: atomicPolicyRelease,
			ShortHelp: "Release one atomic-policy source phase"}},
		Sends: []rpc.MessageSendDecl{{Message: "bgp:update"}},
	}
	var runErr error
	code := sdk.RunOrDeclare(&registration, func() int {
		plugin, err := sdk.NewFromEnv(name)
		if err != nil {
			runErr = err
			return 1
		}
		defer plugin.Close() //nolint:errcheck // Run reports SDK transport failure.
		plugin.OnFilter(func(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
			input, err := route.Input()
			if err != nil {
				return nil, err
			}
			return atomicPolicyAnswer(&input)
		})
		plugin.OnExecuteCommand(func(_ string, command string, args []string, _ string, _ []sdk.PipeOperation) (string, any, error) {
			if command != atomicPolicyRelease {
				return rpc.StatusError, nil, atomicPolicyFixtureError("The atomic-policy fixture received an unknown command.")
			}
			return releaseAtomicPolicySource(plugin, args)
		})
		runErr = plugin.Run(context.Background(), &registration)
		if runErr != nil {
			return 1
		}
		return 0
	})
	if code != 0 {
		if runErr == nil {
			return atomicPolicyFixtureError("The atomic-policy fixture declaration failed.")
		}
	}
	return runErr
}

func atomicPolicyFixtureError(problem string) error {
	return opererr.New(problem, "The atomic-policy interoperability proof cannot finish.", atomicPolicyRetry)
}

// releaseAtomicPolicySource sends a readiness token only to the source socket.
// The source owns every route FRR judges; this command originates none of them.
func releaseAtomicPolicySource(plugin *sdk.Plugin, args []string) (string, any, error) {
	if len(args) != 6 {
		return rpc.StatusError, nil, atomicPolicyFixtureError("The source release requires source IP next-hop IP phase baseline|mutation.")
	}
	if args[0] != "source" {
		return rpc.StatusError, nil, atomicPolicyFixtureError("The source release is missing the source keyword.")
	}
	if args[2] != "next-hop" {
		return rpc.StatusError, nil, atomicPolicyFixtureError("The source release is missing the next-hop keyword.")
	}
	if args[4] != "phase" {
		return rpc.StatusError, nil, atomicPolicyFixtureError("The source release is missing the phase keyword.")
	}
	source, err := netip.ParseAddr(args[1])
	if err != nil {
		return rpc.StatusError, nil, atomicPolicyFixtureError("The source release has an invalid source IP.")
	}
	if !source.Is4() {
		return rpc.StatusError, nil, atomicPolicyFixtureError("The source release requires an IPv4 source.")
	}
	nextHop, err := netip.ParseAddr(args[3])
	if err != nil {
		return rpc.StatusError, nil, atomicPolicyFixtureError("The source release has an invalid next-hop IP.")
	}
	if !nextHop.Is4() {
		return rpc.StatusError, nil, atomicPolicyFixtureError("The source release requires an IPv4 next hop.")
	}
	var ordinal byte
	switch args[5] {
	case "baseline":
		ordinal = 1
	case "mutation":
		ordinal = 2
	default:
		return rpc.StatusError, nil, atomicPolicyFixtureError("The source release phase must be baseline or mutation.")
	}
	prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 255, 233, ordinal}), 32)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	announced, withdrawn, err := plugin.UpdateRoute(ctx, source.String(),
		"update text origin igp nhop "+nextHop.String()+" nlri add "+prefix.String())
	if err != nil {
		return rpc.StatusError, nil, err
	}
	if announced != 1 {
		return rpc.StatusError, nil, atomicPolicyFixtureError("The source release did not announce one readiness token.")
	}
	if withdrawn != 0 {
		return rpc.StatusError, nil, atomicPolicyFixtureError("The source release unexpectedly withdrew a route.")
	}
	return rpc.StatusDone, map[string]uint32{"announced": announced, "withdrawn": withdrawn}, nil
}

// atomicPolicyAnswer uses MED only to select a fixed fixture operation. The
// copied result is owned by the SDK. No shared state or source alias survives.
// Unsafe answers deliberately challenge RFC 4271 Section 5.1.6; the engine,
// rather than this fixture, must refuse them. UPDATE body layout (Section4.3):
// [0:2] withdrawn length | withdrawals | attribute length(2) | attributes | NLRI.
func atomicPolicyAnswer(input *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
	if input.Direction != atomicPolicyExportDirection {
		return nil, errors.New("atomic policy callback is not export")
	}
	if input.Filter != "rewrite" {
		return nil, errors.New("atomic policy callback has an unknown filter")
	}
	if len(input.Raw) > 4096 {
		return nil, errors.New("atomic policy fixture body exceeds 4096 bytes")
	}
	// RFC 4271 Section 4.3.
	sections, err := wire.ParseUpdateSections(input.Raw)
	if err != nil {
		return nil, err
	}
	nlri := sections.NLRI(input.Raw)
	if len(nlri) == 0 {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
	}
	if len(sections.Withdrawn(input.Raw)) != 0 {
		return nil, errors.New("atomic policy fixture unexpectedly combines withdrawals and announcements")
	}
	attrs := sections.Attrs(input.Raw)
	attrsStart := len(input.Raw) - len(nlri) - len(attrs)
	iterator := attribute.NewAttrIterator(attrs)
	medOffset, markerStart, markerEnd := -1, -1, -1
	for {
		start := iterator.Offset()
		code, _, value, ok := iterator.Next()
		if !ok {
			break
		}
		if code == attribute.AttrMED {
			if len(value) != 4 {
				return nil, errors.New("atomic policy fixture MED length is not four")
			}
			medOffset = attrsStart + iterator.Offset() - len(value)
		}
		if code == attribute.AttrAtomicAggregate {
			if len(value) != 0 {
				return nil, errors.New("atomic policy fixture marker length is not zero")
			}
			markerStart, markerEnd = attrsStart+start, attrsStart+iterator.Offset()
		}
	}
	if iterator.Remaining() != 0 {
		return nil, errors.New("atomic policy fixture attributes are truncated")
	}
	if medOffset < 0 {
		return nil, errors.New("atomic policy fixture announcement has no MED phase")
	}
	metric := binary.BigEndian.Uint32(input.Raw[medOffset:])
	output := bytes.Clone(input.Raw)
	binary.BigEndian.PutUint32(output[medOffset:], 77)
	// This fixture selects five deliberate cases, not the protocol MED domain.
	switch metric {
	case 10: // Useful attribute edit, including marked originals and the fence.
	case 20, 40: // A is marked; independent C is unmarked.
		if len(nlri) != 4 {
			return nil, errors.New("atomic policy readdressing fixture is not one /24")
		}
		if nlri[0] != 24 {
			return nil, errors.New("atomic policy readdressing fixture prefix length is not 24")
		}
		output[len(output)-4] = 25
		output = append(output, 0)
	case 30: // Strip the marker but retain B's exact original identity.
		if markerStart < 0 {
			return nil, errors.New("atomic policy marker-strip fixture is unmarked")
		}
		copy(output[markerStart:], output[markerEnd:])
		output = output[:len(output)-(markerEnd-markerStart)]
		binary.BigEndian.PutUint16(output[2:4], uint16(len(attrs)-(markerEnd-markerStart)))
	case 60: // Useful exact subset E of marked D+E; source ORIGIN also changed.
		// Recovery elects one original candidate at a time. Keep the same
		// subset decision for that candidate rather than failing the callback.
		switch {
		case bytes.Equal(nlri, []byte{24, 10, 42, 4, 24, 10, 42, 5}):
			start := len(output) - len(nlri)
			copy(output[start:], nlri[4:])
			output = output[:start+4]
		case bytes.Equal(nlri, []byte{24, 10, 42, 4}):
			return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}, nil
		case bytes.Equal(nlri, []byte{24, 10, 42, 5}):
			// E is already the complete retained subset.
		default:
			return nil, errors.New("atomic policy subset fixture has unknown original destinations")
		}
		binary.BigEndian.PutUint32(output[medOffset:], 88)
	default:
		return nil, fmt.Errorf("atomic policy fixture has unknown MED phase %d", metric)
	}
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Raw: output}, nil
}
