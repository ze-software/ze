// Design: docs/architecture/web-interface.md -- browser adapters over native source checks and salvage.
// Related: backup_command.go -- the sole bounded transfer and command adapters.
package web

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

const backupSourcePath = backupWorkspacePath + "sources/"

type backupSourcePageData struct {
	Path     string
	Bytes    int64
	Status   string
	Warning  bool
	Error    string
	Warnings []string
	Report   string
	Salvage  string
}

func (m *backupWorkspaces) sourceRoute(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) == 0 {
		if r.Method == http.MethodPost {
			m.uploadSource(w, r)
			return
		}
		http.Redirect(w, r, backupWorkspacePath, http.StatusSeeOther)
		return
	}
	if len(parts) == 1 {
		m.source(w, r, parts[0])
		return
	}
	if len(parts) == 2 {
		if parts[1] == "download" {
			if r.Method == http.MethodGet {
				m.downloadSource(w, r, parts[0])
				return
			}
		}
	}
	backupMissing(w)
}

func (m *backupWorkspaces) uploadSource(w http.ResponseWriter, r *http.Request) {
	// Base64 paste is streamed too. Its encoded allowance is larger, but the
	// native owner applies the same 64 MiB decoded bound to both input forms.
	r.Body = http.MaxBytesReader(w, r.Body, int64(base64.StdEncoding.EncodedLen(int(backupArtifactBytesMax)))+(64<<10))
	reader, err := r.MultipartReader()
	if err != nil {
		backupSourceUploadFailure(w, err)
		return
	}
	part, err := reader.NextPart()
	if err != nil {
		backupSourceUploadFailure(w, err)
		return
	}
	var input io.Reader
	var name string
	switch part.FormName() {
	case "source":
		if part.FileName() == "" {
			backupSourceUploadFailure(w, errors.New("no data file was selected"))
			return
		}
		name, input = filepath.Base(part.FileName()), part
	case "content":
		if part.FileName() != "" {
			backupSourceUploadFailure(w, errors.New("the base64 field must contain pasted text"))
			return
		}
		name, input = "pasted-data.bin", base64.NewDecoder(base64.StdEncoding, part)
	default:
		// Multipart field names are open client input; an unknown name starts
		// no private transfer and never selects another command.
		backupSourceUploadFailure(w, errors.New("select a data file or use the base64 paste field"))
		return
	}
	received, err := m.sendChecked(r, "request config artifact bytes begin name "+backupWord(name), input, func() error {
		if _, err := reader.NextPart(); !errors.Is(err, io.EOF) {
			return &backupCommandError{status: http.StatusBadRequest, failure: opererr.New("Submit one source at a time.",
				"No source was retained.", "Open /backups/ and select one data file or paste one base64 value")}
		}
		return nil
	})
	if err != nil {
		// sendChecked joins the input failure with abort. Input-only status
		// and removal claims apply only when abort succeeded; an independent
		// cleanup failure must retain the whole operation's server verdict.
		if joined, ok := err.(interface{ Unwrap() []error }); ok {
			if len(joined.Unwrap()) > 1 {
				backupCommandFailure(w, err)
				return
			}
		}
		if _, malformed := errors.AsType[base64.CorruptInputError](err); malformed {
			backupSourceUploadFailure(w, err)
			return
		}
		if _, oversized := errors.AsType[*http.MaxBytesError](err); oversized {
			backupRefuse(w, http.StatusRequestEntityTooLarge, "The data source exceeds its upload allowance.",
				"No complete source was published; a source may contain at most 64 MiB after base64 decoding.",
				"Open /backups/ and select a smaller data file or paste a smaller base64 value")
			return
		}
		backupCommandFailure(w, err)
		return
	}
	if received.Transfer == "" {
		backupRefuse(w, http.StatusBadGateway, "The source upload returned no completed source identity.",
			"The upload result is not confirmed, and no source page can be opened.", "Reopen /backups/ and upload the source again")
		return
	}
	http.Redirect(w, r, backupSourceURL(received.Transfer), http.StatusSeeOther)
}

