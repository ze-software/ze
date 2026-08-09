// Design: docs/architecture/storage/smart-health.md -- SMART disk health ioctl library
// Detail: smart_linux.go — ATA/NVMe ioctl detection and control

package smart

import (
	"encoding/binary"
	"strings"
)

// Info holds SMART health data obtained via direct ioctl.
// A nil Info means SMART detection was not attempted (e.g. testdata
// mode or device not found). A non-nil Info with Unavailable set means
// the device does not support SMART or privileges are insufficient.
type Info struct {
	Healthy         bool   `json:"healthy"`
	TempCelsius     int    `json:"temp-celsius,omitempty"`
	PowerOnHours    uint64 `json:"power-on-hours,omitempty"`
	ErrorCount      uint64 `json:"error-count"`
	PercentUsed     int    `json:"percent-used,omitempty"`
	AvailableSpare  int    `json:"available-spare,omitempty"`
	Unavailable     bool   `json:"unavailable,omitempty"`
	UnavailableNote string `json:"unavailable-note,omitempty"`
}

// SelfTestType identifies a SMART self-test type.
type SelfTestType uint8

const (
	SelfTestShort    SelfTestType = 1
	SelfTestExtended SelfTestType = 2
)

// ParseNVMeBuf extracts Info from a raw 512-byte NVMe SMART log page.
// NVMe spec 1.4, Figure 194: SMART / Health Information Log.
func ParseNVMeBuf(buf *[512]byte) *Info {
	info := &Info{}

	info.Healthy = buf[0] == 0

	if kelvin := int(binary.LittleEndian.Uint16(buf[1:3])); kelvin > 0 {
		info.TempCelsius = kelvin - 273
	}

	info.AvailableSpare = int(buf[3])
	info.PercentUsed = int(buf[5])

	info.PowerOnHours = binary.LittleEndian.Uint64(buf[128:136])
	info.ErrorCount = binary.LittleEndian.Uint64(buf[176:184])

	return info
}

// NvmeNamespace strips the partition suffix (e.g. "p1") from an NVMe
// device name so the admin ioctl targets the namespace, not a partition.
func NvmeNamespace(name string) string {
	idx := strings.LastIndex(name, "p")
	if idx < 0 {
		return name
	}
	suffix := name[idx+1:]
	for _, c := range suffix {
		if c < '0' || c > '9' {
			return name
		}
	}
	if suffix == "" {
		return name
	}
	return name[:idx]
}
