package install

import (
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestGenerateConfig(t *testing.T) {
	cfg := generateConfig(configParams{
		iface:       "eth0",
		network:     "10.0.0.0/24",
		image:       "/images/gokrazy.img",
		serverIP:    "10.0.0.1",
		sshUsername: "admin",
		sshPassHash: "$2a$10$fakehash",
	})

	assert.Contains(t, cfg, "dhcp-server {")
	assert.Contains(t, cfg, "tftp-server {")
	assert.Contains(t, cfg, "image-server {")
	assert.Contains(t, cfg, "service {")
}

func TestGenerateConfigPXEBlock(t *testing.T) {
	cfg := generateConfig(configParams{
		iface:       "eth0",
		network:     "10.0.0.0/24",
		image:       "/images/gokrazy.img",
		serverIP:    "10.0.0.1",
		sshUsername: "admin",
		sshPassHash: "$2a$10$fakehash",
	})

	assert.Contains(t, cfg, "pxe {")
	assert.Contains(t, cfg, "tftp-server 10.0.0.1;")
	assert.Contains(t, cfg, "bootfile-bios ipxe.pxe;")
	assert.Contains(t, cfg, "bootfile-uefi ipxe.efi;")
}

func TestGenerateConfigNetwork(t *testing.T) {
	tests := []struct {
		name    string
		network string
		server  string
		start   string
		stop    string
	}{
		{
			name:    "/24 standard",
			network: "10.0.0.0/24",
			server:  "10.0.0.1",
			start:   "10.0.0.2",
			stop:    "10.0.0.254",
		},
		{
			name:    "/28 small subnet",
			network: "192.168.1.0/28",
			server:  "192.168.1.1",
			start:   "192.168.1.2",
			stop:    "192.168.1.14",
		},
		{
			name:    "/30 minimal",
			network: "10.1.1.0/30",
			server:  "10.1.1.1",
			start:   "10.1.1.2",
			stop:    "10.1.1.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := generateConfig(configParams{
				iface:       "eth0",
				network:     tt.network,
				image:       "/images/gokrazy.img",
				serverIP:    tt.server,
				sshUsername: "admin",
				sshPassHash: "$2a$10$fakehash",
			})

			assert.Contains(t, cfg, "start "+tt.start+";")
			assert.Contains(t, cfg, "stop "+tt.stop+";")
		})
	}
}

func TestGenerateConfigServerIP(t *testing.T) {
	cfg := generateConfig(configParams{
		iface:       "eth0",
		network:     "10.0.0.0/24",
		image:       "/images/gokrazy.img",
		serverIP:    "10.0.0.1",
		sshUsername: "admin",
		sshPassHash: "$2a$10$fakehash",
	})

	// Server IP appears in: default-router, tftp-server (PXE block), listen-interface is name-based
	assert.Contains(t, cfg, "default-router 10.0.0.1;")
	assert.Contains(t, cfg, "tftp-server 10.0.0.1;")
}

func TestValidateFlags(t *testing.T) {
	t.Run("all missing", func(t *testing.T) {
		errs := validateFlags("", "", "", "", "")
		assert.True(t, len(errs) >= 5, "should report all missing flags, got %d", len(errs))
	})

	t.Run("invalid network prefix", func(t *testing.T) {
		errs := validateFlags("lo0", "10.0.0.0/31", "/dev/null", "admin", "pass")
		found := false
		for _, e := range errs {
			if strings.Contains(e.Error(), "prefix") {
				found = true
			}
		}
		assert.True(t, found, "should reject /31 prefix")
	})

	t.Run("invalid network too large", func(t *testing.T) {
		errs := validateFlags("lo0", "10.0.0.0/7", "/dev/null", "admin", "pass")
		found := false
		for _, e := range errs {
			if strings.Contains(e.Error(), "prefix") {
				found = true
			}
		}
		assert.True(t, found, "should reject /7 prefix")
	})

	t.Run("config injection in interface name", func(t *testing.T) {
		errs := validateFlags("eth0;}", "10.0.0.0/24", "/dev/null", "admin", "pass")
		found := false
		for _, e := range errs {
			if strings.Contains(e.Error(), "forbidden") {
				found = true
			}
		}
		assert.True(t, found, "should reject interface with forbidden chars")
	})

	t.Run("config injection in username", func(t *testing.T) {
		errs := validateFlags("lo0", "10.0.0.0/24", "/dev/null", "admin;}", "pass")
		found := false
		for _, e := range errs {
			if strings.Contains(e.Error(), "forbidden") {
				found = true
			}
		}
		assert.True(t, found, "should reject username with forbidden chars")
	})

	t.Run("image not regular file", func(t *testing.T) {
		errs := validateFlags("lo0", "10.0.0.0/24", os.TempDir(), "admin", "pass")
		found := false
		for _, e := range errs {
			if strings.Contains(e.Error(), "not a regular file") {
				found = true
			}
		}
		assert.True(t, found, "should reject directory as image")
	})

	t.Run("image does not exist", func(t *testing.T) {
		errs := validateFlags("lo0", "10.0.0.0/24", "/nonexistent/image.img", "admin", "pass")
		found := false
		for _, e := range errs {
			if strings.Contains(e.Error(), "image") {
				found = true
			}
		}
		assert.True(t, found, "should reject missing image")
	})
}

