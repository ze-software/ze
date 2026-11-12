// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- both ESP forms on one inbound state: the complete path re-presents on a policy-checked device
// RFC: rfc/short/rfc4301.md -- every inbound packet is checked against its SA's selectors (Section 5.2)
// Overview: espform_linux.go -- the dual-form receiver that writes into this device
// Related: espform.go -- the datagram written here

//go:build linux

package dataplane

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/privilege"
)

// espFormTUNName names the TUN device the dual-form receiver writes re-presented
// datagrams into. A packet decapsulated from such a datagram arrives on this device, so
// it is the name an input-interface firewall rule sees for that traffic.
const espFormTUNName = "ze-espform"

// espFormTUNMTU is MAX_MTU, the largest IPv4 datagram (drivers/net/tun.c sets max_mtu
// to it for a TUN). A re-presented datagram can be a reassembled one of that size, so
// the device's own limit is never smaller than what the receiver writes into it.
const espFormTUNMTU = 0xFFFF

// espFormTUNClone is the TUN clone device. Opening it and naming an interface with
// TUNSETIFF creates that interface.
const espFormTUNClone = "/dev/net/tun"

// espFormTUNAddr is the one IPv4 address ze-espform carries, 192.0.0.8/32, the address
// RFC 7600 Section 6 reserves: "Reserved IPv4 address 192.0.0.8/32 to be used as the
// "IPv4 dummy address" (Section 4.8)." RFC 7600 Section 4.8 uses it where a source is
// owed and no real one exists: "The reserved IPv4 dummy address (192.0.0.8/32; see
// Section 6) MUST be used as its source address." Linux gives it the same role: __icmp_send
// (net/ipv4/icmp.c, v7.0 line 996) falls back to it, `fl4.saddr = htonl(INADDR_DUMMY);`,
// when a device has no IPv4 address, and include/uapi/linux/in.h defines INADDR_DUMMY as
// 0xc0000008, "Dummy address for src of ICMP replies if no real address is set
// (RFC7600)". It is an address that exists only so a device is not address-less, which
// is exactly the job here (espFormTUNAddress says why the device needs one).
var espFormTUNAddr = net.IPv4(192, 0, 0, 8)

// espFormTUN is the device the complete path writes re-presented datagrams into.
//
// WHY A TUN, and not the loopback this receiver once used. A datagram written to a TUN
// is received by the stack as if it arrived on that device (drivers/net/tun.c
// tun_get_user hands it to netif_rx), so after XFRM decapsulates it skb->dev is the TUN.
// The inbound policy check then reads that device's disable_policy, which this device
// carries as 0, where loopback carries 1 and skips the check (see the sysctls below).
//
// The device is NOT persistent: closing the fd removes it (__tun_detach unregisters a
// TUN without IFF_PERSIST once its last queue detaches), so a daemon that dies leaves no
// device behind. Ze never reads from it: no route points into it, and whatever the
// kernel itself sends out of it is dropped once its queue fills.
//
// NOT safe for concurrent use with Close; the receiver closes it only after its reader
// has stopped writing.
type espFormTUN struct {
	fd int
}

// openESPFormTUN creates ze-espform in the caller's network namespace, sets the sysctls
// that make its inbound traffic policy-checked, and brings it up.
//
// Any failure closes the fd, which removes the device, and returns an error naming the
// step. The caller MUST refuse the SA then: a receiver without this device cannot
// re-present the refused ESP form on a device whose policy check runs. Every failure is
// an operator error that names the cause and its fix: the open of the clone device, the
// naming of the interface, a sysctl the receive path depends on, and each later step of
// the configuration (openESPFormTUNClone, espFormTUNSetIffRefusal, espFormTUNSysctls,
// espFormTUNStepRefusal).
func openESPFormTUN() (*espFormTUN, error) {
	fd, err := openESPFormTUNClone(espFormTUNHostOps)
	if err != nil {
		return nil, err
	}
	tun := &espFormTUN{fd: fd}

	if err := tun.configure(); err != nil {
		if cerr := tun.Close(); cerr != nil {
			return nil, errors.Join(err, cerr)
		}
		return nil, err
	}
	return tun, nil
}

// The clone node, as the kernel declares it. drivers/net/tun.c (v7.0) registers
// `static struct miscdevice tun_miscdev = { .minor = TUN_MINOR, .name = "tun",
// .nodename = "net/tun", ... }`, and include/linux/miscdevice.h defines `#define
// TUN_MINOR 200` under the misc major, 10. devtmpfs gives a node with no declared mode
// 0600 (drivers/base/devtmpfs.c, devtmpfs_create_node: `if (req.mode == 0) req.mode =
// 0600;`), and tun_miscdev declares none, so a node Ze creates carries the mode the
// kernel itself would have given it. /dev/net is created 0755, the mode of the
// directories devtmpfs creates for a nodename (create_path, `err = dev_mkdir(path,
// 0755);`).
const (
	espFormTUNNodeDir   = "/dev/net"
	espFormTUNNodeMajor = 10
	espFormTUNNodeMinor = 200
	espFormTUNNodeMode  = unix.S_IFCHR | 0o600
	espFormTUNDirMode   = 0o755
)

// espFormTUNNodeOps holds the system calls that open the clone device and, where the
// node is missing, create it. espFormTUNHostOps is the real set; a test replaces it to
// drive every errno without privilege. Every path is one of the constants above: the
// caller passes none, so nothing outside /dev/net/tun is ever created.
type espFormTUNNodeOps struct {
	open  func(path string, flags int, mode uint32) (int, error)
	mkdir func(path string, mode uint32) error
	mknod func(path string, mode uint32, dev int) error
	rmdir func(path string) error
}

// espFormTUNHostOps is the clone device as the host's system calls see it.
var espFormTUNHostOps = espFormTUNNodeOps{
	open:  unix.Open,
	mkdir: unix.Mkdir,
	mknod: unix.Mknod,
	rmdir: unix.Rmdir,
}

