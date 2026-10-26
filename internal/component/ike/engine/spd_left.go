// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: apply.go -- releaseSPD, which keeps the catch-all while this ledger is not empty
// Related: child.go -- the Child SA policy removals that report here

package engine

import (
	"errors"
	"log/slog"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// spdSelector is the identity of one SPD entry: the fields a backend matches a removal
// on. Two entries with the same selector are one entry in the database.
type spdSelector struct {
	source, target   string
	dir              dataplane.SADir
	upperProto       uint8
	srcPort, dstPort dataplane.PortMatch
	ifID             uint32
}

func selectorOf(p *dataplane.SPParams) spdSelector {
	var sel spdSelector
	if p.Src != nil {
		sel.source = p.Src.String()
	}
	if p.Dst != nil {
		sel.target = p.Dst.String()
	}
	sel.dir = p.Dir
	sel.upperProto = p.UpperProto
	sel.srcPort = p.SrcPort
	sel.dstPort = p.DstPort
	sel.ifID = p.IfID
	return sel
}

// spdLeftover is the ledger of Ze SPD entries a removal failed to take out of the
// dataplane. releaseSPD (apply.go) reads it before it removes the catch-all: an entry
// still installed with no catch-all beneath it is the window RFC 4301 Section 4.4.1's
// final entry exists to close, so a non-empty ledger keeps the catch-all in place.
//
// It is process-wide because the SPD is: a Child SA policy that failed to leave during
// a rekey is still in the database at exit, whichever session removed it. A later
// successful removal of the same selector takes the entry back out of the ledger.
//
// Safe for concurrent use: Child SA teardown runs on each session's goroutine.
type spdLeftover struct {
	mu      sync.Mutex
	entries map[spdSelector]dataplane.SPParams
}

// spdLeft is the engine's one ledger.
var spdLeft = &spdLeftover{entries: map[spdSelector]dataplane.SPParams{}}

// removeZePolicy removes one SPD entry Ze installed and records the outcome in the
// ledger. It returns the backend's error unchanged, so each caller keeps its own log.
//
// Three refusals do not leave the entry behind. ENOENT is the kernel saying it holds no
// such entry (dataplane/policy_owner.go), so the entry is gone. A PolicyOwnedError means
// another peer's Child SA owns the selector, and that peer's own removal answers for it.
// dataplane.ErrNotSupported is the backend saying it cannot hold this entry at all, from
// a check it makes before any write reaches the dataplane and that refused the install
// the same way: the VPP backend for an entry naming no interface (the IKE bypass, which
// VPP has no node-wide SPD for) or when it created no SPD, and a platform with no XFRM.
// A kernel answering the raw errno of a platform with no XFRM (isXFRMUnsupported) is the
// same refusal, the one installChildSA kept a degraded Child SA on. The entry was never
// installed, so recording it would make warnSPDLeft name entries
// that do not exist and keep the catch-all on every exit.
//
// Why the refusal and not a record of which installs succeeded: the ledger's question
// is whether the entry is in the dataplane, and the backend's removal answer is the
// one fact about that taken at exit. An install whose error came back after the entry
// landed is still removed, and still recorded if that removal fails.
func removeZePolicy(dp dataplane.Dataplane, p dataplane.SPParams) error {
	err := dp.RemovePolicyParams(p)
	spdLeft.record(&p, err)
	return err
}

func (l *spdLeftover) record(p *dataplane.SPParams, err error) {
	sel := selectorOf(p)
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, owned := errors.AsType[*dataplane.PolicyOwnedError](err); owned {
		return
	}
	// policyGone (child.go) is the one answer to whether the entry left the dataplane,
	// so the ledger and the cross-peer record a Child SA keeps never disagree.
	if policyGone(err) {
		delete(l.entries, sel)
		return
	}
	l.entries[sel] = *p
}

// describe names every entry still in the ledger as the exact `ip xfrm policy delete`
// command that removes it, one string each and sorted, for the warning releaseSPD
// logs. It is empty when every removal succeeded.
//
// The command carries every field of spdSelector, because two entries that share
// direction and prefixes still differ by protocol, port or if_id: the IKE bypass on
// port 500 and the one on port 4500 are two entries, and a line naming neither port
// left the operator unable to tell which command removes which. Priority is not
// written: it ranks an entry but is no part of its identity, so the kernel does not
// match a removal on it and `ip xfrm policy delete` takes no priority argument.
func (l *spdLeftover) describe() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	names := make([]string, 0, len(l.entries))
	for sel := range l.entries {
		p := l.entries[sel]
		names = append(names, deleteCommand(&p))
	}
	slices.Sort(names)
	return names
}

