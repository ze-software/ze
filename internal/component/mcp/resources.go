// Design: docs/architecture/mcp/overview.md -- MCP resources capability and UI asset serving
// Related: streamable.go -- method dispatch for resources/list and resources/read

package mcp

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"strings"

	mcpui "github.com/ze-software/ze/internal/component/mcp/ui"
)

const uiScheme = "ui://"

const maxURILength = 2048

const maxPathDepth = 8

var (
	errInvalidURI       = errors.New("mcp: invalid uri")
	errResourceNotFound = errors.New("mcp: resource not found")
)

var mimeByExt = map[string]string{
	".html":  "text/html",
	".htm":   "text/html",
	".css":   "text/css",
	".js":    "application/javascript",
	".json":  "application/json",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".gif":   "image/gif",
	".ico":   "image/x-icon",
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".ttf":   "font/ttf",
}

var textMIMEPrefixes = []string{"text/", "application/javascript", "application/json", "image/svg+xml"}

func sniffMIME(name string) string {
	ext := path.Ext(name)
	if m, ok := mimeByExt[ext]; ok {
		return m
	}
	return "application/octet-stream"
}

func isTextMIME(mime string) bool {
	for _, prefix := range textMIMEPrefixes {
		if strings.HasPrefix(mime, prefix) {
			return true
		}
	}
	return false
}

func listResources() []map[string]any {
	result := make([]map[string]any, 0)
	_ = fs.WalkDir(mcpui.FS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		if info.Size() == 0 {
			return nil
		}
		result = append(result, map[string]any{
			"uri":      uiScheme + p,
			"name":     p,
			"mimeType": sniffMIME(p),
		})
		return nil
	})
	return result
}

func validateResourceURI(uri string) (string, error) {
	if len(uri) > maxURILength {
		return "", errInvalidURI
	}
	if !strings.HasPrefix(uri, uiScheme) {
		return "", errInvalidURI
	}
	rel := strings.TrimPrefix(uri, uiScheme)
	if rel == "" {
		return "", errInvalidURI
	}
	cleaned := path.Clean(rel)
	if cleaned != rel || strings.HasPrefix(cleaned, "..") || strings.HasPrefix(cleaned, "/") {
		return "", errInvalidURI
	}
	if strings.Count(cleaned, "/")+1 > maxPathDepth {
		return "", errInvalidURI
	}
	return cleaned, nil
}

func readResource(uri string) (map[string]any, error) {
	cleaned, err := validateResourceURI(uri)
	if err != nil {
		return nil, err
	}
	data, readErr := fs.ReadFile(mcpui.FS, cleaned)
	if readErr != nil {
		return nil, errResourceNotFound
	}
	if len(data) == 0 {
		return nil, errResourceNotFound
	}
	mime := sniffMIME(cleaned)
	rc := map[string]any{
		"uri":      uri,
		"mimeType": mime,
	}
	if isTextMIME(mime) {
		rc["text"] = string(data)
	} else {
		rc["blob"] = base64.StdEncoding.EncodeToString(data)
	}
	return rc, nil
}

func (s *Streamable) resourcesList(sess *session, req *request) *response {
	if !sess.ClientSupportsResources() {
		return s.fail(req.ID, -32601, "method not found: resources/list")
	}
	return s.ok(req.ID, map[string]any{"resources": s.cachedResources})
}

func (s *Streamable) resourcesRead(sess *session, req *request) *response {
	if !sess.ClientSupportsResources() {
		return s.fail(req.ID, -32601, "method not found: resources/read")
	}
	var params struct {
		URI string `json:"uri"`
	}
	if len(req.Params) > 0 {
		_ = json.Unmarshal(req.Params, &params)
	}
	if params.URI == "" {
		return s.fail(req.ID, -32602, "invalid uri")
	}
	content, err := readResource(params.URI)
	if err != nil {
		code := -32602
		msg := "invalid uri"
		if errors.Is(err, errResourceNotFound) {
			msg = "resource not found"
		}
		return s.fail(req.ID, code, msg)
	}
	return s.ok(req.ID, map[string]any{"contents": []map[string]any{content}})
}
