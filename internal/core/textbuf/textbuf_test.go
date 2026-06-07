package textbuf

import (
	"math"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUint(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "0", Uint(0))
	assert.Equal(t, "1", Uint(1))
	assert.Equal(t, "255", Uint(255))
	assert.Equal(t, "18446744073709551615", Uint(math.MaxUint64))
}

func TestUint8(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "0", Uint8(0))
	assert.Equal(t, "255", Uint8(math.MaxUint8))
}

func TestUint16(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "0", Uint16(0))
	assert.Equal(t, "65535", Uint16(math.MaxUint16))
}

func TestUint32(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "0", Uint32(0))
	assert.Equal(t, "4294967295", Uint32(math.MaxUint32))
}

func TestInt(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "0", Int(0))
	assert.Equal(t, "-1", Int(-1))
	assert.Equal(t, "9223372036854775807", Int(math.MaxInt64))
	assert.Equal(t, "-9223372036854775808", Int(math.MinInt64))
}

func TestAddr(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "10.0.0.1", Addr(netip.MustParseAddr("10.0.0.1")))
	assert.Equal(t, "2001:db8::1", Addr(netip.MustParseAddr("2001:db8::1")))
	assert.Equal(t, "0.0.0.0", Addr(netip.AddrFrom4([4]byte{})))
	assert.Equal(t, "::", Addr(netip.AddrFrom16([16]byte{})))
}

func TestHex(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", Hex(nil))
	assert.Equal(t, "", Hex([]byte{}))
	assert.Equal(t, "deadbeef", Hex([]byte{0xde, 0xad, 0xbe, 0xef}))
	assert.Equal(t, "00ff", Hex([]byte{0x00, 0xff}))
}

func TestHexLargeData(t *testing.T) {
	t.Parallel()
	data := make([]byte, 64)
	for i := range data {
		data[i] = byte(i)
	}
	got := Hex(data)
	assert.Len(t, got, 128)
	assert.True(t, strings.HasPrefix(got, "000102"))
	assert.True(t, strings.HasSuffix(got, "3f"))
}

func TestBufferChain(t *testing.T) {
	t.Parallel()
	var b Buffer
	got := b.Str("hello").Byte(' ').Uint32(42).Byte(':').Addr(netip.MustParseAddr("10.0.0.1")).String()
	assert.Equal(t, "hello 42:10.0.0.1", got)
}

func TestBufferAllTypes(t *testing.T) {
	t.Parallel()
	var b Buffer
	got := b.
		Uint8(1).Byte('-').
		Uint16(2).Byte('-').
		Uint32(3).Byte('-').
		Uint(4).Byte('-').
		Int(-5).Byte('-').
		Hex([]byte{0xab}).Byte('-').
		Addr(netip.MustParseAddr("::1")).
		String()
	assert.Equal(t, "1-2-3-4--5-ab-::1", got)
}

func TestBufferGrowBeyond128(t *testing.T) {
	t.Parallel()
	var b Buffer
	long := strings.Repeat("x", 200)
	got := b.Str(long).String()
	assert.Equal(t, long, got)
	assert.Len(t, got, 200)
}

func TestBufferEmpty(t *testing.T) {
	t.Parallel()
	var b Buffer
	assert.Equal(t, "", b.String())
}

func TestAppendUint(t *testing.T) {
	t.Parallel()
	dst := []byte("prefix:")
	dst = AppendUint(dst, 42)
	assert.Equal(t, "prefix:42", string(dst))
}

func TestAppendInt(t *testing.T) {
	t.Parallel()
	dst := []byte("val=")
	dst = AppendInt(dst, -7)
	assert.Equal(t, "val=-7", string(dst))
}

func TestAppendAddr(t *testing.T) {
	t.Parallel()
	dst := []byte("nh=")
	dst = AppendAddr(dst, netip.MustParseAddr("192.168.1.1"))
	assert.Equal(t, "nh=192.168.1.1", string(dst))
}

func TestAppendHex(t *testing.T) {
	t.Parallel()
	dst := []byte("0x")
	dst = AppendHex(dst, []byte{0xca, 0xfe})
	assert.Equal(t, "0xcafe", string(dst))
}

func TestAppendToEmptyDst(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "99", string(AppendUint(nil, 99)))
	assert.Equal(t, "-1", string(AppendInt(nil, -1)))
	assert.Equal(t, "10.0.0.1", string(AppendAddr(nil, netip.MustParseAddr("10.0.0.1"))))
	assert.Equal(t, "ff", string(AppendHex(nil, []byte{0xff})))
}

func TestBufferWriteAfterString(t *testing.T) {
	t.Parallel()
	var b Buffer
	s1 := b.Reset().Str("hello").String()
	assert.Equal(t, "hello", s1)
	s2 := b.Str(" world").String()
	assert.Equal(t, "hello world", s2)
	assert.Equal(t, "hello", s1)
}

func TestBufferFreezeAfterSlice(t *testing.T) {
	t.Parallel()
	const msg = "BUG: textbuf write after String"
	var b Buffer
	b.Reset().Str("hello")
	_ = b.Slice()
	assert.PanicsWithValue(t, msg, func() { b.Byte('x') })
	b.Reset().Str("hello")
	_ = b.Slice()
	assert.PanicsWithValue(t, msg, func() { _, _ = b.WriteString("x") })
	b.Reset().Str("hello")
	_ = b.Slice()
	assert.PanicsWithValue(t, msg, func() { _ = b.WriteByte('x') })
	b.Reset().Str("hello")
	_ = b.Slice()
	assert.PanicsWithValue(t, msg, func() { _, _ = b.WriteRune('x') })
}