// openESPFormTUNClone opens /dev/net/tun, creating the node first when it is missing.
// Every failure is an operator error naming the cause and its fix.
//
// WHY Ze creates the node, as wireguard-go and OpenVPN tell their users to: a container
// started without `--device /dev/net/tun` has no node, yet the kernel and the runtime
// may still let it make one and open it.
//   - The driver is reachable through the node alone. misc_open (drivers/char/misc.c,
//     v7.0) finds the misc device by minor, and for a fixed minor with no driver it runs
//     `request_module("char-major-%d-%d", MISC_MAJOR, minor);`; tun.c declares
//     `MODULE_ALIAS_MISCDEV(TUN_MINOR);`, so opening a node c 10 200 loads tun.
//   - vfs_mknod (fs/namei.c, v7.0) creates a character device only for a caller that is
//     `capable(CAP_MKNOD)`, which is judged in the initial user namespace, and only when
//     the device cgroup grants mknod (devcgroup_inode_mknod, DEVCG_ACC_MKNOD).
//   - runc appends to every container's device rules "allow mknod for any device" (`c
//     *:* m`) and `c 10:200 rwm` (libcontainer/specconv, AllowedDevices), and Docker
//     grants CAP_MKNOD by default and mounts /dev as a tmpfs without nodev. A default
//     Docker container can therefore create the node and open it.
//
// Ze does not check for CAP_MKNOD or a writable /dev/net before it tries: the kernel's
// own answer to mknod is the authority (a CAP_MKNOD held in a user namespace looks
// present and is refused), so Ze tries once and reports what the kernel said.
//
// Ze creates only on ENOENT, the one answer that means the node is absent. A node that
// exists is never replaced and its mode never changed, whatever the open answered.
func openESPFormTUNClone(ops espFormTUNNodeOps) (int, error) {
	fd, err := ops.open(espFormTUNClone, unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err == nil {
		return fd, nil
	}
	if !errors.Is(err, unix.ENOENT) {
		return -1, espFormTUNOpenRefusal(err)
	}
	if cerr := createESPFormTUNNode(ops); cerr != nil {
		return -1, espFormTUNMissingRefusal(fmt.Errorf("open %s: %w, then %w", espFormTUNClone, err, cerr))
	}
	fd, err = ops.open(espFormTUNClone, unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, espFormTUNOpenRefusal(err)
	}
	return fd, nil
}

// createESPFormTUNNode creates /dev/net, when it is missing, and the node c 10 200 in
// it. A /dev/net Ze created is removed again when the node cannot be made, so a refused
// attempt leaves nothing behind. A node that another process created in the meantime
// (EEXIST) is no failure: the reopen judges it.
func createESPFormTUNNode(ops espFormTUNNodeOps) error {
	mkdirErr := ops.mkdir(espFormTUNNodeDir, espFormTUNDirMode)
	createdDir := mkdirErr == nil
	if mkdirErr != nil && !errors.Is(mkdirErr, unix.EEXIST) {
		return fmt.Errorf("create the directory %s: %w", espFormTUNNodeDir, mkdirErr)
	}

	dev := int(unix.Mkdev(espFormTUNNodeMajor, espFormTUNNodeMinor)) //nolint:gosec // Mkdev(10, 200) is 0xac8, far inside int
	err := ops.mknod(espFormTUNClone, espFormTUNNodeMode, dev)
	if err == nil {
		return nil
	}
	if errors.Is(err, unix.EEXIST) {
		// refuse-by-default: guards openESPFormTUNClone -- a node another process made between the open and here is opened by the reopen, which reports it if it is wrong
		return nil
	}
	err = fmt.Errorf("create the node %s (c %d %d): %w", espFormTUNClone, espFormTUNNodeMajor, espFormTUNNodeMinor, err)
	if !createdDir {
		return err
	}
	if rerr := ops.rmdir(espFormTUNNodeDir); rerr != nil {
		return errors.Join(err, fmt.Errorf("remove the directory %s Ze created: %w", espFormTUNNodeDir, rerr))
	}
	return err
}

// espFormTUNWhy is why every refusal of the device matters. The SA is refused because
// of what the host lacks, never because of what the operator configured.
const espFormTUNWhy = "Ze uses the TUN device to create the " + espFormTUNName + " interface. " +
	"Without it, Ze cannot receive bare ESP on a Child SA that must accept both ESP forms " +
	"and still run the inbound policy check (RFC 4301 Section 5.2), so it refuses the SA. " +
	"This is not a configuration error"

// espFormTUNFixDevice is the fix for a container that was started without the device.
const espFormTUNFixDevice = "in Docker: docker run --device /dev/net/tun (compose: devices: [/dev/net/tun])"

// espFormTUNMissingRefusal is the refusal for a node that does not exist and that Ze
// could not create. cause carries the open's ENOENT and the creation's own error.
func espFormTUNMissingRefusal(cause error) error {
	return opererr.Wrap(cause,
		"the TUN device node "+espFormTUNClone+" does not exist, and Ze could not create it",
		espFormTUNWhy,
		espFormTUNFixDevice+"; on a host: modprobe tun, or mknod "+espFormTUNClone+" c 10 200")
}

// espFormTUNOpenRefusal maps the errno of an open of /dev/net/tun to its cause.
//   - EPERM: the device cgroup refuses the access (devcgroup_check_permission answers
//     -EPERM from the legacy controller and from a BPF device program alike,
//     security/device_cgroup.c and kernel/bpf/cgroup.c), or a security module does. A
//     container whose runtime did not grant the device meets this.
//   - EACCES: the node's file mode refuses the user Ze runs as, or the mount is nodev
//     (may_open, fs/namei.c: a device on a nodev mount answers -EACCES).
//   - ENODEV, ENXIO: no driver answers for c 10 200. misc_open answers -ENODEV when
//     request_module found no tun module; ENXIO is a node whose major no driver holds.
//   - ENOENT here comes after Ze created the node, so something removed it again.
func espFormTUNOpenRefusal(err error) error {
	cause := fmt.Errorf("open %s: %w", espFormTUNClone, err)
	if errors.Is(err, unix.EPERM) {
		return opererr.Wrap(cause,
			"the device cgroup refuses Ze access to the TUN device "+espFormTUNClone+" (c 10 200)",
			"a container started without the TUN device can hold the node but not open it. "+espFormTUNWhy,
			espFormTUNFixDevice)
	}
	if errors.Is(err, unix.EACCES) {
		return opererr.Wrap(cause,
			"Ze has no permission to open the TUN device "+espFormTUNClone,
			"the node's mode refuses the user Ze runs as, or /dev is mounted with nodev. "+espFormTUNWhy,
			"run ze as root; if /dev is mounted with nodev: mount -o remount,dev /dev")
	}
	if errors.Is(err, unix.ENODEV) {
		return espFormTUNNoDriverRefusal(cause)
	}
	if errors.Is(err, unix.ENXIO) {
		return espFormTUNNoDriverRefusal(cause)
	}
	if errors.Is(err, unix.ENOENT) {
		return opererr.Wrap(cause,
			"the TUN device node "+espFormTUNClone+" was removed right after Ze created it",
			"another program on this host removes the node. "+espFormTUNWhy,
			"ls -l "+espFormTUNClone+" (find what removes it, then restart ze)")
	}
	return opererr.Wrap(cause,
		"Ze cannot open the TUN device "+espFormTUNClone,
		"the node must be the character device c 10 200. "+espFormTUNWhy,
		"ls -l "+espFormTUNClone+" (it must read crw------- or wider, 10, 200); "+espFormTUNFixDevice)
}

// espFormTUNNoDriverRefusal is the refusal for a kernel with no TUN driver.
func espFormTUNNoDriverRefusal(cause error) error {
	return opererr.Wrap(cause,
		"the kernel has no TUN driver: "+espFormTUNClone+" opened no device",
		"the kernel found no driver for c 10 200 and could not load the tun module. "+espFormTUNWhy,
		"modprobe tun, or use a kernel built with CONFIG_TUN")
}

// espFormTUNSetIffRefusal maps the errno of TUNSETIFF, which names the interface.
//   - EPERM: tun_set_iff (drivers/net/tun.c, v7.0) creates a TUN only for a caller that
//     is `ns_capable(net->user_ns, CAP_NET_ADMIN)` in the network namespace Ze runs in.
//   - EBUSY, EINVAL: an interface named ze-espform exists already. tun_set_iff answers
//     -EINVAL when that interface is not a single-queue TUN, and tun_attach -EBUSY when
//     another process holds it.
func espFormTUNSetIffRefusal(err error) error {
	cause := fmt.Errorf("TUNSETIFF %s: %w", espFormTUNName, err)
	if errors.Is(err, unix.EPERM) {
		return opererr.Wrap(cause,
			"Ze has no permission to create the "+espFormTUNName+" TUN interface",
			"creating a TUN interface needs CAP_NET_ADMIN in the network namespace Ze runs in, which the "+
				"command below grants through the ze unit; in Docker, start the container with --cap-add NET_ADMIN. "+espFormTUNWhy,
			privilege.DaemonCapabilitiesFix)
	}
	if errors.Is(err, unix.EBUSY) {
		return espFormTUNNameTakenRefusal(cause)
	}
	if errors.Is(err, unix.EINVAL) {
		return espFormTUNNameTakenRefusal(cause)
	}
	return opererr.Wrap(cause,
		"Ze cannot create the "+espFormTUNName+" TUN interface",
		espFormTUNWhy,
		espFormTUNFixShow)
}

// espFormTUNNameTakenRefusal is the refusal for an interface name another holder took.
func espFormTUNNameTakenRefusal(cause error) error {
	return opererr.Wrap(cause,
		"an interface named "+espFormTUNName+" already exists, so Ze cannot create its own",
		"another program, or another ze in this network namespace, holds the name. "+espFormTUNWhy,
		"ip link delete "+espFormTUNName)
}

// espFormTUNFixShow shows whether an interface holds the name ze-espform, for a
// TUNSETIFF that failed before the name was Ze's: one listed here is another holder's.
const espFormTUNFixShow = "ip link show " + espFormTUNName + " (no interface may hold the name before Ze creates it)"

// espFormTUNStepRefusal is the refusal for a step of the configuration of the ze-espform
// interface Ze just created: what reads it back, gives it its address, sets its MTU or
// brings it up. call names the netlink call for the cause, and step the step as a verb
// phrase whose object is the interface ("bring it up"). EPERM is a process without
// CAP_NET_ADMIN in the network namespace Ze runs in, which the unit install grants. Any
// other answer comes after TUNSETIFF made the name Ze's, so no other holder explains it,
// and the interface is removed again (openESPFormTUN closes it), so a listing of the links
// would show nothing: the fix lists the receiver's warnings, which carry the kernel's
// answer for each attempt.
func espFormTUNStepRefusal(call, step string, err error) error {
	cause := fmt.Errorf("%s %s: %w", call, espFormTUNName, err)
	if errors.Is(err, unix.EPERM) {
		return opererr.Wrap(cause,
			"Ze created the "+espFormTUNName+" interface, but has no permission to "+step,
			"configuring an interface needs CAP_NET_ADMIN in the network namespace Ze runs in, which the "+
				"command below grants through the ze unit; in Docker, start the container with --cap-add NET_ADMIN. "+espFormTUNWhy,
			privilege.DaemonCapabilitiesFix)
	}
	return opererr.Wrap(cause,
		"Ze created the "+espFormTUNName+" interface but could not "+step+", so it removed the interface again",
		"the kernel's answer after this names why. "+espFormTUNWhy,
		ESPFormFixLog)
}

// configure names the device, writes its sysctls, and brings it up. Every failure is an
// operator error naming the cause and its fix.
func (t *espFormTUN) configure() error {
	ifr, err := unix.NewIfreq(espFormTUNName)
	if err != nil {
		// NewIfreq refuses only a name too long for IFNAMSIZ, and the name is a constant.
		panic("BUG: the interface name " + espFormTUNName + " does not fit an ifreq: " + err.Error())
	}
	// IFF_TUN: the device carries IP datagrams, no link-layer header. IFF_NO_PI: no
	// packet-information prefix, so tun_get_user reads the protocol from the version
	// nibble and a write is the datagram alone.
	ifr.SetUint16(unix.IFF_TUN | unix.IFF_NO_PI)
	if err := unix.IoctlIfreq(t.fd, unix.TUNSETIFF, ifr); err != nil {
		return espFormTUNSetIffRefusal(err)
	}

	if err := espFormTUNSysctls(espFormTUNHostSysctls); err != nil {
		return err
	}

	link, err := netlink.LinkByName(espFormTUNName)
	if err != nil {
		return espFormTUNStepRefusal("LinkByName", "read it back", err)
	}
	if err := espFormTUNAddress(link); err != nil {
		return err
	}
	if err := netlink.LinkSetMTU(link, espFormTUNMTU); err != nil {
		return espFormTUNStepRefusal("LinkSetMTU", "set its MTU to "+strconv.Itoa(espFormTUNMTU), err)
	}
	// Up, because the stack drops a datagram received on a device that is not running
	// (net/core/dev.c enqueue_to_backlog checks netif_running).
	if err := netlink.LinkSetUp(link); err != nil {
		return espFormTUNStepRefusal("LinkSetUp", "bring it up", err)
	}
	return nil
}

// espFormTUNAddress gives ze-espform its one IPv4 address, without which the loose
// reverse-path filter drops every datagram the complete path writes into it.
//
// The kernel, read at v7.0 in net/ipv4/fib_frontend.c:
//   - fib_validate_source, line 433: `int r = secpath_exists(skb) ? 0 :
//     IN_DEV_RPFILTER(idev);`. The outer ESP-in-UDP datagram carries no secpath yet, so
//     the filter applies to it, at the larger of conf/all and the device's value: 2.
//   - __fib_validate_source, line 369: `no_addr = idev->ifa_list == NULL;`. The route
//     back to the peer leaves through the real link, so it does not match ze-espform,
//     and line 403, `if (no_addr) goto last_resort;`, then sends an address-less device
//     to line 416, where `if (rpf)` drops the datagram (SKB_DROP_REASON_IP_RPFILTER).
//   - With an address, no_addr is false, the loose filter (rpf 2) skips the strict
//     drop and makes a second lookup with the device as output interface, which answers
//     0 or 1. Both are accepted.
//
// rp_filter 0 would not avoid this: conf/all can raise it, which is why Ze writes 2,
// the loosest value conf/all cannot override (espFormTUNSysctls). Where /proc/sys is
// read-only, the device keeps the value conf/default gave it, and espFormTUNSysctls
// accepts it only when the value the kernel applies is not 1.
// The decapsulated inner packet is not filtered at all: XFRM has given it a secpath, so
// r is 0 for it.
//
// Scope host, so the address is never chosen as the source of traffic leaving the host
// (inet_select_addr skips an address whose scope is narrower than the destination's).
// IFA_F_NOPREFIXROUTE, so no prefix route is added for it, and a /32 so it covers no
// one else's addresses.
func espFormTUNAddress(link netlink.Link) error {
	addr := &netlink.Addr{
		IPNet: &net.IPNet{IP: espFormTUNAddr, Mask: net.CIDRMask(32, 32)},
		Scope: unix.RT_SCOPE_HOST,
		Flags: unix.IFA_F_NOPREFIXROUTE,
	}
	if err := netlink.AddrAdd(link, addr); err != nil {
		// Without the address, reverse-path filtering drops every re-presented datagram.
		return espFormTUNStepRefusal("AddrAdd", "give it the address "+espFormTUNAddr.String()+"/32", err)
	}
	return nil
}

// The /proc/sys directories of the IPv4 settings Ze reads: ze-espform's own, conf/default,
// which a new ze-espform copies (espFormTUNSysctls), and conf/all, which the kernel
// combines with each of them when it reads one.
const (
	espFormTUNConf4        = "/proc/sys/net/ipv4/conf/" + espFormTUNName + "/"
	espFormTUNConf4Default = "/proc/sys/net/ipv4/conf/default/"
	espFormTUNConf4All     = "/proc/sys/net/ipv4/conf/all/"
)

// espFormTUNSysctlOps reads and writes one integer /proc/sys file, and tests whether a
// write to one would be refused without writing it. espFormTUNHostSysctls is the real
// set; a test replaces it to drive a read-only /proc/sys, and every errno, without
// privilege.
type espFormTUNSysctlOps struct {
	read     func(path string) (int, error)
	write    func(path, value string) error
	writable func(path string) error
}

// espFormTUNHostSysctls is /proc/sys as the host's file system sees it.
var espFormTUNHostSysctls = espFormTUNSysctlOps{
	read:     readESPFormTUNSysctl,
	write:    writeESPFormTUNSysctl,
	writable: probeESPFormTUNSysctlWritable,
}

// espFormTUNSysctl names one IPv4 setting Ze owns on ze-espform. The zero value names
// none; only Ze produces a value, so a switch over it lists every one and panics on the
// zero (a Ze defect alone reaches it).
type espFormTUNSysctl uint8

const (
	espFormTUNSysctlUnspecified espFormTUNSysctl = iota
	espFormTUNDisablePolicy
	espFormTUNRPFilter
)

// espFormTUNSysctlOrder is every setting Ze owns on ze-espform, in the order Ze settles
// them: the policy check first, because it is the reason the device exists.
var espFormTUNSysctlOrder = [...]espFormTUNSysctl{espFormTUNDisablePolicy, espFormTUNRPFilter}

// leaf is the setting's file name under /proc/sys/net/ipv4/conf/<device>/.
func (k espFormTUNSysctl) leaf() string {
	switch k {
	case espFormTUNDisablePolicy:
		return "disable_policy"
	case espFormTUNRPFilter:
		return "rp_filter"
	case espFormTUNSysctlUnspecified:
	}
	panic("BUG: espFormTUNSysctl " + strconv.Itoa(int(k)) + " names no IPv4 setting")
}

// want is the value Ze gives ze-espform. settleESPFormTUNUnwritable says why each one.
func (k espFormTUNSysctl) want() int {
	switch k {
	case espFormTUNDisablePolicy:
		return 0
	case espFormTUNRPFilter:
		return 2
	case espFormTUNSysctlUnspecified:
	}
	panic("BUG: espFormTUNSysctl " + strconv.Itoa(int(k)) + " names no IPv4 setting")
}

// espFormTUNSubject is the ze-espform whose settings Ze judges. espFormTUNCreated is the
// device a receiver has just created: Ze reads its own values and writes them.
// espFormTUNPredicted is the device ESPFormTUNAvailable predicts before any receiver
// creates it: Ze reads conf/default, which the device would copy, and writes nothing.
// The zero value names neither, and a switch over it panics on the zero.
type espFormTUNSubject uint8

const (
	espFormTUNSubjectUnspecified espFormTUNSubject = iota
	espFormTUNCreated
	espFormTUNPredicted
)

// dir is the /proc/sys directory that holds the subject's own values.
func (s espFormTUNSubject) dir() string {
	switch s {
	case espFormTUNCreated:
		return espFormTUNConf4
	case espFormTUNPredicted:
		// A new device starts with a copy of conf/default (espFormTUNSysctls, WHERE THE
		// VALUES COME FROM), so conf/default holds what ze-espform would take.
		return espFormTUNConf4Default
	case espFormTUNSubjectUnspecified:
	}
	panic("BUG: espFormTUNSubject " + strconv.Itoa(int(s)) + " names no ze-espform")
}

// set gives the subject's file at path Ze's value. For the created device that is the
// write. For the predicted one it is only the test whether the write would be refused,
// which writes nothing, and path is the conf/default file: every conf/<name> directory
// is a copy of one sysctl table (net/ipv4/devinet.c, __devinet_sysctl_register: `t =
// kmemdup(&devinet_sysctl, sizeof(*t), GFP_KERNEL_ACCOUNT);`), in which both settings
// are `DEVINET_SYSCTL_RW_ENTRY`, mode 0644, under the same /proc/sys mount. The kernel
// therefore answers a write to conf/default/<leaf> as it would answer the same write to
// conf/ze-espform/<leaf>.
func (s espFormTUNSubject) set(ops espFormTUNSysctlOps, path, value string) error {
	switch s {
	case espFormTUNCreated:
		return ops.write(path, value)
	case espFormTUNPredicted:
		return ops.writable(path)
	case espFormTUNSubjectUnspecified:
	}
	panic("BUG: espFormTUNSubject " + strconv.Itoa(int(s)) + " names no ze-espform")
}

// espFormTUNSetting is one IPv4 setting as the kernel holds it for a subject: the
// subject's own value, and the conf/all value the kernel combines with it.
type espFormTUNSetting struct {
	key     espFormTUNSysctl
	subject espFormTUNSubject
	dev     int // the subject's own value: ze-espform's, or the conf/default it would copy
	all     int
}

// path is the /proc/sys file of the subject's own value.
func (s espFormTUNSetting) path() string {
	return s.subject.dir() + s.key.leaf()
}

// espFormTUNSysctls sets the per-device settings that decide how the stack receives
// what the complete path writes into ze-espform. It writes a setting only where the
// device does not already hold Ze's value.
//
// WHERE THE VALUES COME FROM. A new device starts with a copy of conf/default
// (net/ipv4/devinet.c v7.0, inetdev_init: `memcpy(&in_dev->cnf,
// dev_net(dev)->ipv4.devconf_dflt, sizeof(in_dev->cnf));`), and the kernel combines
// each value with conf/all whenever it reads one (include/linux/inetdevice.h). A host
// or a container whose conf/default already holds Ze's values needs no write at all.
// That is what lets Ze run where /proc/sys is mounted read-only, as Docker mounts it
// for a container that is not privileged, while Docker still accepts `--sysctl
// net.ipv4.conf.default.<name>=<value>` for the container's own network namespace.
//
// Where the device's value differs from Ze's, Ze writes it. When the write is refused
// because /proc/sys is read-only (EROFS) or Ze may not write it (EACCES), the value
// the kernel applies decides: the SA goes on when that value already gives the receive
// path what it needs, and is refused with an operator error naming the setting and its
// fix when it does not. Any other write failure refuses the SA with an operator error
// whose fix gives conf/default Ze's value (espFormTUNSysctlWriteRefusal).
//
// A setting Ze cannot read is an error: Ze never assumes a value it did not read.
//
// The rule is settleESPFormTUNSysctls, which ESPFormTUNAvailable applies too, to the
// device Ze would create, so the ze doctor check predicts this function's answer.
func espFormTUNSysctls(ops espFormTUNSysctlOps) error {
	// RFC 4301 Section 5.2
	return settleESPFormTUNSysctls(ops, espFormTUNCreated)
}

// settleESPFormTUNSysctls is the one rule for ze-espform's IPv4 settings. The receiver
// applies it to the device it created (espFormTUNSysctls) and the probe to the device it
// would create (ESPFormTUNAvailable), so the two cannot disagree. For each setting, in
// order, it reads the subject's value and conf/all's. Where the subject already holds
// Ze's value it moves on. Otherwise it sets the value (espFormTUNSubject.set): a write
// that is refused because /proc/sys is read-only or Ze may not write it
// (espFormTUNSysctlBlocked) leaves the decision to the value the kernel applies
// (settleESPFormTUNUnwritable), and any other failure is an operator error with its fix
// (espFormTUNSysctlWriteRefusal), so a refused install or MOBIKE move names one.
func settleESPFormTUNSysctls(ops espFormTUNSysctlOps, subject espFormTUNSubject) error {
	for _, key := range espFormTUNSysctlOrder {
		setting, err := readESPFormTUNSetting(ops, subject, key)
		if err != nil {
			return err
		}
		if setting.dev == key.want() {
			continue
		}
		err = subject.set(ops, setting.path(), strconv.Itoa(key.want()))
		if err == nil {
			continue
		}
		if !espFormTUNSysctlBlocked(err) {
			return espFormTUNSysctlWriteRefusal(setting, err)
		}
		// RFC 4301 Section 5.2
		if refusal := settleESPFormTUNUnwritable(setting, err); refusal != nil {
			return refusal
		}
	}
	return nil
}

// settleESPFormTUNUnwritable decides a setting Ze could not give its value, because
// /proc/sys is read-only or Ze may not write it (cause). It answers nil when the value
// the kernel applies already gives the receive path what it needs, and otherwise the
// refusal that names the setting, the value the kernel applies and its fix.
func settleESPFormTUNUnwritable(setting espFormTUNSetting, cause error) error {
	switch setting.key {
	case espFormTUNDisablePolicy:
		// RFC 4301 Section 5.2: "If an IPsec system receives an inbound packet on an SA
		// and the packet's header fields are not consistent with the selectors for the
		// SA, it MUST discard the packet."
		//
		// disable_policy 0 is the whole fix. The input path and the input route set
		// IPSKB_NOPOLICY from the receiving device's disable_policy (net/ipv4/ip_input.c
		// and net/ipv4/route.c, `if (IN_DEV_ORCONF(in_dev, NOPOLICY)) IPCB(skb)->flags |=
		// IPSKB_NOPOLICY;`), and __xfrm_policy_check2 returns before any policy is read
		// when that flag is set. With 0 here, the decapsulated packet is matched against
		// the SA's selectors as RFC 4301 Section 5.2 requires.
		//
		// IN_DEV_ORCONF ORs this value with conf/all/disable_policy (include/linux/
		// inetdevice.h, v7.0: `(IPV4_DEVCONF_ALL_RO(net, attr) || IN_DEV_CONF_GET((in_dev),
		// attr))`), so an operator who sets conf/all/disable_policy=1 disables the inbound
		// policy check on every device, this one included. That is the operator's choice
		// for the whole host, and Ze does not override it. What Ze owns is the device's
		// own value, so that is what it requires: a device left at 1 means the kernel
		// applies 1 whatever conf/all holds, and the bypass this device exists to close is
		// back.
		if setting.dev == 0 {
			// refuse-by-default: guards settleESPFormTUNSysctls -- a device already at 0 runs the policy check, so no write was owed and nothing is refused
			return nil
		}
		return espFormTUNDisablePolicyRefusal(setting, cause)
	case espFormTUNRPFilter:
		// rp_filter 2, loose. The re-presented datagram carries the peer's own address as
		// its source, and the route back to the peer leaves through the real link, never
		// through ze-espform. Strict reverse-path filtering (1) would drop every datagram
		// for that reason; loose (2) asks that the source be routable at all, and that the
		// device hold an IPv4 address (espFormTUNAddress). The kernel applies the larger of
		// conf/all and this value (inetdevice.h: IN_DEV_RPFILTER is IN_DEV_MAXCONF,
		// `max(IPV4_DEVCONF_ALL_RO(...), IN_DEV_CONF_GET(...))`), so 2 here is loose
		// whatever conf/all says, which is why Ze writes 2 wherever it can.
		//
		// Where Ze cannot write, any applied value but 1 still lets the datagram through
		// (net/ipv4/fib_frontend.c, v7.0):
		//   - 0: fib_validate_source, line 436, `if (!r && !fib_num_tclassid_users(net) &&`,
		//     accepts without a reverse-path lookup, and the full check it can fall back
		//     to drops only for `rpf == 1` (line 405) or `if (rpf)` at last_resort (line
		//     417).
		//   - 2, or any other value but 1: __fib_validate_source skips `if (rpf == 1) goto
		//     e_rpf;` and accepts the second lookup, because the device holds an address.
		// The applied value is only what conf/all holds now: on a host where Ze cannot
		// write, whoever raises conf/all/rp_filter to 1 later also drops the re-presented
		// datagrams.
		//
		// IPv4 only. The complete path re-presents only what the ip4:50 reader receives,
		// and no IPv6 dual-form SA exists to feed it: Ze has no IPv6 IKE transport yet, so
		// InstallSA refuses one (plan/post-release/features/spec-ike-ipv6-transport.md).
		if max(setting.all, setting.dev) != 1 {
			// refuse-by-default: guards settleESPFormTUNSysctls -- the write was refused, but the rp_filter the kernel applies already passes every re-presented datagram (fib_validate_source), so the device is usable as it is
			return nil
		}
		return espFormTUNRPFilterRefusal(setting, cause)
	case espFormTUNSysctlUnspecified:
	}
	panic("BUG: espFormTUNSysctl " + strconv.Itoa(int(setting.key)) + " names no IPv4 setting")
}

// readESPFormTUNSetting reads one IPv4 setting of the subject and of conf/all. A value it
// cannot read is an operator error: the caller cannot tell what the kernel applies, so
// it does not assume one.
func readESPFormTUNSetting(ops espFormTUNSysctlOps, subject espFormTUNSubject, key espFormTUNSysctl) (espFormTUNSetting, error) {
	setting := espFormTUNSetting{key: key, subject: subject}
	dev, err := ops.read(setting.path())
	if err != nil {
		return setting, espFormTUNSysctlReadRefusal(subject, setting.path(), err)
	}
	all, err := ops.read(espFormTUNConf4All + key.leaf())
	if err != nil {
		return setting, espFormTUNSysctlReadRefusal(subject, espFormTUNConf4All+key.leaf(), err)
	}
	setting.dev, setting.all = dev, all
	return setting, nil
}

// espFormTUNSysctlBlocked reports whether a write to /proc/sys failed because the file
// system or the file's mode refuses it: EROFS where /proc/sys is mounted read-only, as
// in a container that is not privileged, and EACCES where the file's mode refuses the
// user Ze runs as. Only these two let the value the kernel applies decide.
func espFormTUNSysctlBlocked(err error) bool {
	return errors.Is(err, unix.EROFS) || errors.Is(err, unix.EACCES)
}

// espFormTUNSysctlBlockedReason names, for the operator, why Ze could not write path.
func espFormTUNSysctlBlockedReason(path string, err error) string {
	if errors.Is(err, unix.EROFS) {
		return "/proc/sys is mounted read-only, as Docker mounts it in a container that is not privileged"
	}
	return "Ze has no permission to write " + path
}

// espFormTUNSysctlFixDefault is the part of every sysctl refusal that says where the fix
// goes, and that the refusal is no fault of the operator's configuration.
const espFormTUNSysctlFixDefault = " Ze creates " + espFormTUNName + " with the values of net.ipv4.conf.default, " +
	"so setting them there before ze starts is enough. This is not a configuration error"

// sysctlWhy is the part of every sysctl refusal that says what the SA loses. The created
// device refuses the SA in hand; the predicted one tells the operator that every such SA
// would be refused, before any is.
func (s espFormTUNSubject) sysctlWhy() string {
	switch s {
	case espFormTUNCreated:
		return "Ze then cannot receive bare ESP on a Child SA that must accept both ESP forms " +
			"and still run the inbound policy check (RFC 4301 Section 5.2), so it refuses the SA." +
			espFormTUNSysctlFixDefault
	case espFormTUNPredicted:
		return "Ze then could not receive bare ESP on a Child SA that must accept both ESP forms " +
			"and still run the inbound policy check (RFC 4301 Section 5.2), so it would refuse every such SA." +
			espFormTUNSysctlFixDefault
	case espFormTUNSubjectUnspecified:
	}
	panic("BUG: espFormTUNSubject " + strconv.Itoa(int(s)) + " names no ze-espform")
}

// settingProblem states, for the operator, the value the subject holds, the value the
// kernel applies with conf/all (applied), and why Ze could not give the device its own
// value. The created device has its values; the predicted one does not exist yet, so the
// sentence speaks of the interface Ze would create and of the conf/default it would copy.
func (s espFormTUNSubject) settingProblem(setting espFormTUNSetting, applied string, cause error) string {
	held := setting.key.leaf() + " " + strconv.Itoa(setting.dev)
	all := strconv.Itoa(setting.all)
	want := strconv.Itoa(setting.key.want())
	reason := espFormTUNSysctlBlockedReason(setting.path(), cause)
	switch s {
	case espFormTUNCreated:
		return "the " + espFormTUNName + " interface has " + held + " and conf/all has " + all +
			", so the kernel applies " + applied + ", and Ze cannot set the interface to " + want + ": " + reason
	case espFormTUNPredicted:
		return "the " + espFormTUNName + " interface Ze would create takes " + held + " from conf/default, and conf/all has " + all +
			", so the kernel would apply " + applied + ", and Ze could not set the interface to " + want + ": " + reason
	case espFormTUNSubjectUnspecified:
	}
	panic("BUG: espFormTUNSubject " + strconv.Itoa(int(s)) + " names no ze-espform")
}

// espFormTUNDisablePolicyRefusal is the refusal for a device at disable_policy 1 that Ze
// cannot set to 0. The kernel then applies 1, whatever conf/all holds (IN_DEV_ORCONF).
func espFormTUNDisablePolicyRefusal(policy espFormTUNSetting, cause error) error {
	return opererr.Wrap(cause,
		policy.subject.settingProblem(policy, "1", cause),
		"with disable_policy 1 the kernel skips the inbound IPsec policy check for every packet "+
			espFormTUNName+" receives, and running that check is why the interface exists. "+policy.subject.sysctlWhy(),
		"in Docker: docker run --sysctl net.ipv4.conf.default.disable_policy=0; on a host: sysctl -w net.ipv4.conf.default.disable_policy=0")
}

// espFormTUNRPFilterRefusal is the refusal for a strict reverse-path filter that Ze
// cannot loosen. The kernel applies the larger of conf/all and the device's value
// (IN_DEV_MAXCONF), and the caller found that larger value to be 1.
func espFormTUNRPFilterRefusal(rpf espFormTUNSetting, cause error) error {
	return opererr.Wrap(cause,
		rpf.subject.settingProblem(rpf, "strict reverse-path filtering (1)", cause),
		"strict reverse-path filtering drops every datagram Ze re-presents on "+espFormTUNName+
			", because the route back to the peer leaves through the real interface. "+rpf.subject.sysctlWhy(),
		"in Docker: docker run --sysctl net.ipv4.conf.default.rp_filter=2; on a host: sysctl -w net.ipv4.conf.default.rp_filter=2")
}

// espFormTUNSysctlWriteRefusal is the refusal for a write of a setting that failed for a
// reason other than a read-only /proc/sys or the file's mode (an I/O error, say), which
// leaves the value the kernel applies unknown to Ze, so the SA is refused. The interface
// copies conf/default when Ze creates it, so the fix gives conf/default Ze's value, and the
// next attempt owes no write.
func espFormTUNSysctlWriteRefusal(setting espFormTUNSetting, cause error) error {
	want := strconv.Itoa(setting.key.want())
	return opererr.Wrap(cause,
		"Ze cannot set "+setting.path()+" to "+want,
		setting.subject.sysctlWhy(),
		"sudo sysctl -w net.ipv4.conf.default."+setting.key.leaf()+"="+want)
}

// espFormTUNSysctlReadRefusal is the refusal for a setting Ze could not read. The created
// device's SA is refused rather than run on an assumed value; the probe cannot tell
// whether an SA would be refused, and says so.
func espFormTUNSysctlReadRefusal(subject espFormTUNSubject, path string, cause error) error {
	return opererr.Wrap(cause, "Ze cannot read "+path, subject.readWhy(), "mount -t proc proc /proc")
}

// readWhy is why a setting Ze could not read matters, for the subject.
func (s espFormTUNSubject) readWhy() string {
	switch s {
	case espFormTUNCreated:
		return "Ze reads it to learn whether the kernel runs the inbound policy check (RFC 4301 Section 5.2) on " +
			espFormTUNName + " and lets the datagrams it re-presents there through. Without the value it cannot " +
			"tell, so it refuses the SA rather than assume. This is not a configuration error"
	case espFormTUNPredicted:
		return "Ze reads it to learn whether the kernel would run the inbound policy check (RFC 4301 Section 5.2) on the " +
			espFormTUNName + " interface Ze creates for a Child SA that must accept both ESP forms, and let the datagrams " +
			"Ze re-presents there through. Without the value it cannot tell whether such an SA would be refused. " +
			"This is not a configuration error"
	case espFormTUNSubjectUnspecified:
	}
	panic("BUG: espFormTUNSubject " + strconv.Itoa(int(s)) + " names no ze-espform")
}

// readESPFormTUNSysctl reads one integer /proc/sys file.
func readESPFormTUNSysctl(path string) (int, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // path is built from constants: /proc/sys plus the fixed device name, conf/default or conf/all
	if err != nil {
		return 0, err
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return 0, fmt.Errorf("%s holds %q, not an integer: %w", path, raw, err)
	}
	return value, nil
}

// writeESPFormTUNSysctl writes one value into an existing /proc/sys file. It never
// creates the file: a missing one means the device or the setting does not exist, and
// that is reported.
func writeESPFormTUNSysctl(path, value string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0) //nolint:gosec // path is built from constants: /proc/sys plus the fixed device name
	if err != nil {
		return fmt.Errorf("dataplane: open %s to set it to %s: %w", path, value, err)
	}
	_, werr := f.WriteString(value)
	cerr := f.Close()
	if werr != nil {
		return fmt.Errorf("dataplane: set %s to %s: %w", path, value, werr)
	}
	if cerr != nil {
		return fmt.Errorf("dataplane: close %s after setting it to %s: %w", path, value, cerr)
	}
	return nil
}

