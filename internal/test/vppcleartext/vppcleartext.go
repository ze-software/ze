// Design: docs/architecture/testing/interop.md -- the cleartext probe of the real VPP ipsec scenario
// Related: ../../component/ike/dataplane/vpp_real_cleartext_integration_test.go -- TestVPPRealCleartextProbe, which installs this SPD and SA in VPP
// Related: ../../le/test/deployment/vppevidencecleartext.go -- runCleartextProbe, which sends frames against them
//
// Package vppcleartext declares, once, the inbound SPD and SA of the real VPP
// cleartext probe. Two programs need the same values: the IKE dataplane test
// that installs the entries and the SA through the VPP backend, and the
// deployment run that builds its probe frames from them and decrypts against the
// SA's key. Each imports this package, so neither carries a copy.
package vppcleartext

// The probe interface's subnet and the three inbound entries. Local is the
// local side of every entry; each entry names a different remote /24, so no
// frame matches two entries and the priorities decide nothing among them.
const (
	Local   = "100.64.45.0/24"
	Protect = "100.64.46.0/24"
	Discard = "100.64.47.0/24"
	Bypass  = "100.64.48.0/24" //nolint:gosec // G101: IPsec SPD BYPASS policy IPv4 test prefix, not a credential.
)

// The inbound tunnel-mode SA the PROTECT entry names.
const (
	// Host is the probe interface's own address, which the deployment run gives
	// it, and the SA's local tunnel end.
	Host = "100.64.45.1"
	// TunnelRemote is the SA's remote tunnel end, the source of its ESP.
	TunnelRemote = "100.64.46.1"
	// SPI is the SA's SPI.
	SPI = uint32(0x45450001)
	// KeyLen is the SA's AES-256-GCM key: 32 octets of key, then the SaltLen
	// octets of salt RFC 4106 appends to it.
	KeyLen = 36
	// SaltLen is the salt at the end of the key.
	SaltLen = 4
	// keyFirst is the first octet of the key; each next octet is one more.
	keyFirst = 0x40
)

// ReportPrefix starts the line the dataplane test prints to report the SPD it
// bound, which the deployment run reads.
const ReportPrefix = "ze-vpp-cleartext:"

// Key answers the SA's key: keyFirst, keyFirst+1, ... for KeyLen octets.
func Key() [KeyLen]byte {
	var key [KeyLen]byte
	for i := range key {
		key[i] = byte(keyFirst + i)
	}
	return key
}
