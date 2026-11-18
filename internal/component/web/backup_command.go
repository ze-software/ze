// Design: docs/architecture/web-interface.md -- native forms consume shared artifact commands.
package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/configtext"
	"github.com/ze-software/ze/internal/core/opererr"
)

type backupAnswer struct {
	Artifact     string          `json:"artifact"`
	Transfer     string          `json:"transfer"`
	Label        string          `json:"label"`
	Digest       string          `json:"digest"`
	Names        []string        `json:"names"`
	Total        int64           `json:"total"`
	Next         int64           `json:"next"`
	Complete     bool            `json:"complete"`
	Base         string          `json:"base"`
	Review       string          `json:"review"`
	Diff         string          `json:"diff"`
	DiffTransfer string          `json:"diff-transfer"`
	Message      string          `json:"message"`
	Saved        bool            `json:"saved"`
	Content      json.RawMessage `json:"content"`
	HTTPStatus   int             `json:"http-status"`
}

type backupCommandError struct {
	status  int
	failure *opererr.Error
}

func (e *backupCommandError) Error() string { return e.failure.Error() }
func (e *backupCommandError) Unwrap() error { return e.failure }

func (m *backupWorkspaces) command(ctx context.Context, r *http.Request, line string) (backupAnswer, error) {
	response, err := m.response(ctx, r, line)
	if err != nil {
		return backupAnswer{}, err
	}
	return decodeBackupAnswer(response)
}

// decodeBackupAnswer is the document and refusal decoder shared by forms and
// admission-only requests. Successful native inventories retain their array
// shape and MUST NOT be passed here when the caller needs only admission.
func decodeBackupAnswer(response *plugin.Response) (backupAnswer, error) {
	var answer backupAnswer
	if response.Data != nil {
		encoded, err := json.Marshal(response.Data)
		if err != nil {
			return answer, err
		}
		if err := json.Unmarshal(encoded, &answer); err != nil {
			return answer, err
		}
	}
	if response.Status == plugin.StatusError {
		status := answer.HTTPStatus
		if status == 0 {
			status = http.StatusBadRequest
		}
		failure, err := opererr.FromRendered(response.Error)
		if err != nil {
			failure = opererr.Wrap(errors.Join(err, errors.New(response.Error)),
				"The backup command returned an incomplete error message.",
				"The command did not return the explanation and recovery needed to continue.",
				"Run ze show doctor, then reopen /backups/ and retry")
		}
		return answer, &backupCommandError{status: status, failure: failure}
	}
	return answer, nil
}

// response retains the native outcome and document for an inspection whose
// failed or partial result remains useful. The adapter MUST render both.
func (m *backupWorkspaces) response(ctx context.Context, r *http.Request, line string) (*plugin.Response, error) {
	response, err := m.dispatch(ctx, plugin.CallerIdentity{Username: GetUsernameFromRequest(r), RemoteAddr: r.RemoteAddr,
		Authorizer: authorizerForRequest(r, m.authorizer)}, line)
	if err != nil {
		var denied interface{ PermissionDenied() bool }
		if errors.As(err, &denied) {
			if denied.PermissionDenied() {
				return nil, &backupCommandError{status: http.StatusForbidden, failure: opererr.Wrap(err,
					plugin.UnauthorizedMessage, "This account cannot inspect or change private backup data.",
					"Open /login and sign in with an account allowed to edit configuration")}
			}
		}
		return nil, err
	}
	if response == nil {
		return nil, errors.New("the daemon returned no artifact answer")
	}
	return response, nil
}

func backupCommandFailure(w http.ResponseWriter, err error) {
	if refusal := backupSingleCommandError(err); refusal != nil {
		var failure *opererr.Error = refusal.failure //nolint:modernize,staticcheck // inferdecl/ST1023: Pin the complete native refusal type at the HTTP error boundary.
		http.Error(w, failure.Error(), refusal.status)
		return
	}
	backupStorageFailure(w, err)
}