func TestResolveServerIP(t *testing.T) {
	t.Run("override takes precedence", func(t *testing.T) {
		ip, err := resolveServerIP("lo0", "192.168.1.1")
		require.NoError(t, err)
		assert.Equal(t, "192.168.1.1", ip)
	})

	t.Run("invalid override", func(t *testing.T) {
		_, err := resolveServerIP("lo0", "not-an-ip")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid --address")
	})

	t.Run("ipv6 override rejected", func(t *testing.T) {
		_, err := resolveServerIP("lo0", "::1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "IPv4")
	})
}

func TestPasswordHashing(t *testing.T) {
	hash, err := hashPassword("secret123")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(hash, "$2a$"), "should be bcrypt hash")

	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte("secret123"))
	assert.NoError(t, err, "hash should verify against original password")

	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte("wrong"))
	assert.Error(t, err, "hash should not verify against wrong password")
}

func TestDHCPRange(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		server string
		start  string
		stop   string
	}{
		{"server at .1", "10.0.0.0/24", "10.0.0.1", "10.0.0.2", "10.0.0.254"},
		{"server at .254", "10.0.0.0/24", "10.0.0.254", "10.0.0.1", "10.0.0.253"},
		{"server at .100 mid-range", "10.0.0.0/24", "10.0.0.100", "10.0.0.1", "10.0.0.254"},
		{"/28 subnet", "192.168.1.0/28", "192.168.1.1", "192.168.1.2", "192.168.1.14"},
		{"/30 minimal", "10.1.1.0/30", "10.1.1.1", "10.1.1.2", "10.1.1.2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := netip.MustParsePrefix(tt.prefix)
			server := netip.MustParseAddr(tt.server)
			start, stop := dhcpRange(prefix, server)
			assert.Equal(t, tt.start, start.String())
			assert.Equal(t, tt.stop, stop.String())
		})
	}
}

func TestSafeConfigValue(t *testing.T) {
	assert.True(t, safeConfigValue("eth0"))
	assert.True(t, safeConfigValue("admin"))
	assert.True(t, safeConfigValue("my-interface"))

	assert.False(t, safeConfigValue(""))
	assert.False(t, safeConfigValue("eth0;"))
	assert.False(t, safeConfigValue("eth0}"))
	assert.False(t, safeConfigValue("eth0{"))
	assert.False(t, safeConfigValue("eth 0"))
	assert.False(t, safeConfigValue("eth\t0"))
	assert.False(t, safeConfigValue("eth\n0"))
}

func TestDirOf(t *testing.T) {
	assert.Equal(t, "/images", dirOf("/images/gokrazy.img"))
	assert.Equal(t, "/var/lib", dirOf("/var/lib/image.raw"))
	assert.Equal(t, ".", dirOf("image.raw"))
}

func TestGenerateConfigSSHCredentials(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.img")
	require.NoError(t, os.WriteFile(tmpFile, []byte("fake"), 0o600))

	cfg := generateConfig(configParams{
		iface:       "eth0",
		network:     "10.0.0.0/24",
		image:       tmpFile,
		serverIP:    "10.0.0.1",
		sshUsername: "operator",
		sshPassHash: "$2a$10$hashvalue",
	})

	assert.Contains(t, cfg, "ssh-username operator;")
	assert.Contains(t, cfg, `ssh-password-hash "$2a$10$hashvalue";`)
}
