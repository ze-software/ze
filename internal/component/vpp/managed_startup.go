// Design: docs/guide/vpp.md -- explicit managed startup controls
// Related: startupconf.go -- only the selected configuration reaches native VPP
package vpp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

// A nil plugin choice consumes its YANG default, including existing internal
// struct-literal callers. It never means "try whichever plugin is installed".
func startupPluginChoice(choice *bool, path string) (bool, error) {
	if choice != nil {
		return *choice, nil
	}
	schema, err := config.YANGSchema()
	if err != nil {
		return false, err
	}
	return config.SchemaDefaultBool(schema, path)
}

// Empty preserves existing internal zero-value callers. Delivered config
// permits only the three explicit enum values and never reaches that case.
func validateStartupPageMode(mode string) error {
	// Raw config and struct callers may supply unknown strings; refuse below.
	switch mode {
	case "", "2M", "1G", memoryPageModeBase:
		return nil
	}
	return opererr.New("VPP memory page mode "+strconv.Quote(mode)+" is unsupported",
		"Buffers and statistics require the configured hugepage mode or explicit native base pages.", "set vpp memory hugepage-size 2M")
}

func (settings *VPPSettings) managedStartupPlugins() (dpdk, afPacket bool, failure error) {
	dpdk, failure = startupPluginChoice(settings.Plugins.DPDK, "vpp/plugins/dpdk")
	if failure != nil {
		return
	}
	afPacket, failure = startupPluginChoice(settings.Plugins.AFPacket, "vpp/plugins/af-packet")
	if failure != nil {
		return
	}
	if !dpdk {
		if len(settings.DPDK.Interfaces) > 0 {
			failure = opererr.New("VPP DPDK interfaces require the DPDK plugin",
				"Configured PCI interfaces cannot be served while vpp plugins dpdk is false.", "set vpp plugins dpdk true")
		}
	}
	return
}

func parseStartupPluginChoice(name string, data json.RawMessage) (*bool, error) {
	var value *string
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("vpp plugins %s: expected true or false: %w", name, err)
	}
	if value == nil {
		return nil, fmt.Errorf("vpp plugins %s: expected true or false", name)
	}
	if *value != yangTrue {
		if *value != "false" {
			return nil, fmt.Errorf("vpp plugins %s: expected true or false", name)
		}
	}
	selected := *value == yangTrue
	return &selected, nil
}

func parseStartupCLIFile(data json.RawMessage) (string, error) {
	var value *string
	if err := json.Unmarshal(data, &value); err != nil {
		return "", fmt.Errorf("vpp startup-cli-file: expected a path string: %w", err)
	}
	if value == nil {
		return "", errors.New("vpp startup-cli-file: expected a path string")
	}
	if err := validateStartupCLIPath(*value); err != nil {
		return "", err
	}
	return *value, nil
}

const startupCLIPathFix = "set vpp startup-cli-file /absolute/path/to/commands.cli"

// This is pure VPP-token validation, not a filesystem query. The exact path
// enters one synchronous native exec request; no shell, quoting or normalization
// may reinterpret it. The manager treats an absent optional leaf as a no-op.
func validateStartupCLIPath(path string) error {
	if path == "" {
		return nil
	}
	valid := filepath.IsAbs(path)
	if !utf8.ValidString(path) {
		valid = false
	}
	for _, character := range path {
		if !unicode.IsPrint(character) {
			valid = false
			break
		}
		if unicode.IsSpace(character) {
			valid = false
			break
		}
		if strings.ContainsRune("{}\"'\\#;", character) {
			valid = false
			break
		}
	}
	if !valid {
		return opererr.New("VPP startup CLI file path is not one absolute native token",
			"The path must be absolute and contain no whitespace, control characters, braces, quotes, backslashes, comment or command separators.",
			startupCLIPathFix)
	}
	return nil
}

type startupCLIFile interface {
	io.Reader
	io.Closer
	Stat() (os.FileInfo, error)
}
type startupCLIOpener func(string) (startupCLIFile, error)

func startupCLIFix(path string) string {
	return "Provide a readable regular file at " + strconv.Quote(path) + ", or delete vpp startup-cli-file"
}
func startupCLIFileError(path string, cause error) error {
	return opererr.Wrap(cause, "VPP startup CLI file "+strconv.Quote(path)+" is unavailable",
		"Ze must open the original readable regular file before it starts managed VPP.", startupCLIFix(path))
}

// File checks belong to managed startup/doctor, not pure candidate validation.
// The platform opener is nonblocking, so a FIFO cannot hang before Stat. A
// one-byte probe proves readability without allocating or reading the full CLI
// program. Native VPP alone parses that program. Close preserves original errors.
func checkManagedStartupCLI(settings *VPPSettings, open startupCLIOpener) (failure error) {
	if !settings.Enabled {
		return nil
	}
	if settings.External {
		return nil
	}
	if settings.StartupCLIFile == "" {
		return nil
	}
	path := settings.StartupCLIFile
	if err := validateStartupCLIPath(path); err != nil {
		return err
	}
	file, err := open(path)
	if err != nil {
		return startupCLIFileError(path, err)
	}
	if file == nil {
		return startupCLIFileError(path, errors.New("the original file opener returned no file"))
	}
	defer func() {
		if err := file.Close(); err != nil {
			failure = errors.Join(failure, startupCLIFileError(path, err))
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return startupCLIFileError(path, err)
	}
	if !info.Mode().IsRegular() {
		return startupCLIFileError(path, errors.New("the path is not a regular file"))
	}
	var sample [1]byte
	if _, err := file.Read(sample[:]); err != nil {
		if !errors.Is(err, io.EOF) {
			return startupCLIFileError(path, err)
		}
	}
	return nil
}

const doctorVPPStartupCLIFileCode = "doctor-vpp-startup-cli-file"

func managedStartupCLIDoctorCheck() diagnostic.DoctorCheck {
	return diagnostic.DoctorCheck{Name: "vpp-startup-cli-file", Phase: diagnostic.DoctorPhasePostConfig, Order: 825,
		Component: componentVPP, Dependencies: []string{"config-tree", "filesystem"}, Platforms: []string{diagnostic.DoctorPlatformAny}, Codes: []string{doctorVPPStartupCLIFileCode},
		Check: func(input diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
			tree, ok := input.Tree.(*config.Tree)
			if !ok {
				return nil
			}
			if tree == nil {
				return nil
			}
			container := tree.GetContainer(componentVPP)
			if container == nil {
				return nil
			}
			enabled, _ := container.Get("enabled")
			if enabled != yangTrue {
				return nil
			}
			external, _ := container.Get("external")
			if external == yangTrue {
				return nil
			}
			path, _ := container.Get("startup-cli-file")
			if path == "" {
				return nil
			}
			if err := validateStartupCLIPath(path); err != nil {
				return []diagnostic.Diagnostic{{Code: doctorVPPStartupCLIFileCode, Severity: diagnostic.SeverityError, Message: err.Error(), Fix: startupCLIPathFix}}
			}
			settings := &VPPSettings{Enabled: true, StartupCLIFile: path}
			err := checkManagedStartupCLI(settings, openStartupCLIFile)
			if err == nil {
				return nil
			}
			return []diagnostic.Diagnostic{{Code: doctorVPPStartupCLIFileCode, Severity: diagnostic.SeverityError, Message: err.Error(), Fix: startupCLIFix(path)}}
		}}
}