func TestBufferResetUnfreezes(t *testing.T) {
	t.Parallel()
	var b Buffer
	b.Reset().Str("first")
	_ = b.String()
	got := b.Reset().Str("second").String()
	assert.Equal(t, "second", got)
}

func TestBufferStringZeroCopyHeap(t *testing.T) {
	t.Parallel()
	var b Buffer
	long := strings.Repeat("x", 200)
	s := b.Reset().Str(long).String()
	assert.Equal(t, long, s)
}

func TestBufferStringCopiesStack(t *testing.T) {
	t.Parallel()
	var b Buffer
	s := b.Reset().Str("short").String()
	assert.Equal(t, "short", s)
	b.Reset().Str("overwritten")
	assert.Equal(t, "short", s)
}

func TestBufferGrowPreservesContent(t *testing.T) {
	t.Parallel()
	var b Buffer
	got := b.Reset().Str("hello").Grow(200).Str(" world").String()
	assert.Equal(t, "hello world", got)
}

func TestBufferGrowNoOpWhenSufficient(t *testing.T) {
	t.Parallel()
	var b Buffer
	b.Reset()
	b.Grow(10)
	assert.Equal(t, 0, b.Len())
	got := b.Str("hi").String()
	assert.Equal(t, "hi", got)
}

func TestBufferGrowAfterString(t *testing.T) {
	t.Parallel()
	var b Buffer
	s := b.Reset().Str("x").String()
	assert.Equal(t, "x", s)
	got := b.Grow(10).Str("y").String()
	assert.Equal(t, "xy", got)
	assert.Equal(t, "x", s)
}

func TestBufferWrite(t *testing.T) {
	t.Parallel()
	var b Buffer
	b.Reset()
	n, err := b.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "hello", b.String())
}

func TestPoolGetRelease(t *testing.T) {
	t.Parallel()
	b := Get()
	got := b.Str("pooled").String()
	assert.Equal(t, "pooled", got)
	b.Release()
}

func TestPoolStringSurvivesReuse(t *testing.T) {
	t.Parallel()
	b := Get()
	b.Str(strings.Repeat("x", 200))
	s := b.String()
	b.Release()
	b2 := Get()
	b2.Str("overwrite")
	_ = b2.String()
	b2.Release()
	assert.Equal(t, 200, len(s))
	assert.Equal(t, strings.Repeat("x", 200), s)
}

func TestPoolFreezeAfterRelease(t *testing.T) {
	t.Parallel()
	b := Get()
	b.Str("hello")
	b.Release()
	assert.PanicsWithValue(t, "BUG: textbuf write after String", func() { b.Str("more") })
}

func TestPoolDoubleReleaseNoop(t *testing.T) {
	t.Parallel()
	b := Get()
	_ = b.Str("x").String()
	b.Release()
	b.Release()
}

func TestPoolTransfersHeapSliceOnString(t *testing.T) {
	t.Parallel()
	b := Get()
	b.Str(strings.Repeat("a", 300))
	s := b.String()
	assert.Equal(t, 300, len(s))
	b.Release()
	b2 := Get()
	assert.Equal(t, len(b2.arr), cap(b2.b))
	b2.Release()
}

func TestPoolPreservesCapacityWithoutString(t *testing.T) {
	t.Parallel()
	b := Get()
	b.Grow(300)
	b.Release()
	b2 := Get()
	assert.GreaterOrEqual(t, cap(b2.b), 300)
	b2.Release()
}

func TestReleaseNoopOnStackBuffer(t *testing.T) {
	t.Parallel()
	var b Buffer
	b.Reset().Str("stack")
	_ = b.String()
	b.Release()
	got := b.Reset().Str("still works").String()
	assert.Equal(t, "still works", got)
}

func TestWriteString(t *testing.T) {
	t.Parallel()
	var b Buffer
	b.Reset()
	n, err := b.WriteString("hello")
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	n, err = b.WriteString(" world")
	assert.NoError(t, err)
	assert.Equal(t, 6, n)
	assert.Equal(t, "hello world", b.String())
}

func TestWriteByte(t *testing.T) {
	t.Parallel()
	var b Buffer
	b.Reset()
	assert.NoError(t, b.WriteByte('a'))
	assert.NoError(t, b.WriteByte(':'))
	assert.NoError(t, b.WriteByte('b'))
	assert.Equal(t, "a:b", b.String())
}

func TestWriteRune(t *testing.T) {
	t.Parallel()
	var b Buffer
	b.Reset()
	n, err := b.WriteRune('A')
	assert.NoError(t, err)
	assert.Equal(t, 1, n)
	n, err = b.WriteRune('é')
	assert.NoError(t, err)
	assert.Equal(t, 2, n)
	n, err = b.WriteRune('\U0001F600')
	assert.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, "Aé\U0001F600", b.String())
}

func TestWriteAfterStringHeap(t *testing.T) {
	t.Parallel()
	var b Buffer
	long := strings.Repeat("x", 200)
	s1 := b.Reset().Str(long).String()
	assert.Equal(t, long, s1)
	s2 := b.Str("after").String()
	assert.Equal(t, "after", s2)
	assert.Equal(t, long, s1)
}
