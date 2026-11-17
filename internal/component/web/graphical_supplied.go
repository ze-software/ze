// Design: docs/architecture/web-interface.md -- native supplied-input inspection.
package web

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// HandleSuppliedInspectionForm renders inputs for exactly one currently
// offered registered descriptor. The caller MUST wrap it with authentication.
func HandleSuppliedInspectionForm(renderer *Renderer, trees AdminCommandTree) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if GetUsernameFromRequest(r) == "" {
			writeSuppliedInspectionAnswer(w, renderer, suppliedInspectionRefusal("Sign in before selecting an inspection.", nil).Error(), true, http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet {
			writeSuppliedInspectionAnswer(w, renderer, suppliedInspectionRefusal("Select inspection inputs with the inspection form.", nil).Error(), true, http.StatusMethodNotAllowed)
			return
		}
		query, err := parseSuppliedInspectionQuery(r)
		if err != nil {
			writeSuppliedInspectionAnswer(w, renderer, err.Error(), true, http.StatusBadRequest)
			return
		}
		selected, err := suppliedInspectionSelection(trees, query)
		if err != nil {
			writeSuppliedInspectionAnswer(w, renderer, err.Error(), true, http.StatusBadRequest)
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := io.WriteString(w, string(renderer.renderComponent("supplied inspection inputs", graphicalSuppliedInputs(&selected.view)))); err != nil {
			return
		}
		if _, err := io.WriteString(w, string(renderer.renderComponent("supplied inspection clear result", graphicalSuppliedClearAnswer()))); err != nil {
			return
		}
	}
}

func parseSuppliedInspectionQuery(r *http.Request) (string, error) {
	if len(r.URL.RawQuery) > maxCommandLength*3 {
		return "", suppliedInspectionRefusal("The inspection selection is too long.", nil)
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return "", suppliedInspectionRefusal("The inspection selection cannot be read.", err)
	}
	for name, values := range query {
		if name != webCommandField {
			return "", suppliedInspectionRefusal("The inspection selection contains an unknown field.", nil)
		}
		if len(values) != 1 {
			return "", suppliedInspectionRefusal("The inspection selection was repeated.", nil)
		}
	}
	return query.Get(webCommandField), nil
}

// HandleSuppliedInspection dispatches caller bytes through their registered
// owner and the shared operational formatter. The caller MUST apply
// authentication and same-origin POST middleware. It opens no editor,
// records no CLI history and never interprets a browser filename as a path.
func HandleSuppliedInspection(renderer *Renderer, mgr *EditorManager, dispatch CommandDispatcher, trees AdminCommandTree, authorizer aaa.Authorizer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := GetUsernameFromRequest(r)
		if username == "" {
			writeSuppliedInspectionAnswer(w, renderer, suppliedInspectionRefusal("Sign in before inspecting supplied configuration.", nil).Error(), true, http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			writeSuppliedInspectionAnswer(w, renderer, suppliedInspectionRefusal("The inspection form was not submitted.", nil).Error(), true, http.StatusMethodNotAllowed)
			return
		}
		if r.URL.RawQuery != "" {
			writeSuppliedInspectionAnswer(w, renderer, suppliedInspectionRefusal("Submit inspection choices in the form body.", nil).Error(), true, http.StatusBadRequest)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, editorContentMax)
		if err := readSuppliedInspectionForm(r); err != nil {
			writeSuppliedInspectionAnswer(w, renderer, err.Error(), true, suppliedInspectionErrorStatus(err))
			return
		}
		selected, err := suppliedInspectionSelection(trees, r.PostForm.Get(webCommandField))
		if err != nil {
			writeSuppliedInspectionAnswer(w, renderer, err.Error(), true, http.StatusBadRequest)
			return
		}
		line, err := selected.invocation(r)
		if err != nil {
			writeSuppliedInspectionAnswer(w, renderer, err.Error(), true, suppliedInspectionErrorStatus(err))
			return
		}
		ctx := r.Context()
		if plugin.CallerAuthorizer(ctx) == nil {
			ctx = plugin.WithCallerAuthorizer(ctx, authorizer)
		}
		output, rendered, complete, executionErr := executeTerminalOperationalResult(ctx, dispatch,
			username, r.RemoteAddr, line, mgr.currentCLIFormat(username))
		status := http.StatusOK
		if !complete {
			status = http.StatusBadRequest
			if operationalPermissionDenied(executionErr) {
				status = http.StatusForbidden
			}
		}
		if writeSuppliedInspectionAnswer(w, renderer, output, !complete, status) {
			rendered.TransportComplete()
		}
	}
}

func readSuppliedInspectionForm(r *http.Request) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return suppliedInspectionRefusal("The inspection form has an invalid content type.", err)
	}
	switch media {
	case "multipart/form-data":
		// The whole body is capped at this same bound, so multipart file
		// data stays in memory and cannot spill to daemon temporary files.
		err = r.ParseMultipartForm(editorContentMax)
	case "application/x-www-form-urlencoded":
		err = r.ParseForm()
	default:
		return suppliedInspectionRefusal("Use the native file or paste inspection form.", nil)
	}
	if err != nil {
		return suppliedInspectionRefusal("The complete inspection form could not be read; its limit is 4 MiB.", err)
	}
	return nil
}

