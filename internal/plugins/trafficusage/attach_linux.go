//go:build linux

// Design: docs/architecture/traffic/traffic-usage.md -- traffic-usage Linux TCX attacher (load, attach, read maps, detach)

package trafficusage

import (
	"errors"
	"fmt"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
)

// tcxAttacher loads the pure-Go eBPF collection and attaches the ingress/egress
// programs to an interface via TCX links. Maps are kept in-process (no bpffs
// pinning); ze owns the full lifecycle.
type tcxAttacher struct{}

func newAttacher() attacher { return tcxAttacher{} }

// Available prepares this process to load eBPF programs. Removing the memlock
// rlimit is the one step needed before loading on a kernel older than 5.11;
// on a later kernel it costs a probe map, and a process without CAP_BPF is
// refused that map and then the rlimit, so the error carries the kernel's
// EPERM and the monitor classifies it as a permission refusal.
func (tcxAttacher) Available() error {
	if err := rlimit.RemoveMemlock(); err != nil {
		return fmt.Errorf("remove memlock rlimit (need CAP_BPF/CAP_SYS_RESOURCE): %w", err)
	}
	return nil
}

// Attach builds, loads, and attaches the ingress+egress programs to ifindex.
// Its error carries the kernel's answer, marked for classifyAttachError by
// markAttachError.
func (tcxAttacher) Attach(ifindex int, _ string, maxEntries uint32, trackIP bool) (attachment, error) {
	coll, err := ebpf.NewCollection(buildCollectionSpec(maxEntries, trackIP))
	if err != nil {
		return nil, markAttachError("load eBPF collection", err)
	}

	ingressLink, err := link.AttachTCX(link.TCXOptions{
		Interface: ifindex,
		Program:   coll.Programs[progIngress],
		Attach:    ebpf.AttachTCXIngress,
	})
	if err != nil {
		closeAll(coll)
		return nil, markAttachError("attach TCX ingress", err)
	}

	egressLink, err := link.AttachTCX(link.TCXOptions{
		Interface: ifindex,
		Program:   coll.Programs[progEgress],
		Attach:    ebpf.AttachTCXEgress,
	})
	if err != nil {
		closeAll(coll, ingressLink)
		return nil, markAttachError("attach TCX egress", err)
	}

	return &tcxAttachment{
		coll:    coll,
		links:   []link.Link{ingressLink, egressLink},
		trackIP: trackIP,
	}, nil
}

// markAttachError wraps a failed step's error with the marker the monitor
// classifies by. cilium/ebpf answers ErrNotSupported for a kernel without
// the TCX attach point or a feature the programs need, which is errNoTCX. A
// VerifierError is the verifier judging the program, whose EACCES is no
// permission refusal, which is errProgramRejected. Every other error keeps
// the kernel's errno for classifyAttachError to read.
func markAttachError(step string, err error) error {
	if errors.Is(err, ebpf.ErrNotSupported) {
		return fmt.Errorf("%s: %w: %w", step, errNoTCX, err)
	}
	if _, ok := errors.AsType[*ebpf.VerifierError](err); ok {
		return fmt.Errorf("%s: %w: %w", step, errProgramRejected, err)
	}
	return fmt.Errorf("%s: %w", step, err)
}

// tcxAttachment holds one interface's loaded collection and TCX links.
type tcxAttachment struct {
	coll    *ebpf.Collection
	links   []link.Link
	trackIP bool
}

// Counts reads all maps and returns the current absolute byte counters.
func (a *tcxAttachment) Counts() (counts, error) {
	c := counts{
		ingressPort: make(map[portProto]uint64),
		egressPort:  make(map[portProto]uint64),
		mapEntries:  make(map[string]int),
	}
	if err := readPortMap(a.coll.Maps[mapPortIngress], c.ingressPort); err != nil {
		return c, fmt.Errorf("read port ingress map: %w", err)
	}
	if err := readPortMap(a.coll.Maps[mapPortEgress], c.egressPort); err != nil {
		return c, fmt.Errorf("read port egress map: %w", err)
	}
	c.mapEntries["port_ingress"] = len(c.ingressPort)
	c.mapEntries["port_egress"] = len(c.egressPort)

	if a.trackIP {
		c.ingressIP = make(map[uint32]uint64)
		c.egressIP = make(map[uint32]uint64)
		if err := readIPMap(a.coll.Maps[mapIPIngress], c.ingressIP); err != nil {
			return c, fmt.Errorf("read ip ingress map: %w", err)
		}
		if err := readIPMap(a.coll.Maps[mapIPEgress], c.egressIP); err != nil {
			return c, fmt.Errorf("read ip egress map: %w", err)
		}
		c.mapEntries["ip_ingress"] = len(c.ingressIP)
		c.mapEntries["ip_egress"] = len(c.egressIP)
	}
	return c, nil
}

// Close detaches the TCX links and closes the collection.
func (a *tcxAttachment) Close() error {
	var errs []error
	for _, l := range a.links {
		if err := l.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	a.links = nil
	if a.coll != nil {
		a.coll.Close()
		a.coll = nil
	}
	return errors.Join(errs...)
}

// closeAll releases a collection and any links on an error path.
func closeAll(coll *ebpf.Collection, links ...link.Link) {
	for _, l := range links {
		if l == nil {
			continue
		}
		if err := l.Close(); err != nil {
			logger().Warn("traffic-usage: link close on cleanup", "error", err)
		}
	}
	if coll != nil {
		coll.Close()
	}
}

func readPortMap(m *ebpf.Map, dst map[portProto]uint64) error {
	if m == nil {
		return nil
	}
	var k bpfPortKey
	var v uint64
	it := m.Iterate()
	for it.Next(&k, &v) {
		dst[portProto{port: k.Port, proto: k.Protocol}] = v
	}
	return it.Err()
}

func readIPMap(m *ebpf.Map, dst map[uint32]uint64) error {
	if m == nil {
		return nil
	}
	var k uint32
	var v uint64
	it := m.Iterate()
	for it.Next(&k, &v) {
		dst[k] = v
	}
	return it.Err()
}
