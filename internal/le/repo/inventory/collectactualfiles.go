// Design: docs/architecture/plugin/feature-gates.md -- every report field uses one committed snapshot.
// Related: registrations.go -- reachable plugin and schema declarations.
package repoinventory

import (
	"bufio"
	"bytes"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func (s *Snapshot) collect() (Inventory, error) {
	reader, err := s.readRegistrations()
	if err != nil {
		return Inventory{}, err
	}
	inv := Inventory{
		Generated: time.Now().UTC().Format(generatedLayout),
		Families:  make(map[string]string), Capabilities: make(map[string]string),
		RPCsByModule: make(map[string]int), TestCounts: make(map[string]int),
	}
	names := make([]string, 0, len(reader.plugins))
	for name := range reader.plugins {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		registration := reader.plugins[name]
		plugin := registration.plugin
		inv.Plugins = append(inv.Plugins, plugin)
		for _, family := range plugin.Families {
			if _, exists := inv.Families[family]; exists {
				continue
			}
			inv.Families[family] = plugin.Name
			inv.FamilySupport = append(inv.FamilySupport, Family{
				Family: family, Plugin: plugin.Name, Decode: plugin.HasDecoder, Encode: plugin.HasEncoder,
				RouteEncode: registration.routeEncoder, ConfigNLRI: registration.configBuilder,
			})
		}
		for _, code := range plugin.Capabilities {
			key := textbuf.StringUint(uint64(code))
			if _, exists := inv.Capabilities[key]; !exists {
				inv.Capabilities[key] = plugin.Name
			}
		}
		if registration.yang != "" {
			identity := yangModuleName(registration.yang)
			if !reader.moduleIdentities[identity] {
				for _, name := range plugin.YANGFiles {
					if yangModuleName(string(s.blobs[name])) == identity {
						reader.modules[identity] = path.Dir(name)
						reader.moduleIdentities[identity] = true
					}
				}
			}
		}
	}
	slices.SortFunc(inv.FamilySupport, func(a, b Family) int { return strings.Compare(a.Family, b.Family) })
	for name, dir := range reader.modules {
		source := "infrastructure"
		if plugin, found := pluginDirOf(dir + "/"); found {
			source = "plugin:" + plugin
		}
		inv.YANGModules = append(inv.YANGModules, YANGModule{Name: name, Source: source})
	}
	slices.SortFunc(inv.YANGModules, func(a, b YANGModule) int { return strings.Compare(a.Name, b.Name) })
	if err := s.collectFiles(&inv); err != nil {
		return Inventory{}, err
	}
	return inv, nil
}

func (s *Snapshot) collectFiles(inv *Inventory) error {
	var coverage textbuf.Buffer
	stats := make(map[string]*AreaStats, len(codeAreas))
	packages := make(map[string]bool)
	for _, area := range codeAreas {
		stats[area] = &AreaStats{Area: area + "/"}
	}
	for _, name := range s.paths {
		raw := s.blobs[name]
		area, _, _ := strings.Cut(name, "/")
		if area == "test" && strings.HasSuffix(name, ".ci") {
			coverage.Write(raw) //nolint:errcheck // textbuf.Buffer never fails.
			coverage.Byte('\n')
			_, rest, _ := strings.Cut(name, "/")
			if suite, _, nested := strings.Cut(rest, "/"); nested {
				inv.TestCounts[suite]++
				inv.TotalTests++
			}
		}
		areaStats, sourceArea := stats[area]
		if !sourceArea {
			continue
		}
		if strings.HasSuffix(name, ".go") {
			areaStats.Files++
			directory := path.Dir(name)
			if !packages[directory] {
				packages[directory] = true
				areaStats.Packages++
			}
		}
		scanner := bufio.NewScanner(bytes.NewReader(raw))
		for scanner.Scan() {
			if strings.HasSuffix(name, ".go") {
				areaStats.Lines++
			}
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("reading committed %s: %w", name, err)
		}
		if strings.HasSuffix(name, yangSuffix) {
			statements, err := yang.Parse(string(raw), name)
			if err != nil {
				return fmt.Errorf("parse committed schema %s: %w", name, err)
			}
			for _, module := range statements {
				for _, statement := range module.SubStatements() {
					if statement.Keyword == "rpc" {
						moduleName := path.Base(name)
						inv.RPCsByModule[moduleName]++
						inv.TotalRPCs++
						inv.RPCList = append(inv.RPCList, RPC{Name: statement.Argument, Module: moduleName})
					}
				}
			}
		}
	}
	ciContent := coverage.String()
	for index := range inv.RPCList {
		inv.RPCList[index].Covered = rpcHasCoverage(inv.RPCList[index].Name, ciContent)
	}
	slices.SortFunc(inv.RPCList, func(a, b RPC) int {
		if a.Module != b.Module {
			return strings.Compare(a.Module, b.Module)
		}
		return strings.Compare(a.Name, b.Name)
	})
	for _, area := range codeAreas {
		inv.PackageStats = append(inv.PackageStats, *stats[area])
	}
	return nil
}
