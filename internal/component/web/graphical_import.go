// Design: docs/architecture/web-interface.md -- candidate-only graphical import.
// Related: cli_editor.go -- the same shared editor load authority.

package web

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/opererr"
)

// HandleGraphicalImport stages supplied content through the shared editor.
// The caller MUST authenticate and apply same-origin middleware. The handler
// checks the same edit permission as CLI load before reading configuration.
func HandleGraphicalImport(mgr *EditorManager, schema *config.Schema, renderer *Renderer, authorizer aaa.Authorizer, suppliedTrees ...AdminCommandTree) http.HandlerFunc {
	var suppliedTree AdminCommandTree
	if len(suppliedTrees) > 0 {
		suppliedTree = suppliedTrees[0]
	}
	return func(w http.ResponseWriter, r *http.Request) {
		username := GetUsernameFromRequest(r)
		if username == "" {
			writeGraphicalImportError(w, r, renderer, opererr.New("Sign in before importing configuration.",
				"No authenticated session is available for this import.",
				"Open /login and sign in, then reopen Import into candidate"), http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			writeGraphicalImportError(w, r, renderer, graphicalImportRefusal("The import form was not submitted."), http.StatusMethodNotAllowed)
			return
		}
		if !authorizeWebConfigMutation(w, r, authorizer, username, webCommandConfigSave) {
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, editorContentMax)
		imported, err := readGraphicalImport(r, schema)
		if err != nil {
			status := http.StatusBadRequest
			if _, tooLarge := errors.AsType[*http.MaxBytesError](err); tooLarge {
				status = http.StatusRequestEntityTooLarge
			}
			writeGraphicalImportError(w, r, renderer, err, status)
			return
		}
		if _, err := mgr.GetOrCreate(username); err != nil {
			writeGraphicalImportError(w, r, renderer, editorSessionRefusal(err), http.StatusInternalServerError)
			return
		}
		result, err := imported.stage(mgr, username)
		if err != nil {
			writeGraphicalImportError(w, r, renderer, err, http.StatusBadRequest)
			return
		}
		writeGraphicalImportResult(w, r, mgr, schema, renderer, authorizer, username, imported.path, result, suppliedTree)
	}
}

// graphicalCandidateImport is an admitted form with privately owned path,
// command and content. Its zero value does not represent an admitted import;
// readGraphicalImport is the sole constructor and no nil pointer is valid.
type graphicalCandidateImport struct {
	path    []string
	command cli.CommandWords
	content string
}

// graphicalImportChoicesData gives each native form distinct control IDs.
type graphicalImportChoicesData struct {
	LocationID string
	ActionID   string
}

// GraphicalImportDefaults declares the native forms' initial scope and load
// operation. The rendered selects and offline browser replay MUST consume this
// authority rather than infer defaults from option order or copy their values.
func GraphicalImportDefaults() (location, operation string) {
	return "absolute", "merge"
}

// readGraphicalImport bounds its loops and file buffering by the caller's
// editorContentMax body reader. Multipart file memory uses that same bound,
// so a request cannot spill uploaded files to temporary disk storage.
func readGraphicalImport(r *http.Request, schema *config.Schema) (*graphicalCandidateImport, error) {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return nil, graphicalImportRefusal("The import form has an invalid content type.")
	}
	switch media {
	case "multipart/form-data":
		err = r.ParseMultipartForm(editorContentMax)
	case "application/x-www-form-urlencoded":
		err = r.ParseForm()
	default:
		return nil, graphicalImportRefusal("This import must use the file or paste form.")
	}
	if err != nil {
		return nil, opererr.Wrap(err, "The import form cannot be read.", candidateUnchangedWhy,
			"Choose a smaller file or paste less content so the complete import form is at most 4 MiB, then submit Import into candidate again")
	}
	for name, values := range r.PostForm {
		switch name {
		case "source", "location", "action", "path", "content":
		default:
			return nil, graphicalImportRefusal("The import form contains an unknown field.")
		}
		if len(values) != 1 {
			return nil, graphicalImportRefusal("An import choice was submitted more than once.")
		}
	}
	if r.MultipartForm != nil {
		for name, files := range r.MultipartForm.File {
			if name != "file" {
				return nil, graphicalImportRefusal("The import form contains an unknown file field.")
			}
			if len(files) != 1 {
				return nil, graphicalImportRefusal("Select one configuration file for the import.")
			}
		}
	}
	where := r.PostForm.Get("path")
	if len(where) > maxCommandLength {
		return nil, graphicalImportRefusal("The selected import path is too long.")
	}
	path, err := decodeConfigPath(where)
	if err != nil {
		return nil, opererr.Wrap(err, "The selected import path is invalid.", candidateUnchangedWhy,
			"Reload the YANG editor, select the item and reopen Import into candidate")
	}
	if len(path) > 0 {
		if schema.LookupTokenPath(path) == nil {
			return nil, graphicalImportRefusal("The selected import path is not in the configuration schema.")
		}
	}
	source := r.PostForm.Get("source")
	content, filename, err := readGraphicalImportContent(r, source)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(content) == "" {
		return nil, graphicalImportRefusal("The imported configuration is empty.")
	}
	words := []string{source, r.PostForm.Get("location"), r.PostForm.Get("action")}
	if source == "file" {
		words = append(words, filename)
	}
	// A browser filename is a label for supplied bytes. The shared grammar
	// and command lexer admit it without reading the daemon's filesystem.
	if _, _, _, _, err := cli.ParseLoadArgs(words); err != nil {
		return nil, opererr.Wrap(err, "The import choices are invalid.", candidateUnchangedWhy,
			"Choose File or Paste, Configuration root or Selected item, and Merge or Replace in Import into candidate")
	}
	command, err := cli.ParseCommandWords(graphicalCommandLine(append([]string{verbLoad}, words...)))
	if err != nil {
		return nil, err
	}
	return &graphicalCandidateImport{path: path, command: command, content: content}, nil
}

