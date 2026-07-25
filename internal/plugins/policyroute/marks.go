// Design: docs/architecture/core-design.md — policy-based routing mark allocation

package policyroute

import (
	"fmt"
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	fwmarkBase = 0x50000
	fwmarkMax  = 0x5FFFF

	autoTableBase = 2000
	autoTableMax  = 2999
)

type allocator struct {
	mu sync.Mutex

	nextMark  uint32
	nextTable uint32

	markByKey map[string]uint32
	tableByNH map[netip.Addr]uint32
}

func newAllocator() *allocator {
	return &allocator{
		nextMark:  fwmarkBase,
		nextTable: autoTableBase,
		markByKey: make(map[string]uint32),
		tableByNH: make(map[netip.Addr]uint32),
	}
}

func (a *allocator) reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.nextMark = fwmarkBase
	a.nextTable = autoTableBase
	a.markByKey = make(map[string]uint32)
	a.tableByNH = make(map[netip.Addr]uint32)
}

func (a *allocator) allocateMark(key string) (uint32, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if mark, ok := a.markByKey[key]; ok {
		return mark, nil
	}

	if a.nextMark > fwmarkMax {
		return 0, fmt.Errorf("fwmark range exhausted (%d marks allocated)", len(a.markByKey))
	}

	mark := a.nextMark
	a.nextMark++
	a.markByKey[key] = mark
	return mark, nil
}

func (a *allocator) allocateTable(nh netip.Addr) (uint32, bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if tbl, ok := a.tableByNH[nh]; ok {
		return tbl, false, nil
	}

	if a.nextTable > autoTableMax {
		return 0, false, fmt.Errorf("auto table range exhausted (%d tables allocated)", len(a.tableByNH))
	}

	tbl := a.nextTable
	a.nextTable++
	a.tableByNH[nh] = tbl
	return tbl, true, nil
}

func markKey(policyName string, table uint32) string {
	return policyName + ":" + textbuf.StringUint32(table)
}

func markKeyNextHop(policyName string, nh netip.Addr) string {
	return policyName + ":nh:" + nh.String()
}