// backupSingleCommandError removes only singleton joins from a command's
// cleanup adapter. A separate abort failure must stay with the command failure
// and receive the shared storage-failure response, rather than disappear behind
// the first command refusal's status and text.
func backupSingleCommandError(err error) *backupCommandError {
	for {
		if refusal, ok := err.(*backupCommandError); ok { //nolint:errorlint // Match this exact node; a separate joined cleanup cause must not disappear.
			return refusal
		}
		joined, ok := err.(interface{ Unwrap() []error })
		if !ok {
			return nil
		}
		causes := joined.Unwrap()
		if len(causes) != 1 {
			return nil
		}
		err = causes[0]
	}
}

func backupWord(value string) string { return configtext.QuoteCommandWord(value) }

func (m *backupWorkspaces) abort(r *http.Request, id string) error {
	// A disconnected transport still owes removal of its private transfer.
	_, err := m.command(context.WithoutCancel(r.Context()), r, "request config artifact upload abort transfer "+backupWord(id))
	if refusal := backupSingleCommandError(err); refusal != nil {
		if refusal.status == http.StatusNotFound {
			return nil
		}
	}
	return err
}

func (m *backupWorkspaces) send(r *http.Request, begin string, input io.Reader) (answer backupAnswer, err error) {
	return m.sendChecked(r, begin, input, nil)
}

func (m *backupWorkspaces) sendChecked(r *http.Request, begin string, input io.Reader, complete func() error) (answer backupAnswer, err error) {
	started, err := m.command(r.Context(), r, begin)
	if err != nil {
		return answer, err
	}
	finished := false
	defer func() {
		if !finished {
			err = errors.Join(err, m.abort(r, started.Transfer))
		}
	}()
	buffer := make([]byte, command.ContentOctetsMax)
	offset := int64(0)
	// The command owner's admitted byte allowance bounds every append. The
	// request deadline and context bound a reader that supplies no further bytes.
	for {
		count, readErr := input.Read(buffer)
		if count > 0 {
			line := "request config artifact upload append transfer " + backupWord(started.Transfer) + " offset " + strconv.FormatInt(offset, 10) + " content " + backupWord(base64.StdEncoding.EncodeToString(buffer[:count]))
			appended, appendErr := m.command(r.Context(), r, line)
			if appendErr != nil {
				return answer, appendErr
			}
			offset = appended.Next
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return answer, readErr
		}
		if count == 0 {
			return answer, io.ErrNoProgress
		}
	}
	if complete != nil {
		if err := complete(); err != nil {
			return answer, err
		}
	}
	answer, err = m.command(r.Context(), r, "request config artifact upload finish transfer "+backupWord(started.Transfer))
	if err != nil {
		return answer, err
	}
	finished = true
	return answer, nil
}

func (m *backupWorkspaces) read(r *http.Request, transfer string, output io.Writer) (err error) {
	defer func() { err = errors.Join(err, m.abort(r, transfer)) }()
	return m.copyTransfer(r, transfer, output)
}

// copyTransfer leaves an explicit source retained. Its owning form MUST abort
// it when closed; a download is not the source's last use.
func (m *backupWorkspaces) copyTransfer(r *http.Request, transfer string, output io.Writer) error {
	offset := int64(0)
	// Each immutable source is at most 64 MiB. Every nonfinal answer must
	// advance, so even a broken producer cannot create an unbounded read loop.
	for range 1 + int(backupArtifactBytesMax/int64(command.ContentOctetsMax)) {
		answer, err := m.command(r.Context(), r, "show config artifact transfer read transfer "+backupWord(transfer)+" offset "+strconv.FormatInt(offset, 10))
		if err != nil {
			return err
		}
		var envelope struct {
			Blob json.RawMessage `json:"blob"`
		}
		if err := json.Unmarshal(answer.Content, &envelope); err != nil {
			return err
		}
		content, err := command.DecodeBlob(envelope.Blob)
		if err != nil {
			return err
		}
		written, err := output.Write(content)
		if err != nil {
			return err
		}
		if written != len(content) {
			return io.ErrShortWrite
		}
		if answer.Complete {
			return nil
		}
		if answer.Next <= offset {
			return errors.New("the artifact transfer did not advance")
		}
		offset = answer.Next
	}
	return fmt.Errorf("the artifact transfer exceeded its %d byte bound", backupArtifactBytesMax)
}
