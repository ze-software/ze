//go:build ze_bgp

// Design: docs/architecture/testing/runner-architecture.md -- native supplied inspection replay.

package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/command"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/web"
	webtesting "github.com/ze-software/ze/internal/component/web/testing"
)

// browserCorpusInspection retains only controls the actual selected form
// rendered. Selecting another command replaces its fields and selected files.
// It is constructed from a successful native form response, never a zero value.
type browserCorpusInspection struct {
	controls map[string]browserCorpusInspectionControl
	values   url.Values
	files    map[string]browserCorpusLoad
}

type browserCorpusInspectionControl struct {
	field string
	file  bool
}

// inspectionAction reads native control ownership from the rendered form,
// rather than carrying a second list of commands, fields or content budgets.
func (b *browserCorpus) inspectionAction(root string, action *webtesting.WBAction, consumed map[string]bool) (bool, error) {
	if action.Kind == "open" {
		b.inspection = nil
		return false, nil
	}
	id := action.Values["id"]
	if !strings.HasPrefix(id, "graphical-supplied-") {
		return false, nil
	}
	if b.graphical == nil {
		return true, errors.New("supplied inspection control has no selected graphical page")
	}
	if action.Kind == "click" {
		if id == "graphical-supplied-toggle" {
			return true, nil
		}
	}
	if action.Kind == "select" {
		if id == "graphical-supplied-command" {
			response, err := b.inspectionRequest(http.MethodGet,
				"/config/inspect/form?command="+url.QueryEscape(action.Values["value"]), nil, "")
			if err != nil {
				return true, err
			}
			if response.Code != http.StatusOK {
				return true, fmt.Errorf("native inspection selection refused: %s", response.Body.String())
			}
			b.inspection, err = browserInspectionForm(response.Body)
			return true, err
		}
	}
	if b.inspection == nil {
		return true, errors.New("supplied inspection has no replayed native command selection")
	}
	if action.Kind == "click" {
		if id == "graphical-supplied-submit" {
			return true, b.inspectionSubmit(consumed)
		}
	}
	control, found := b.inspection.controls[id]
	if !found {
		return true, fmt.Errorf("control %q is absent from the selected native inspection", id)
	}
	if action.Kind == "upload" {
		if !control.file {
			return true, errors.New("native inspection upload did not select a file control")
		}
		file := browserCorpusLoad{}
		if err := file.upload(root, action.Values["file"]); err != nil {
			return true, err
		}
		b.inspection.files[control.field] = file
		return true, nil
	}
	if action.Kind == "fill" {
		if control.file {
			return true, errors.New("native inspection file control cannot accept pasted text")
		}
		b.inspection.values.Set(control.field, action.Values["value"])
		return true, nil
	}
	return true, fmt.Errorf("action %q has no native supplied inspection composition", action.Kind)
}

// browserInspectionForm reads IDs and field names from the native producer.
// The tokenizer is bounded by that producer's finite rendered form, and only
// named input/textarea controls become payload bindings.
func browserInspectionForm(body io.Reader) (*browserCorpusInspection, error) {
	form := &browserCorpusInspection{
		controls: make(map[string]browserCorpusInspectionControl),
		values:   make(url.Values), files: make(map[string]browserCorpusLoad),
	}
	tokens := html.NewTokenizer(body)
	for {
		kind := tokens.Next()
		if kind == html.ErrorToken {
			if err := tokens.Err(); !errors.Is(err, io.EOF) {
				return nil, err
			}
			break
		}
		if kind != html.StartTagToken {
			if kind != html.SelfClosingTagToken {
				continue
			}
		}
		token := tokens.Token()
		if token.Data != "input" {
			if token.Data != "textarea" {
				continue
			}
		}
		attributes := make(map[string]string, len(token.Attr))
		for _, attribute := range token.Attr {
			attributes[attribute.Key] = attribute.Val
		}
		name := attributes["name"]
		if name == "" {
			continue
		}
		if attributes["type"] == "hidden" {
			form.values.Set(name, attributes["value"])
			continue
		}
		if id := attributes["id"]; id != "" {
			form.controls[id] = browserCorpusInspectionControl{field: name, file: attributes["type"] == "file"}
		}
	}
	if form.values.Get("command") == "" {
		return nil, errors.New("native inspection form has no selected command")
	}
	if len(form.controls) == 0 {
		return nil, errors.New("native inspection form has no addressable input controls")
	}
	return form, nil
}

