//go:build ze_lg || ze_web

// Design: docs/architecture/hub-architecture.md -- zefs-backed TLS certificate storage
//
// The build constraint is the DISJUNCTION of its two consumers'. service_web.go
// (ze_web) and service_lg.go (ze_lg) each build a blobCertStore for their
// listener's TLS material, and nothing else refers to it. A daemon with neither
// listener has no self-signed certificate to store.

package hub

import (
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// blobCertStore implements selfcert.CertStore backed by zefs blob storage.
type blobCertStore struct {
	store storage.Storage
}

func (s *blobCertStore) ReadCert() ([]byte, error) { return s.store.ReadFile(zefs.KeyWebCert.Pattern) }
func (s *blobCertStore) ReadKey() ([]byte, error)  { return s.store.ReadFile(zefs.KeyWebKey.Pattern) }
func (s *blobCertStore) WriteCert(data []byte) error {
	return s.store.WriteFile(zefs.KeyWebCert.Pattern, data, 0o600)
}
func (s *blobCertStore) WriteKey(data []byte) error {
	return s.store.WriteFile(zefs.KeyWebKey.Pattern, data, 0o600)
}
func (s *blobCertStore) Exists() bool {
	return s.store.Exists(zefs.KeyWebCert.Pattern) && s.store.Exists(zefs.KeyWebKey.Pattern)
}