func (selected *suppliedInspectionCommand) invocation(r *http.Request) (string, error) {
	allowed := map[string]bool{webCommandField: true}
	fileFields := make(map[string]bool, len(selected.view.Sources))
	for _, source := range selected.view.Sources {
		allowed[source.Field] = true
		allowed[source.FileField] = true
		fileFields[source.FileField] = true
	}
	for index := range selected.defs {
		def := &selected.defs[index]
		if def.Name() != selected.input.ContentArg {
			allowed[def.Name()] = true
		}
	}
	for name, values := range r.PostForm {
		if !allowed[name] {
			return "", suppliedInspectionRefusal("The inspection form contains an unknown field.", nil)
		}
		if len(values) != 1 {
			return "", suppliedInspectionRefusal("An inspection field was submitted more than once.", nil)
		}
		if fileFields[name] {
			if values[0] != "" {
				return "", suppliedInspectionRefusal("A file input must contain a native file selection.", nil)
			}
		}
	}
	if r.MultipartForm != nil {
		for name, files := range r.MultipartForm.File {
			if !fileFields[name] {
				return "", suppliedInspectionRefusal("The inspection form contains an unknown file field.", nil)
			}
			if len(files) != 1 {
				return "", suppliedInspectionRefusal("Choose one file for each inspection input.", nil)
			}
		}
	}
	sources := make(map[string]string, len(selected.view.Sources))
	var single []byte
	var label string
	total := 0
	for _, source := range selected.view.Sources {
		data, filename, err := selected.readSource(r, source)
		if err != nil {
			return "", err
		}
		if len(data) > selected.decodedMax-total {
			return "", selected.sizeRefusal()
		}
		total += len(data)
		if len(selected.input.Fields) > 0 {
			if !utf8.Valid(data) {
				return "", suppliedInspectionRefusal("Comparison inputs must contain valid UTF-8 text.", nil)
			}
			sources[source.Field] = string(data)
		} else {
			single, label = data, filename
		}
	}
	content := single
	if len(selected.input.Fields) > 0 {
		var err error
		content, err = json.Marshal(sources)
		if err != nil {
			return "", suppliedInspectionRefusal("The inspection inputs could not be composed.", err)
		}
	}
	if len(content) > selected.decodedMax {
		return "", selected.sizeRefusal()
	}
	encoded := base64.StdEncoding.EncodeToString(content)
	if len(encoded) > selected.encodedMax {
		return "", selected.sizeRefusal()
	}
	values := make(map[string]string, len(selected.defs))
	for index := range selected.defs {
		def := &selected.defs[index]
		if def.Name() == selected.input.ContentArg {
			continue
		}
		value := r.PostForm.Get(def.Name())
		if def.Kind() == command.ArgFlag {
			if value != "" {
				if value != boolTrue {
					return "", suppliedInspectionRefusal("An inspection selection is invalid.", nil)
				}
			}
		}
		values[def.Name()] = value
	}
	if selected.input.NameArg != "" {
		if values[selected.input.NameArg] == "" {
			values[selected.input.NameArg] = label
		}
	}
	values[selected.input.ContentArg] = encoded
	var invocation textbuf.Buffer
	command.WriteInvocation(&invocation, strings.Fields(selected.path), command.InvocationArgs(selected.defs), values)
	line := invocation.String()
	words, err := cli.ParseCommandWords(line)
	if err != nil {
		return "", suppliedInspectionRefusal("The inspection choices could not be encoded.", err)
	}
	tokens := words.Tokens()
	pathLength := len(strings.Fields(selected.path))
	if _, err := command.ValidateArgs(tokens[pathLength:], selected.defs, nil); err != nil {
		return "", suppliedInspectionRefusal("The inspection choices do not match the command's declared inputs.", err)
	}
	return line, nil
}

