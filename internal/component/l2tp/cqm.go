// Design: plan/spec-l2tp-9-observer.md -- CQM bucket aggregation
// Related: observer.go -- observer owns sample rings, calls addEcho

package l2tp

import "time"

// BucketState tags a CQM bucket with the session's lifecycle phase.
type BucketState uint8

const (
	BucketStateEstablished BucketState = iota
	BucketStateNegotiating
	BucketStateDown
)

const (
	bucketStateEstablishedStr = "established"
	bucketStateNegotiatingStr = "negotiating"
	bucketStateDownStr        = "down"
)

func (s BucketState) String() string {
	switch s {
	case BucketStateEstablished:
		return bucketStateEstablishedStr
	case BucketStateNegotiating:
		return bucketStateNegotiatingStr
	case BucketStateDown:
		return bucketStateDownStr
	default:
		return stateUnknown
	}
}

// CQMBucket holds one 100-second aggregated sample.
type CQMBucket struct {
	Start     time.Time
	State     BucketState
	EchoCount uint16
	MinRTT    time.Duration
	MaxRTT    time.Duration
	SumRTT    time.Duration
}

// AvgRTT returns the mean RTT, or zero when no echoes were recorded.
func (b *CQMBucket) AvgRTT() time.Duration {
	if b.EchoCount == 0 {
		return 0
	}
	return b.SumRTT / time.Duration(b.EchoCount)
}

// addEcho folds one echo RTT sample into the running aggregation.
func (b *CQMBucket) addEcho(rtt time.Duration) {
	if rtt < 0 {
		rtt = 0
	}
	b.EchoCount++
	b.SumRTT += rtt
	if b.EchoCount == 1 || rtt < b.MinRTT {
		b.MinRTT = rtt
	}
	if rtt > b.MaxRTT {
		b.MaxRTT = rtt
	}
}

// BucketInterval is the fixed duration of one CQM sample bucket.
const BucketInterval = 100 * time.Second

// sampleRing is a circular buffer of CQMBucket values.
type sampleRing struct {
	buckets []CQMBucket
	head    int
	count   int
}

func newSampleRing(capacity int) *sampleRing {
	return &sampleRing{buckets: make([]CQMBucket, capacity)}
}

func (r *sampleRing) append(b CQMBucket) {
	r.buckets[r.head] = b
	r.head = (r.head + 1) % len(r.buckets)
	if r.count < len(r.buckets) {
		r.count++
	}
}

func (r *sampleRing) snapshot() []CQMBucket {
	if r.count == 0 {
		return nil
	}
	result := make([]CQMBucket, r.count)
	start := (r.head - r.count + len(r.buckets)) % len(r.buckets)
	for i := range r.count {
		result[i] = r.buckets[(start+i)%len(r.buckets)]
	}
	return result
}

func (r *sampleRing) reset() {
	for i := range r.buckets {
		r.buckets[i] = CQMBucket{}
	}
	r.head = 0
	r.count = 0
}