func backupSourceUploadFailure(w http.ResponseWriter, err error) {
	backupRefuse(w, http.StatusBadRequest, "The data source could not be uploaded: "+err.Error(),
		"No complete source was retained and no live configuration was changed.",
		"Open /backups/ and select one data file or paste one base64 value")
}

func (m *backupWorkspaces) sourceInfo(r *http.Request, id string) (backupAnswer, error) {
	return m.command(r.Context(), r, "show config artifact transfer read transfer "+backupWord(id)+" offset 0")
}

func (m *backupWorkspaces) source(w http.ResponseWriter, r *http.Request, id string) {
	info, err := m.sourceInfo(r, id)
	if err != nil {
		backupCommandFailure(w, err)
		return
	}
	data := backupSourcePageData{Path: backupSourceURL(id), Bytes: info.Total}
	if r.Method == http.MethodGet {
		m.page(w, backupPageData{Source: &data}, http.StatusOK)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if err := r.ParseForm(); err != nil {
		backupFormRefusal(w)
		return
	}
	if len(r.PostForm) != 1 {
		backupFormRefusal(w)
		return
	}
	if len(r.PostForm["action"]) != 1 {
		backupFormRefusal(w)
		return
	}
	action := r.PostForm.Get("action")
	if action == "close" {
		if err := m.abort(r, id); err != nil {
			backupCommandFailure(w, err)
			return
		}
		http.Redirect(w, r, backupWorkspacePath, http.StatusSeeOther)
		return
	}
	var line string
	switch action {
	case "check":
		line = "show config artifact check transfer " + backupWord(id)
	case "repair":
		line = "request config artifact repair transfer " + backupWord(id)
	default:
		// The action is an open form value, never a command fragment.
		backupFormRefusal(w)
		return
	}
	response, err := m.response(r.Context(), r, line)
	if err != nil {
		backupCommandFailure(w, err)
		return
	}
	report, err := json.MarshalIndent(response.Data, "", "  ")
	if err != nil {
		backupStorageFailure(w, err)
		return
	}
	var answer backupAnswer
	if err := json.Unmarshal(report, &answer); err != nil {
		backupStorageFailure(w, err)
		return
	}
	data.Report, data.Warnings = string(report), response.Warnings
	if answer.Artifact != "" {
		data.Salvage = backupWorkspaceURL(answer.Artifact) + "download"
	}
	status := http.StatusOK
	switch response.Status {
	case plugin.StatusDone:
		data.Status = "The native operation completed. The uploaded source is unchanged."
	case plugin.StatusWarning:
		data.Warning = true
		data.Status = "Warning: the salvage is incomplete. Review its native report before using the download. The uploaded source is unchanged."
	case plugin.StatusError:
		data.Error = response.Error
		status = answer.HTTPStatus
		if status == 0 {
			status = http.StatusBadRequest
		}
	default:
		// Native response statuses are an open boundary. An unfamiliar status
		// cannot establish that checking or salvage completed.
		data.Error = opererr.New("The native operation did not return a completed outcome.",
			"The uploaded source is retained, but its result is not confirmed.", "Reopen /backups/ and retry the operation").Error()
		status = http.StatusBadGateway
	}
	m.page(w, backupPageData{Source: &data}, status)
}

func (m *backupWorkspaces) downloadSource(w http.ResponseWriter, r *http.Request, id string) {
	info, err := m.sourceInfo(r, id)
	if err != nil {
		backupCommandFailure(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="uploaded-source.bin"`)
	// A refused later chunk MUST end as an incomplete HTTP body, rather than
	// a successful chunked response containing only the downloaded prefix.
	w.Header().Set("Content-Length", strconv.FormatInt(info.Total, 10))
	if err := m.copyTransfer(r, id, w); err != nil {
		reportBackupStorageFailure()
	}
}

// Keep source identity opaque; the path carries no caller-supplied filename.
func backupSourceURL(id string) string { return backupSourcePath + strings.Trim(id, "/") + "/" }
