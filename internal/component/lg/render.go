// Design: docs/architecture/web-interface.md -- LG template rendering
// Overview: server.go -- LG server and route registration

package lg

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
)

// lgFuncMap defines the template functions available to LG templates.
var lgFuncMap = template.FuncMap{
	"stateClass": func(state string) string {
		switch state {
		case "established":
			return "state-up"
		case "idle", "active", "connect", "opensent", "openconfirm":
			return "state-down"
		}
		return "state-unknown"
	},
	"formatNum": formatNumCommas,
	"formatASPath": func(v any) string {
		arr, ok := v.([]any)
		if !ok {
			return ""
		}
		var parts []string
		for _, a := range arr {
			parts = append(parts, fmt.Sprintf("%v", a))
		}
		return strings.Join(parts, " ")
	},
	"formatCommunities": func(v any) string {
		arr, ok := v.([]any)
		if !ok {
			return ""
		}
		var parts []string
		for _, a := range arr {
			parts = append(parts, fmt.Sprintf("%v", a))
		}
		return strings.Join(parts, ", ")
	},
	"isBest": func(v any) bool {
		route, ok := v.(map[string]any)
		if !ok {
			return false
		}
		return getBool(route, "best")
	},
}

// parseLGTemplates parses all embedded HTML template files.
func parseLGTemplates() (*template.Template, error) {
	tplFS, err := fs.Sub(templatesFS, "templates")
	if err != nil {
		return nil, fmt.Errorf("lg: embedded templates sub-fs: %w", err)
	}

	t, err := template.New("").Option("missingkey=zero").Funcs(lgFuncMap).ParseFS(tplFS, "*.html")
	if err != nil {
		return nil, fmt.Errorf("lg: parse templates: %w", err)
	}

	return t, nil
}

// formatNumCommas formats a value as an integer with comma separators.
// Handles float64, int, int64, and string inputs. Returns the value as-is for non-numeric types.
func formatNumCommas(v any) string {
	var n int64

	switch val := v.(type) {
	case float64:
		n = int64(val)
	case int:
		n = int64(val)
	case int64:
		n = val
	case string:
		var f float64
		if _, err := fmt.Sscanf(val, "%f", &f); err != nil {
			return val
		}
		n = int64(f)
	case nil:
		return ""
	case bool:
		return fmt.Sprintf("%v", val)
	case []any, map[string]any:
		return fmt.Sprintf("%v", val)
	}

	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	s := fmt.Sprintf("%d", n)
	length := len(s)

	var result strings.Builder
	if negative {
		result.WriteByte('-')
	}

	for i, c := range s {
		if i > 0 && (length-i)%3 == 0 {
			result.WriteByte(',')
		}
		result.WriteRune(c)
	}

	return result.String()
}

// renderPage renders a full HTML page with layout wrapper.
// Both inner content and layout are rendered to buffers before writing to w,
// so a template error never produces a partial 200 response.
func (s *LGServer) renderPage(w http.ResponseWriter, name string, data map[string]any) {
	var content bytes.Buffer
	if err := s.templates.ExecuteTemplate(&content, name, data); err != nil {
		s.logger.Warn("template render error", "template", name, "error", err)
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}

	layoutData := map[string]any{
		"Title":     data["Title"],
		"ActiveTab": data["ActiveTab"],
		"Content":   template.HTML(content.String()), //nolint:gosec // pre-rendered trusted template output
	}

	var page bytes.Buffer
	if err := s.templates.ExecuteTemplate(&page, "layout", layoutData); err != nil {
		s.logger.Warn("layout render error", "error", err)
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write(page.Bytes()); err != nil {
		s.logger.Debug("write page failed", "error", err)
	}
}

// renderFragment renders an HTML fragment (no layout wrapper).
// Rendered to buffer first to avoid partial 200 responses on template errors.
func (s *LGServer) renderFragment(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, name, data); err != nil {
		s.logger.Warn("fragment render error", "template", name, "error", err)
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write(buf.Bytes()); err != nil {
		s.logger.Debug("write fragment failed", "error", err)
	}
}

// renderToString renders a template to a string.
func (s *LGServer) renderToString(name string, data any) string {
	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, name, data); err != nil {
		s.logger.Warn("render to string error", "template", name, "error", err)
		return ""
	}
	return buf.String()
}
