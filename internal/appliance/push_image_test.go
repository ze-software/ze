// Design: docs/architecture/appliance/ota-push.md -- partitioned image updates.

package appliance

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// pushDiskFixture models the four-partition GPT layout gok overwrite --full
// produces. Distinct metadata, boot, root and persistent bytes expose a reader
// that starts at the wrong offset or runs past its partition.
func pushDiskFixture(payload string) (disk, root, boot []byte) {
	disk = make([]byte, 128*512)
	for i := range 440 {
		disk[i] = 0xa5
	}
	copy(disk[440:446], "diskid")
	disk[510], disk[511] = 0x55, 0xaa
	header := disk[512:1024]
	copy(header, "EFI PART")
	binary.LittleEndian.PutUint32(header[8:], 0x10000)
	binary.LittleEndian.PutUint32(header[12:], 92)
	binary.LittleEndian.PutUint64(header[24:], 1)
	binary.LittleEndian.PutUint64(header[32:], 127)
	binary.LittleEndian.PutUint64(header[40:], 34)
	binary.LittleEndian.PutUint64(header[48:], 94)
	binary.LittleEndian.PutUint64(header[72:], 2)
	binary.LittleEndian.PutUint32(header[80:], 4)
	binary.LittleEndian.PutUint32(header[84:], 128)
	for i := range 4 {
		entry := disk[1024+i*128 : 1024+(i+1)*128]
		entry[0], entry[16] = byte(i+1), byte(i+10)
		binary.LittleEndian.PutUint64(entry[32:], uint64(34+i*8))
		binary.LittleEndian.PutUint64(entry[40:], uint64(41+i*8))
	}
	binary.LittleEndian.PutUint32(header[88:], crc32.ChecksumIEEE(disk[1024:1536]))
	binary.LittleEndian.PutUint32(header[16:], crc32.ChecksumIEEE(header[:92]))
	boot = disk[34*512 : 42*512]
	boot[0] = 0xeb
	binary.LittleEndian.PutUint16(boot[11:], 512)
	binary.LittleEndian.PutUint16(boot[19:], 8)
	copy(boot[54:], "FAT16   ")
	boot[510], boot[511] = 0x55, 0xaa
	copy(boot[512:], "kernel and boot config")
	root = disk[42*512 : 42*512+96+len(payload)]
	copy(root, "hsqs")
	binary.LittleEndian.PutUint32(root[12:], 4096)
	binary.LittleEndian.PutUint16(root[28:], 4)
	binary.LittleEndian.PutUint64(root[40:], uint64(len(root)))
	copy(root[96:], payload)
	copy(disk[58*512:], "PERSISTENT CONFIG MUST NEVER BE UPLOADED")
	return disk, root, boot
}

func writePushFixture(t *testing.T, data []byte) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ze.img")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	return file
}

// addPushMetadataHandlers extends the existing protocol fixtures to answer
// boot/code uploads with the same end-to-end hash contract as the root upload.
func addPushMetadataHandlers(mux *http.ServeMux) {
	for _, path := range []string{"PUT /update/bootonly", "PUT /update/mbr"} {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if r.Header.Get("X-Gokrazy-Update-Hash") == "crc32" {
				hash := crc32.NewIEEE()
				_, _ = hash.Write(body)
				_, _ = io.WriteString(w, hex.EncodeToString(hash.Sum(nil)))
				return
			}
			hash := sha256.Sum256(body)
			_, _ = io.WriteString(w, hex.EncodeToString(hash[:]))
		})
	}
}

