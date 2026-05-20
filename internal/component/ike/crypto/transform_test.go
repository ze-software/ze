package crypto

import (
	"errors"
	"testing"
)

func TestTransformRegistryLookup(t *testing.T) {
	tests := []struct {
		name       string
		algoName   string
		wantID     EncryptionID
		wantKeyLen uint16
		wantAEAD   bool
	}{
		{"aes128gcm", "aes128gcm", ENCR_AES_GCM_16, 128, true},
		{"aes256gcm", "aes256gcm", ENCR_AES_GCM_16, 256, true},
		{"aes128", "aes128", ENCR_AES_CBC, 128, false},
		{"aes256", "aes256", ENCR_AES_CBC, 256, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc, err := LookupEncryption(tt.algoName)
			if err != nil {
				t.Fatalf("LookupEncryption(%q) error: %v", tt.algoName, err)
			}
			if enc.ID != tt.wantID {
				t.Errorf("ID = %d, want %d", enc.ID, tt.wantID)
			}
			if enc.KeyLength != tt.wantKeyLen {
				t.Errorf("KeyLength = %d, want %d", enc.KeyLength, tt.wantKeyLen)
			}
			if enc.IsAEAD != tt.wantAEAD {
				t.Errorf("IsAEAD = %v, want %v", enc.IsAEAD, tt.wantAEAD)
			}
		})
	}
}

func TestTransformRegistryUnknown(t *testing.T) {
	_, err := LookupEncryption("chacha20")
	if !errors.Is(err, ErrUnsupportedAlgorithm) {
		t.Errorf("LookupEncryption(chacha20) = %v, want ErrUnsupportedAlgorithm", err)
	}

	_, err = LookupPRF("md5")
	if !errors.Is(err, ErrUnsupportedAlgorithm) {
		t.Errorf("LookupPRF(md5) = %v, want ErrUnsupportedAlgorithm", err)
	}

	_, err = LookupIntegrity("md5")
	if !errors.Is(err, ErrUnsupportedAlgorithm) {
		t.Errorf("LookupIntegrity(md5) = %v, want ErrUnsupportedAlgorithm", err)
	}

	_, err = LookupDHGroup(13)
	if !errors.Is(err, ErrUnsupportedAlgorithm) {
		t.Errorf("LookupDHGroup(13) = %v, want ErrUnsupportedAlgorithm", err)
	}

	_, err = LookupDHGroup(21)
	if !errors.Is(err, ErrUnsupportedAlgorithm) {
		t.Errorf("LookupDHGroup(21) = %v, want ErrUnsupportedAlgorithm", err)
	}
}

func TestTransformRegistryPRF(t *testing.T) {
	tests := []struct {
		name    string
		wantID  PRFID
		wantKey uint16
		wantOut uint16
	}{
		{"sha256", PRF_HMAC_SHA2_256, 32, 32},
		{"sha384", PRF_HMAC_SHA2_384, 48, 48},
		{"sha512", PRF_HMAC_SHA2_512, 64, 64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prf, err := LookupPRF(tt.name)
			if err != nil {
				t.Fatalf("LookupPRF(%q) error: %v", tt.name, err)
			}
			if prf.ID != tt.wantID {
				t.Errorf("ID = %d, want %d", prf.ID, tt.wantID)
			}
			if prf.KeyLength != tt.wantKey {
				t.Errorf("KeyLength = %d, want %d", prf.KeyLength, tt.wantKey)
			}
			if prf.OutputLength != tt.wantOut {
				t.Errorf("OutputLength = %d, want %d", prf.OutputLength, tt.wantOut)
			}
		})
	}
}

func TestTransformRegistryIntegrity(t *testing.T) {
	tests := []struct {
		name      string
		wantID    IntegrityID
		wantKey   uint16
		wantTrunc uint16
	}{
		{"sha256", AUTH_HMAC_SHA2_256_128, 32, 16},
		{"sha384", AUTH_HMAC_SHA2_384_192, 48, 24},
		{"sha512", AUTH_HMAC_SHA2_512_256, 64, 32},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			integ, err := LookupIntegrity(tt.name)
			if err != nil {
				t.Fatalf("LookupIntegrity(%q) error: %v", tt.name, err)
			}
			if integ.ID != tt.wantID {
				t.Errorf("ID = %d, want %d", integ.ID, tt.wantID)
			}
			if integ.KeyLength != tt.wantKey {
				t.Errorf("KeyLength = %d, want %d", integ.KeyLength, tt.wantKey)
			}
			if integ.TruncatedLength != tt.wantTrunc {
				t.Errorf("TruncatedLength = %d, want %d", integ.TruncatedLength, tt.wantTrunc)
			}
		})
	}
}

func TestTransformRegistryDHGroup(t *testing.T) {
	tests := []struct {
		id     uint8
		wantID DHGroupID
	}{
		{14, DH_MODP_2048},
		{19, DH_ECP_256},
		{20, DH_ECP_384},
	}
	for _, tt := range tests {
		dh, err := LookupDHGroup(tt.id)
		if err != nil {
			t.Fatalf("LookupDHGroup(%d) error: %v", tt.id, err)
		}
		if dh.ID != tt.wantID {
			t.Errorf("LookupDHGroup(%d).ID = %d, want %d", tt.id, dh.ID, tt.wantID)
		}
	}
}

func TestTransformRegistryBoundary(t *testing.T) {
	for _, id := range []uint8{0, 1, 13, 15, 18, 21, 255} {
		_, err := LookupDHGroup(id)
		if !errors.Is(err, ErrUnsupportedAlgorithm) {
			t.Errorf("LookupDHGroup(%d) = %v, want ErrUnsupportedAlgorithm", id, err)
		}
	}
}
