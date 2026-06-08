package textbuf

import (
	"errors"
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

func TestRepeat(t *testing.T) {
	t.Parallel()
	var b Buffer
	assert.Equal(t, "\t\t\t", b.Reset().Repeat("\t", 3).String())
	assert.Equal(t, "aaa", b.Reset().Repeat("a", 3).String())
	assert.Equal(t, "ababab", b.Reset().Repeat("ab", 3).String())
	assert.Equal(t, "", b.Reset().Repeat("x", 0).String())
	assert.Equal(t, "", b.Reset().Repeat("x", -1).String())
	assert.Equal(t, "pre\t\t", b.Reset().Str("pre").Repeat("\t", 2).String())
}

func TestRepeatLarge(t *testing.T) {
	t.Parallel()
	var b Buffer
	got := b.Reset().Repeat("ab", 100).String()
	assert.Len(t, got, 200)
	assert.Equal(t, strings.Repeat("ab", 100), got)
}

func TestPadRight(t *testing.T) {
	t.Parallel()
	var b Buffer
	assert.Equal(t, "hi   ", b.Reset().PadRight("hi", 5).String())
	assert.Equal(t, "exact", b.Reset().PadRight("exact", 5).String())
	assert.Equal(t, "toolong", b.Reset().PadRight("toolong", 3).String())
	assert.Equal(t, "", b.Reset().PadRight("", 0).String())
	assert.Equal(t, "   ", b.Reset().PadRight("", 3).String())
}

func TestPadRightInChain(t *testing.T) {
	t.Parallel()
	var b Buffer
	got := b.Reset().PadRight("alice", 14).Str("01-15").Byte(' ').Str("09:30").String()
	assert.Equal(t, "alice         01-15 09:30", got)
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

func TestHexUpper(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", HexUpper(nil))
	assert.Equal(t, "", HexUpper([]byte{}))
	assert.Equal(t, "DEADBEEF", HexUpper([]byte{0xde, 0xad, 0xbe, 0xef}))
	assert.Equal(t, "00FF", HexUpper([]byte{0x00, 0xff}))
	assert.Equal(t, "0123456789ABCDEF", HexUpper([]byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef}))
}

func TestHexUpperLarge(t *testing.T) {
	t.Parallel()
	data := make([]byte, 65)
	for i := range data {
		data[i] = byte(i)
	}
	got := HexUpper(data)
	assert.Len(t, got, 130)
	assert.True(t, strings.HasPrefix(got, "000102"))
	assert.True(t, strings.HasSuffix(got, "40"))
}

func TestHexBoundary(t *testing.T) {
	t.Parallel()
	data := make([]byte, 32)
	for i := range data {
		data[i] = byte(i)
	}
	got := Hex(data)
	assert.Len(t, got, 64)
	assert.True(t, strings.HasPrefix(got, "000102"))
	assert.True(t, strings.HasSuffix(got, "1f"))

	data33 := make([]byte, 33)
	for i := range data33 {
		data33[i] = byte(i)
	}
	got33 := Hex(data33)
	assert.Len(t, got33, 66)
}

func TestHexUpperBoundary(t *testing.T) {
	t.Parallel()
	data := make([]byte, 64)
	for i := range data {
		data[i] = byte(i)
	}
	got := HexUpper(data)
	assert.Len(t, got, 128)
	assert.True(t, strings.HasPrefix(got, "000102"))

	data65 := make([]byte, 65)
	for i := range data65 {
		data65[i] = byte(i)
	}
	got65 := HexUpper(data65)
	assert.Len(t, got65, 130)
}

func TestBufferHexUpper(t *testing.T) {
	t.Parallel()
	var b Buffer
	got := b.Reset().HexUpper([]byte{0xca, 0xfe, 0xba, 0xbe}).String()
	assert.Equal(t, "CAFEBABE", got)
}

func TestResetWithSize(t *testing.T) {
	t.Parallel()
	var b Buffer

	b.Reset(256)
	assert.GreaterOrEqual(t, cap(b.b), 256)
	b.Str(strings.Repeat("x", 200))
	assert.Equal(t, 200, b.Len())

	b.Reset(64)
	assert.Equal(t, 0, b.Len())
	got := b.Str("hello").String()
	assert.Equal(t, "hello", got)

	b.Reset(300)
	assert.GreaterOrEqual(t, cap(b.b), 300)
	b.Reset(200)
	assert.GreaterOrEqual(t, cap(b.b), 200)
	got = b.Str("ok").String()
	assert.Equal(t, "ok", got)
}

func TestResetSizeBelowInline(t *testing.T) {
	t.Parallel()
	var b Buffer
	b.Reset(64)
	got := b.Str("short").String()
	assert.Equal(t, "short", got)
}

func TestGrowBoundary(t *testing.T) {
	t.Parallel()
	var b Buffer
	b.Reset()
	b.Str("hello")
	remaining := cap(b.b) - len(b.b)
	b.Grow(remaining)
	assert.Equal(t, 5, b.Len())
	got := b.Str(" world").String()
	assert.Equal(t, "hello world", got)

	b.Reset()
	b.Str("x")
	b.Grow(200)
	assert.GreaterOrEqual(t, cap(b.b), 201)
	got = b.Str(strings.Repeat("y", 200)).String()
	assert.Equal(t, "x"+strings.Repeat("y", 200), got)
}

func TestGrowZeroOrNegative(t *testing.T) {
	t.Parallel()
	var b Buffer
	b.Reset().Str("hello")
	b.Grow(0)
	b.Grow(-1)
	assert.Equal(t, "hello", b.String())
}

func TestWriteRuneBoundary(t *testing.T) {
	t.Parallel()
	var b Buffer
	b.Reset()
	n, err := b.WriteRune(0x7f)
	assert.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, "\x7f", b.String())

	b.Reset()
	n, err = b.WriteRune(0x80)
	assert.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, string(rune(0x80)), b.String())
}