func TestPushStreamsPartitions(t *testing.T) {
	for _, test := range []struct {
		name string
		opts pushOpts
		last string
	}{
		{name: "switch", last: "/update/switch"},
		{name: "testboot", opts: pushOpts{testboot: true}, last: "/update/testboot"},
		{name: "no-reboot", opts: pushOpts{noReboot: true}, last: "/update/switch"},
	} {
		t.Run(test.name, func(t *testing.T) {
			disk, root, boot := pushDiskFixture("new software")
			var paths []string
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				user, password, ok := r.BasicAuth()
				if !ok || user != "gokrazy" || password != "update-secret" {
					t.Errorf("updater requires gokrazy user and stored token; got user=%q", user)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if r.URL.Path == "/update/features" {
					_, _ = io.WriteString(w, "updatehash,partuuid,gpt")
					return
				}
				if r.URL.Path == "/" {
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"EEPROM":{}}`)
					return
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				var want []byte
				switch r.URL.Path {
				case "/update/root":
					want = root
				case "/update/bootonly":
					want = boot
				case "/update/mbr":
					want = disk[:440]
				case "/update/switch", "/update/testboot", "/reboot":
					return
				default:
					t.Errorf("unexpected endpoint %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if !bytes.Equal(body, want) {
					t.Errorf("%s payload length=%d, want %d exact partition bytes", r.URL.Path, len(body), len(want))
				}
				hash := crc32.NewIEEE()
				_, _ = hash.Write(body)
				_, _ = io.WriteString(w, hex.EncodeToString(hash.Sum(nil)))
			}))
			defer server.Close()
			config := server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
			if err := doPushUpdater(server.URL+"/", writePushFixture(t, disk), "update-secret", config, test.opts); err != nil {
				t.Fatal(err)
			}
			want := []string{"/update/features", "/", "/update/root", "/update/bootonly", "/update/mbr", test.last}
			if !test.opts.noReboot {
				want = append(want, "/reboot")
			}
			if !reflect.DeepEqual(paths, want) {
				t.Errorf("requests=%v, want %v", paths, want)
			}
		})
	}
}

func TestPushRejectsMalformedImageBeforeNetwork(t *testing.T) {
	valid, _, _ := pushDiskFixture("valid")
	for _, test := range []struct {
		name string
		data []byte
	}{
		{name: "not-a-disk", data: []byte("fake-disk-image-content")},
		{name: "truncated", data: bytes.Clone(valid[:45*512])},
		{name: "bad-header-crc", data: func() []byte { b := bytes.Clone(valid); b[512+16] ^= 1; return b }()},
		{name: "bad-entry-crc", data: func() []byte { b := bytes.Clone(valid); b[1024+32] ^= 1; return b }()},
		{name: "not-squashfs", data: func() []byte { b := bytes.Clone(valid); b[42*512] = 0; return b }()},
		{name: "not-fat", data: func() []byte { b := bytes.Clone(valid); b[34*512+510] = 0; return b }()},
	} {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests++
				w.WriteHeader(http.StatusBadRequest)
			}))
			defer server.Close()
			config := server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
			if err := doPushUpdater(server.URL+"/", writePushFixture(t, test.data), "secret", config, pushOpts{}); err == nil {
				t.Error("malformed image accepted")
			}
			if requests != 0 {
				t.Errorf("malformed image contacted device %d times", requests)
			}
		})
	}
}

func TestPushUsesGokrazyUsername(t *testing.T) {
	transport := &authTransport{token: "stored-token", base: rtFunc(func(r *http.Request) (*http.Response, error) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "gokrazy" || password != "stored-token" {
			t.Errorf("Basic auth user=%q, want gokrazy and stored token", user)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})}
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://router/update/features", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPushDoesNotSwitchAfterUploadFailure(t *testing.T) {
	for _, failed := range []string{"/update/root", "/update/bootonly", "/update/mbr"} {
		t.Run(strings.TrimPrefix(failed, "/update/"), func(t *testing.T) {
			disk, _, _ := pushDiskFixture("new software")
			var requests []string
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.URL.Path)
				if r.URL.Path == "/update/features" {
					_, _ = io.WriteString(w, "updatehash")
					return
				}
				if r.URL.Path == "/" {
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"EEPROM":{}}`)
					return
				}
				if r.URL.Path == failed {
					http.Error(w, "fixture write failure", http.StatusInternalServerError)
					return
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
					return
				}
				hash := crc32.NewIEEE()
				_, _ = hash.Write(body)
				_, _ = io.WriteString(w, hex.EncodeToString(hash.Sum(nil)))
			}))
			defer server.Close()
			config := server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
			if err := doPushUpdater(server.URL+"/", writePushFixture(t, disk), "token", config, pushOpts{}); err == nil {
				t.Fatal("failed partition upload reported success")
			}
			if requests[len(requests)-1] != failed {
				t.Errorf("requests=%v, must stop at %s", requests, failed)
			}
		})
	}
}

func TestApplianceCertificateCoversUpdateAddress(t *testing.T) {
	for _, address := range []string{"192.0.2.27", "2001:db8::27", "router.example.test"} {
		t.Run(strings.ReplaceAll(address, ":", "_"), func(t *testing.T) {
			cfg := DefaultConfig("device")
			cfg.Device.Address = address
			dir := t.TempDir()
			if err := os.MkdirAll(tLSDir(dir, "device"), 0o700); err != nil {
				t.Fatal(err)
			}
			cert, key, err := issueWebLeaf(dir, "device", &cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			pair, err := tls.X509KeyPair(cert, key)
			if err != nil {
				t.Fatal(err)
			}
			if err := pair.Leaf.VerifyHostname(address); err != nil {
				t.Fatalf("device certificate does not verify configured update address: %v", err)
			}
		})
	}
}
