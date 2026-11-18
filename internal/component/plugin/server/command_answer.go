// Design: docs/architecture/api/process-protocol.md -- command outcomes.
// Related: command.go -- the process command's bounded and streamed paths.
package server

import (
	"encoding/json"
	"errors"
	"slices"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// documentPluginResponse reads a bounded answer once, retaining both its
// document and original completed outcome. A lost terminator cannot discard a
// valid document which the reader already observed or expose its transport cause.
func documentPluginResponse(command string, answer *rpc.Answer) (*plugin.Response, error) {
	document, collapseErr := rpc.CollapseAnswer(answer)
	if transportErr := answer.Err(); transportErr != nil {
		return interruptedDocumentResponse(command, document, errors.Join(transportErr, collapseErr))
	}
	if collapseErr != nil {
		var tb textbuf.Buffer
		return &plugin.Response{Status: plugin.StatusError, Error: tb.Str("failed to read answer: ").Err(collapseErr).String()}, nil
	}
	metadata, err := answer.RecordAnswerMetadata()
	if err != nil {
		return nil, err
	}
	// Absence is the actual reader result, never an inference from JSON keys
	// or bytes which happen to resemble an empty collection or failure text.
	metadata.DocumentAbsent = len(document) == 0
	response := &plugin.Response{}
	if !metadata.DocumentAbsent {
		response.Data = plugin.RawJSON(document)
	}
	if err := response.CompleteRecordAnswer(metadata); err != nil {
		return nil, err
	}
	return response, nil
}

// interruptedDocumentResponse cannot recover the original walk's counts:
// their only authority was its missing terminator. It retains one actually
// observed document, when present, and one fixed public fault. These counts
// describe the received document and the interruption, never embedded rows.
func interruptedDocumentResponse(command string, document json.RawMessage, cause error) (*plugin.Response, error) {
	var count uint64
	var observed []rpc.Record
	if len(document) > 0 {
		count = 1
		observed = append(observed, rpc.Record{Item: document})
	}
	reportTruncatedPluginAnswer(command, count, cause)
	observed = append(observed, rpc.Record{Fault: answerTruncatedFault(count)})
	progress, err := rpc.CollapseRecords("", nil, slices.Values(observed))
	if err != nil {
		return nil, err
	}
	response := &plugin.Response{Data: plugin.RawJSON(progress)}
	if err := response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{Count: count, Faults: 1}); err != nil {
		return nil, err
	}
	return response, nil
}

// reportTruncatedPluginAnswer keeps the transport cause in the existing daemon
// diagnostic while both answer shapes expose the same fixed public fault.
func reportTruncatedPluginAnswer(command string, produced uint64, cause error) {
	logger().Warn("plugin answer ended before its terminator",
		"command", command, "records", produced, "error", cause,
		"code", diagnostic.CodeDoctorPluginAnswerUndelivered)
	raiseServerEvent(diagnostic.CodeDoctorPluginAnswerUndelivered, command, "plugin answer ended before its terminator: "+cause.Error())
}