func TestColored(t *testing.T) {
	t.Parallel()
	var b Buffer
	b.Reset()
	b.Colored(ColorBoldRed).Str("warn").Colored(ColorReset)
	assert.Equal(t, "warn", b.String())

	b.Reset()
	b.SetColor(true)
	b.Colored(ColorBoldRed).Str("warn").Colored(ColorReset)
	assert.Equal(t, "\033[1;31mwarn\033[0m", b.String())

	b.Reset()
	b.SetColor(false)
	b.Colored(ColorBrightGreen).Str("ok")
	assert.Equal(t, "ok", b.String())
}

func TestGetPoolInlineAfterString(t *testing.T) {
	t.Parallel()
	b := Get()
	b.Str(strings.Repeat("x", 200))
	_ = b.String()
	assert.Equal(t, len(b.arr), cap(b.b))
	b.Release()
}

func TestGetPoolResetsColor(t *testing.T) {
	t.Parallel()
	b := Get()
	b.SetColor(true)
	b.Colored(ColorBoldRed).Str("red")
	_ = b.String()
	b.Release()

	b2 := Get()
	b2.Colored(ColorBoldRed).Str("plain")
	assert.Equal(t, "plain", b2.String())
	b2.Release()
}

func TestHexAllocBoundary(t *testing.T) {
	data32 := make([]byte, 32)
	allocs := testing.AllocsPerRun(10, func() {
		_ = Hex(data32)
	})
	assert.LessOrEqual(t, allocs, 1.0)

	data33 := make([]byte, 33)
	allocs = testing.AllocsPerRun(10, func() {
		_ = Hex(data33)
	})
	assert.LessOrEqual(t, allocs, 2.0)
}

func TestHexUpperAllocBoundary(t *testing.T) {
	data64 := make([]byte, 64)
	allocs := testing.AllocsPerRun(10, func() {
		_ = HexUpper(data64)
	})
	assert.LessOrEqual(t, allocs, 1.0)

	data65 := make([]byte, 65)
	allocs = testing.AllocsPerRun(10, func() {
		_ = HexUpper(data65)
	})
	assert.LessOrEqual(t, allocs, 2.0)
}

func TestGrowExactCapacity(t *testing.T) {
	t.Parallel()
	var b Buffer
	b.Reset()
	b.Str("hi")
	remaining := cap(b.b) - len(b.b)
	b.Grow(remaining)
	got := b.Str(strings.Repeat("x", remaining)).String()
	assert.Len(t, got, 2+remaining)

	b.Reset()
	b.Str("hi")
	b.Grow(remaining + 1)
	assert.GreaterOrEqual(t, cap(b.b), 2+remaining+1)
}

func TestPrefix(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "10.0.0.0/24", Prefix(netip.MustParsePrefix("10.0.0.0/24")))
	assert.Equal(t, "2001:db8::/32", Prefix(netip.MustParsePrefix("2001:db8::/32")))
	assert.Equal(t, "0.0.0.0/0", Prefix(netip.MustParsePrefix("0.0.0.0/0")))
}

func TestMAC(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "de:ad:be:ef:ca:fe", MAC([]byte{0xde, 0xad, 0xbe, 0xef, 0xca, 0xfe}))
	assert.Equal(t, "00:00:00:00:00:00", MAC([]byte{0, 0, 0, 0, 0, 0}))
	assert.Equal(t, "", MAC([]byte{0xde, 0xad}))
	assert.Equal(t, "", MAC(nil))
}

func TestHostPort(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "127.0.0.1:8080", HostPort("127.0.0.1", 8080))
	assert.Equal(t, "localhost:22", HostPort("localhost", 22))
	assert.Equal(t, "10.0.0.1:179", HostPort("10.0.0.1", 179))
	assert.Equal(t, "0.0.0.0:0", HostPort("0.0.0.0", 0))
	assert.Equal(t, "host:65535", HostPort("host", 65535))
}

func TestJoin(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", Join(nil, ", "))
	assert.Equal(t, "", Join([]string{}, ", "))
	assert.Equal(t, "one", Join([]string{"one"}, ", "))
	assert.Equal(t, "a, b, c", Join([]string{"a", "b", "c"}, ", "))
	assert.Equal(t, "x:y:z", Join([]string{"x", "y", "z"}, ":"))
	assert.Equal(t, "ab", Join([]string{"a", "b"}, ""))
}

