// VALIDATES: Stop and unwindLocked release the KERNEL workers before they stop
// the PPP drivers.
// PREVENTS: restoring the inverted order, which deadlocks shutdown -- Driver.Stop
// waits for a per-session frame reader parked in a blocking read(2) on
// /dev/ppp, and only the kernel worker's PPPoX close can wake it.

package l2tp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// Why a source-order ratchet rather than a behavioral test: the deadlock needs
// a REAL /dev/ppp channel descriptor. close(2) on any substitute a test can
// build -- pipe, socketpair, os.File -- does unblock a reader, so a faked
// Driver.Stop returns promptly and the inverted order passes. The only property
// that can be asserted on every host is the one the fix turns on: the call to
// stopKernelWorkersLocked precedes the pppDrivers loop. The end-to-end proof is
// test/plugin/show-l2tp-{sessions,history,session-detail}.ci run with
// CAP_NET_ADMIN (make ze-netns-l2tp-test).
func TestStopReleasesKernelWorkersBeforePPPDrivers(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "subsystem.go", nil, 0)
	if err != nil {
		t.Fatalf("parse subsystem.go: %v", err)
	}

	for _, fn := range []string{"Stop", "unwindLocked"} {
		t.Run(fn, func(t *testing.T) {
			body := funcBody(file, fn)
			if body == nil {
				t.Fatalf("func (*Subsystem) %s not found in subsystem.go", fn)
			}
			kernel := kernelWorkerStopLine(fset, body)
			ppp := pppDriverStopLine(fset, body)
			switch {
			case kernel == 0:
				t.Fatalf("%s no longer calls stopKernelWorkersLocked: the kernel resources a blocked /dev/ppp reader waits on are never released", fn)
			case ppp == 0:
				t.Fatalf("%s no longer stops the PPP drivers", fn)
			case kernel > ppp:
				t.Fatalf("%s stops the PPP drivers (line %d) BEFORE releasing the kernel workers (line %d): Driver.Stop then waits forever for a session reader parked in read(2) on /dev/ppp, which only the worker's PPPoX close can wake",
					fn, ppp, kernel)
			}
		})
	}
}

// funcBody returns the body of the named method on *Subsystem.
func funcBody(file *ast.File, name string) *ast.BlockStmt {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != name || fn.Recv == nil {
			continue
		}
		return fn.Body
	}
	return nil
}

// kernelWorkerStopLine reports the line of the stopKernelWorkersLocked call, or 0.
func kernelWorkerStopLine(fset *token.FileSet, body *ast.BlockStmt) int {
	line := 0
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == "stopKernelWorkersLocked" && line == 0 {
			line = fset.Position(call.Pos()).Line
		}
		return true
	})
	return line
}

// pppDriverStopLine reports the line of the `range s.pppDrivers` loop, or 0.
func pppDriverStopLine(fset *token.FileSet, body *ast.BlockStmt) int {
	line := 0
	ast.Inspect(body, func(n ast.Node) bool {
		rng, ok := n.(*ast.RangeStmt)
		if !ok {
			return true
		}
		sel, ok := rng.X.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == "pppDrivers" && line == 0 {
			line = fset.Position(rng.Pos()).Line
		}
		return true
	})
	return line
}
