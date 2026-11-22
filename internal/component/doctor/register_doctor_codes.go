// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/doctor.

package doctor

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorPlatformDetect                       = "doctor-platform-detect"
	codeDoctorPlatformUnknown                      = "doctor-platform-unknown"
	codeDoctorPlatformPerm                         = "doctor-platform-perm"
	codeDoctorPlatformContainerRO                  = "doctor-platform-container-ro"
	codeDoctorStorageRecoveryPending               = "doctor-storage-recovery-pending"
	codeDoctorStorePermissions                     = "doctor-store-permissions"
	codeDoctorConfigMissing                        = "doctor-config-missing"
	codeDoctorListenUnavailable                    = "doctor-listen-unavailable"
	codeDoctorListenerSetUnknown                   = "doctor-listener-set-unknown"
	codeDoctorDiskSpace                            = "doctor-disk-space"
	codeDoctorRuntimeNotChecked                    = "doctor-runtime-not-checked"
	codeDoctorClockSkew                            = "doctor-clock-skew"
	codeDoctorAS112WatchdogMissingWithdraw         = "doctor-as112-watchdog-missing-withdraw"
	codeDoctorAS112GlobalOriginUncoordinated       = "doctor-as112-global-origin-uncoordinated"
	codeDoctorAS112RedistributeOriginUncoordinated = "doctor-as112-redistribute-origin-uncoordinated"
	codeDoctorAS112RedistributeNotImported         = "doctor-as112-redistribute-not-imported"
	codeDoctorDaemonCapabilityMissing              = "doctor-daemon-capability-missing"
	codeDoctorBFDPort                              = "doctor-bfd-port"
	codeDoctorIPsecListen                          = "doctor-ipsec-listen"
	codeDoctorTFTPListen                           = "doctor-tftp-listen"
	codeDoctorImageListen                          = "doctor-image-listen"
	codeDoctorNTPListen                            = "doctor-ntp-listen"
	codeDoctorMachineIDMissing                     = "doctor-machine-id-missing"
	codeDoctorWriteDestination                     = "doctor-write-destination"
	codeDoctorRandomSeed                           = "doctor-random-seed"
	codeDoctorConfigRootUnclaimed                  = "doctor-config-root-unclaimed"
	codeDoctorConfigClaimsUnavailable              = "doctor-config-claims-unavailable"
	codeDoctorStorageNotGrown                      = "doctor-storage-not-grown"
	exampleDoctorJSON                              = "ze doctor --json"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorPlatformDetect,
		Title:       "Platform detection failed",
		Description: "Runtime platform detection encountered an error.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        codeDoctorPlatformUnknown,
		Title:       "Unknown runtime platform",
		Description: "The runtime platform could not be identified as gokrazy, systemd, container, or plain Linux.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        codeDoctorPlatformPerm,
		Title:       "Gokrazy /perm not writable",
		Description: "Running on gokrazy but /perm is not writable. Config persistence and state storage require a writable /perm partition.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        codeDoctorPlatformContainerRO,
		Title:       "Container with read-only root",
		Description: "Running in a container with a read-only root filesystem. Writable volumes must be mounted for config and state.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        codeDoctorStorageRecoveryPending,
		Title:       "Full-store recovery source remains unresolved",
		Description: "A private full-store recovery staging directory remains, or the store's retained directory authority could not be inspected. Another recovery source is refused until the retained source is resolved. The diagnostic names the source and its inspection command; no doctor check consumes it.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        codeDoctorStorePermissions,
		Title:       "Store permissions refused",
		Description: "The store contains a directory or file with unsafe permissions, a different owner, or an unsupported node such as a symlink. The diagnostic names the path and the required chmod or chown. Ownership is checked against the caller even for root.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        codeDoctorConfigMissing,
		Title:       "Config file not found",
		Description: "No config file could be resolved from storage. Ze cannot determine which services to check.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        doctorConfigParse,
		Title:       "Config parse or load failure",
		Description: "The config file was found but could not be parsed, or the load validation the daemon applies at startup refused it. The message names the failing line or check; no later check ran.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        codeDoctorListenUnavailable,
		Title:       "Listen address unavailable",
		Description: "A configured listen address/port could not be bound.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        codeDoctorListenerSetUnknown,
		Title:       "Listener set cannot be derived",
		Description: "ze doctor could not load the YANG schema it derives the listen addresses to check from, so no schema-declared listener (web, SSH, gNMI, API and the rest) was checked. The schema is built into ze, so this means the ze binary is incomplete: install a complete ze build and run ze doctor again.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-listener-set-unknown"},
	},
	{
		Code:        codeDoctorDiskSpace,
		Title:       "Low disk space on config partition",
		Description: "The partition containing the config directory has less than 5% free space. Managed store writes may fail.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        codeDoctorRuntimeNotChecked,
		Title:       "Runtime checks did not run",
		Description: "Some doctor checks read the live state of the running daemon (the runtime phase). This ze doctor answered in its own process, because no daemon answered it: the daemon is not running, or the SSH login to it failed. Those checks did not run, so this answer says nothing about the daemon's live state. Start the daemon with ze start, then run show doctor from an account that can log in to it.",
		Examples:    []string{"ze start", "ze show doctor"},
	},
	{
		Code:        codeDoctorClockSkew,
		Title:       "System clock skewed",
		Description: "The system clock differs from NTP by more than 5 minutes. TLS validation, BGP timers, and log correlation may be affected.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        codeDoctorAS112WatchdogMissingWithdraw,
		Title:       "AS112 route missing watchdog withdraw marker",
		Description: "A BGP update block announces an AS112 covering prefix (192.175.48.0/24, 192.31.196.0/24, 2620:4f:8000::/48, or 2001:4:112::/48) without a watchdog{withdraw true} marker. The marker's absence defaults to already-announced, so the route is advertised at startup before AS112 health is confirmed (RFC 7534 Section 3.3).",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-as112-watchdog-missing-withdraw"},
	},
	{
		Code:        codeDoctorAS112GlobalOriginUncoordinated,
		Title:       "AS112 global origin override to a public ASN",
		Description: "A BGP session sets asn.local 112 with the replace-as local-option while eBGP-peering a non-private-use remote ASN (RFC 6996 Section 4), making this node an uncoordinated global AS112 origin. RFC 7534 Section 3.2/Section 5 requires coordination before deploying outside a local-use mirror.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-as112-global-origin-uncoordinated"},
	},
	{
		Code:        codeDoctorAS112RedistributeOriginUncoordinated,
		Title:       "AS112 redistribute origin to a public ASN",
		Description: "The as112 service originates its covering prefixes as AS112 (asn 112, the default) via redistribute { destination bgp { import as112 } } while an eBGP session to a non-private-use remote ASN (RFC 6996 Section 4) exists, making this node an uncoordinated global AS112 origin. RFC 7534 Section 3.2/Section 5 requires coordination; restrict the route with an egress community/prefix filter or set an operator/private asn.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-as112-redistribute-origin-uncoordinated"},
	},
	{
		Code:        codeDoctorAS112RedistributeNotImported,
		Title:       "AS112 redistribute knob set but not imported into BGP",
		Description: "service as112 is enabled and sets a redistribute-only knob (an explicit asn or a community) but no redistribute { destination bgp { import as112 } } is configured. Those knobs only affect the BGP-originated covering prefixes and are ignored without the import, so the covering prefixes are never originated into BGP -- the common wiring mistake where an operator expects the routes but forgot the import. Add the import, or remove the knob if the node serves DNS only.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-as112-redistribute-not-imported"},
	},
	{
		Code:  codeDoctorDaemonCapabilityMissing,
		Title: "The ze daemon runs without part of the daemon capability set",
		Description: "An informational row: the ze daemon lacks one of the capabilities the " +
			"systemd unit grants (CAP_NET_ADMIN, CAP_NET_RAW, CAP_NET_BIND_SERVICE, CAP_BPF), or " +
			"Ze cannot determine its own capability set. This is not a problem by itself and never " +
			"makes the box not ready: a config that uses nothing a missing capability serves runs " +
			"without it, as an interfaces-and-SSH config runs with CAP_NET_ADMIN alone. Each " +
			"configured feature that needs a capability checks for it and reports its own impact " +
			"under its own code. The fix names the remedy for this process: when the daemon runs " +
			"under ze.service, stop ze with sudo systemctl stop ze and rewrite the unit with " +
			"sudo ze install systemd --force --start; otherwise restart ze as root or holding the " +
			"capabilities named.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-daemon-capability-missing"},
	},
	{
		Code:        codeDoctorBFDPort,
		Title:       "BFD control port unavailable",
		Description: "BFD is configured but the UDP control port could not be bound.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-bfd-port"},
	},
	{
		Code:        codeDoctorIPsecListen,
		Title:       "IPsec IKE listener unavailable",
		Description: "IPsec is configured but an IKE UDP listener port could not be bound.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-ipsec-listen"},
	},
	{
		Code:        codeDoctorTFTPListen,
		Title:       "TFTP listener unavailable",
		Description: "The TFTP server is configured but its UDP listener port could not be bound.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-tftp-listen"},
	},
	{
		Code:        codeDoctorImageListen,
		Title:       "Image server listener unavailable",
		Description: "The image server is configured but its HTTP listener port could not be bound.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-image-listen"},
	},
	{
		Code:        codeDoctorNTPListen,
		Title:       "NTP listener unavailable",
		Description: "NTP is configured but UDP port 123 could not be bound.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-ntp-listen"},
	},
	{
		Code:        codeDoctorMachineIDMissing,
		Title:       "Machine ID missing",
		Description: "The platform expects /etc/machine-id to exist and contain a stable machine identifier, but it is missing or empty.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-machine-id-missing"},
	},
	{
		Code:        codeDoctorWriteDestination,
		Title:       "Write destination not writable",
		Description: "A configured file write destination (persist path, output directory) is not writable.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-write-destination"},
	},
	{
		Code:        codeDoctorRandomSeed,
		Title:       "No random seed persistence",
		Description: "The platform has no known random-seed persistence service. On gokrazy, randomd saves to /perm/random.seed; on systemd, systemd-random-seed.service saves to /var/lib/systemd/random-seed. Without seed persistence, early-boot entropy may be poor, weakening cryptographic operations like BGP TCP-AO or TLS key generation.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-random-seed"},
	},
	{
		Code:         codeDoctorConfigRootUnclaimed,
		Title:        "Config subtree delivered to nobody",
		Description:  "A config subtree is stored but no plugin and no handler receives it, so it has no effect. The daemon selects plugins for a config change by matching the changed path against the config roots each plugin declares; a path that matches nothing is accepted and logged at Info level only. Either the owning plugin is not built into this binary, or it did not load, or its config root declaration is missing. Check `ze plugin list` for the owning plugin.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-config-root-unclaimed"},
		RelatedCodes: []string{codeDoctorConfigClaimsUnavailable},
	},
	{
		Code:         codeDoctorConfigClaimsUnavailable,
		Title:        "Config delivery could not be checked",
		Description:  "The doctor could not build the list of config roots this build delivers, so it did not check whether the configured subtrees reach anything. This is reported rather than passed over: a check that cannot see its subject has cleared nothing.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-config-claims-unavailable"},
		RelatedCodes: []string{codeDoctorConfigRootUnclaimed},
	},
	{
		Code:        codeDoctorStorageNotGrown,
		Title:       "Persistent storage is smaller than its disk",
		Description: "On an appliance, the persistent partition or its filesystem does not reach the end of the disk, so the rest of the disk is not used. The installer grows both when it installs a system. A system that was written to its disk another way, or whose growth failed at install, keeps the size of the image. Run request system storage grow to grow the storage with no reboot, and show host storage to read the sizes. The same code is reported as a warning when the sizes cannot be read, because the storage may then be smaller than its disk.",
		Examples:    []string{exampleDoctorJSON, "ze request system storage grow", "ze show host storage"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: doctor: doctor code registration refused: " + err.Error())
		}
	}
}