func TestBufferPrefix(t *testing.T) {
	t.Parallel()
	var b Buffer
	got := b.Reset().Str("pfx=").Prefix(netip.MustParsePrefix("192.168.1.0/24")).String()
	assert.Equal(t, "pfx=192.168.1.0/24", got)
}

func TestBufferQuoted(t *testing.T) {
	t.Parallel()
	var b Buffer
	assert.Equal(t, `"hello"`, b.Reset().Quoted("hello").String())
	assert.Equal(t, `"has \"quotes\""`, b.Reset().Quoted(`has "quotes"`).String())
	assert.Equal(t, `"tab\there"`, b.Reset().Quoted("tab\there").String())
}

func TestBufferErr(t *testing.T) {
	t.Parallel()
	var b Buffer
	assert.Equal(t, "open: file not found", b.Reset().Str("open: ").Err(errors.New("file not found")).String())
	assert.Equal(t, "ok", b.Reset().Str("ok").Err(nil).String())
}

func TestBufferMAC(t *testing.T) {
	t.Parallel()
	var b Buffer
	got := b.Reset().Str("src=").MAC([]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}).String()
	assert.Equal(t, "src=aa:bb:cc:dd:ee:ff", got)
	assert.Equal(t, "empty", b.Reset().Str("empty").MAC([]byte{1, 2}).String())
}

func TestBufferJoin(t *testing.T) {
	t.Parallel()
	var b Buffer
	assert.Equal(t, "a, b, c", b.Reset().Join([]string{"a", "b", "c"}, ", ").String())
	assert.Equal(t, "pre:x|y", b.Reset().Str("pre:").Join([]string{"x", "y"}, "|").String())
	assert.Equal(t, "", b.Reset().Join(nil, ",").String())
}

func TestBufferFloat(t *testing.T) {
	t.Parallel()
	var b Buffer
	assert.Equal(t, "99.5", b.Reset().Float(99.5, 1).String())
	assert.Equal(t, "3.14", b.Reset().Float(3.14159, 2).String())
	assert.Equal(t, "100.0", b.Reset().Float(100.0, 1).String())
}

func TestBufferPadLeft(t *testing.T) {
	t.Parallel()
	var b Buffer
	assert.Equal(t, "   42", b.Reset().PadLeft("42", 5).String())
	assert.Equal(t, "exact", b.Reset().PadLeft("exact", 5).String())
	assert.Equal(t, "toolong", b.Reset().PadLeft("toolong", 3).String())
	assert.Equal(t, "   ", b.Reset().PadLeft("", 3).String())
	assert.Equal(t, "  éà", b.Reset().PadLeft("éà", 4).String())
}

func TestPadRightRunes(t *testing.T) {
	t.Parallel()
	var b Buffer
	assert.Equal(t, "éà   ", b.Reset().PadRight("éà", 5).String())
	assert.Equal(t, "abc  ", b.Reset().PadRight("abc", 5).String())
}

func TestAppendPrefix(t *testing.T) {
	t.Parallel()
	dst := []byte("route=")
	dst = AppendPrefix(dst, netip.MustParsePrefix("10.0.0.0/8"))
	assert.Equal(t, "route=10.0.0.0/8", string(dst))
}

func TestAppendMAC(t *testing.T) {
	t.Parallel()
	dst := []byte("mac=")
	dst = AppendMAC(dst, []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab})
	assert.Equal(t, "mac=01:23:45:67:89:ab", string(dst))
	assert.Equal(t, "short", string(AppendMAC([]byte("short"), []byte{1, 2})))
}

// TestNoescapeStackResidence verifies that var b Buffer stays on the stack
// when used locally. This is the noescape trick (same as strings.Builder via
// abi.NoEscape). If this test fails after a Go compiler update, DO NOT fix
// the test. The noescape function no longer hides the self-referential slice
// from escape analysis. Compare against:
//   - $(go env GOROOT)/src/strings/builder.go  (copyCheck)
//   - $(go env GOROOT)/src/internal/abi/escape.go  (NoEscape)
//
// Then update our noescape + inlineSlice to match whatever technique the
// stdlib switched to.
func TestNoescapeStackResidence(t *testing.T) {
	allocs := testing.AllocsPerRun(100, func() {
		var b Buffer
		b.Reset().Str("peer:").Uint(65536).Byte(':').Uint(1)
		_ = b.Bytes()
	})
	assert.Equal(t, 0.0, allocs, "var b Buffer + Reset + Bytes must be zero-alloc; noescape may be broken -- see test comment")

	allocs = testing.AllocsPerRun(100, func() {
		b := Get()
		_ = b.Reset().Str("peer:").Uint(65536).Byte(':').Uint(1).Slice()
		b.Release()
	})
	assert.Equal(t, 0.0, allocs, "Get + Slice + Release must be zero-alloc")
}
