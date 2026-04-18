// Design: docs/architecture/core-design.md -- tc backend Linux implementation

//go:build linux

package trafficnetlink

import (
	"fmt"

	"github.com/vishvananda/netlink"

	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
)

// backend implements traffic.Backend using vishvananda/netlink tc API.
type backend struct{}

func newBackend() (traffic.Backend, error) {
	return &backend{}, nil
}

// Apply receives the full desired state and programs tc configuration on each
// named interface: replace root qdisc, rebuild classes and filters.
// Cleanup of interfaces removed from config is the caller's responsibility
// (the component must track previous state and call QdiscDel for removed interfaces).
func (b *backend) Apply(desired map[string]traffic.InterfaceQoS) error {
	for name, qos := range desired {
		link, err := netlink.LinkByName(name)
		if err != nil {
			return fmt.Errorf("trafficnetlink: interface %q: %w", name, err)
		}
		if err := b.applyInterface(link, &qos); err != nil {
			return fmt.Errorf("trafficnetlink: interface %q: %w", name, err)
		}
	}
	return nil
}

func (b *backend) applyInterface(link netlink.Link, qos *traffic.InterfaceQoS) error {
	linkIdx := link.Attrs().Index

	// Replace root qdisc.
	rootQdisc, err := translateQdisc(qos.Qdisc, linkIdx)
	if err != nil {
		return fmt.Errorf("translate qdisc: %w", err)
	}
	if err := netlink.QdiscReplace(rootQdisc); err != nil {
		return fmt.Errorf("qdisc replace: %w", err)
	}

	// Add classes under root qdisc (for classful qdiscs like HTB/HFSC).
	rootHandle := rootQdisc.Attrs().Handle
	for i, tc := range qos.Qdisc.Classes {
		class, err := translateClass(qos.Qdisc.Type, tc, linkIdx, rootHandle, uint32(i+1))
		if err != nil {
			return fmt.Errorf("class %q: translate: %w", tc.Name, err)
		}
		if err := netlink.ClassAdd(class); err != nil {
			return fmt.Errorf("class %q: add: %w", tc.Name, err)
		}

		// Add filters for this class.
		classHandle := class.Attrs().Handle
		for _, f := range tc.Filters {
			filter, err := translateFilter(f, linkIdx, rootHandle, classHandle)
			if err != nil {
				return fmt.Errorf("class %q filter: %w", tc.Name, err)
			}
			if err := netlink.FilterAdd(filter); err != nil {
				return fmt.Errorf("class %q filter add: %w", tc.Name, err)
			}
		}
	}

	return nil
}

// ListQdiscs returns current tc state for an interface.
func (b *backend) ListQdiscs(ifaceName string) (traffic.InterfaceQoS, error) {
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return traffic.InterfaceQoS{}, fmt.Errorf("trafficnetlink: interface %q: %w", ifaceName, err)
	}

	qdiscs, err := netlink.QdiscList(link)
	if err != nil {
		return traffic.InterfaceQoS{}, fmt.Errorf("trafficnetlink: list qdiscs: %w", err)
	}

	qos := traffic.InterfaceQoS{Interface: ifaceName}
	if len(qdiscs) > 0 {
		qos.Qdisc.Type = raiseQdiscType(qdiscs[0])
	}

	return qos, nil
}

// Close releases resources. No persistent state to clean up.
func (b *backend) Close() error {
	return nil
}
