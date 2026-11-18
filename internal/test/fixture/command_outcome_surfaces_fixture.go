// Design: docs/architecture/api/architecture.md -- buffered command outcomes.
// Related: command_outcome_fixture.go -- the actual external SDK provider.
// Related: plugin_fixture_13_rest.go -- the shared HTTP fixture client.
// Related: plugin_fixture_config_refusal.go -- the shared pinned-TLS gRPC client.
// Related: ../cli/cmd_mcp.go -- the independent MCP protocol driver.

package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	zepb "github.com/ze-software/ze/api/proto"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/command-outcome-rest", commandOutcomeREST, StopsDaemon)
	Register("plugin/command-outcome-grpc", commandOutcomeGRPC, StopsDaemon)
	Register("plugin/command-outcome-mcp", commandOutcomeMCP, StopsDaemon)
}

// These credentials are synthetic fixture input, shared with each draft's
// listener configuration. They never come from an operator's environment.
const commandOutcomeSurfaceToken = "command-outcome-token"

type commandOutcomeQuestion struct {
	command string
	outcome string
	count   int
}

// Each adapter observes the same four bounded requests: a healthy control and
// its requested outcome, below and above the actual record writer's threshold.
func commandOutcomeQuestions(outcome string) []commandOutcomeQuestion {
	questions := make([]commandOutcomeQuestion, 0, 4)
	for _, size := range []struct {
		name  string
		count int
	}{{name: "short", count: 2}, {name: "long", count: rpc.AnswerBufferThreshold + 3}} {
		for _, current := range []string{"healthy", outcome} {
			questions = append(questions, commandOutcomeQuestion{
				command: "show answer-outcome " + current + " " + size.name,
				outcome: current,
				count:   size.count,
			})
		}
	}
	return questions
}

func commandOutcomeSurfaceArgs(args []string) (string, string, error) {
	if len(args) != 2 {
		return "", "", errors.New("command outcome adapter requires a port and partial, failed or warning")
	}
	port, err := strconv.ParseUint(args[0], 10, 16)
	if err != nil {
		return "", "", fmt.Errorf("command outcome port %q: %w", args[0], err)
	}
	if port == 0 {
		return "", "", errors.New("command outcome port must be nonzero")
	}
	if err := commandOutcomeExpected(args[1]); err != nil {
		return "", "", err
	}
	return net.JoinHostPort("127.0.0.1", args[0]), args[1], nil
}

func commandOutcomeExpected(outcome string) error {
	switch outcome {
	case "partial", "failed", "warning":
		return nil
	default:
		return fmt.Errorf("unknown command outcome %q", outcome)
	}
}

// REST executes through the real HTTP listener after SDK startup has admitted
// the external provider. The common HTTP helper owns request framing and reads.
func commandOutcomeREST(ctx context.Context, args []string) error {
	address, outcome, err := commandOutcomeSurfaceArgs(args)
	if err != nil {
		return err
	}
	return Observe(ctx, "answer-outcome-observer", &sdk.Registration{}, func(ctx context.Context, _ *sdk.Plugin) error {
		base := "http://" + address + "/api/v1"
		headers := map[string]string{
			headerContentType: mediaTypeJSON,
			"Authorization":   "Bearer " + commandOutcomeSurfaceToken,
		}
		if _, ready := waitHTTP13(ctx, base+"/commands", headers, 20, 100*time.Millisecond); !ready {
			return errors.New("command outcome REST listener did not become ready")
		}
		for _, question := range commandOutcomeQuestions(outcome) {
			result, err := httpRequest13(ctx, http.MethodPost, base+"/execute", headers, map[string]string{"command": question.command})
			if err != nil {
				return fmt.Errorf("REST %s: %w", question.command, err)
			}
			if result.status != http.StatusOK {
				return fmt.Errorf("REST %s: HTTP %d: %s", question.command, result.status, result.body)
			}
			var reply commandOutcomeBufferedReply
			if err := json.Unmarshal(result.body, &reply); err != nil {
				return fmt.Errorf("REST %s: response: %w", question.command, err)
			}
			if err := reply.check(question); err != nil {
				return fmt.Errorf("REST %s: %w", question.command, err)
			}
		}
		fmt.Fprintf(os.Stderr, "OK: REST %s outcome and healthy control preserved rows across the record threshold\n", outcome)
		return nil
	})
}