// probeESPFormTUNSysctlWritable answers whether a write to path would be refused, and
// writes nothing: it opens the file for writing, as writeESPFormTUNSysctl does, and
// closes it at once. A nil answer means the write's open would succeed.
//
// WHY AN OPEN, and not access(2) with W_OK. The receiver's write is refused at its open,
// so an open with the same flags and the same credentials gets the kernel's own answer:
//   - EROFS from a read-only /proc/sys: do_dentry_open (fs/open.c) takes write access to
//     the mount for every FMODE_WRITE open, `error = file_get_write_access(f);`, which
//     calls mnt_get_write_access and refuses a read-only mount, as Docker mounts
//     /proc/sys in a container that is not privileged.
//   - EACCES from the sysctl's own permission: proc_sys_permission (fs/proc/
//     proc_sysctl.c) asks sysctl_perm, and net_ctl_permissions (net/sysctl_net.c) gives
//     the owner's mode bits to a caller with CAP_NET_ADMIN, `if (ns_capable_noaudit(
//     net->user_ns, CAP_NET_ADMIN))`, judged on the effective credentials.
//
// access(2) would answer EROFS on the read-only mount too, but it judges the real user
// and drops every capability when that user is not root (fs/open.c,
// access_override_creds: `if (!uid_eq(override_cred->uid, root_uid))
// cap_clear(override_cred->cap_effective);`). Ze's systemd unit runs ze as User=ze with
// CAP_NET_ADMIN as an ambient capability, so access(2) would answer EACCES for a file ze
// can write. faccessat2 with AT_EACCESS keeps the effective credentials, but where the
// kernel or a seccomp profile refuses faccessat2, golang.org/x/sys/unix Faccessat falls
// back to the stat mode bits and answers root "writable" on a read-only mount.
//
// The open cannot change a value. The kernel runs a sysctl's handler only from read(2)
// and write(2) (proc_sys_read and proc_sys_write, through proc_sys_call_handler);
// proc_sys_open only takes a poll event, proc_sys_file_operations has no release, and the
// open carries neither O_CREAT nor O_TRUNC.
func probeESPFormTUNSysctlWritable(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0) //nolint:gosec // path is built from constants: /proc/sys/net/ipv4/conf/default plus a setting's leaf
	if err != nil {
		return fmt.Errorf("dataplane: open %s to learn whether Ze may set it: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("dataplane: close %s, which the probe opened and did not write: %w", path, err)
	}
	return nil
}

// inject writes one complete IP datagram into ze-espform, which the stack then receives
// as if it arrived on that device. A TUN write fills no header field, so pkt carries
// its own IPv4 header checksum (writeESPForm computes it).
func (t *espFormTUN) inject(pkt []byte) error {
	n, err := unix.Write(t.fd, pkt)
	if err != nil {
		return err
	}
	if n != len(pkt) {
		return fmt.Errorf("dataplane: wrote %d of %d octets into %s", n, len(pkt), espFormTUNName)
	}
	return nil
}

// Close releases the fd, which removes the device. It MUST be called once, after the
// reader that writes into it has stopped.
func (t *espFormTUN) Close() error {
	return unix.Close(t.fd)
}