func readGraphicalImportContent(r *http.Request, source string) (content, filename string, err error) {
	switch source {
	case "terminal":
		if r.MultipartForm != nil {
			if len(r.MultipartForm.File) > 0 {
				return "", "", graphicalImportRefusal("A paste import cannot also contain a file.")
			}
		}
		return r.PostForm.Get("content"), "", nil
	case "file":
		// The multipart file is read below; a missing selection is refused.
	default:
		return "", "", graphicalImportRefusal("The configuration input source is unknown.")
	}
	if _, hasPaste := r.PostForm["content"]; hasPaste {
		return "", "", graphicalImportRefusal("A file import cannot also contain pasted content.")
	}
	if r.MultipartForm == nil {
		return "", "", graphicalImportRefusal("No configuration file was selected.")
	}
	files := r.MultipartForm.File["file"]
	if len(files) != 1 {
		return "", "", graphicalImportRefusal("Select one configuration file for the import.")
	}
	filename = files[0].Filename
	if len(filename) > 512 {
		return "", "", graphicalImportRefusal("The configuration filename is too long.")
	}
	if !validPathToken(filename) {
		return "", "", graphicalImportRefusal("The configuration filename contains invalid characters.")
	}
	file, err := files[0].Open()
	if err != nil {
		return "", "", opererr.Wrap(err, "The selected configuration file cannot be opened.", candidateUnchangedWhy,
			"Select the local file again and submit Import into candidate")
	}
	data, readErr := io.ReadAll(file)
	if err := errors.Join(readErr, file.Close()); err != nil {
		return "", "", opererr.Wrap(err, "The selected configuration file cannot be read.", candidateUnchangedWhy,
			"Select the local file again and submit Import into candidate")
	}
	return string(data), filename, nil
}

// stage uses the manager's existing per-user lock across shared parsing and
// atomic load. It neither invokes the legacy upload writer nor commits.
func (imported *graphicalCandidateImport) stage(mgr *EditorManager, username string) (contract.ConfigCommandResult, error) {
	return mgr.ConfigCommand(username, contract.ConfigCommandRequest{
		Line: imported.command.String(), Path: imported.path, Content: &imported.content,
	})
}

// GraphicalImportError projects an actual import error into the refusal the
// native form displays. The HTTP writer and corpus MUST share this producer;
// callers MUST supply a non-nil error from an attempted import.
func GraphicalImportError(err error) *opererr.Error {
	if refusal, typed := errors.AsType[*opererr.Error](err); typed {
		return refusal
	}
	return opererr.Wrap(err, "The configuration could not be imported.", candidateUnchangedWhy,
		"Correct the configuration described in this message, then select the file or paste the corrected content in Import into candidate")
}

func writeGraphicalImportError(w http.ResponseWriter, r *http.Request, renderer *Renderer, err error, status int) {
	refusal := GraphicalImportError(err)
	if r.Header.Get("HX-Request") == htmxRequestTrue {
		// Retain the selected file and pasted text so the operator can correct
		// the input. Only the out-of-band error panel needs to change.
		w.Header().Set("HX-Reswap", "none")
	}
	if renderer == nil {
		http.Error(w, refusal.Error(), status)
		return
	}
	WriteOOBError(w, renderer, "configuration import", refusal.Error(), status)
}

func writeGraphicalImportResult(w http.ResponseWriter, r *http.Request, mgr *EditorManager, schema *config.Schema, renderer *Renderer, authorizer aaa.Authorizer, username string, path []string, result contract.ConfigCommandResult, suppliedTree AdminCommandTree) {
	tree, err := mgr.viewTree(username)
	if err != nil {
		writeGraphicalImportError(w, r, renderer, opererr.Wrap(err, "The imported candidate cannot be displayed.",
			"The import was staged, but the updated view could not be read.",
			"Reload the YANG editor and review your candidate before committing"), http.StatusInternalServerError)
		return
	}
	path = graphicalSurvivingPath(schema, tree, path)
	where := showPathPrefix
	if len(path) > 0 {
		where += encodeConfigPath(path) + "/"
	}
	w.Header().Set("HX-Push-Url", where)
	output := result.Feedback
	if result.Output != "" {
		output += "\n" + result.Output
	}
	output += "\nReview the candidate changes and commit separately to apply them."
	if renderer == nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if _, err := w.Write([]byte(output)); err != nil {
			return
		}
		return
	}
	data := buildFragmentData(schema, tree, path)
	data.Username = username
	data.ReadOnly = !canEdit(r, authorizer)
	data.Actions = mgr.graphicalActions(schema, tree, path, username)
	data.Actions.Supplied = buildGraphicalSuppliedPanel(suppliedTree)
	if data.Actions.Output != "" {
		output += "\n" + data.Actions.Output
	}
	data.Actions.Output = output
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write([]byte(renderer.renderComponent("candidate import response", oobResponse(data)))); err != nil {
		return
	}
	if _, err := w.Write([]byte(renderer.renderComponent("candidate import commit state", oobSaveOK(saveOK(r, authorizer, mgr.ChangeCount(username)))))); err != nil {
		return
	}
}

func graphicalImportRefusal(problem string) *opererr.Error {
	return opererr.New(problem, candidateUnchangedWhy,
		"Open Import into candidate, choose a file or paste configuration, then submit the form again")
}
