// Design: plan/spec-install-3-image-server.md -- HTTP image/boot file serving

package imageserver

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

const (
	sshHost = "127.0.0.1"
	sshPort = "2222"
)

type imageHandler struct {
	imageDir   string
	bootDir    string
	zefsPath   string
	serverAddr string
	serverPort int
}

func newMux(cfg imageConfig, zefsPath, serverAddr string) *http.ServeMux {
	h := &imageHandler{
		imageDir:   cfg.ImageDirectory,
		bootDir:    cfg.BootDirectory,
		zefsPath:   zefsPath,
		serverAddr: serverAddr,
		serverPort: cfg.ListenPort,
	}

	mux := http.NewServeMux()
	if cfg.ImageDirectory != "" {
		mux.HandleFunc("/install/image/", h.serveImage)
	}
	if cfg.BootDirectory != "" {
		if serverAddr != "" {
			mux.HandleFunc("/install/boot/boot.ipxe", h.serveBootIPXE)
		}
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

func (h *imageHandler) serveBootIPXE(w http.ResponseWriter, r *http.Request) {
	if _, err := os.Stat(filepath.Join(h.bootDir, "boot.ipxe")); err == nil {
		h.serveFromDir(w, r, h.bootDir, "boot.ipxe")
		return
	}

	imgName, err := latestImage(h.imageDir)
	if err != nil {
		http.Error(w, "no image found in image-directory", http.StatusServiceUnavailable)
		return
	}

	tb := textbuf.Get()
	defer tb.Release()

	baseURL := tb.Reset().Str("http://").Str(h.serverAddr).String()
	if h.serverPort != 0 && h.serverPort != 80 {
		baseURL = tb.Reset().Str("http://").Str(h.serverAddr).Str(":").Int(int64(h.serverPort)).String()
	}

	var portArg string
	if h.serverPort != 0 && h.serverPort != 80 {
		portArg = tb.Reset().Str(" ze.port=").Int(int64(h.serverPort)).String()
	}

	script := tb.Reset().
		Str("#!ipxe\nkernel ").Str(baseURL).Str("/install/boot/vmlinuz ze.server=").Str(h.serverAddr).
		Str(" ze.image=").Str(imgName).Str(portArg).Str(" ip=dhcp panic=-1\n").
		Str("initrd ").Str(baseURL).Str("/install/boot/initrd.img.gz\n").
		Str("boot\n").
		String()

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(script)) //nolint:errcheck // best-effort HTTP write
}

func latestImage(dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("no image directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var imgs []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".img") && !strings.HasPrefix(e.Name(), ".") {
			imgs = append(imgs, e.Name())
		}
	}
	if len(imgs) == 0 {
		return "", fmt.Errorf("no .img files")
	}
	sort.Strings(imgs)
	return imgs[len(imgs)-1], nil
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
