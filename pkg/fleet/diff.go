// Design: docs/architecture/fleet-config.md — deterministic line-based config diff and patch
// Related: chain.go — SHA-1 chain folding over applied diff payloads
// Related: envelope.go — config-fetch diff mode envelopes

package fleet

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"
)

// Diff protocol bounds and operation kinds.
const (
	// MaxDiffOps is the upper bound on edit operations permitted in one diff.
	MaxDiffOps = 65536
	// DiffOpCopy copies Count lines starting at 0-based line Start from base.
	DiffOpCopy = "copy"
	// DiffOpInsert inserts Lines verbatim into the reconstructed target.
	DiffOpInsert = "ins"

	// maxDiffInputBytes caps the raw JSON diff size accepted by ApplyDiff before
	// decoding to bound decoder memory while allowing diffs whose reconstructed
	// output reaches rpc.MaxMessageSize (16 MiB).
	maxDiffInputBytes = 32 << 20

	// myersMaxDistance caps Myers edit distance per non-unique gap between
	// unique-line anchors so ComputeDiff stays bounded in time and memory.
	myersMaxDistance = 1024
)

// DiffOp is one operation in a deterministic line-based configuration diff.
// For DiffOpCopy, Start and Count select lines from base and Lines is empty.
// For DiffOpInsert, Lines holds the inserted lines (with exact line endings
// preserved) and Start and Count are zero.
type DiffOp struct {
	Op    string   `json:"op"`
	Start int      `json:"start,omitempty"`
	Count int      `json:"count,omitempty"`
	Lines []string `json:"lines,omitempty"`
}

type diffBuilder struct {
	ops      []DiffOp
	exceeded bool
}

func (b *diffBuilder) addCopy(start, count int) {
	if b.exceeded {
		return
	}
	if count <= 0 {
		return
	}
	if n := len(b.ops); n > 0 {
		prev := &b.ops[n-1]
		if prev.Op == DiffOpCopy && prev.Start+prev.Count == start {
			prev.Count += count
			return
		}
	}
	if len(b.ops) >= MaxDiffOps {
		b.exceeded = true
		return
	}
	b.ops = append(b.ops, DiffOp{
		Op:    DiffOpCopy,
		Start: start,
		Count: count,
	})
}

func (b *diffBuilder) addIns(lines []string) {
	if b.exceeded {
		return
	}
	if len(lines) == 0 {
		return
	}
	if n := len(b.ops); n > 0 {
		prev := &b.ops[n-1]
		if prev.Op == DiffOpInsert {
			prev.Lines = append(prev.Lines, lines...)
			return
		}
	}
	if len(b.ops) >= MaxDiffOps {
		b.exceeded = true
		return
	}
	copied := make([]string, len(lines))
	copy(copied, lines)
	b.ops = append(b.ops, DiffOp{
		Op:    DiffOpInsert,
		Lines: copied,
	})
}

type lineAnchor struct {
	baseIdx   int
	targetIdx int
}

type myersRun struct {
	isCopy bool
	start  int
	count  int
}

// ComputeDiff computes a deterministic, terminator-preserving line diff from
// base to target serialized as compact JSON []DiffOp. It sets worthSending to
// false when the operation count exceeds MaxDiffOps, when target is not valid
// UTF-8 (so JSON string encoding cannot preserve exact bytes), or when the
// encoded delta is at least as large as target.
func ComputeDiff(base, target []byte) ([]byte, bool, error) {
	if !utf8.Valid(target) {
		return nil, false, nil
	}
	baseLines := splitLinesPreserve(base)
	targetLines := splitLinesPreserve(target)

	var b diffBuilder
	diffLines(&b, baseLines, targetLines)
	if b.exceeded {
		return nil, false, nil
	}
	ops := b.ops
	if ops == nil {
		ops = []DiffOp{}
	}
	delta, err := json.Marshal(ops)
	if err != nil {
		return nil, false, fmt.Errorf("fleet: marshal diff: %w", err)
	}
	worthSending := len(ops) <= MaxDiffOps && len(delta) < len(target)
	return delta, worthSending, nil
}

