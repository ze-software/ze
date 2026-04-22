package l2tp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSampleRingAppendAndSnapshot(t *testing.T) {
	r := newSampleRing(3)
	require.Nil(t, r.snapshot())

	b1 := CQMBucket{Start: time.Unix(0, 0), EchoCount: 1}
	b2 := CQMBucket{Start: time.Unix(100, 0), EchoCount: 2}
	r.append(b1)
	r.append(b2)
	snap := r.snapshot()
	require.Len(t, snap, 2)
	require.Equal(t, uint16(1), snap[0].EchoCount)
	require.Equal(t, uint16(2), snap[1].EchoCount)
}

func TestSampleRingWrap(t *testing.T) {
	r := newSampleRing(2)
	r.append(CQMBucket{EchoCount: 1})
	r.append(CQMBucket{EchoCount: 2})
	r.append(CQMBucket{EchoCount: 3})

	snap := r.snapshot()
	require.Len(t, snap, 2)
	require.Equal(t, uint16(2), snap[0].EchoCount)
	require.Equal(t, uint16(3), snap[1].EchoCount)
}

func TestSampleRingReset(t *testing.T) {
	r := newSampleRing(3)
	r.append(CQMBucket{EchoCount: 1})
	r.reset()
	require.Nil(t, r.snapshot())
}

func TestCQMBucketAddEcho(t *testing.T) {
	var b CQMBucket
	b.addEcho(10 * time.Millisecond)
	b.addEcho(30 * time.Millisecond)
	b.addEcho(20 * time.Millisecond)

	require.Equal(t, uint16(3), b.EchoCount)
	require.Equal(t, 10*time.Millisecond, b.MinRTT)
	require.Equal(t, 30*time.Millisecond, b.MaxRTT)
	require.Equal(t, 60*time.Millisecond, b.SumRTT)
	require.Equal(t, 20*time.Millisecond, b.AvgRTT())
}

func TestCQMBucketAddEchoNegativeRTTClamped(t *testing.T) {
	var b CQMBucket
	b.addEcho(-5 * time.Millisecond)
	require.Equal(t, time.Duration(0), b.MinRTT)
	require.Equal(t, time.Duration(0), b.SumRTT)
}

func TestCQMBucketAvgRTTZeroEchoes(t *testing.T) {
	var b CQMBucket
	require.Equal(t, time.Duration(0), b.AvgRTT())
}

func TestBucketStateString(t *testing.T) {
	require.Equal(t, "established", BucketStateEstablished.String())
	require.Equal(t, "negotiating", BucketStateNegotiating.String())
	require.Equal(t, "down", BucketStateDown.String())
	require.Equal(t, "unknown", BucketState(99).String())
}
