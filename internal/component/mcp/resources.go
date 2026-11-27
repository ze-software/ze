// Design: docs/architecture/mcp/overview.md -- MCP resources capability, UI assets, and read-only ze:// state resources
// Related: streamable.go -- method dispatch for resources/list and resources/read
// Related: caching.go -- supplies both results' ttlMs and cacheScope

package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"strings"

	mcpui "github.com/ze-software/ze/internal/component/mcp/ui"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	uiScheme          = "ui://"
	zeScheme          = "ze://"
	triageResourceURI = "ze://triage"
	triageCommandLine = "show system triage"
)

const (
	resourceKeyMIMEType   = "mimeType"
	msgInvalidURI         = "invalid uri"
	msgResourceNotFound   = "resource not found"
	whyInvalidResourceURI = "resources/read requires a non-empty ui:// or ze:// resource URI in params.uri."
	whyResourceNotFound   = "Only registered ui:// assets and ze://triage (when command dispatch is active) are readable MCP resources."
	fixListMCPResources   = "call resources/list to inspect available resource URIs"
)

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
	".json":  mediaTypeJSON,
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

var textMIMEPrefixes = []string{"text/", "application/javascript", mediaTypeJSON, "image/svg+xml"}

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
			schemaFormatURI:     uiScheme + p,
			toolKeyName:         p,
			resourceKeyMIMEType: sniffMIME(p),
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
		schemaFormatURI:     uri,
		resourceKeyMIMEType: mime,
	}
	if isTextMIME(mime) {
		rc[contentTypeText] = string(data)
	} else {
		rc["blob"] = base64.StdEncoding.EncodeToString(data)
	}
	return rc, nil
}

// resourcesList answers resources/list for every conformant caller.
//
// There is deliberately NO client-capability gate here. `resources` is a member
// of ServerCapabilities, not of ClientCapabilities -- whose complete member set
// in MCP 2026-07-28 is `experimental`, `roots`, `sampling`, `elicitation` and
// `extensions`. A conformant client therefore never declares `resources`. A
// gate on it refused every conformant caller, while server/discover advertised
// `capabilities.resources` and tools/list published `_meta.ui.resourceUri`
// pointing at these very assets.
//
// The -32021 requirement it was modeled on ("a server MUST NOT rely on
// capabilities the client has not declared") governs capabilities the CLIENT
// provides, which this server still enforces for the Tasks extension. Serving a
// capability the server itself advertises is not relying on the client for
// anything.
func (s *Streamable) resourcesList(req *request) *response {
	return s.ok(req.ID, map[string]any{"resources": s.cachedResources})
}

// resourcesRead answers resources/read. It is ungated for clientCapabilities
// for the same reason as resourcesList; read-only ze:// state resources enforce
// per-identity command scope before dispatching. Cache hints are NEVER stamped
// here: stampCacheHints (caching.go) is the sole hint-producing site.
func (s *Streamable) resourcesRead(ctx context.Context, scope requestScope, req *request, remoteAddr string) *response {
	var params struct {
		URI string `json:"uri"`
	}
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.fail(req.ID, rpcInvalidParams, opererr.Wrap(err, msgInvalidURI, whyInvalidResourceURI, fixListMCPResources).Error())
		}
	}
	if params.URI == "" {
		return s.fail(req.ID, rpcInvalidParams, opererr.New(msgInvalidURI, whyInvalidResourceURI, fixListMCPResources).Error())
	}
	if params.URI == triageResourceURI {
		return s.readTriageResource(ctx, scope, req, remoteAddr)
	}
	if strings.HasPrefix(params.URI, zeScheme) {
		return s.fail(req.ID, rpcInvalidParams, opererr.New(msgResourceNotFound+": "+params.URI, whyResourceNotFound, fixListMCPResources).Error())
	}
	content, err := readResource(params.URI)
	if err != nil {
		msg := msgInvalidURI
		if errors.Is(err, errResourceNotFound) {
			msg = msgResourceNotFound
		}
		return s.fail(req.ID, rpcInvalidParams, msg)
	}
	return s.ok(req.ID, map[string]any{"contents": []map[string]any{content}})
}

// readTriageResource serves the read-only ze://triage snapshot through the
// command dispatcher after enforcing the caller's command scope.
func (s *Streamable) readTriageResource(ctx context.Context, scope requestScope, req *request, remoteAddr string) *response {
	if s.cfg.Dispatch == nil {
		return s.fail(req.ID, rpcInvalidParams, opererr.New(msgResourceNotFound+": "+triageResourceURI, whyResourceNotFound, fixListMCPResources).Error())
	}
	if denied := s.checkCommandScope(req.ID, scope.Identity, triageCommandLine); denied != nil {
		return denied
	}
	caller := plugin.CallerIdentity{
		Username:   scope.Identity.Name,
		RemoteAddr: remoteAddr,
	}
	rendered, err := s.cfg.Dispatch.JSON(ctx, caller, triageCommandLine)
	if err != nil {
		opErr := opererr.Wrap(
			err,
			"failed to read "+triageResourceURI,
			"The daemon could not complete 'show system triage' for this resource read.",
			"run 'ze show system triage' or check daemon health with 'ze show doctor'",
		)
		resp := s.fail(req.ID, rpcInternalError, opErr.Error())
		resp.completion = rendered
		return resp
	}
	resp := s.ok(req.ID, map[string]any{
		"contents": []map[string]any{
			{
				schemaFormatURI:     triageResourceURI,
				resourceKeyMIMEType: mediaTypeJSON,
				contentTypeText:     rendered.Output,
			},
		},
	})
	resp.completion = rendered
	return resp
}
