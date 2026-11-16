// Design: docs/research/l2tpv2-ze-integration.md -- retained native PPP unit

package kernel

import (
	"errors"
	"fmt"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

func duplicateSessionDescriptor(fd int) (int, error) {
	return unix.FcntlInt(uintptr(fd), unix.F_DUPFD_CLOEXEC, 0)
}

func stampSessionBirth(identity ppp.SessionIdentity, tag string) error {
	if identity.NativeIndex == 0 || uint64(identity.NativeIndex) > uint64(^uint(0)>>1) {
		return errors.New("ppp: native birth has an invalid interface index")
	}
	index := int(identity.NativeIndex)
	link, err := netlink.LinkByIndex(index)
	if err != nil {
		return fmt.Errorf("read native PPP interface before birth stamp: %w", err)
	}
	attrs := link.Attrs()
	if attrs.Index != index || attrs.Name != identity.InterfaceName {
		return errors.New("ppp: native birth lookup does not identify the owned PPP interface")
	}
	if attrs.Alias != "" && attrs.Alias != tag {
		return errors.New("ppp: owned PPP interface already carries another birth tag")
	}
	if attrs.Alias != tag {
		if err := netlink.LinkSetAlias(link, tag); err != nil {
			return fmt.Errorf("write native PPP birth tag: %w", err)
		}
	}
	confirmed, err := netlink.LinkByIndex(index)
	if err != nil {
		return fmt.Errorf("read back native PPP birth tag: %w", err)
	}
	attrs = confirmed.Attrs()
	if attrs.Index != index || attrs.Name != identity.InterfaceName || attrs.Alias != tag {
		return errors.New("ppp: native PPP birth tag could not be confirmed")
	}
	return nil
}
