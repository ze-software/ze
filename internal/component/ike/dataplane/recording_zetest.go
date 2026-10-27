//go:build zetest

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- dataplane backend registry
// Related: noop.go -- the other unprivileged test backend, which records nothing
// Related: register_recording_zetest.go -- its registration, in the same build only
//
// The recording backend holds the SPD writes it is given in memory and answers
// ListPolicies from them, so an unprivileged .ci can assert what the engine installed
// through `show vpn ipsec dataplane policy`: the SPD catch-all above all, whose failed
// install now fails the apply (installUnmatched, engine/unmatched.go). It is selected
// with ze.test.ike.dataplane=recording.
//
// It exists only in a zetest build, the functional-test daemon
// (internal/le/test/functional/binaries.go adds the tag), the same gate the test-only
// plugins use (cmd/ze/plugins_zetest.go). No shipped ze registers it, so
// ze.test.ike.dataplane=recording on a shipped build names an unknown backend and the
// dataplane load fails (TestRecordingBackendAbsentFromShippedBuild).

package dataplane

import (
	"cmp"
	"fmt"
	"net"
	"slices"
	"sync"
)

func newRecordingBackend() (Dataplane, error) {
	return &recordingDataplane{policies: map[recordedPolicyKey]SPParams{}}, nil
}

// recordingDataplane is safe for concurrent use: the engine installs from its apply
// path and the show command reads from the RPC goroutine.
type recordingDataplane struct {
	mu       sync.Mutex
	policies map[recordedPolicyKey]SPParams
}

// recordedPolicyKey is the identity the kernel gives a policy: its selector, its
// direction and its if_id (SPParams, Owner). An install under a key already held
// replaces the entry, as xfrmBackend.InstallPolicy upserts. The owner refusal of a
// second installer is not modeled: this backend records, it does not arbitrate.
type recordedPolicyKey struct {
	source, target string
	dir            SADir
	upperProto     uint8
	sourcePort     PortMatch
	targetPort     PortMatch
	ifID           uint32
}

func recordedKey(p *SPParams) recordedPolicyKey {
	return recordedPolicyKey{
		source:     recordedPrefix(p.Src),
		target:     recordedPrefix(p.Dst),
		dir:        p.Dir,
		upperProto: p.UpperProto,
		sourcePort: p.SrcPort,
		targetPort: p.DstPort,
		ifID:       p.IfID,
	}
}

// recordedPrefix spells a selector prefix; a nil prefix is the wildcard.
func recordedPrefix(prefix *net.IPNet) string {
	if prefix == nil {
		return ""
	}
	return prefix.String()
}

// RequiresForwardPolicy selects the generic in/out model; this fixture forwards no packets.
func (d *recordingDataplane) RequiresForwardPolicy() bool { return false }

func (d *recordingDataplane) InstallSA(SAParams) error             { return nil }
func (d *recordingDataplane) RemoveSA(uint32, net.IP, uint8) error { return nil }
func (d *recordingDataplane) Close() error                         { return nil }

// CatchAllSupported answers yes: the backend holds an entry bound to no interface.
func (d *recordingDataplane) CatchAllSupported() error { return nil }

func (d *recordingDataplane) InstallPolicy(p SPParams) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.policies[recordedKey(&p)] = p
	return nil
}

func (d *recordingDataplane) RemovePolicyParams(p SPParams) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.policies, recordedKey(&p))
	return nil
}

// RemovePolicy removes every entry with this source, destination and direction, the
// three fields the call carries.
func (d *recordingDataplane) RemovePolicy(src, dst *net.IPNet, dir SADir) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	source, target := recordedPrefix(src), recordedPrefix(dst)
	for key := range d.policies {
		if key.source != source {
			continue
		}
		if key.target != target {
			continue
		}
		if key.dir == dir {
			delete(d.policies, key)
		}
	}
	return nil
}

// ListSAs refuses, for the reason noopDataplane.ListSAs gives: the SA writes are
// accepted and not recorded, so an empty table would be a claim about state this
// backend does not hold.
func (d *recordingDataplane) ListSAs(uint32) ([]SAInfo, error) {
	return nil, fmt.Errorf("%w: the recording dataplane records SPD writes only, so it cannot enumerate the SAD", ErrNotSupported)
}

// ListPolicies answers the recorded SPD, ordered by priority and then direction, so
// the catch-all is last as the kernel searches it.
func (d *recordingDataplane) ListPolicies() ([]PolicyInfo, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]PolicyInfo, 0, len(d.policies))
	for key := range d.policies {
		p := d.policies[key]
		out = append(out, PolicyInfo{
			Src:        p.Src,
			Dst:        p.Dst,
			SrcPort:    p.SrcPort,
			DstPort:    p.DstPort,
			Dir:        p.Dir,
			UpperProto: p.UpperProto,
			Priority:   p.Priority,
			IfIndex:    p.IfIndex,
			IfID:       p.IfID,
			Action:     p.Action,
			Mode:       p.Mode,
			ReqID:      p.ReqID,
			Owner:      p.Owner,
			OwnerKnown: true,
		})
	}
	slices.SortFunc(out, func(a, b PolicyInfo) int {
		if c := cmp.Compare(a.Priority, b.Priority); c != 0 {
			return c
		}
		return cmp.Compare(a.Dir, b.Dir)
	})
	return out, nil
}
