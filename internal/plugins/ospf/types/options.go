// Design: docs/architecture/ospf/ospf-1-types.md -- OSPF Options bit field
// Related: lstype.go -- LSA headers carry Options next to LS type

package types

// OptionsLen is the Options field width in octets.
const OptionsLen = 1

// Options is the OSPF capability bit field carried in Hellos, DDs, and LSAs. It is
// 32-bit in memory so one shared type holds both the OSPFv2 8-bit Options (RFC 2328)
// and the OSPFv3 24-bit Options (RFC 5340 sec A.2). The wire width is the codec's
// concern: the OSPFv2 codec writes/reads one octet (WriteTo / OptionsFromBytes); the
// OSPFv3 codec uses three. The bit assignments differ per version; each codec sets the
// bits it understands.
type Options uint32

const (
	// OptionE is the ExternalRoutingCapability bit.
	OptionE Options = 0x02
	// OptionMC is the multicast forwarding capability bit.
	OptionMC Options = 0x04
	// OptionNP is the NSSA Type-7 capability bit.
	OptionNP Options = 0x08
	// OptionL is the LLS data-block capability bit.
	OptionL Options = 0x10
	// OptionDC is the demand-circuit capability bit.
	OptionDC Options = 0x20
	// OptionO is the opaque-LSA capability bit.
	OptionO Options = 0x40
	// OptionDN is the down-bit used by VPN extensions.
	OptionDN Options = 0x80
)

// OptionsFromBytes decodes a one-octet Options field.
func OptionsFromBytes(b []byte) (Options, error) {
	if len(b) != OptionsLen {
		return 0, ErrWrongLength
	}
	return Options(b[0]), nil
}

// Has reports whether bit is set.
func (o Options) Has(bit Options) bool { return o&bit != 0 }

// Set returns o with bit set.
func (o Options) Set(bit Options) Options { return o | bit }

// Clear returns o with bit cleared.
func (o Options) Clear(bit Options) Options { return o &^ bit }

// WriteTo writes the one-octet Options field into buf at off.
func (o Options) WriteTo(buf []byte, off int) int {
	buf[off] = byte(o)
	return OptionsLen
}

// AppendTo appends the comma-separated set option names to dst without allocating.
func (o Options) AppendTo(dst []byte) []byte {
	wrote := false
	for _, item := range optionNames {
		if !o.Has(item.bit) {
			continue
		}
		if wrote {
			dst = append(dst, ',')
		}
		dst = append(dst, item.name...)
		wrote = true
	}
	if !wrote {
		dst = append(dst, "none"...)
	}
	return dst
}

// String returns a comma-separated list of set option names.
func (o Options) String() string {
	var scratch [32]byte
	return string(o.AppendTo(scratch[:0]))
}

var optionNames = [...]struct {
	bit  Options
	name string
}{
	{OptionE, "E"},
	{OptionMC, "MC"},
	{OptionNP, "N/P"},
	{OptionL, "L"},
	{OptionDC, "DC"},
	{OptionO, "O"},
	{OptionDN, "DN"},
}
