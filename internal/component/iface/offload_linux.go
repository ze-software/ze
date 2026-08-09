// Design: docs/architecture/iface/offload.md -- ethtool offload and sysfs steering

//go:build linux

package iface

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ethtool legacy per-feature ioctl command codes.
const (
	ethtoolSSG  = 0x00000019 // ETHTOOL_SSG  (set scatter-gather)
	ethtoolSTSO = 0x0000001f // ETHTOOL_STSO (set TCP segmentation offload)
	ethtoolSGSO = 0x00000024 // ETHTOOL_SGSO (set generic segmentation offload)
	ethtoolSLRO = 0x00000028 // ETHTOOL_SLRO (set large receive offload)
	ethtoolSGRO = 0x0000002c // ETHTOOL_SGRO (set generic receive offload)
)

const sysClassNetDir = "/sys/class/net"

// ethtoolValue mirrors struct ethtool_value { __u32 cmd; __u32 data; }.
type ethtoolValue struct {
	cmd  uint32
	data uint32
}

// ifreqOffload mirrors struct ifreq for SIOCETHTOOL. Same layout as
// host/ethtool_linux.go ifreqEthtool but local to avoid coupling.
type ifreqOffload struct {
	name [unix.IFNAMSIZ]byte
	data uintptr
	_    [16]byte
}

// applyOffloads applies the configured offload settings for the named interface.
// nil config means no offload block: no calls made. Each non-nil *bool field
// triggers exactly one ethtool ioctl or sysfs write.
func applyOffloads(ifaceName string, cfg *offloadConfig) {
	if cfg == nil {
		return
	}
	log := loggerPtr.Load()

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		log.Warn("iface offload: socket", "iface", ifaceName, "err", err)
		return
	}
	defer func() { _ = unix.Close(fd) }()

	applyEthtool := func(name string, cmd uint32, enable *bool) {
		if enable == nil {
			return
		}
		var val uint32
		if *enable {
			val = 1
		}
		if err := setEthtoolFeature(fd, ifaceName, cmd, val); err != nil {
			log.Warn("iface offload: ethtool feature", "iface", ifaceName, "feature", name, "err", err)
		}
	}

	applyEthtool("gro", ethtoolSGRO, cfg.GRO)
	applyEthtool("gso", ethtoolSGSO, cfg.GSO)
	applyEthtool("sg", ethtoolSSG, cfg.SG)
	applyEthtool("tso", ethtoolSTSO, cfg.TSO)
	applyEthtool("lro", ethtoolSLRO, cfg.LRO)

	if cfg.HWTCOffload != nil {
		if err := setEthtoolFeatureByName(fd, ifaceName, "hw-tc-offload", *cfg.HWTCOffload); err != nil {
			log.Warn("iface offload: hw-tc-offload", "iface", ifaceName, "err", err)
		}
	}

	if cfg.RPS != nil {
		if err := applyRPS(ifaceName, *cfg.RPS); err != nil {
			log.Warn("iface offload: rps", "iface", ifaceName, "err", err)
		}
	}
	if cfg.RFS != nil {
		if err := applyRFSQueues(ifaceName, *cfg.RFS); err != nil {
			log.Warn("iface offload: rfs", "iface", ifaceName, "err", err)
		}
	}
}

// applyRFSGlobal sets /proc/sys/net/core/rps_sock_flow_entries once based
// on whether any interface in the config enables RFS. Called from applyConfig
// after all per-interface offloads so the global is not toggled per-interface.
func applyRFSGlobal(cfg *ifaceConfig) {
	anyRFS, anyRFSConfigured := rfsState(cfg)
	if !anyRFSConfigured {
		return
	}

	globalPath := "/proc/sys/net/core/rps_sock_flow_entries"
	var value string
	if anyRFS {
		value = "32768"
	} else {
		value = "0"
	}
	if err := os.WriteFile(globalPath, []byte(value), 0o600); err != nil {
		loggerPtr.Load().Warn("iface offload: rfs global", "err", err)
	}
}

// rfsState scans all L2 interfaces for RFS config. Returns (anyEnabled, anyConfigured).
// anyConfigured is true if at least one interface has RFS set (true or false).
func rfsState(cfg *ifaceConfig) (anyEnabled, anyConfigured bool) {
	check := func(o *offloadConfig) {
		if o != nil && o.RFS != nil {
			anyConfigured = true
			if *o.RFS {
				anyEnabled = true
			}
		}
	}
	for _, e := range cfg.Ethernet {
		check(e.Offload)
	}
	for _, e := range cfg.Dummy {
		check(e.Offload)
	}
	for _, e := range cfg.Veth {
		check(e.Offload)
	}
	for _, e := range cfg.Bridge {
		check(e.Offload)
	}
	return anyEnabled, anyConfigured
}