func (selected *suppliedInspectionCommand) readSource(r *http.Request, source GraphicalSuppliedSource) ([]byte, string, error) {
	pasted := r.PostForm.Get(source.Field)
	var headerName string
	var data []byte
	if r.MultipartForm != nil {
		files := r.MultipartForm.File[source.FileField]
		if len(files) > 0 {
			if pasted != "" {
				return nil, "", suppliedInspectionRefusal("Choose a file or paste text for each input, without combining both.", nil)
			}
			header := files[0]
			if header.Size > int64(selected.decodedMax) {
				return nil, "", selected.sizeRefusal()
			}
			file, err := header.Open()
			if err != nil {
				return nil, "", suppliedInspectionRefusal("The selected file cannot be opened.", err)
			}
			data, err = io.ReadAll(io.LimitReader(file, int64(selected.decodedMax)+1))
			err = errors.Join(err, file.Close())
			if err != nil {
				return nil, "", suppliedInspectionRefusal("The selected file cannot be read.", err)
			}
			headerName = path.Base(strings.ReplaceAll(header.Filename, `\`, "/"))
		}
	}
	if data == nil {
		if len(pasted) > selected.decodedMax {
			return nil, "", selected.sizeRefusal()
		}
		data = []byte(pasted)
	}
	if len(data) > selected.decodedMax {
		return nil, "", selected.sizeRefusal()
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil, "", suppliedInspectionRefusal("Each inspection input needs a selected file or pasted configuration.", nil)
	}
	return data, headerName, nil
}

type suppliedInspectionSize struct{ limit int }

func (size *suppliedInspectionSize) Error() string {
	return fmt.Sprintf("The complete inspection input exceeds %s bytes; select smaller inputs.", suppliedInspectionNumber(size.limit))
}

func (selected *suppliedInspectionCommand) sizeRefusal() error {
	return suppliedInspectionRefusal("The supplied inputs are too large.", &suppliedInspectionSize{limit: selected.decodedMax})
}

func suppliedInspectionErrorStatus(err error) int {
	if _, large := errors.AsType[*http.MaxBytesError](err); large {
		return http.StatusRequestEntityTooLarge
	}
	if _, large := errors.AsType[*suppliedInspectionSize](err); large {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}

func suppliedInspectionRefusal(problem string, cause error) *opererr.Error {
	return opererr.Wrap(cause, problem, "The supplied inspection did not change the candidate or running configuration.",
		"Open Inspect supplied configuration, choose the registered inspection and select a smaller valid file or paste its configuration, then inspect again")
}

func writeSuppliedInspectionAnswer(w http.ResponseWriter, renderer *Renderer, output string, failed bool, status int) bool {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, err := io.WriteString(w, string(renderer.renderComponent("supplied inspection answer", graphicalSuppliedAnswer(GraphicalSuppliedAnswer{Output: output, Failed: failed}))))
	return err == nil
}