// deleteCommand writes the `ip xfrm policy delete` command whose selector is p's.
//
// A nil prefix is written as the wildcard of the family the kernel writer gives it
// (the destination's, IPv4 when both are nil), because `ip xfrm` needs a family to
// match the entry the kernel holds. A port the selector constrains is written with
// its protocol, which `ip xfrm` requires before sport and dport.
func deleteCommand(p *dataplane.SPParams) string {
	ipv6 := p.Dst != nil && p.Dst.IP.To4() == nil
	var b strings.Builder
	b.WriteString("ip xfrm policy delete ")
	b.WriteString(dirLabel(p.Dir))
	b.WriteString(" src ")
	b.WriteString(prefixLabel(p.Src, ipv6))
	b.WriteString(" dst ")
	b.WriteString(prefixLabel(p.Dst, ipv6))
	if p.UpperProto != 0 {
		b.WriteString(" proto ")
		b.WriteString(strconv.Itoa(int(p.UpperProto)))
	}
	if !p.SrcPort.IsAny() {
		b.WriteString(" sport ")
		b.WriteString(portLabel(p.SrcPort))
	}
	if !p.DstPort.IsAny() {
		b.WriteString(" dport ")
		b.WriteString(portLabel(p.DstPort))
	}
	if p.IfID != 0 {
		b.WriteString(" if_id ")
		b.WriteString(string(strconv.AppendUint(nil, uint64(p.IfID), 10)))
	}
	return b.String()
}

// prefixLabel names a selector prefix, writing a nil one as its family's wildcard.
func prefixLabel(n *net.IPNet, ipv6 bool) string {
	if n != nil {
		return n.String()
	}
	if ipv6 {
		return "::/0"
	}
	return "0.0.0.0/0"
}

// portLabel names a port match. The XFRM backend installs only an exact port
// (xfrmSelectorPort refuses any other mask), so a partial mask can only come from
// another backend; it is written as port/mask, which `ip xfrm` refuses, rather than
// as the bare port, which would name a narrower entry than the one left behind.
func portLabel(m dataplane.PortMatch) string {
	port := strconv.Itoa(int(m.Port))
	if m.Mask == 0xffff {
		return port
	}
	return port + "/0x" + string(strconv.AppendUint(nil, uint64(m.Mask), 16))
}

// dirLabel is the `ip xfrm policy` keyword for a direction, so an operator can paste
// the warning's entries into the command that removes them.
func dirLabel(dir dataplane.SADir) string {
	switch dir {
	case dataplane.SADirIn:
		return "dir in"
	case dataplane.SADirOut:
		return "dir out"
	case dataplane.SADirFwd:
		return "dir fwd"
	}
	return "dir unknown"
}

// warnSPDLeft logs the entries the ledger holds and reports whether there were any.
// The catch-all stays installed when it returns true.
func warnSPDLeft(log *slog.Logger) bool {
	left := spdLeft.describe()
	if len(left) == 0 {
		return false
	}
	log.Warn("ike: the SPD catch-all stays installed after exit, because these Ze SPD entries could not be removed; "+
		"without the catch-all beneath them, traffic they do not name would meet no final entry. "+
		"Remove each by running the command listed for it in entries, then remove the catch-all the same way, "+
		"or start ze again: its next exit removes them",
		"entries", left)
	return true
}