// ApplyDiff strictly decodes delta as []DiffOp and reconstructs target bytes
// from base without modifying base. It rejects negative maxOutput, empty or
// oversized delta payloads, unknown JSON fields, trailing JSON data, more than
// MaxDiffOps operations, out-of-bounds copy ranges, and any reconstructed
// output exceeding maxOutput bytes (checked before allocating output).
func ApplyDiff(base, delta []byte, maxOutput int) ([]byte, error) {
	if maxOutput < 0 {
		return nil, errors.New("fleet: maxOutput must be non-negative")
	}
	if len(delta) == 0 {
		return nil, errors.New("fleet: empty diff")
	}
	if len(delta) > maxDiffInputBytes {
		return nil, fmt.Errorf("fleet: diff size %d exceeds max %d", len(delta), maxDiffInputBytes)
	}
	if !utf8.Valid(delta) {
		return nil, errors.New("fleet: diff is not valid UTF-8")
	}

	ops, err := decodeStrictDiffOps(delta)
	if err != nil {
		return nil, err
	}
	if len(ops) > MaxDiffOps {
		return nil, fmt.Errorf("fleet: diff op count %d exceeds max %d", len(ops), MaxDiffOps)
	}

	baseLines := splitLinesPreserve(base)
	totalBytes, err := validateAndMeasureDiffOps(ops, baseLines, maxOutput)
	if err != nil {
		return nil, err
	}

	out := make([]byte, 0, totalBytes)
	for _, op := range ops {
		switch op.Op {
		case DiffOpCopy:
			for j := op.Start; j < op.Start+op.Count; j++ {
				out = append(out, baseLines[j]...)
			}
		case DiffOpInsert:
			for _, line := range op.Lines {
				out = append(out, line...)
			}
		}
	}
	return out, nil
}

func decodeStrictDiffOps(delta []byte) ([]DiffOp, error) {
	dec := json.NewDecoder(bytes.NewReader(delta))
	dec.DisallowUnknownFields()
	var ops []DiffOp
	if err := dec.Decode(&ops); err != nil {
		return nil, fmt.Errorf("fleet: decode diff: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("fleet: diff has unexpected trailing JSON value")
		}
		return nil, fmt.Errorf("fleet: diff has unexpected trailing data: %w", err)
	}
	if ops == nil {
		return nil, errors.New("fleet: diff must be a JSON array")
	}
	return ops, nil
}

func validateAndMeasureDiffOps(ops []DiffOp, baseLines []string, maxOutput int) (int, error) {
	baseCount := len(baseLines)
	totalBytes := 0
	for i, op := range ops {
		switch op.Op {
		case DiffOpCopy:
			n, err := validateCopyOp(i, op, baseLines, baseCount, totalBytes, maxOutput)
			if err != nil {
				return 0, err
			}
			totalBytes = n
		case DiffOpInsert:
			n, err := validateInsertOp(i, op, totalBytes, maxOutput)
			if err != nil {
				return 0, err
			}
			totalBytes = n
		default:
			return 0, fmt.Errorf("fleet: diff op %d: unknown op %q", i, op.Op)
		}
	}
	return totalBytes, nil
}

func validateCopyOp(idx int, op DiffOp, baseLines []string, baseCount, totalBytes, maxOutput int) (int, error) {
	if op.Start < 0 {
		return 0, fmt.Errorf("fleet: diff op %d: negative copy start %d", idx, op.Start)
	}
	if op.Count < 1 {
		return 0, fmt.Errorf("fleet: diff op %d: copy count %d must be >= 1", idx, op.Count)
	}
	if op.Start > baseCount {
		return 0, fmt.Errorf("fleet: diff op %d: copy start %d out of bounds (base lines %d)", idx, op.Start, baseCount)
	}
	if op.Count > baseCount-op.Start {
		return 0, fmt.Errorf("fleet: diff op %d: copy range start=%d count=%d exceeds base lines %d", idx, op.Start, op.Count, baseCount)
	}
	if len(op.Lines) != 0 {
		return 0, fmt.Errorf("fleet: diff op %d: copy op must not carry lines", idx)
	}
	for j := op.Start; j < op.Start+op.Count; j++ {
		lineLen := len(baseLines[j])
		if lineLen > maxOutput-totalBytes {
			return 0, fmt.Errorf("fleet: diff output exceeds max %d bytes", maxOutput)
		}
		totalBytes += lineLen
	}
	return totalBytes, nil
}

