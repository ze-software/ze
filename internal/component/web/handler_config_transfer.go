// Design: docs/architecture/web-interface.md -- Config download/upload endpoints
// Related: handler_config.go -- authz gate + command constants
// Related: handler_config_commit.go -- audit helper + commit path
// Related: editor.go -- CommittedConfig + ApplyCommittedContent

package web

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// maxConfigUploadBytes caps an uploaded configuration body. A ze config is text
// (kilobytes in practice); this bound rejects accidental or hostile large
// uploads before they are buffered.
const maxConfigUploadBytes = 4 << 20 // 4 MiB

// configDownloadFilename is the attachment name offered to the browser when the
// committed configuration is downloaded; configDownloadDisposition is the
// precomputed Content-Disposition header value (constant to avoid runtime
// string concatenation per ai/rules/performance.md).
const (
	configDownloadFilename    = "ze.conf"
	configDownloadDisposition = `attachment; filename="ze.conf"`
)

// HandleConfigDownload returns a GET handler that streams the committed
// configuration to the client as a downloadable text attachment. The stream is
// RAW and unmasked: it carries the real ze:bcrypt password hashes so the
// download can round-trip (download -> edit -> upload) byte-exactly. Because the
// raw hash is a credential over the local CLI path, this route MUST be gated
// behind edit-authz (editWrap in service_web.go); read-only sessions get 403.
// Do not relax to authWrap. The download is audit-logged.
func HandleConfigDownload(mgr *EditorManager, recorder audit.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		data, err := mgr.committedConfig()
		if err != nil {
			http.Error(w, "read config", http.StatusInternalServerError)
			return
		}

		recordWebAudit(recorder, r, username, audit.ActionConfigDownload, configDownloadFilename)

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", configDownloadDisposition)
		if _, writeErr := w.Write(data); writeErr != nil {
			serverLogger.Warn("config download write failed", "user", username, "error", writeErr)
		}
	}
}

// HandleConfigUpload returns a POST handler that replaces the whole configuration
// from an uploaded file or form field (AC-4). The content is validated through
// the same validator the editor uses before commit; an invalid config is
// rejected (HTTP 400) with the validation error and nothing is applied. A valid
// config is written and the reload hook fired via EditorManager.ApplyCommittedContent,
// then audit-logged. RBAC is enforced twice: at the route (editMutationWrap,
// read-only denied) and here (authorizeWebConfigMutation), matching web commit.
func HandleConfigUpload(mgr *EditorManager, validate func(content, path string) error, configPath string, authorizer aaa.Authorizer, recorder audit.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		username := GetUsernameFromRequest(r)
		if username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !authorizeWebConfigMutation(w, r, authorizer, username, webCommandConfigCommit) {
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxConfigUploadBytes)
		content, err := readUploadedConfig(r)
		if err != nil {
			var tb textbuf.Buffer
			http.Error(w, tb.Str("upload: ").Err(err).String(), http.StatusBadRequest)
			return
		}

		if validate != nil {
			if verr := validate(content, configPath); verr != nil {
				// Invalid config: reject, apply nothing (AC-4 invalid branch).
				var tb textbuf.Buffer
				http.Error(w, tb.Str("config validation failed:\n").Err(verr).String(), http.StatusBadRequest)
				return
			}
		}

		if applyErr := mgr.applyCommittedContent(content); applyErr != nil {
			var tb textbuf.Buffer
			http.Error(w, tb.Str("applying config: ").Err(applyErr).String(), http.StatusInternalServerError)
			return
		}

		recordWebAudit(recorder, r, username, audit.ActionConfigUpload, "uploaded configuration")

		htmxRedirect(w, r, "/")
	}
}

// readUploadedConfig extracts the configuration text from an upload request,
// accepting (in order) a multipart file part named "config", a form field named
// "config" (multipart or urlencoded), or the raw request body (text/plain).
func readUploadedConfig(r *http.Request) (string, error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if file, _, err := r.FormFile("config"); err == nil {
			defer func() { _ = file.Close() }()
			data, readErr := io.ReadAll(file)
			if readErr != nil {
				return "", readErr
			}
			return string(data), nil
		}
		if v := r.FormValue("config"); v != "" {
			return v, nil
		}
		return "", errors.New("no config file or field in multipart upload")
	}

	if err := r.ParseForm(); err == nil {
		if v := r.PostForm.Get("config"); v != "" {
			return v, nil
		}
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", errors.New("empty upload")
	}
	return string(data), nil
}
