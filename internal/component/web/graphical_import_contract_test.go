// Design: docs/architecture/web-components.md -- native import defaults and refusal projection.

package web

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// TestGraphicalImportDefaultsRenderNativeChoices reads the actual rendered
// file and paste selects, including explicit selection rather than option order.
func TestGraphicalImportDefaultsRenderNativeChoices(t *testing.T) {
	location, operation := GraphicalImportDefaults()
	var body bytes.Buffer
	if err := graphicalImportPanel(&GraphicalActions{}).Render(context.Background(), &body); err != nil {
		t.Fatal(err)
	}
	tokens := html.NewTokenizer(&body)
	name := ""
	selected := 0
	selects := 0
	// Tokenization ends at the finite rendered component's EOF.
	for {
		switch tokens.Next() {
		case html.ErrorToken:
			if err := tokens.Err(); !errors.Is(err, io.EOF) {
				t.Fatal(err)
			}
			if selects != 4 {
				t.Fatalf("native choice controls = %d, want file and paste scope/operation", selects)
			}
			return
		case html.StartTagToken:
			token := tokens.Token()
			if token.Data == "select" {
				selected = 0
				for _, attribute := range token.Attr {
					if attribute.Key == "name" {
						name = attribute.Val
					}
				}
				continue
			}
			if token.Data != "option" {
				continue
			}
			value := ""
			chosen := false
			for _, attribute := range token.Attr {
				switch attribute.Key {
				case "value":
					value = attribute.Val
				case "selected":
					chosen = true
				}
			}
			if !chosen {
				continue
			}
			selected++
			want := ""
			switch name {
			case "location":
				want = location
			case "action":
				want = operation
			default:
				t.Fatalf("unexpected native choice %q", name)
			}
			if value != want {
				t.Errorf("rendered %s default = %q, authority = %q", name, value, want)
			}
		case html.TextToken, html.SelfClosingTagToken, html.CommentToken, html.DoctypeToken:
			continue
		case html.EndTagToken:
			if tokens.Token().Data == "select" {
				if selected != 1 {
					t.Errorf("native %s explicitly selected %d options, want 1", name, selected)
				}
				selects++
				name = ""
			}
		}
	}
}

// TestGraphicalImportErrorMatchesResponse binds the shared projection to the
// actual HTTP response for both parser errors and already typed form refusals.
func TestGraphicalImportErrorMatchesResponse(t *testing.T) {
	for _, cause := range []error{errors.New("malformed configuration"), graphicalImportRefusal("The import form is invalid.")} {
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/config/import", http.NoBody)
		writeGraphicalImportError(response, request, nil, cause, http.StatusBadRequest)
		refusal := GraphicalImportError(cause)
		if response.Code != http.StatusBadRequest {
			t.Errorf("refusal status = %d", response.Code)
		}
		if strings.TrimSuffix(response.Body.String(), "\n") != refusal.Error() {
			t.Errorf("native error response differs from the corpus projection: %q", response.Body.String())
		}
		if !errors.Is(refusal, cause) {
			t.Error("import error projection lost its actual cause")
		}
	}
}
