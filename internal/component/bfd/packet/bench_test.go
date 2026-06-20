package packet

import (
	"testing"
)

// BenchmarkRoundTrip exercises the buffer-pool / WriteTo / ParseControl
// path that the engine uses on every received and transmitted packet.
// b.ReportAllocs() catches any regression that puts an allocation back
// onto the hot path.
func BenchmarkRoundTrip(b *testing.B) {
	c := Control{
		Version:                   1,
		State:                     StateUp,
		DetectMult:                3,
		Length:                    MandatoryLen,
		MyDiscriminator:           0xCAFEBABE,
		YourDiscriminator:         0xDEADBEEF,
		DesiredMinTxInterval:      300_000,
		RequiredMinRxInterval:     300_000,
		RequiredMinEchoRxInterval: 0,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		pb := Acquire()
		buf := pb.Data()
		c.WriteTo(buf, 0)
		got, _, err := ParseControl(buf[:MandatoryLen])
		Release(pb)
		if err != nil {
			b.Fatalf("ParseControl: %v", err)
		}
		if got != c {
			b.Fatalf("round-trip mismatch")
		}
	}
}
