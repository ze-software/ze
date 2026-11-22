// Design: docs/architecture/api/process-protocol.md -- command-result projection.
// Related: docs/functional-tests.md -- external observer fixtures.

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"os"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/test/externalyang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	commandOutcomeHealthy                = "healthy"
	commandOutcomePartial                = "partial"
	commandOutcomeShort                  = "short"
	commandOutcomeLong                   = "long"
	commandOutcomeAcknowledgementRefused = "fixture acknowledgement refused"
)

func init() {
	Register("plugin/command-outcome-provider", commandOutcomeProvider)
	Register("plugin/command-outcome-observer", commandOutcomeObserver, StopsDaemon)
}

// commandOutcomeProvider is an ordinary external SDK plugin. Its row failures
// and SDK failure/warning terminators cross the real daemon dispatcher before
// a second external plugin reads them; it shares no in-process state with it.
func commandOutcomeProvider(ctx context.Context, args []string) error {
	if len(args) > 1 {
		return errors.New("command-outcome-provider accepts only the optional web argument")
	}
	if len(args) == 1 {
		if args[0] != "web" {
			return errors.New("command-outcome-provider accepts only the optional web argument")
		}
	}
	plugin, err := newObserver(externalyang.CommandOutcomePlugin)
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // the run result owns transport failures
	registration := &sdk.Registration{}
	if len(args) == 1 {
		declaration := externalyang.CommandOutcomeWeb()
		registration.Schemas = []sdk.SchemaDecl{{
			Module: declaration.Module.Name, Namespace: declaration.Module.Namespace, YANGText: declaration.Module.Text,
		}}
	}
	const outcomeShortHelp = "Show the external command outcome fixture"
	registration.Commands = []sdk.CommandDecl{
		{Name: "show answer-outcome healthy short", ShortHelp: outcomeShortHelp},
		{Name: "show answer-outcome healthy long", ShortHelp: outcomeShortHelp},
		{Name: "show answer-outcome partial short", ShortHelp: outcomeShortHelp},
		{Name: "show answer-outcome partial long", ShortHelp: outcomeShortHelp},
		{Name: "show answer-outcome failed short", ShortHelp: outcomeShortHelp},
		{Name: "show answer-outcome failed long", ShortHelp: outcomeShortHelp},
		{Name: "show answer-outcome warning short", ShortHelp: outcomeShortHelp},
		{Name: "show answer-outcome warning long", ShortHelp: outcomeShortHelp},
	}
	plugin.OnExecuteCommand(func(_ string, command string, args []string, _ string, _ []sdk.PipeOperation) (string, any, error) {
		if len(args) != 0 {
			return rpc.StatusError, nil, errors.New("command outcome fixture takes no arguments")
		}
		parts := strings.Fields(command)
		if len(parts) != 4 {
			return rpc.StatusError, nil, fmt.Errorf("unknown command outcome fixture %q", command)
		}
		count := 2
		switch parts[3] {
		case commandOutcomeShort:
		case commandOutcomeLong:
			count = rpc.AnswerBufferThreshold + 3
		default:
			return rpc.StatusError, nil, fmt.Errorf("unknown command outcome size %q", parts[3])
		}
		status, faults := rpc.StatusDone, 0
		switch parts[2] {
		case commandOutcomeHealthy:
		case commandOutcomePartial:
			faults = 1
		case outcomeFailed:
			// A Go callback error aborts before the SDK writes rows. StatusError
			// beside Records is the public API for rows plus a failure tail.
			status = rpc.StatusError
		case severityWarning:
			status = rpc.StatusWarning
		default:
			return rpc.StatusError, nil, fmt.Errorf("unknown command outcome %q", parts[2])
		}
		return status, sdk.Records{Key: "items", Rows: commandOutcomeRows(count, faults)}, nil
	})
	return plugin.Run(ctx, registration)
}

// commandOutcomeRows emits a fixed bounded sequence. The long answer passes
// the wire writer's threshold, so its outcome arrives after the streaming head.
func commandOutcomeRows(count, faults int) iter.Seq[rpc.RowRecord] {
	return func(yield func(rpc.RowRecord) bool) {
		for ordinal := range count {
			row := rpc.RawRow(`{"ordinal":` + strconv.Itoa(ordinal) + `}`)
			if !yield(rpc.RowRecord{Item: &row}) {
				return
			}
		}
		for range faults {
			row := rpc.RawRow(`{"cause":"` + commandOutcomeAcknowledgementRefused + `"}`)
			if !yield(rpc.RowRecord{Fault: &row}) {
				return
			}
		}
	}
}

