// Design: plan/spec-install-3-image-server.md -- HTTP image/boot file serving

package imageserver

import (
	"net/http"
	"path/filepath"
	"strings"
)

type imageHandler struct {
	imageDir string
	bootDir  string
}

func newMux(cfg imageConfig) *http.ServeMux {
	h := &imageHandler{
		imageDir: cfg.ImageDirectory,
		bootDir:  cfg.BootDirectory,
	}

	mux := http.NewServeMux()
	if cfg.ImageDirectory != "" {
		mux.HandleFunc("/install/image/", h.serveImage)
	}
	if cfg.BootDirectory != "" {
		mux.HandleFunc("/install/boot/", h.serveBoot)
	}
	return mux
}

func (h *imageHandler) serveImage(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/install/image/")
	h.serveFromDir(w, r, h.imageDir, name)
}

func (h *imageHandler) serveBoot(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/install/boot/")
	h.serveFromDir(w, r, h.bootDir, name)
}

func (h *imageHandler) serveFromDir(w http.ResponseWriter, r *http.Request, dir, name string) {
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		http.NotFound(w, r)
		return
	}
	if name == "." || name == ".." || strings.ContainsRune(name, 0) {
		http.NotFound(w, r)
		return
	}

	cleaned := filepath.Clean(name)
	if cleaned != name {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, filepath.Join(dir, cleaned))
}
