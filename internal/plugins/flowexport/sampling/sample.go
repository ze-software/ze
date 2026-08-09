// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- Sampled packet types

package sampling

// SampledPacket holds a packet captured via tc sample + psample.
// Value type: Header is an owned copy of the truncated packet bytes.
type SampledPacket struct {
	IfIndex  uint32
	Rate     uint32
	OrigSize uint32
	Header   []byte
}

const (
	// DefaultGroup is the default psample group ID.
	DefaultGroup uint32 = 1

	// DefaultTruncSize is the default number of header bytes to capture.
	DefaultTruncSize uint32 = 128

	// SampleFilterPriority is the tc filter priority for the sample action.
	// Mirror uses priority 1; sampling uses 100 to avoid conflict.
	SampleFilterPriority uint16 = 100
)
