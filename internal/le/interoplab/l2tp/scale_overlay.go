// Design: docs/labs/l2tp-interop.md -- deterministic legal SID collisions.
// Related: register_scale.go -- only the scale binary declares this overlay.
// Related: internal/component/l2tp/session.go -- allocateSessionID is guarded here.
package l2tp

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/le/test/overlay"
)

const (
	scaleAllocatorPath           = "internal/component/l2tp/session.go"
	scaleAllocatorSourceBytesMax = 1 << 20
)

// prepareScaleAllocatorOverlay changes only the candidate selection in the
// scale daemon. The caller MUST run cleanup after its build, including failure.
// Production source and other binaries never receive this replacement.
func prepareScaleAllocatorOverlay(root string) (string, func() error, error) {
	source, err := readScaleAllocatorSource(filepath.Join(root, filepath.FromSlash(scaleAllocatorPath)))
	if err != nil {
		return "", nil, err
	}
	changed, err := scaleAllocatorReplacement(source)
	if err != nil {
		return "", nil, err
	}
	path, err := overlay.Write(root, "l2tp-scale-allocator", overlay.File{Rel: scaleAllocatorPath, Content: string(changed)})
	if err != nil {
		return "", nil, err
	}
	return path, func() error { return os.RemoveAll(filepath.Dir(path)) }, nil
}

func readScaleAllocatorSource(path string) (source []byte, resultErr error) {
	file, err := os.Open(path) //nolint:gosec // path is the checkout's fixed scaleAllocatorPath, never peer input
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	source, err = io.ReadAll(io.LimitReader(file, scaleAllocatorSourceBytesMax+1))
	if err != nil {
		return nil, err
	}
	if len(source) > scaleAllocatorSourceBytesMax {
		return nil, errors.New("L2TP allocator source exceeds its 1 MiB instrumentation bound")
	}
	return source, nil
}

// scaleAllocatorReplacement accepts the current complete method's parsed shape,
// not merely a matching assignment somewhere in a changed allocator. Refusing
// drift keeps an identity fix from silently bypassing this scenario's proof.
func scaleAllocatorReplacement(source []byte) ([]byte, error) {
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, scaleAllocatorPath, source, 0)
	if err != nil {
		return nil, fmt.Errorf("parse L2TP allocator instrumentation input: %w", err)
	}
	var allocator *ast.FuncDecl
	for _, declaration := range file.Decls {
		method, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if method.Name.Name != "allocateSessionID" {
			continue
		}
		if allocator != nil {
			return nil, errors.New("L2TP allocator instrumentation found multiple allocateSessionID declarations")
		}
		allocator = method
	}
	if allocator == nil {
		return nil, errors.New("L2TP allocator instrumentation cannot find allocateSessionID; review the allocator source shape")
	}
	var canonical bytes.Buffer
	if err := format.Node(&canonical, positions, allocator); err != nil {
		return nil, err
	}
	if canonical.String() != scaleAllocatorExpected {
		return nil, errors.New("L2TP allocateSessionID source shape changed; review the scale instrumentation before rebuilding")
	}
	// The complete shape above proves these indices and node kinds. Only the
	// candidate's value changes: retain rand.Read, its error, zero validation,
	// collision validation and the production retry limit verbatim.
	loop, ok := allocator.Body.List[1].(*ast.RangeStmt)
	if !ok {
		return nil, errors.New("L2TP allocator range shape changed; review the scale instrumentation before rebuilding")
	}
	assignment, ok := loop.Body.List[1].(*ast.AssignStmt)
	if !ok {
		return nil, errors.New("L2TP allocator assignment shape changed; review the scale instrumentation before rebuilding")
	}
	offset := positions.Position(assignment.End()).Offset
	changed := make([]byte, 0, len(source)+len(scaleAllocatorCandidate))
	changed = append(changed, source[:offset]...)
	changed = append(changed, scaleAllocatorCandidate...)
	changed = append(changed, source[offset:]...)
	return changed, nil
}

// The full method is a drift guard for an intentionally narrow source overlay.
// Parse without comments so documentation and whitespace edits need no update.
const scaleAllocatorExpected = `func (t *L2TPTunnel) allocateSessionID() uint16 {
	var buf [2]byte
	for range maxAllocRetries {
		if _, err := rand.Read(buf[:]); err != nil {
			return 0
		}
		sid := binary.BigEndian.Uint16(buf[:])
		if sid == 0 {
			continue
		}
		if _, exists := t.sessions[sid]; !exists {
			return sid
		}
	}
	return 0
}`

// Scan every legal nonzero uint16 once. Exhaustion leaves zero for the existing
// zero guard; the existing retry bound still governs random reads on that path.
const scaleAllocatorCandidate = `
		sid = 0
		for candidate := uint32(1); candidate < 1<<16; candidate++ {
			if _, exists := t.sessions[uint16(candidate)]; !exists {
				sid = uint16(candidate)
				break
			}
		}`