func setEthtoolFeature(fd int, ifaceName string, cmd, val uint32) error {
	ev := ethtoolValue{cmd: cmd, data: val}
	var ifr ifreqOffload
	copy(ifr.name[:], ifaceName)
	ifr.data = uintptr(unsafe.Pointer(&ev)) //nolint:gosec // ethtool SIOCETHTOOL ioctl requires a raw pointer to ethtoolValue

	_, _, errno := unix.Syscall(unix.SYS_IOCTL,
		uintptr(fd), uintptr(unix.SIOCETHTOOL), uintptr(unsafe.Pointer(&ifr))) //nolint:gosec // ethtool SIOCETHTOOL ioctl requires a raw ifreq pointer
	if errno != 0 {
		return fmt.Errorf("ethtool cmd 0x%x on %s: %w", cmd, ifaceName, errno)
	}
	return nil
}

// setEthtoolFeatureByName uses the ETHTOOL_SFEATURES API to set a named
// feature. Used for hw-tc-offload which has no legacy per-feature ioctl.
func setEthtoolFeatureByName(fd int, ifaceName, feature string, enable bool) error {
	idx, err := findFeatureIndex(fd, ifaceName, feature)
	if err != nil {
		return err
	}
	return setSFeature(fd, ifaceName, idx, enable)
}

// ETHTOOL_GSSET_INFO, ETHTOOL_GSTRINGS, ETHTOOL_SFEATURES constants.
const (
	ethtoolGSsetInfo  = 0x00000037
	ethtoolGStrings   = 0x0000001b
	ethtoolSFeatures  = 0x00000013
	ethSSFeatures     = 0x00000004 // ETH_SS_FEATURES string set ID
	ethtoolStringSize = 32         // ETH_GSTRING_LEN
	maxFeatureCount   = 4096       // sane upper bound; Linux ~80 features
)

func ethtoolIoctl(fd int, ifaceName string, data unsafe.Pointer) error {
	var ifr ifreqOffload
	copy(ifr.name[:], ifaceName)
	ifr.data = uintptr(data)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL,
		uintptr(fd), uintptr(unix.SIOCETHTOOL), uintptr(unsafe.Pointer(&ifr))) //nolint:gosec // ethtool SIOCETHTOOL ioctl requires a raw ifreq pointer
	if errno != 0 {
		return errno
	}
	return nil
}

// ssetInfo mirrors struct ethtool_sset_info from <linux/ethtool.h>.
// ssetMask is __u64 in the kernel; data[] follows at offset 16.
type ssetInfo struct {
	cmd       uint32
	reserved  uint32
	ssetMask  uint64
	ssetCount uint32
}

func findFeatureIndex(fd int, ifaceName, feature string) (uint32, error) {
	info := ssetInfo{cmd: ethtoolGSsetInfo, ssetMask: 1 << ethSSFeatures}
	if err := ethtoolIoctl(fd, ifaceName, unsafe.Pointer(&info)); err != nil { //nolint:gosec // ethtool struct pointer
		return 0, fmt.Errorf("GSSET_INFO: %w", err)
	}
	count := info.ssetCount
	if count == 0 {
		return 0, fmt.Errorf("no features reported for %s", ifaceName)
	}
	if count > maxFeatureCount {
		return 0, fmt.Errorf("feature count %d exceeds maximum %d for %s", count, maxFeatureCount, ifaceName)
	}

	headerSize := 12 // cmd(4) + string_set(4) + len(4)
	bufSize := headerSize + int(count)*ethtoolStringSize
	buf := make([]byte, bufSize)
	*(*uint32)(unsafe.Pointer(&buf[0])) = ethtoolGStrings                        //nolint:gosec // kernel struct layout
	*(*uint32)(unsafe.Pointer(&buf[4])) = ethSSFeatures                          //nolint:gosec // kernel struct layout
	*(*uint32)(unsafe.Pointer(&buf[8])) = count                                  //nolint:gosec // kernel struct layout
	if err := ethtoolIoctl(fd, ifaceName, unsafe.Pointer(&buf[0])); err != nil { //nolint:gosec // ethtool struct pointer
		return 0, fmt.Errorf("GSTRINGS: %w", err)
	}

	for i := range count {
		off := headerSize + int(i)*ethtoolStringSize
		name := strings.TrimRight(string(buf[off:off+ethtoolStringSize]), "\x00")
		if name == feature {
			return i, nil
		}
	}
	return 0, fmt.Errorf("feature %q not found on %s", feature, ifaceName)
}