func (b *browserCorpus) inspectionSubmit(consumed map[string]bool) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, values := range b.inspection.values {
		for _, value := range values {
			if err := writer.WriteField(name, value); err != nil {
				return err
			}
		}
	}
	for name, file := range b.inspection.files {
		part, err := writer.CreateFormFile(name, filepath.Base(file.step.source))
		if err != nil {
			return err
		}
		if _, err := io.WriteString(part, file.step.body); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	response, err := b.inspectionRequest(http.MethodPost, "/config/inspect", &body, writer.FormDataContentType())
	if err != nil {
		return err
	}
	if response.Code != http.StatusOK {
		// Only an adjacent exact native problem may declare a size refusal.
		// Authentication, authorization, unknown commands and producer failures
		// remain findings with standalone input validation still active.
		problem := browserInspectionProblem(response.Body)
		if response.Code != http.StatusRequestEntityTooLarge {
			return fmt.Errorf("native supplied inspection refused: %s", problem)
		}
		if b.importError == "" {
			return fmt.Errorf("native supplied inspection has an undeclared size refusal: %s", problem)
		}
		if b.importError != problem {
			return fmt.Errorf("native supplied inspection size refusal differs from its declaration: %s", problem)
		}
		b.refused++
	}
	for _, file := range b.inspection.files {
		consumed[file.step.source] = true
	}
	b.inspected++
	return nil
}

// browserInspectionProblem reads the native result's complete first line, the
// same Problem the operator sees. Substring declarations cannot claim a refusal.
func browserInspectionProblem(body io.Reader) string {
	tokens := html.NewTokenizer(body)
	result := false
	for {
		kind := tokens.Next()
		if kind == html.ErrorToken {
			return "the native inspection returned no result"
		}
		if kind == html.StartTagToken {
			for _, attribute := range tokens.Token().Attr {
				if attribute.Key == "id" {
					if attribute.Val == "graphical-supplied-result" {
						result = true
					}
				}
			}
		}
		if kind == html.TextToken {
			if result {
				problem, _, _ := strings.Cut(string(tokens.Text()), "\n")
				return problem
			}
		}
	}
}

// inspectionRequest drives the actual native adapter and registered daemon
// dispatcher in memory. The web manager deliberately has no editor factories
// or store: this read must never open or mutate a candidate.
func (b *browserCorpus) inspectionRequest(method, target string, body io.Reader, contentType string) (response *httptest.ResponseRecorder, err error) {
	resolved, err := configyang.DefaultLoader()
	if err != nil {
		return nil, err
	}
	tree := configyang.BuildCommandTree(resolved)
	renderer, err := web.NewRenderer()
	if err != nil {
		return nil, err
	}
	dispatcher := pluginserver.NewDispatcher()
	if err := pluginserver.LoadBuiltins(dispatcher, configyang.WireMethodToPath(resolved),
		configyang.PathToDescription(resolved), configyang.PathToHelp(resolved), configyang.PathToArgDefs(resolved)); err != nil {
		return nil, err
	}
	var policy aaa.Authorizer
	if b.auth != nil {
		if b.identity == nil {
			return nil, errors.New("browser authentication required before supplied inspection")
		}
		policy = aaa.BindProfiles(authz.StoreAuthorizer{Store: b.identity.policy}, b.identity.profiles)
	}
	dispatcher.SetAuthorizer(policy)
	dispatch := func(ctx context.Context, caller plugin.CallerIdentity, line string) (*plugin.Response, error) {
		return dispatcher.Dispatch(&pluginserver.CommandContext{
			Username: caller.Username, RemoteAddr: caller.RemoteAddr, Authorizer: caller.Authorizer,
			RequestContext: ctx, Surface: "web", Sender: plugin.OperatorSender(),
		}, line)
	}
	manager := web.NewEditorManager(nil, "", nil, nil, nil)
	defer func() { err = errors.Join(err, manager.Close()) }()
	trees := func() *command.Node { return tree }
	var handler http.Handler = web.HandleSuppliedInspectionForm(renderer, trees)
	if method == http.MethodPost {
		handler = web.HandleSuppliedInspection(renderer, manager, dispatch, trees, policy)
	}
	request := httptest.NewRequestWithContext(b.context, method, target, body)
	request.Header.Set("Content-Type", contentType)
	if b.auth == nil {
		handler = web.InsecureMiddleware(handler)
	} else {
		request.SetBasicAuth(b.identity.user, b.identity.password)
		handler = web.AuthMiddlewareWithAudit(web.NewSessionStore(nil),
			aaa.WithProfileAuthorizer(&b.auth.local, policy), func(http.ResponseWriter, *http.Request) {}, handler, nil)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response, nil
}
