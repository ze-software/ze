package archerrorinventory

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

// scanSources answers the writer rows of a tree that holds only the given
// files, keyed by slash path relative to the root. The files reach the scan
// through its overlay, so the test needs no fixture directory.
func scanSources(t *testing.T, files map[string]string) []Row {
	t.Helper()
	overlay := make(map[string]overlayFile, len(files))
	for path, text := range files {
		overlay[path] = overlayFile{text: text, present: true}
	}
	rows, err := writerRows(t.TempDir(), overlay)
	if err != nil {
		t.Fatalf("writer scan: %v", err)
	}
	return rows
}

// rowAt answers the one row the scan found at file:line, failing the test
// when there is none or more than one.
func rowAt(t *testing.T, rows []Row, file string, line int) Row {
	t.Helper()
	var found []Row
	for _, row := range rows {
		if row.File == file && row.Line == line {
			found = append(found, row)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s:%d has %d rows, want 1: %+v", file, line, len(found), rows)
	}
	return found[0]
}

// lineOf answers the 1-based line of the only line of text ending in marker.
func lineOf(t *testing.T, text, marker string) int {
	t.Helper()
	line := 0
	for number, content := range strings.Split(text, "\n") {
		if !strings.HasSuffix(content, marker) {
			continue
		}
		if line != 0 {
			t.Fatalf("marker %q is on two lines", marker)
		}
		line = number + 1
	}
	if line == 0 {
		t.Fatalf("marker %q is on no line", marker)
	}
	return line
}

const classifyFile = "internal/component/web/classify.go"

// classifySource hands one writer, http.Error, every argument shape the
// classifier tells apart. Each call carries a marker comment naming its case.
const classifySource = `package web

import (
	"errors"
	"net/http"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// fixText is a package constant whose text names a command.
const fixText = "run ze config validate to see the line"

func typedDirect(w http.ResponseWriter) {
	http.Error(w, opererr.New("the peer is unknown", "no peer has that address", "ze show bgp peer").Error(), 400) // case:typed-direct
}

func typedLocal(w http.ResponseWriter) {
	refusal := opererr.New("the peer is unknown", "no peer has that address", "ze show bgp peer")
	http.Error(w, refusal.Error(), 400) // case:typed-local
}

func typedParameter(w http.ResponseWriter, refusal *opererr.Error) {
	http.Error(w, refusal.Error(), 400) // case:typed-parameter
}

func typedHelper(w http.ResponseWriter) {
	http.Error(w, unknownPeer("x").Error(), 400) // case:typed-helper
}

// unknownPeer is declared to answer error, and every answer it gives is an
// operator error.
func unknownPeer(name string) error {
	if name == "" {
		return nil
	}
	return opererr.Wrap(nil, "the peer "+name+" is unknown", "no peer has that name", "ze show bgp peer")
}

func plainErrorsNew(w http.ResponseWriter) {
	http.Error(w, errors.New("bad").Error(), 400) // case:plain-errors-new
}

func namesCommand(w http.ResponseWriter) {
	http.Error(w, "the config does not parse: run ze config validate", 400) // case:names-command
}

func namesCommandConstant(w http.ResponseWriter) {
	http.Error(w, "the config does not parse: "+fixText, 400) // case:names-command-constant
}

func namesCommandChain(w http.ResponseWriter) {
	var text textbuf.Buffer
	text.Str("the inventory is stale: run ").Str("./le arch error-inventory report")
	http.Error(w, text.String(), 400) // case:names-command-chain
}

func namesConfigStatement(w http.ResponseWriter) {
	http.Error(w, "no host name: write ` + "`set system host-name <name>`" + `", 400) // case:names-config
}

func emptyFix(w http.ResponseWriter) {
	http.Error(w, opererr.New("the peer is unknown", "no peer has that address", "").Error(), 400) // case:empty-fix
}

func problemOnly(w http.ResponseWriter, refusal *opererr.Error) {
	http.Error(w, refusal.Problem(), 400) // case:problem-only
}

func zeroLiteral(w http.ResponseWriter) {
	http.Error(w, (&opererr.Error{}).Error(), 400) // case:zero-literal
}

func notACommand(w http.ResponseWriter) {
	http.Error(w, "the size is wrong", 400) // case:not-a-command
}

func plainHelper(w http.ResponseWriter) {
	http.Error(w, mixedHelper(true).Error(), 400) // case:plain-helper
}

// mixedHelper answers an operator error on one path and a plain one on the
// other, so its answer is not the operator error type.
func mixedHelper(typed bool) error {
	if typed {
		return opererr.New("p", "w", "ze status")
	}
	return errors.New("plain")
}
`

// TestErrorInventoryClassifiesWriterArgument proves AC-3 and A-4: the same
// writer reads typed when it is handed the operator error type carrying a
// fix, plain-names-command when the literal text of a plain message names a
// command, and plain otherwise. Method: one http.Error call per argument
// shape in one source file, scanned through the writer scan, each row looked
// up by the line its case marker sits on. The cases next to the three AC-3
// ones pin where the rule stops: an empty fix literal, a message that keeps
// only the problem, a zero operator error and a helper with a plain path are
// plain, and "size " is no ze command.
func TestErrorInventoryClassifiesWriterArgument(t *testing.T) {
	rows := scanSources(t, map[string]string{classifyFile: classifySource})

	for _, tc := range []struct {
		marker string
		want   Status
	}{
		{"case:typed-direct", StatusTyped},
		{"case:typed-local", StatusTyped},
		{"case:typed-parameter", StatusTyped},
		{"case:typed-helper", StatusTyped},
		{"case:plain-errors-new", StatusPlain},
		{"case:names-command", StatusPlainNamesCommand},
		{"case:names-command-constant", StatusPlainNamesCommand},
		{"case:names-command-chain", StatusPlainNamesCommand},
		{"case:names-config", StatusPlainNamesCommand},
		{"case:empty-fix", StatusPlain},
		{"case:problem-only", StatusPlain},
		{"case:zero-literal", StatusPlain},
		{"case:not-a-command", StatusPlain},
		{"case:plain-helper", StatusPlain},
	} {
		t.Run(tc.marker, func(t *testing.T) {
			row := rowAt(t, rows, classifyFile, lineOf(t, classifySource, tc.marker))
			if row.Status != tc.want {
				t.Errorf("status %s, want %s (message %q)", row.Status, tc.want, row.Message)
			}
			if row.Surface != SurfaceWeb {
				t.Errorf("surface %s, want web", row.Surface)
			}
			if row.Writer != "http.Error" {
				t.Errorf("writer %q, want http.Error", row.Writer)
			}
			if row.Source != SourceWriter {
				t.Errorf("source %s, want writer", row.Source)
			}
			if row.PreRank != PreRankOperatorError {
				t.Errorf("pre-rank %d, want %d", row.PreRank, PreRankOperatorError)
			}
			if row.Package != "internal/component/web" {
				t.Errorf("package %q, want internal/component/web", row.Package)
			}
		})
	}
	if len(rows) != 14 {
		t.Errorf("the scan found %d rows, want one per case (14)", len(rows))
	}
}

// bufferWrites is how many statements write the buffer in
// TestErrorInventoryReadsEveryBufferWrite. The walk that resolved a chain's
// root name again for each earlier write grew exponentially in this count, so
// at 40 the test never returned.
const bufferWrites = 40

// TestErrorInventoryReadsEveryBufferWrite proves the message text of a
// buffer written one statement at a time is every write, in source order,
// read once each. Method: a textbuf.Buffer written bufferWrites times, the
// last write naming a command, handed to http.Error; the row's message must
// be the writes joined and its status plain-names-command.
func TestErrorInventoryReadsEveryBufferWrite(t *testing.T) {
	var source, want strings.Builder
	source.WriteString("package web\n\nimport (\n\t\"net/http\"\n\n\t\"github.com/ze-software/ze/internal/core/textbuf\"\n)\n\n")
	source.WriteString("func longBuffer(w http.ResponseWriter) {\n\tvar text textbuf.Buffer\n")
	for index := range bufferWrites {
		word := "w" + strconv.Itoa(index) + " "
		source.WriteString("\ttext.Str(\"" + word + "\")\n")
		want.WriteString(word)
	}
	source.WriteString("\ttext.Str(\"run \").Str(\"ze status\")\n")
	want.WriteString("run ze status")
	source.WriteString("\thttp.Error(w, text.String(), 400) // case:long-buffer\n}\n")

	text := source.String()
	rows := scanSources(t, map[string]string{classifyFile: text})
	row := rowAt(t, rows, classifyFile, lineOf(t, text, "case:long-buffer"))
	if row.Message != want.String() {
		t.Errorf("message %q, want %q", row.Message, want.String())
	}
	if row.Status != StatusPlainNamesCommand {
		t.Errorf("status %s, want %s", row.Status, StatusPlainNamesCommand)
	}
}

// TestErrorInventoryNamesCommand pins the token rule plain-names-command
// rests on: a ze or ./le command, or a config statement, at a word start.
// Method: the predicate over texts on each side of the rule.
func TestErrorInventoryNamesCommand(t *testing.T) {
	for _, tc := range []struct {
		text string
		want bool
	}{
		{"run ze config validate", true},
		{"ze status", true},
		{"try `ze show bgp peer`", true},
		{"(ze start <config>)", true},
		{"run ./le arch error-inventory report", true},
		{"'./le test qemu'", true},
		{"`set system host-name <name>`", true},
		{"set bgp peer 192.0.2.1 remote as 65001", true},
		{"\"delete bgp peer x\"", true},
		{"the size is wrong", false},
		{"maze runner", false},
		{"ze", false},
		{"ze 42", false},
		{"bin/le report", false},
		{"the value is not set yet", false},
		{"bad", false},
		{"", false},
	} {
		if got := namesCommand(tc.text); got != tc.want {
			t.Errorf("namesCommand(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

// writerSources holds one call per row of the writer table, at the package
// each writer is reached from, plus calls that look like writers and are not.
var writerSources = map[string]string{
	"cmd/ze/dispatch.go": `package main

func dispatch(err error) int {
	return reportDispatchError(err) // writer:reportDispatchError
}
`,
	"internal/component/config/cmd.go": `package config

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/command"
)

func refuse(err error) command.LocalAnswer {
	fmt.Fprintf(os.Stderr, "error: %v\n", err) // writer:Fprintf
	fmt.Fprintln(os.Stderr, "no config")       // writer:Fprintln
	fmt.Fprint(os.Stderr, "no config\n")       // writer:Fprint
	fmt.Fprintln(os.Stdout, "a result")        // not-a-writer:stdout
	os.Stderr.WriteString(command.LocalErrorLine(err))           // writer:WriteString
	return command.LocalRefusal(1, command.LocalErrorLine(err)) // writer:LocalRefusal
}
`,
	"internal/component/mcp/tools.go": `package mcp

func answer(line string) {
	_ = refusalResult(line) // writer:refusalResult
	_ = ErrResult(line)     // writer:ErrResult
	_ = (&Streamable{}).fail(nil, 1, line) // writer:fail
}
`,
	"internal/component/api/rest/server.go": `package rest

func refuse(w any) {
	writeError(w, 400, "bad request") // writer:writeError
}
`,
	"internal/component/lg/server.go": `package lg

func refuse(w any) {
	writeJSONError(w, 400, "bad request") // writer:writeJSONError
}
`,
	"internal/component/web/fragment.go": `package web

import "github.com/ze-software/ze/internal/core/errorfragment"

func refuse() []byte {
	return errorfragment.Render(400, "bad request") // writer:Render
}
`,
	"internal/component/gnmi/get.go": `package gnmi

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func refuse() error {
	_ = status.Error(codes.NotFound, "no such path") // writer:gnmi-Error
	return status.Errorf(codes.NotFound, "no path %s", "x") // writer:gnmi-Errorf
}
`,
	"internal/component/api/grpc/server.go": `package grpc

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func refuse() error {
	return status.Error(codes.NotFound, "no such command") // writer:grpc-Error
}
`,
	"internal/le/le/human/human.go": `package human

func refuse() error {
	return &Refusal{message: "run ./le test unit"} // writer:Refusal
}

type Refusal struct{ message string }
`,
	"internal/le/arch/sample/report.go": `package sample

type Finding struct {
	File   string
	Detail string
}

type Other struct{ Detail string }

func findings() []Finding {
	_ = Other{Detail: "not a finding"} // not-a-writer:other
	return []Finding{
		{File: "a.go", Detail: "no fix"}, // writer:Finding-elided
	}
}

func one() Finding {
	return Finding{File: "b.go", Detail: "run ./le arch sample check"} // writer:Finding
}
`,
	"internal/le/arch/sample/refuse.go": `package sample

import (
	"errors"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

func refuse() {
	leaction.ReportError(errors.New("cache missing")) // writer:ReportError
}
`,
	"internal/le/arch/sample/stderr.go": `package sample

import (
	"fmt"
	"os"
)

func warn() {
	fmt.Fprintln(os.Stderr, "le's own stderr") // not-a-writer:le-stderr
}
`,
	"internal/test/runner/stderr.go": `package runner

import (
	"fmt"
	"os"
)

func warn() {
	fmt.Fprintln(os.Stderr, "the test runner") // not-a-writer:test
}
`,
}

// TestErrorInventoryFindsEachWriter proves the writer table: every writer
// it declares is found at a call written the way its package reaches it,
// with the declared surface and pre-rank, and a call of the same shape the
// table does not declare is not a row (stdout, a struct not named for a
// finding, le's own stderr, the test runner). Method: one call per writer,
// each marked, scanned once.
func TestErrorInventoryFindsEachWriter(t *testing.T) {
	rows := scanSources(t, writerSources)

	want := []struct {
		file    string
		marker  string
		writer  string
		surface Surface
		rank    PreRank
		status  Status
	}{
		{"cmd/ze/dispatch.go", "writer:reportDispatchError", "reportDispatchError", SurfaceLocalCLI, PreRankOperatorError, StatusPlain},
		{"internal/component/config/cmd.go", "writer:Fprintf", "fmt.Fprintf(os.Stderr)", SurfaceLocalCLI, PreRankOperatorError, StatusPlain},
		{"internal/component/config/cmd.go", "writer:Fprintln", "fmt.Fprintln(os.Stderr)", SurfaceLocalCLI, PreRankOperatorError, StatusPlain},
		{"internal/component/config/cmd.go", "writer:Fprint", "fmt.Fprint(os.Stderr)", SurfaceLocalCLI, PreRankOperatorError, StatusPlain},
		{"internal/component/config/cmd.go", "writer:WriteString", "os.Stderr.WriteString", SurfaceLocalCLI, PreRankOperatorError, StatusPlain},
		{"internal/component/config/cmd.go", "writer:LocalRefusal", "command.LocalRefusal", SurfaceLocalCLI, PreRankOperatorError, StatusPlain},
		{"internal/component/mcp/tools.go", "writer:refusalResult", "refusalResult", SurfaceMCP, PreRankOperatorError, StatusPlain},
		{"internal/component/mcp/tools.go", "writer:ErrResult", "mcp.ErrResult", SurfaceMCP, PreRankOperatorError, StatusPlain},
		{"internal/component/mcp/tools.go", "writer:fail", "(*Streamable).fail", SurfaceMCP, PreRankOperatorError, StatusPlain},
		{"internal/component/api/rest/server.go", "writer:writeError", "writeError", SurfaceREST, PreRankOperatorError, StatusPlain},
		{"internal/component/lg/server.go", "writer:writeJSONError", "writeJSONError", SurfaceREST, PreRankOperatorError, StatusPlain},
		{"internal/component/web/fragment.go", "writer:Render", "errorfragment.Render", SurfaceWeb, PreRankOperatorError, StatusPlain},
		{"internal/component/gnmi/get.go", "writer:gnmi-Error", "status.Error", SurfaceGNMI, PreRankOperatorError, StatusPlain},
		{"internal/component/gnmi/get.go", "writer:gnmi-Errorf", "status.Errorf", SurfaceGNMI, PreRankOperatorError, StatusPlain},
		{"internal/component/api/grpc/server.go", "writer:grpc-Error", "status.Error", SurfaceGRPC, PreRankOperatorError, StatusPlain},
		{"internal/le/le/human/human.go", "writer:Refusal", "human.Refusal", SurfaceTooling, PreRankTooling, StatusPlainNamesCommand},
		{"internal/le/arch/sample/refuse.go", "writer:ReportError", "action.ReportError", SurfaceTooling, PreRankTooling, StatusPlain},
		{"internal/le/arch/sample/report.go", "writer:Finding-elided", "Finding", SurfaceTooling, PreRankTooling, StatusPlain},
		{"internal/le/arch/sample/report.go", "writer:Finding", "Finding", SurfaceTooling, PreRankTooling, StatusPlainNamesCommand},
	}
	for _, tc := range want {
		t.Run(tc.marker, func(t *testing.T) {
			row := rowAt(t, rows, tc.file, lineOf(t, writerSources[tc.file], tc.marker))
			if row.Writer != tc.writer {
				t.Errorf("writer %q, want %q", row.Writer, tc.writer)
			}
			if row.Surface != tc.surface {
				t.Errorf("surface %s, want %s", row.Surface, tc.surface)
			}
			if row.PreRank != tc.rank {
				t.Errorf("pre-rank %d, want %d", row.PreRank, tc.rank)
			}
			if row.Status != tc.status {
				t.Errorf("status %s, want %s (message %q)", row.Status, tc.status, row.Message)
			}
		})
	}
	// LocalErrorLine is no writer: it formats the message LocalRefusal and
	// the stderr write hand on, so each of those lines is one row.
	if len(rows) != len(want) {
		var found []string
		for _, row := range rows {
			found = append(found, row.File+":"+strconv.Itoa(row.Line)+" "+row.Writer)
		}
		slices.Sort(found)
		t.Errorf("the scan found %d rows, want %d:\n%s", len(rows), len(want), strings.Join(found, "\n"))
	}
}