// gRPC opens an actual TLS channel and calls ZeService.Execute. This deliberately
// does not substitute an SDK DispatchCommand call for the transport under test.
func commandOutcomeGRPC(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("command outcome gRPC requires a port, certificate path and outcome")
	}
	address, outcome, err := commandOutcomeSurfaceArgs([]string{args[0], args[2]})
	if err != nil {
		return err
	}
	return Observe(ctx, "answer-outcome-observer", &sdk.Registration{}, func(ctx context.Context, _ *sdk.Plugin) error {
		connection, err := configRefusalClient(address, args[1])
		if err != nil {
			return err
		}
		defer connection.Close() //nolint:errcheck // Closing the client changes no completed assertion.
		requestCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+commandOutcomeSurfaceToken)
		client := zepb.NewZeServiceClient(connection)
		for _, question := range commandOutcomeQuestions(outcome) {
			result, err := client.Execute(requestCtx, &zepb.CommandRequest{Command: question.command}, grpc.WaitForReady(true))
			if err != nil {
				return fmt.Errorf("TLS gRPC %s: %w", question.command, err)
			}
			reply := commandOutcomeBufferedReply{Status: result.GetStatus(), Data: result.GetData(), Error: result.GetError(), Warnings: result.GetWarnings()}
			if err := reply.check(question); err != nil {
				return fmt.Errorf("TLS gRPC %s: %w", question.command, err)
			}
		}
		fmt.Fprintf(os.Stderr, "OK: TLS gRPC %s outcome and healthy control preserved rows across the record threshold\n", outcome)
		return nil
	})
}

// This DTO is decoded from REST or copied from the received protobuf response.
// It carries no producer state and grants no native or process authority.
type commandOutcomeBufferedReply struct {
	Status   string          `json:"status"`
	Data     json.RawMessage `json:"data"`
	Error    string          `json:"error"`
	Warnings []string        `json:"warnings"`
}

func (reply commandOutcomeBufferedReply) check(question commandOutcomeQuestion) error {
	status, failure := rpc.VerdictDone, ""
	switch question.outcome {
	case "healthy":
	case "partial":
		status = rpc.VerdictPartial
	case "failed":
		status, failure = rpc.VerdictAborted, rpc.AnswerFailureUnstated
	case "warning":
		status = rpc.VerdictWarning
	default:
		return fmt.Errorf("unknown buffered outcome %q", question.outcome)
	}
	if reply.Status != status {
		return fmt.Errorf("status %q, want %q", reply.Status, status)
	}
	if reply.Error != failure {
		return fmt.Errorf("failure %q, want %q", reply.Error, failure)
	}
	if len(reply.Warnings) != 0 {
		return fmt.Errorf("SDK fixture supplied no warning lines, got %v", reply.Warnings)
	}
	return commandOutcomeSurfaceDocument(reply.Data, question)
}

// MCP reuses the independent protocol driver, which owns protocol revision,
// request metadata, headers and readiness. Only its documented result lines are
// decoded here, so this fixture carries no second MCP request implementation.
func commandOutcomeMCP(ctx context.Context, args []string) error {
	_, outcome, err := commandOutcomeSurfaceArgs(args)
	if err != nil {
		return err
	}
	return Observe(ctx, "answer-outcome-observer", &sdk.Registration{}, func(ctx context.Context, _ *sdk.Plugin) error {
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		questions := commandOutcomeQuestions(outcome)
		var script strings.Builder
		script.WriteString("wait-tool ze_execute\n")
		for _, question := range questions {
			params, err := json.Marshal(map[string]any{"name": "ze_execute", "arguments": map[string]string{"command": question.command}})
			if err != nil {
				return err
			}
			script.WriteString("probe tools/call ")
			script.Write(params)
			script.WriteByte('\n')
		}
		// Four requests to the fixed fixture produce at most 522 small item
		// records and two fixed faults. No peer controls this output budget.
		var stdout bytes.Buffer
		client := exec.CommandContext(ctx, executable, "test", "mcp", "--port", args[0], "--token", commandOutcomeSurfaceToken) //nolint:gosec // The fixture chooses the executable and every argument.
		client.Stdin = strings.NewReader(script.String())
		client.Stdout = &stdout
		client.Stderr = os.Stderr
		if err := client.Run(); err != nil {
			return fmt.Errorf("MCP outcome driver: %w", err)
		}
		index := 0
		for line := range strings.SplitSeq(stdout.String(), "\n") {
			if !strings.HasPrefix(line, "probe status=") {
				continue
			}
			if index == len(questions) {
				return errors.New("MCP outcome driver returned an unexpected extra result")
			}
			if err := commandOutcomeMCPResult(line, questions[index]); err != nil {
				return fmt.Errorf("MCP %s: %w", questions[index].command, err)
			}
			index++
		}
		if index != len(questions) {
			return fmt.Errorf("MCP driver returned %d results, want %d", index, len(questions))
		}
		fmt.Fprintf(os.Stderr, "OK: MCP %s outcome and healthy control preserved rows across the record threshold\n", outcome)
		return nil
	})
}