func validateInsertOp(idx int, op DiffOp, totalBytes, maxOutput int) (int, error) {
	if op.Start != 0 {
		return 0, fmt.Errorf("fleet: diff op %d: ins op must not set start", idx)
	}
	if op.Count != 0 {
		return 0, fmt.Errorf("fleet: diff op %d: ins op must not set count", idx)
	}
	if len(op.Lines) == 0 {
		return 0, fmt.Errorf("fleet: diff op %d: ins op requires at least one line", idx)
	}
	for _, line := range op.Lines {
		lineLen := len(line)
		if lineLen > maxOutput-totalBytes {
			return 0, fmt.Errorf("fleet: diff output exceeds max %d bytes", maxOutput)
		}
		totalBytes += lineLen
	}
	return totalBytes, nil
}

// splitLinesPreserve splits data into lines keeping each line's exact trailing
// terminator (\n, \r\n, or none on the final line). Empty input returns nil.
func splitLinesPreserve(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	lines := strings.SplitAfter(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func commonPrefixSuffix(a, b []string) (prefix, suffix int) {
	maxPrefix := min(len(a), len(b))
	for prefix < maxPrefix && a[prefix] == b[prefix] {
		prefix++
	}
	maxSuffix := min(len(a)-prefix, len(b)-prefix)
	for suffix < maxSuffix && a[len(a)-1-suffix] == b[len(b)-1-suffix] {
		suffix++
	}
	return prefix, suffix
}

func diffLines(b *diffBuilder, baseLines, targetLines []string) {
	prefix, suffix := commonPrefixSuffix(baseLines, targetLines)
	if prefix > 0 {
		b.addCopy(0, prefix)
	}
	midBase := baseLines[prefix : len(baseLines)-suffix]
	midTarget := targetLines[prefix : len(targetLines)-suffix]

	if len(midBase) == 0 {
		b.addIns(midTarget)
	} else if len(midTarget) > 0 {
		diffMiddleWithAnchors(b, prefix, midBase, midTarget)
	}

	if suffix > 0 {
		b.addCopy(len(baseLines)-suffix, suffix)
	}
}

func diffMiddleWithAnchors(b *diffBuilder, baseOffset int, midBase, midTarget []string) {
	anchors := findUniqueAnchors(midBase, midTarget)
	prevB, prevT := 0, 0
	for _, anc := range anchors {
		if b.exceeded {
			return
		}
		diffGap(b, baseOffset+prevB, midBase[prevB:anc.baseIdx], midTarget[prevT:anc.targetIdx])
		b.addCopy(baseOffset+anc.baseIdx, 1)
		prevB = anc.baseIdx + 1
		prevT = anc.targetIdx + 1
	}
	if b.exceeded {
		return
	}
	diffGap(b, baseOffset+prevB, midBase[prevB:], midTarget[prevT:])
}

func diffGap(b *diffBuilder, baseStart int, gb, gt []string) {
	if b.exceeded {
		return
	}
	if len(gt) == 0 {
		return
	}
	if len(gb) == 0 {
		b.addIns(gt)
		return
	}
	prefix, suffix := commonPrefixSuffix(gb, gt)
	if prefix > 0 {
		b.addCopy(baseStart, prefix)
	}
	subB := gb[prefix : len(gb)-suffix]
	subT := gt[prefix : len(gt)-suffix]
	if len(subB) == 0 {
		b.addIns(subT)
	} else if len(subT) > 0 {
		runs, ok := myersDiff(subB, subT, myersMaxDistance)
		if ok {
			subBaseStart := baseStart + prefix
			for _, run := range slices.Backward(runs) {
				if run.isCopy {
					b.addCopy(subBaseStart+run.start, run.count)
				} else {
					b.addIns(subT[run.start : run.start+run.count])
				}
			}
		} else {
			b.addIns(subT)
		}
	}
	if suffix > 0 {
		b.addCopy(baseStart+len(gb)-suffix, suffix)
	}
}

// findUniqueAnchors returns the deterministic Longest Increasing Subsequence of
// lines appearing exactly once in both midBase and midTarget.
func findUniqueAnchors(midBase, midTarget []string) []lineAnchor {
	baseCount := make(map[string]int, len(midBase))
	basePos := make(map[string]int, len(midBase))
	for i, line := range midBase {
		baseCount[line]++
		basePos[line] = i
	}
	targetCount := make(map[string]int, len(midTarget))
	for _, line := range midTarget {
		targetCount[line]++
	}
	var pairs []lineAnchor
	for j, line := range midTarget {
		if targetCount[line] == 1 && baseCount[line] == 1 {
			pairs = append(pairs, lineAnchor{baseIdx: basePos[line], targetIdx: j})
		}
	}
	if len(pairs) <= 1 {
		return pairs
	}

	tails := make([]int, 0, len(pairs))
	prev := make([]int, len(pairs))
	for i, p := range pairs {
		lo, hi := 0, len(tails)
		for lo < hi {
			mid := lo + (hi-lo)/2
			if pairs[tails[mid]].baseIdx < p.baseIdx {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		if lo > 0 {
			prev[i] = tails[lo-1]
		} else {
			prev[i] = -1
		}
		if lo == len(tails) {
			tails = append(tails, i)
		} else {
			tails[lo] = i
		}
	}

	out := make([]lineAnchor, len(tails))
	idx := tails[len(tails)-1]
	for k := len(tails) - 1; k >= 0; k-- {
		out[k] = pairs[idx]
		idx = prev[idx]
	}
	return out
}

// myersDiff computes an exact shortest edit script from a to b when the edit
// distance is at most maxD, returning runs in reverse order (from end to start)
// so the caller walks them backwards to emit forward operations.
func myersDiff(a, b []string, maxD int) ([]myersRun, bool) {
	n, m := len(a), len(b)
	x0 := 0
	for x0 < n && x0 < m && a[x0] == b[x0] {
		x0++
	}
	if n == m && x0 == n {
		if n == 0 {
			return nil, true
		}
		return []myersRun{{isCopy: true, start: 0, count: n}}, true
	}

	limit := min(n+m, maxD)
	trace := make([][]int32, 0, min(limit, 32)+1)
	trace = append(trace, []int32{int32(x0)})

	foundD := -1
	for d := 1; d <= limit; d++ {
		prev := trace[d-1]
		prevOffset := d - 1
		cur := make([]int32, 2*d+1)
		for i := range cur {
			cur[i] = -1
		}
		for k := -d; k <= d; k += 2 {
			_, x, ok := myersChoosePrev(prev, prevOffset, d, k, n, m)
			if !ok {
				continue
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			cur[k+d] = int32(x)
			if x == n && y == m {
				foundD = d
				break
			}
		}
		if foundD >= 0 {
			break
		}
		trace = append(trace, cur)
	}
	if foundD < 0 {
		return nil, false
	}

	var runs []myersRun
	x, y := n, m
	for step := foundD; step >= 1; step-- {
		k := x - y
		prev := trace[step-1]
		prevOffset := step - 1
		prevK, startX, ok := myersChoosePrev(prev, prevOffset, step, k, n, m)
		if !ok {
			return nil, false
		}
		if x > startX {
			runs = append(runs, myersRun{isCopy: true, start: startX, count: x - startX})
		}
		prevX := int(prev[prevK+prevOffset])
		prevY := prevX - prevK
		if prevK == k+1 {
			runs = append(runs, myersRun{isCopy: false, start: prevY, count: 1})
		}
		x, y = prevX, prevY
	}
	if x > 0 {
		runs = append(runs, myersRun{isCopy: true, start: 0, count: x})
	}
	return runs, true
}

func myersChoosePrev(prev []int32, prevOffset, d, k, n, m int) (prevK, x int, ok bool) {
	canDown := false
	xDown := 0
	if k < d {
		cand := int(prev[k+1+prevOffset])
		if cand >= 0 && cand-k <= m {
			canDown = true
			xDown = cand
		}
	}
	canRight := false
	xRight := 0
	if k > -d {
		cand := int(prev[k-1+prevOffset])
		if cand >= 0 && cand+1 <= n {
			canRight = true
			xRight = cand + 1
		}
	}
	if canDown && canRight {
		if xDown >= xRight {
			return k + 1, xDown, true
		}
		return k - 1, xRight, true
	}
	if canDown {
		return k + 1, xDown, true
	}
	if canRight {
		return k - 1, xRight, true
	}
	return 0, 0, false
}
