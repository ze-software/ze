// Design: docs/architecture/wire/buffer-writer.md — buffer-first encoding
// RFC: rfc/short/rfc7296.md — Vendor ID payload (Section 3.12)
package wire

// PayloadVendorID is the Vendor ID payload (type 43).
type PayloadVendorID struct {
	VendorIDData []byte
}

func (p *PayloadVendorID) Type() uint8 { return PayloadTypeVendorID }

func (p *PayloadVendorID) WriteTo(buf []byte, off int) int {
	copy(buf[off:], p.VendorIDData)
	return len(p.VendorIDData)
}

func (p *PayloadVendorID) Len() int { return len(p.VendorIDData) }

func (p *PayloadVendorID) ReadFrom(data []byte) error {
	p.VendorIDData = make([]byte, len(data))
	copy(p.VendorIDData, data)
	return nil
}