// commandOutcomeObserver checks one outcome on both sides of the threshold,
// paired with a healthy control. Observe owns the real daemon's shutdown only
// after every answer and assertion completes; no BGP peer is needed here.
func commandOutcomeObserver(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("command-outcome-observer requires partial, failed or warning")
	}
	outcome := args[0]
	switch outcome {
	case commandOutcomePartial, outcomeFailed, severityWarning:
	default:
		return fmt.Errorf("unknown command outcome observer %q", outcome)
	}
	return Observe(ctx, "answer-outcome-observer", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		for _, size := range []string{commandOutcomeShort, commandOutcomeLong} {
			for _, current := range []string{commandOutcomeHealthy, outcome} {
				if err := commandOutcomeCheck(ctx, plugin, current, size); err != nil {
					return err
				}
			}
		}
		fmt.Fprintf(os.Stderr, "OK: %s outcome and healthy control survived external daemon dispatch at 2 and %d rows\n", outcome, rpc.AnswerBufferThreshold+3)
		return nil
	})
}

// commandOutcomeCheck compares the actual answer metadata and independently
// decoded rows, so counting the collapsed document once cannot satisfy it.
func commandOutcomeCheck(ctx context.Context, plugin *sdk.Plugin, outcome, size string) error {
	command := "show answer-outcome " + outcome + " " + size
	answer, err := plugin.DispatchCommandAnswer(ctx, command)
	if err != nil {
		return fmt.Errorf("%s: %w", command, err)
	}
	document, err := rpc.CollapseAnswer(answer)
	if err != nil {
		return fmt.Errorf("%s: collapse: %w", command, err)
	}
	if err := answer.Err(); err != nil {
		return fmt.Errorf("%s: incomplete transport: %w", command, err)
	}
	metadata, err := answer.RecordAnswerMetadata()
	if err != nil {
		return fmt.Errorf("%s: metadata: %w", command, err)
	}
	count := uint64(2)
	answerType := rpc.AnswerTypeDocument
	if size == commandOutcomeLong {
		count = rpc.AnswerBufferThreshold + 3
		answerType = rpc.AnswerTypeMap
	}
	if answer.Type != answerType {
		return fmt.Errorf("%s: answer type %q, want %q", command, answer.Type, answerType)
	}
	if metadata.Count != count {
		return fmt.Errorf("%s: produced count %d, want %d", command, metadata.Count, count)
	}
	verdict, faults, message, warning := rpc.VerdictDone, uint64(0), "", false
	switch outcome {
	case commandOutcomeHealthy:
	case commandOutcomePartial:
		verdict, faults = rpc.VerdictPartial, 1
	case outcomeFailed:
		verdict, message = rpc.VerdictAborted, rpc.AnswerFailureUnstated
	case severityWarning:
		verdict, warning = rpc.VerdictWarning, true
	default:
		return fmt.Errorf("unknown expected command outcome %q", outcome)
	}
	if metadata.Faults != faults {
		return fmt.Errorf("%s: refused count %d, want %d", command, metadata.Faults, faults)
	}
	if answer.Verdict() != verdict {
		return fmt.Errorf("%s: verdict %q, want %q", command, answer.Verdict(), verdict)
	}
	if answer.Message() != message {
		return fmt.Errorf("%s: message %q, want %q", command, answer.Message(), message)
	}
	if metadata.Warning != warning {
		return fmt.Errorf("%s: warning %t, want %t", command, metadata.Warning, warning)
	}
	var decoded struct {
		Items []struct {
			Ordinal int `json:"ordinal"`
		} `json:"items"`
		Faults []struct {
			Cause string `json:"cause"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(document, &decoded); err != nil {
		return fmt.Errorf("%s: document: %w", command, err)
	}
	if uint64(len(decoded.Items)) != count {
		return fmt.Errorf("%s: document has %d items, want %d", command, len(decoded.Items), count)
	}
	if uint64(len(decoded.Faults)) != faults {
		return fmt.Errorf("%s: document has %d faults, want %d", command, len(decoded.Faults), faults)
	}
	for ordinal, row := range decoded.Items {
		if row.Ordinal != ordinal {
			return fmt.Errorf("%s: item %d changed to %d", command, ordinal, row.Ordinal)
		}
	}
	for _, row := range decoded.Faults {
		if row.Cause != commandOutcomeAcknowledgementRefused {
			return fmt.Errorf("%s: fault cause changed to %q", command, row.Cause)
		}
	}
	return nil
}
