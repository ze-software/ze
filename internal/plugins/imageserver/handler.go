// Design: plan/spec-install-3-image-server.md -- HTTP image/boot file serving

package imageserver

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

const (
	sshHost = "127.0.0.1"
	sshPort = "2222"
)

type imageHandler struct {
	imageDir string
	bootDir  string
	zefsPath string
}

func newMux(cfg imageConfig, zefsPath string) *http.ServeMux {
	h := &imageHandler{
		imageDir: cfg.ImageDirectory,
		bootDir:  cfg.BootDirectory,
		zefsPath: zefsPath,
	}

	mux := http.NewServeMux()
	if cfg.ImageDirectory != "" {
		mux.HandleFunc("/install/image/", h.serveImage)
	}
	if cfg.BootDirectory != "" {
		mux.HandleFunc("/install/boot/", h.serveBoot)
	}
	if zefsPath != "" {
		mux.HandleFunc("/install/database.zefs", h.serveZefs)
	}
	return mux
}

func buildZefsDB(dir, username, passwordHash string) (string, error) {
	path := filepath.Join(dir, "database.zefs")
	store, err := zefs.Create(path)
	if err != nil {
		return "", err
	}

	entries := []struct{ key, value string }{
		{zefs.KeySSHUsername.Key(sshHost, sshPort), username},
		{zefs.KeySSHPassword.Key(sshHost, sshPort), passwordHash},
		{zefs.KeyLocalAdminUsername.Pattern, username},
		{zefs.KeyLocalAdminPassword.Pattern, passwordHash},
		{zefs.KeySSHDefault.Pattern, sshHost + "/" + sshPort},
	}

	for _, e := range entries {
		if writeErr := store.WriteFile(e.key, []byte(e.value), 0); writeErr != nil {
			// Best-effort cleanup; return the write error (the root cause),
			// not a secondary close/remove failure that would mask it.
			store.Close()   //nolint:errcheck // cleanup; writeErr is the real error
			os.Remove(path) //nolint:errcheck // cleanup; writeErr is the real error
			return "", writeErr
		}
	}

	if err := store.Close(); err != nil {
		os.Remove(path) //nolint:errcheck // cleanup; err is the real error
		return "", err
	}

	return path, nil
}

func (h *imageHandler) serveImage(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/install/image/")
	h.serveFromDir(w, r, h.imageDir, name)
}

func (h *imageHandler) serveBoot(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/install/boot/")
	h.serveFromDir(w, r, h.bootDir, name)
}

// serveZefs serves the pre-provisioned zefs database (SSH username + password
// HASH, no plaintext) to the PXE installer. Served without authentication by
// design: PXE provisioning runs on a trusted install network where the kernel,
// initrd, and disk image are themselves fetched unauthenticated, so the trust
// boundary is the network, not this endpoint.
func (h *imageHandler) serveZefs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	http.ServeFile(w, r, h.zefsPath)
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