func setSFeature(fd int, ifaceName string, idx uint32, enable bool) error {
	nblocks := (idx / 32) + 1
	headerSize := 8 // cmd(4) + size(4)
	blockSize := 16 // 4 uint32 fields per block
	bufSize := headerSize + int(nblocks)*blockSize
	buf := make([]byte, bufSize)
	*(*uint32)(unsafe.Pointer(&buf[0])) = ethtoolSFeatures //nolint:gosec // kernel struct layout
	*(*uint32)(unsafe.Pointer(&buf[4])) = nblocks          //nolint:gosec // kernel struct layout

	blockIdx := idx / 32
	bitIdx := idx % 32
	blockOff := headerSize + int(blockIdx)*blockSize
	*(*uint32)(unsafe.Pointer(&buf[blockOff])) = 1 << bitIdx //nolint:gosec // kernel struct layout
	if enable {
		*(*uint32)(unsafe.Pointer(&buf[blockOff+4])) = 1 << bitIdx //nolint:gosec // kernel struct layout
	}

	if err := ethtoolIoctl(fd, ifaceName, unsafe.Pointer(&buf[0])); err != nil { //nolint:gosec // ethtool struct pointer
		return fmt.Errorf("SFEATURES idx=%d on %s: %w", idx, ifaceName, err)
	}
	return nil
}

// applyRPS enables or disables Receive Packet Steering by writing to
// /sys/class/net/<dev>/queues/rx-*/rps_cpus. Enable sets all CPUs;
// disable sets 0 (no steering).
func applyRPS(ifaceName string, enable bool) error {
	pattern := filepath.Join(sysClassNetDir, ifaceName, "queues", "rx-*", "rps_cpus")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("rps glob: %w", err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("no rx queues found for %s (rps not supported)", ifaceName)
	}

	var value string
	if enable {
		cpuMask, maskErr := allCPUMask()
		if maskErr != nil {
			return maskErr
		}
		value = cpuMask
	} else {
		value = "0"
	}

	for _, path := range matches {
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

// applyRFSQueues sets per-queue rps_flow_cnt for the named interface.
// The global rps_sock_flow_entries is set once by applyRFSGlobal.
func applyRFSQueues(ifaceName string, enable bool) error {
	queuePattern := filepath.Join(sysClassNetDir, ifaceName, "queues", "rx-*", "rps_flow_cnt")
	matches, err := filepath.Glob(queuePattern)
	if err != nil {
		return fmt.Errorf("rfs glob: %w", err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("no rx queues found for %s (rfs not supported)", ifaceName)
	}

	var perQueue string
	if enable {
		perQ := max(32768/len(matches), 1)
		perQueue = strconv.Itoa(perQ)
	} else {
		perQueue = "0"
	}

	for _, path := range matches {
		if err := os.WriteFile(path, []byte(perQueue), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

// allCPUMask returns a hex bitmask with all online CPUs set.
func allCPUMask() (string, error) {
	data, err := os.ReadFile("/sys/devices/system/cpu/online")
	if err != nil {
		return "", fmt.Errorf("read cpu online: %w", err)
	}
	line := strings.TrimSpace(string(data))

	ncpu := 0
	for part := range strings.SplitSeq(line, ",") {
		if lo, hi, ok := strings.Cut(part, "-"); ok {
			loN, loErr := strconv.Atoi(lo)
			hiN, hiErr := strconv.Atoi(hi)
			if loErr == nil && hiErr == nil {
				ncpu += hiN - loN + 1
			}
		} else {
			ncpu++
		}
	}
	if ncpu == 0 {
		ncpu = 1
	}

	nwords := (ncpu + 31) / 32
	parts := make([]string, nwords)
	for i := range parts {
		bitsInWord := min(ncpu-i*32, 32)
		mask := uint32((1 << bitsInWord) - 1)
		var scratch [10]byte
		parts[nwords-1-i] = string(strconv.AppendUint(scratch[:0], uint64(mask), 16))
	}
	return textbuf.Join(parts, ","), nil
}