func commandOutcomeMCPResult(line string, question commandOutcomeQuestion) error {
	const prefix = "probe status=200 code=ok result="
	encoded, present := strings.CutPrefix(line, prefix)
	if !present {
		return fmt.Errorf("HTTP or JSON-RPC failure: %s", line)
	}
	var result struct {
		ResultType string `json:"resultType"`
		IsError    bool   `json:"isError"`
		Content    []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal([]byte(encoded), &result); err != nil {
		return err
	}
	if result.ResultType != "complete" {
		return fmt.Errorf("result type %q, want complete", result.ResultType)
	}
	wantError := false
	switch question.outcome {
	case "healthy", "warning":
	case "partial", "failed":
		wantError = true
	default:
		return fmt.Errorf("unknown MCP outcome %q", question.outcome)
	}
	if result.IsError != wantError {
		return fmt.Errorf("isError %t, want %t", result.IsError, wantError)
	}
	wantContent := 1
	if wantError {
		wantContent = 2
	}
	if question.outcome == "warning" {
		wantContent = 2
	}
	if len(result.Content) != wantContent {
		return fmt.Errorf("content has %d entries, want data plus %d failure entries", len(result.Content), wantContent-1)
	}
	for _, content := range result.Content {
		if content.Type != "text" {
			return fmt.Errorf("content type %q, want text", content.Type)
		}
	}
	if err := commandOutcomeSurfaceDocument(json.RawMessage(result.Content[0].Text), question); err != nil {
		return err
	}
	if question.outcome == "warning" {
		if result.Content[1].Text != "Warning" {
			return fmt.Errorf("warning label changed: %q", result.Content[1].Text)
		}
	}
	if question.outcome == "failed" {
		if result.Content[1].Text != "Error: "+rpc.AnswerFailureUnstated {
			return fmt.Errorf("failure text changed: %q", result.Content[1].Text)
		}
	}
	if question.outcome == "partial" {
		if !strings.Contains(result.Content[1].Text, rpc.VerdictPartial) {
			return fmt.Errorf("refused rows have no partial explanation: %q", result.Content[1].Text)
		}
	}
	return nil
}

// Literal row and cause expectations are independent of the provider's writer.
// Every received item is checked, including the last long-walk ordinal.
func commandOutcomeSurfaceDocument(document json.RawMessage, question commandOutcomeQuestion) error {
	var decoded struct {
		Items []struct {
			Ordinal *int `json:"ordinal"`
		} `json:"items"`
		Faults []struct {
			Cause string `json:"cause"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(document, &decoded); err != nil {
		return fmt.Errorf("data is not the original document: %w", err)
	}
	if len(decoded.Items) != question.count {
		return fmt.Errorf("data has %d items, want %d", len(decoded.Items), question.count)
	}
	faults := 0
	if question.outcome == "partial" {
		faults = 1
	}
	if len(decoded.Faults) != faults {
		return fmt.Errorf("data has %d faults, want %d", len(decoded.Faults), faults)
	}
	for ordinal, item := range decoded.Items {
		if item.Ordinal == nil {
			return fmt.Errorf("item %d lost its ordinal field", ordinal)
		}
		if *item.Ordinal != ordinal {
			return fmt.Errorf("item %d changed to %d", ordinal, *item.Ordinal)
		}
	}
	for _, fault := range decoded.Faults {
		if fault.Cause != "fixture acknowledgement refused" {
			return fmt.Errorf("refused row cause changed to %q", fault.Cause)
		}
	}
	return nil
}
