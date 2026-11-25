package libcontainer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	securejoin "github.com/cyphar/filepath-securejoin"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/cgroups/fs2"
	"github.com/opencontainers/runc/internal/pathrs"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/configs/validate"
	"github.com/opencontainers/runc/libcontainer/intelrdt"
)

const (
	stateFilename    = "state.json"
	execFifoFilename = "exec.fifo"
)

// Create creates a new container with the given id inside a given state
// directory (root), and returns a Container object.
//
// The root is a state directory which many containers can share. It can be
// used later to get the list of containers, or to get information about a
// particular container (see Load).
//
// The id must not be empty and consist of only the following characters:
// ASCII letters, digits, underscore, plus, minus, period. The id must be
// unique and non-existent for the given root path.
func Create(root, id string, config *configs.Config) (*Container, error) {
	if root == "" {
		return nil, errors.New("root not set")
	}
	if err := validateID(id); err != nil {
		return nil, err
	}
	if err := validate.Validate(config); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	stateDir, err := securejoin.SecureJoin(root, id)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(stateDir); err == nil {
		return nil, ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	cm, err := newCgroupV2Manager(config.Cgroups, nil)
	if err != nil {
		return nil, err
	}

	// Check that cgroup does not exist or empty (no processes).
	// Note for cgroup v1 this check is not thorough, as there are multiple
	// separate hierarchies, while both Exists() and GetAllPids() only use
	// one for "devices" controller (assuming others are the same, which is
	// probably true in almost all scenarios). Checking all the hierarchies
	// would be too expensive.
	if cm.Exists() {
		pids, err := cm.GetAllPids()
		// Reading PIDs can race with cgroups removal, so ignore ENOENT and ENODEV.
		if err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, unix.ENODEV) {
			return nil, fmt.Errorf("unable to get cgroup PIDs: %w", err)
		}
		if len(pids) != 0 {
			return nil, fmt.Errorf("container's cgroup is not empty: %d process(es) found", len(pids))
		}
	}

	// Check that cgroup is not frozen. Do not use Exists() here
	// since in cgroup v1 it only checks "devices" controller.
	st, err := cm.GetFreezerState()
	if err != nil {
		return nil, fmt.Errorf("unable to get cgroup freezer state: %w", err)
	}
	if st == cgroups.Frozen {
		return nil, errors.New("container's cgroup unexpectedly frozen")
	}

	// Parent directory is already created above, so Mkdir is enough.
	if err := os.Mkdir(stateDir, 0o711); err != nil {
		return nil, err
	}
	c := &Container{
		id:              id,
		stateDir:        stateDir,
		config:          config,
		cgroupManager:   cm,
		intelRdtManager: intelrdt.NewManager(config, id, ""),
	}
	c.state = &stoppedState{c: c}
	return c, nil
}

// Load takes a path to the state directory (root) and an id of an existing
// container, and returns a Container object reconstructed from the saved
// state. This presents a read only view of the container.
func Load(root, id string) (*Container, error) {
	if root == "" {
		return nil, errors.New("root not set")
	}
	// when load, we need to check id is valid or not.
	if err := validateID(id); err != nil {
		return nil, err
	}
	stateDir, err := securejoin.SecureJoin(root, id)
	if err != nil {
		return nil, err
	}
	state, err := loadState(stateDir)
	if err != nil {
		return nil, err
	}
	r := &nonChildProcess{
		processPid:       state.InitProcessPid,
		processStartTime: state.InitProcessStartTime,
		fds:              state.ExternalDescriptors,
	}
	cm, err := newCgroupV2Manager(state.Config.Cgroups, state.CgroupPaths)
	if err != nil {
		return nil, err
	}
	c := &Container{
		initProcess:          r,
		initProcessStartTime: state.InitProcessStartTime,
		id:                   id,
		config:               &state.Config,
		cgroupManager:        cm,
		intelRdtManager:      intelrdt.NewManager(&state.Config, id, state.IntelRdtPath),
		stateDir:             stateDir,
		created:              state.Created,
	}
	c.state = &loadedState{c: c}
	if err := c.refreshState(); err != nil {
		return nil, err
	}
	return c, nil
}

// newCgroupV2Manager is a ze patch (runc-04-cgroup-v2-manager). Upstream calls
// manager.New and manager.NewWithPaths here, and that package links the
// systemd driver and the cgroup v1 manager. Ze runs on the unified hierarchy
// with no systemd, so the v2 manager is built directly, with the checks
// manager.NewWithPaths made, and anything else is refused.
func newCgroupV2Manager(config *cgroups.Cgroup, paths map[string]string) (cgroups.Manager, error) {
	if config == nil {
		return nil, errors.New("cgroup config must not be nil")
	}
	if config.Systemd {
		return nil, errors.New("the systemd cgroup driver is not supported: only the cgroup v2 filesystem manager is built")
	}
	if !cgroups.IsCgroup2UnifiedMode() {
		return nil, errors.New("the host is not in cgroup v2 unified mode: /sys/fs/cgroup is not a cgroup2 filesystem")
	}
	// The same sanity checks as getUnifiedPath of the manager package: the
	// only key is "" and its value is the unified cgroup path, or empty.
	if len(paths) > 1 {
		return nil, fmt.Errorf("inconsistent cgroup paths: expected a single path, got %+v", paths)
	}
	path := paths[""]
	if path != "" && (filepath.Clean(path) != path || !filepath.IsAbs(path)) {
		return nil, fmt.Errorf("inconsistent cgroup paths: invalid path: %q", path)
	}
	return fs2.NewManager(config, path)
}

func loadState(root string) (*State, error) {
	stateFilePath, err := securejoin.SecureJoin(root, stateFilename)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(stateFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotExist
		}
		return nil, err
	}
	defer f.Close()
	var state *State
	if err := json.NewDecoder(f).Decode(&state); err != nil {
		return nil, err
	}
	// Cgroup v1 fs manager expect Resources to never be nil.
	if state.Config.Cgroups.Resources == nil {
		state.Config.Cgroups.Resources = &cgroups.Resources{}
	}
	return state, nil
}

// validateID checks if the supplied container ID is valid, returning
// the ErrInvalidID in case it is not.
//
// The format of valid ID was never formally defined, instead the code
// was modified to allow or disallow specific characters.
//
// Currently, a valid ID is a non-empty string consisting only of
// the following characters:
// - uppercase (A-Z) and lowercase (a-z) Latin letters;
// - digits (0-9);
// - underscore (_);
// - plus sign (+);
// - minus sign (-);
// - period (.).
//
// In addition, IDs that can't be used to represent a file name
// (such as . or ..) are rejected.

func validateID(id string) error {
	if len(id) < 1 {
		return ErrInvalidID
	}

	// Allowed characters: 0-9 A-Z a-z _ + - .
	for i := range len(id) {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '_':
		case c == '+':
		case c == '-':
		case c == '.':
		default:
			return ErrInvalidID
		}

	}

	if string(os.PathSeparator)+id != pathrs.LexicallyCleanPath(string(os.PathSeparator)+id) {
		return ErrInvalidID
	}

	return nil
}
