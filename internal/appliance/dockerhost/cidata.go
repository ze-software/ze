// Design: docs/architecture/appliance/docker-host.md -- the cidata Lima writes, read in Go
// Related: boot.go -- the boot sequence that consumes the parsed cidata

// Package dockerhost is the guest side of Ze's docker-host image: the program
// gokrazy starts as /user/ze-docker-host does cloud-init's part of the Lima
// guest contract (users, keys, hostname, mounts), supervises the guest's
// services, answers `systemctl` for Lima's and Colima's scripts, and then runs
// Lima's own boot.sh under busybox (plan/spec-ze-image-docker-host.md, D-3).
package dockerhost

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// CidataMount is where the cidata volume is mounted. Lima's own scripts read
// /mnt/lima-cidata by absolute path (the requirement scripts' `sudo cat
// /mnt/lima-cidata/param.env`), so the path is fixed.
const CidataMount = "/mnt/lima-cidata"

// cidataFileMax bounds every cidata file read. Lima's largest, user-data,
// is a few kilobytes; a volume holding more is not a Lima cidata.
const cidataFileMax = 1 << 20

var (
	// userPattern is the user names useradd accepts by default, without a
	// colon, slash or newline that would split a passwd or sudoers line.
	userPattern = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	// hostnamePattern is an RFC 1123 host name, dots allowed.
	hostnamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{0,62}$`)
)

// Mount is one cloud-init `mounts` entry Lima writes for a virtiofs or 9p
// share: the device tag, the absolute mount point, the type and the options.
type Mount struct {
	Tag     string
	Point   string
	FSType  string
	Options string
}

// Cidata is the part of Lima's cidata the boot acts on, every field
// validated by ParseCidata. A Cidata is only built by ParseCidata.
type Cidata struct {
	InstanceID     string
	Hostname       string
	User           string
	UID            uint32
	Comment        string
	Home           string
	Shell          string
	VsockPort      uint32
	AuthorizedKeys []string
	Mounts         []Mount
}

// ParseCidata reads lima.env, meta-data and user-data from dir and validates
// every value that reaches passwd, sudoers, a path or the host name.
func ParseCidata(dir string) (*Cidata, error) {
	limaEnv, err := readEnvFile(filepath.Join(dir, "lima.env"))
	if err != nil {
		return nil, err
	}
	meta, err := readMetaData(filepath.Join(dir, "meta-data"))
	if err != nil {
		return nil, err
	}
	keys, mounts, err := readUserData(filepath.Join(dir, "user-data"))
	if err != nil {
		return nil, err
	}

	c := &Cidata{
		InstanceID:     meta["instance-id"],
		Hostname:       meta["local-hostname"],
		User:           limaEnv["LIMA_CIDATA_USER"],
		Comment:        limaEnv["LIMA_CIDATA_COMMENT"],
		Home:           limaEnv["LIMA_CIDATA_HOME"],
		Shell:          limaEnv["LIMA_CIDATA_SHELL"],
		AuthorizedKeys: keys,
		Mounts:         mounts,
	}
	if c.UID, err = parseUID(limaEnv["LIMA_CIDATA_UID"]); err != nil {
		return nil, err
	}
	if c.VsockPort, err = parseVsockPort(limaEnv["LIMA_CIDATA_VSOCK_PORT"]); err != nil {
		return nil, err
	}
	if err := c.validate(limaEnv["LIMA_CIDATA_IID"]); err != nil {
		return nil, err
	}
	return c, nil
}

// validate checks the string fields; iid is lima.env's copy of the instance
// id, which boot.sh writes to /run/lima-boot-done and Lima compares with
// meta-data's.
func (c *Cidata) validate(iid string) error {
	if c.InstanceID == "" {
		return errors.New("cidata meta-data has no instance-id: the volume is not a Lima cidata")
	}
	if iid != c.InstanceID {
		return fmt.Errorf("cidata instance id differs: meta-data %q, lima.env %q", c.InstanceID, iid)
	}
	if !hostnamePattern.MatchString(c.Hostname) {
		return fmt.Errorf("cidata local-hostname %q is not a host name", c.Hostname)
	}
	if !userPattern.MatchString(c.User) {
		return fmt.Errorf("cidata user %q is not a user name (lower case, digits, '_' and '-')", c.User)
	}
	if c.User == "root" {
		return errors.New("cidata user is root: Lima's user is an unprivileged account")
	}
	if strings.ContainsAny(c.Comment, ":\n") {
		return fmt.Errorf("cidata comment %q holds a ':' or a newline, which would split the passwd line", c.Comment)
	}
	if err := checkHome(c.Home); err != nil {
		return err
	}
	if !path.IsAbs(c.Shell) || path.Clean(c.Shell) != c.Shell {
		return fmt.Errorf("cidata shell %q is not an absolute clean path", c.Shell)
	}
	for _, key := range c.AuthorizedKeys {
		if strings.ContainsAny(key, "\n\r") {
			return errors.New("cidata ssh key holds a line break, which would add a second authorized key")
		}
	}
	for _, m := range c.Mounts {
		if !path.IsAbs(m.Point) || path.Clean(m.Point) != m.Point {
			return fmt.Errorf("cidata mount point %q is not an absolute clean path", m.Point)
		}
	}
	return nil
}

// checkHome accepts a home directory one level under /home only, the shape
// Lima writes (/home/<user>.linux).
func checkHome(home string) error {
	if path.Clean(home) != home {
		return fmt.Errorf("cidata home %q is not a clean path", home)
	}
	if path.Dir(home) != "/home" {
		return fmt.Errorf("cidata home %q is not a directory directly under /home", home)
	}
	return nil
}

// parseUID accepts 1 to 65534: 0 is root, and 65535 is the reserved
// "no user" value (the spec's boundary table).
func parseUID(text string) (uint32, error) {
	uid, err := strconv.ParseUint(text, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("cidata LIMA_CIDATA_UID %q is not a number: %w", text, err)
	}
	if uid == 0 {
		return 0, errors.New("cidata LIMA_CIDATA_UID is 0: Lima's user is never root")
	}
	if uid > 65534 {
		return 0, fmt.Errorf("cidata LIMA_CIDATA_UID %d is above 65534", uid)
	}
	return uint32(uid), nil
}

// parseVsockPort accepts any 32-bit port; 0 means Lima uses no vsock.
func parseVsockPort(text string) (uint32, error) {
	port, err := strconv.ParseUint(text, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("cidata LIMA_CIDATA_VSOCK_PORT %q is not a 32-bit number: %w", text, err)
	}
	return uint32(port), nil
}

// readBounded reads a cidata file, refusing one above cidataFileMax.
func readBounded(name string) ([]byte, error) {
	info, err := os.Stat(name)
	if err != nil {
		return nil, fmt.Errorf("read cidata: %w", err)
	}
	if info.Size() > cidataFileMax {
		return nil, fmt.Errorf("cidata %s is %d bytes, above the %d a Lima cidata file holds", name, info.Size(), cidataFileMax)
	}
	data, err := os.ReadFile(name) //nolint:gosec // the path is the cidata mount, fixed by the boot
	if err != nil {
		return nil, fmt.Errorf("read cidata: %w", err)
	}
	return data, nil
}

// readEnvFile parses KEY=VALUE lines the way boot.sh's `export "$line"` does:
// everything after the first '=' is the value, verbatim.
func readEnvFile(name string) (map[string]string, error) {
	data, err := readBounded(name)
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), "=")
		if !ok {
			continue
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return values, nil
}

// readMetaData parses meta-data's two `key: value` lines.
func readMetaData(name string) (map[string]string, error) {
	data, err := readBounded(name)
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("parse %s: %w", name, err)
	}
	return values, nil
}

// userData is the part of Lima's #cloud-config the boot reads.
type userData struct {
	Mounts [][]string `yaml:"mounts"`
	Users  []struct {
		Name string   `yaml:"name"`
		Keys []string `yaml:"ssh-authorized-keys"`
	} `yaml:"users"`
}

// readUserData answers the first user's authorized keys and the mounts. Lima
// writes exactly one user, the one lima.env names.
func readUserData(name string) ([]string, []Mount, error) {
	data, err := readBounded(name)
	if err != nil {
		return nil, nil, err
	}
	var doc userData
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", name, err)
	}
	if len(doc.Users) != 1 {
		return nil, nil, fmt.Errorf("cidata user-data names %d users, and Lima writes exactly one", len(doc.Users))
	}
	mounts := make([]Mount, 0, len(doc.Mounts))
	for _, entry := range doc.Mounts {
		if len(entry) < 4 {
			return nil, nil, fmt.Errorf("cidata mount %q has fewer than the 4 fields tag, point, type, options", entry)
		}
		mounts = append(mounts, Mount{Tag: entry[0], Point: entry[1], FSType: entry[2], Options: entry[3]})
	}
	return doc.Users[0].Keys, mounts, nil
}
