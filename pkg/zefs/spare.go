// Design: docs/architecture/zefs-format.md -- spare capacity policy
// Related: store.go -- writeFileNoFlush sizes new and grown slots through the policy

package zefs

import "fmt"

// SpareDefault is the percentage a writer reserves when no Spare option is given.
const SpareDefault = 10

// spareMax is the largest percentage Spare accepts.
const spareMax = 100

// Option configures how a BlobStore sizes the slots it writes.
type Option func(*writePolicy) error

// writePolicy is the spare-capacity policy of one open BlobStore. It is a
// property of the writer and is never persisted: every netcapstring header
// carries its own capacity, so a file written with any policy opens with any
// reader, and the next writer states its own policy.
type writePolicy struct {
	sparePercent int
}

// Spare sets the percentage of a value's length a writer reserves beyond it,
// for the key slot and the data slot alike. 0 writes every slot exact-fit, so
// the first growth of a value is a full rewrite. The container is always
// written exact-fit, whatever the percentage. The range is 0 to 100.
func Spare(percent int) Option {
	return func(policy *writePolicy) error {
		if percent < 0 {
			return fmt.Errorf("zefs: spare %d is out of range 0 to %d", percent, spareMax)
		}
		if percent > spareMax {
			return fmt.Errorf("zefs: spare %d is out of range 0 to %d", percent, spareMax)
		}
		policy.sparePercent = percent
		return nil
	}
}

// newWritePolicy applies opts over the default policy. The first invalid
// option refuses the whole open, so a writer never runs with a policy it did
// not ask for.
func newWritePolicy(opts []Option) (writePolicy, error) {
	policy := writePolicy{sparePercent: SpareDefault}
	for _, opt := range opts {
		if err := opt(&policy); err != nil {
			return writePolicy{}, err
		}
	}
	return policy, nil
}

// capacity returns the slot capacity for used bytes: used plus sparePercent of
// used, rounded down. Spare 0 gives used, and the default 10 gives
// used + used/10.
func (p writePolicy) capacity(used int) int {
	return used + used*p.sparePercent/100
}
