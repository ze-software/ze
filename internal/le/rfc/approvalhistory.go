// Design: docs/contributing/rfc-implementation-guide.md -- verdict data cannot forge approval-history carriers.
package rfc

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// approvalHistoryQuery is one ordinary verdict or exact same-carrier pair.
type approvalHistoryQuery struct {
	trailer           string
	binding           string
	unit              string
	unitVerdictPrefix []byte
	rfcVerdict        bool
}

func newApprovalHistoryQuery(trailer, binding string) approvalHistoryQuery {
	key, verdict, _ := strings.Cut(trailer, ": ")
	unit, _, _ := strings.Cut(verdict, ": ")
	return approvalHistoryQuery{
		trailer: trailer, binding: binding, unit: unit,
		unitVerdictPrefix: []byte(key + ": " + unit + ": "),
		rfcVerdict:        key+": " == ApprovalTrailer,
	}
}

// approvalHistoryLanded reads only the immutable commits the history walk named.
// The fixed commit type and length framing follow repo/objects.takeBlob; message
// bytes never delimit records, including when an arbitrary Git body holds NUL.
func approvalHistoryLanded(root, history string, queries []approvalHistoryQuery) (string, int, string) {
	if history == "" {
		return "", -1, ""
	}
	for id := range strings.FieldsSeq(history) {
		if !ValidApprovalTarget(id) {
			return "", -1, "git log returned a noncanonical approval carrier: " + strconv.Quote(id)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := exec.CommandContext(ctx, "git", "cat-file", "--batch") // #nosec G204 -- Fixed Git object query; validated full IDs go through stdin.
	command.Dir = root
	command.Stdin = strings.NewReader(history)
	var complaint bytes.Buffer
	command.Stderr = &complaint
	output, err := command.StdoutPipe()
	if err != nil {
		return "", -1, "cannot open Git approval history: " + err.Error()
	}
	if err := command.Start(); err != nil {
		return "", -1, "cannot read Git approval history: " + err.Error()
	}
	matched, query, readErr := readApprovalHistory(bufio.NewReader(output), history, queries)
	if readErr != nil {
		cancel()
	}
	if err := errors.Join(readErr, command.Wait()); err != nil {
		return "", -1, "cannot read Git approval history: " + err.Error() + ": " + strings.TrimSpace(complaint.String())
	}
	if matched == "" {
		return "", -1, ""
	}
	// Human-readable identity is an opaque display scalar, not a message field
	// separator or evidence about a different, ambient HEAD.
	name, err := approvalGit(root, "log", "-1", "--no-show-signature", "--format=%h %s", matched, "--")
	if err != nil {
		return "", -1, "cannot name the published approval carrier: " + err.Error()
	}
	return strings.TrimSpace(name), query, ""
}

// readApprovalHistory validates one fixed-kind batch frame per retained ID.
// A reusable payload bounds retention by the largest single commit, not the
// entire history. No absence is returned from an unreadable or partial batch.
func readApprovalHistory(reader *bufio.Reader, history string, queries []approvalHistoryQuery) (string, int, error) {
	var payload []byte
	matched := ""
	matchedQuery := -1
	for id := range strings.FieldsSeq(history) {
		header, err := reader.ReadSlice('\n')
		if err != nil {
			return "", -1, fmt.Errorf("missing approval commit header for %s: %w", id, err)
		}
		returned, rest, named := bytes.Cut(header[:len(header)-1], []byte{' '})
		kind, length, typed := bytes.Cut(rest, []byte{' '})
		if !named || !typed || string(returned) != id || string(kind) != "commit" {
			return "", -1, fmt.Errorf("invalid approval commit header for %s: %q", id, header)
		}
		size, err := strconv.Atoi(string(length))
		if err != nil || size < 0 {
			return "", -1, fmt.Errorf("invalid approval commit length for %s: %q", id, length)
		}
		if cap(payload) < size {
			payload = make([]byte, size)
		}
		payload = payload[:size]
		if _, err := io.ReadFull(reader, payload); err != nil {
			return "", -1, fmt.Errorf("incomplete approval commit %s: %w", id, err)
		}
		terminator, err := reader.ReadByte()
		if err != nil || terminator != '\n' {
			return "", -1, fmt.Errorf("invalid approval commit terminator for %s", id)
		}
		_, body, found := bytes.Cut(payload, []byte("\n\n"))
		if !found {
			return "", -1, fmt.Errorf("approval commit %s has no message boundary", id)
		}
		if matched == "" {
			for index, query := range queries {
				if approvalMessageLanded(body, query) {
					matched, matchedQuery = id, index
					break
				}
			}
		}
	}
	if _, err := reader.ReadByte(); !errors.Is(err, io.EOF) {
		if err != nil {
			return "", -1, fmt.Errorf("unreadable trailing approval history: %w", err)
		}
		return "", -1, errors.New("git returned trailing approval commit data")
	}
	return matched, matchedQuery, nil
}

// approvalMessageLanded keeps ordinary verdict and bound-pair semantics over
// one complete raw message, never over text resembling another record header.
func approvalMessageLanded(body []byte, query approvalHistoryQuery) bool {
	verdicts, bindings, unitVerdicts, unitBindings := 0, 0, 0, 0
	for line := range bytes.SplitSeq(body, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if string(line) == query.trailer {
			verdicts++
		}
		if bytes.HasPrefix(line, query.unitVerdictPrefix) {
			unitVerdicts++
		}
		if string(line) == query.binding {
			bindings++
		}
		if query.rfcVerdict && bytes.HasPrefix(line, []byte(ApprovalForTrailer)) {
			if _, boundUnit, valid := ParseApprovalForTrailer(string(line)); valid && boundUnit == query.unit {
				unitBindings++
			}
		}
	}
	if query.binding == "" {
		return verdicts > 0 && unitBindings == 0
	}
	return unitVerdicts == 1 && bindings == 1 && unitBindings == 1
}
