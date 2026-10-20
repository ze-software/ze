// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: apply.go -- releaseSPD, which keeps the catch-all while this ledger is not empty
// Related: child.go -- the Child SA policy removals that report here

package engine

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"syscall"

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
// Two refusals do not leave the entry behind. ENOENT is the kernel saying it holds no
// such entry (dataplane/policy_owner.go), so the entry is gone. A PolicyOwnedError means
// another peer's Child SA owns the selector, and that peer's own removal answers for it.
func removeZePolicy(dp dataplane.Dataplane, p dataplane.SPParams) error {
	err := dp.RemovePolicyParams(p)
	spdLeft.record(&p, err)
	return err
}

func (l *spdLeftover) record(p *dataplane.SPParams, err error) {
	sel := selectorOf(p)
	l.mu.Lock()
	defer l.mu.Unlock()
	if err == nil {
		delete(l.entries, sel)
		return
	}
	if errors.Is(err, syscall.ENOENT) {
		delete(l.entries, sel)
		return
	}
	if _, owned := errors.AsType[*dataplane.PolicyOwnedError](err); owned {
		return
	}
	l.entries[sel] = *p
}

// describe names every entry still in the ledger, one string each, for the warning
// releaseSPD logs. It is empty when every removal succeeded.
func (l *spdLeftover) describe() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	names := make([]string, 0, len(l.entries))
	for sel := range l.entries {
		var b strings.Builder
		b.WriteString(dirLabel(sel.dir))
		b.WriteByte(' ')
		b.WriteString(sel.source)
		b.WriteString(" -> ")
		b.WriteString(sel.target)
		if sel.upperProto != 0 {
			b.WriteString(" proto ")
			b.WriteString(strconv.Itoa(int(sel.upperProto)))
		}
		names = append(names, b.String())
	}
	return names
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
		"Remove each with `ip xfrm policy delete src <source> dst <target> <dir>`, then the catch-all the same way, "+
		"or start ze again: its next exit removes them",
		"entries", left)
	return true
}
