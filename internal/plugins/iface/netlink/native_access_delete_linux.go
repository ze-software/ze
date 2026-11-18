// Design: docs/features/interfaces.md -- native children-first deletion
//go:build linux

package ifacenetlink

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/iface"
)

func (resources *nativeVLANResources) legacyAdmissionLocked() error {
	if resources.phase == nativeVLANBackendClosed {
		return os.ErrClosed
	}
	if resources.phase == nativeVLANBackendClosing {
		return iface.ErrNativeAccessPending
	}
	return nil
}

// nativeVLANDeleteSet MUST be computed and consumed under the same qosMapMu
// fence. Only native Birth-matching original receipts permit cascading delete.
// Foreign/unowned VLAN descendants, including unmarked uncertain creations,
// require explicit children-first deletion. A name/index never adopts them.
func (resources *nativeVLANResources) nativeVLANDeleteSet(target netlink.Link, links []netlink.Link) ([]*nativeVLANResource, error) {
	return resources.nativeVLANDeleteSetOriginal(target, links, nil)
}

func (resources *nativeVLANResources) nativeVLANDeleteSetOriginal(target netlink.Link, links []netlink.Link, original *nativeVLANResource) ([]*nativeVLANResource, error) {
	if len(links) > nativeVLANReplyLimit {
		return nil, errors.New("native interface graph exceeds the deletion limit; delete children explicitly first")
	}
	descendants := map[int]bool{target.Attrs().Index: true}
	children := make(map[int][]int, len(links))
	seen := make(map[int]bool, len(links))
	targetSeen := false
	for _, link := range links {
		if link == nil {
			return nil, errors.New("native interface graph contains an absent link")
		}
		attrs := link.Attrs()
		if attrs == nil {
			return nil, errors.New("native interface graph has absent attributes")
		}
		if attrs.Index <= 0 {
			return nil, errors.New("native interface graph has no positive index")
		}
		if seen[attrs.Index] {
			return nil, errors.New("native interface graph repeats an index")
		}
		seen[attrs.Index] = true
		children[attrs.ParentIndex] = append(children[attrs.ParentIndex], attrs.Index)
		if attrs.Index == target.Attrs().Index {
			if attrs.Name != target.Attrs().Name {
				return nil, errors.New("native delete target changed its name during the graph read")
			}
			if attrs.Alias != target.Attrs().Alias {
				return nil, errors.New("native delete target changed its birth during the graph read")
			}
			if link.Type() != target.Type() {
				return nil, errors.New("native delete target changed its kind during the graph read")
			}
			targetSeen = true
		}
	}
	if !targetSeen {
		return nil, errors.New("native delete target was absent from the complete interface graph")
	}
	queue := []int{target.Attrs().Index}
	// Each native index enters this bounded queue at most once, including cycles.
	for cursor := 0; cursor < len(queue); cursor++ {
		for _, index := range children[queue[cursor]] {
			if !descendants[index] {
				descendants[index] = true
				queue = append(queue, index)
			}
		}
	}
	var retire []*nativeVLANResource
	for _, link := range links {
		if !descendants[link.Attrs().Index] {
			continue
		}
		vlan, ok := link.(*netlink.Vlan)
		if !ok {
			continue
		}
		record := resources.byNameLocked(vlan.Name)
		// The explicitly requested unowned target may be deleted. Its descendants
		// are not explicitly requested and MUST each prove original ownership.
		if record == nil {
			if vlan.Index == target.Attrs().Index {
				if strings.HasPrefix(vlan.Alias, nativeVLANAliasPrefix) {
					return nil, errors.New("native VLAN belongs to another original backend; delete it through its original owner")
				}
				continue
			}
			return nil, errors.New("parent has a foreign or unowned VLAN child; delete children explicitly first")
		}
		if record.leases.Load() != 0 {
			return nil, iface.ErrNativeAccessPending
		}
		if record.identity.Index != vlan.Index {
			return nil, errors.New("native descendant names another original index; delete children explicitly first")
		}
		if original != nil {
			if record == original {
				if vlan.Index == target.Attrs().Index {
					if err := original.createdEcho(vlan); err != nil {
						return nil, err
					}
					if _, unmarked := original.state.(nativeVLANUnmarked); !unmarked {
						return nil, errors.New("original unpublished root lost its unmarked quarantine")
					}
					retire = append(retire, record)
					continue
				}
			}
		}
		birth, err := iface.ParseNativeVLANBirth(vlan.Alias)
		if err != nil {
			return nil, errors.New("native descendant has no confirmed original birth; delete children explicitly first")
		}
		if birth != record.identity.Birth {
			return nil, errors.New("native descendant names another original birth; delete children explicitly first")
		}
		retire = append(retire, record)
	}
	return retire, nil
}

func (resources *nativeVLANResources) deleteLinkLocked(ctx context.Context, link netlink.Link) error {
	return resources.deleteLinkOriginalLocked(ctx, link, nil)
}

// The original argument grants only its exact unpublished root exception. It
// never exempts children, graph bounds, receipt checks or the original ACK.
func (resources *nativeVLANResources) deleteLinkOriginalLocked(ctx context.Context, link netlink.Link, original *nativeVLANResource) error {
	if err := resources.beginLocked(ctx); err != nil {
		return err
	}
	links, err := resources.ops.list()
	if err != nil {
		return err
	}
	retire, err := resources.nativeVLANDeleteSetOriginal(link, links, original)
	if err != nil {
		return err
	}
	if err := resources.beginLocked(ctx); err != nil {
		return err
	}
	if err := resources.ops.remove(link); err != nil {
		return err
	}
	// Only a native success ACK retires the exact before-delete receipts. An
	// error/deadline above retains every original receipt for reconciliation.
	for _, record := range retire {
		if resources.records[record.identity.Birth] != record {
			return errors.New("native descendant receipt changed while deletion was fenced")
		}
		record.state = nativeVLANRetired{}
		delete(resources.records, record.identity.Birth)
	}
	return nil
}

// nativeVLANRollbackLocked is used only while a legitimate creator holds the
// same fence from before LinkAdd. It never reacquires the mutex or adopts a
// Birth, and errors preserve the caller's original creation failure.
func (resources *nativeVLANResources) nativeVLANRollbackLocked(link netlink.Link) error {
	ctx, cancel := context.WithTimeout(context.Background(), nativeVLANOperationLimit)
	defer cancel()
	if err := resources.beginLocked(ctx); err != nil {
		return err
	}
	if link.Attrs().Index == 0 {
		actual, err := resources.ops.byName(link.Attrs().Name)
		if err != nil {
			return err
		}
		link = actual
	}
	return resources.deleteLinkLocked(ctx, link)
}

// A doctor probe has no subscriber creator authority. Its cleanup MUST refuse
// any native-owned target or any VLAN descendant and share the mutation fence.
func nativeVLANProbeDelete(link netlink.Link) error {
	if !qosMapMu.TryLock() {
		return iface.ErrNativeAccessPending
	}
	defer qosMapMu.Unlock()
	links, err := netlink.LinkList()
	if err != nil {
		return err
	}
	resources := &nativeVLANResources{}
	if _, err := resources.nativeVLANDeleteSet(link, links); err != nil {
		return err
	}
	return netlink.LinkDel(link)
}
