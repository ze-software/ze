// Design: docs/guide/migrating-from-vyos.md -- the check of a conversion.
// Related: home_lab_record.go -- the record coverage this runs.
// Related: home_lab_render.go -- the rendered file the CPE runs.
package topology

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/test/labconfig/homecpe"
)

// homeCPEExempt names the scenarios that run a ze-cpe node on purpose with a
// file of their own, each with its reason. The umbrella's Scenarios table
// is the authority: child 9's pppoe-mss-pmtu is not a home-CPE lab scenario.
var homeCPEExempt = map[string]string{
	pppoeMSSPMTUScenario:  "spec-home-cpe-9's scenario dials accel-ppp with its own CPE file (umbrella, Scenarios)",
	pppoeDHCPv6PDScenario: "spec-home-cpe-8's scenario dials accel-ppp with its own CPE file, the IPv6 client the converted configuration does not carry",
}

// checkConversion proves spec-home-cpe-11 AC-1: the record names every line
// of the VyOS source; every home-CPE scenario's CPE runs the rendered
// conversion and no file of its own; `ze config validate` passes on the
// rendered file the CPE mounted; and Ze applied it, the commit-time
// verifiers included, which validate alone does not run: the PPP session is
// up and the bridge, the WireGuard and the xfrm devices exist.
func checkConversion(ctx context.Context, check *interoplab.CheckContext) error {
	if check.Checkout == "" {
		return errors.New("the conversion checker needs the checkout to read the VyOS source and the record; the caller declared none")
	}
	if err := conversionRecordCovers(check.Checkout); err != nil {
		return err
	}
	if err := homeCPEScenariosConverted(); err != nil {
		return err
	}
	validated, err := execOK(ctx, check, nodeCPE, []string{"ze", "config", "validate", zeMountedConfig})
	if err != nil {
		return fmt.Errorf("ze config validate refused the rendered conversion: %w", err)
	}
	if !strings.Contains(validated, "configuration valid") {
		return fmt.Errorf("ze config validate did not report the rendered conversion valid: %q", validated)
	}
	if _, err := waitCPEAddress(ctx, check, subscriberAddress, func(held netip.Addr) bool { return held.String() == subscriberAddress }); err != nil {
		return err
	}
	for _, device := range []string{"br0", "wg0", "vti0"} {
		if _, err := execOK(ctx, check, nodeCPE, []string{"ip", "-o", "link", "show", "dev", device}); err != nil {
			return fmt.Errorf("Ze started with the conversion but the kernel holds no %s: %w", device, err)
		}
	}
	return nil
}

// conversionRecordCovers reads the VyOS source and the record under root and
// runs the coverage check.
func conversionRecordCovers(root string) error {
	directory := filepath.Join(root, filepath.FromSlash(homecpe.Directory))
	source, err := os.ReadFile(filepath.Join(directory, cpeSourceFile)) //nolint:gosec // the checkout root joined with a fixed name
	if err != nil {
		return fmt.Errorf("read the VyOS source: %w", err)
	}
	record, err := os.ReadFile(filepath.Join(directory, cpeRecordFile)) //nolint:gosec // the checkout root joined with a fixed name
	if err != nil {
		return fmt.Errorf("read the conversion record: %w", err)
	}
	rows, err := parseConversionRecord(string(record))
	if err != nil {
		return err
	}
	return recordCoverage(string(source), rows)
}

// homeCPEScenariosConverted refuses every declared scenario whose ze-cpe node
// mounts a configuration other than the rendered conversion, unless
// homeCPEExempt names it, so a hand-written CPE file cannot hide a guide gap
// (spec-home-cpe-11 R-2). Every refusal is reported, in name order.
func homeCPEScenariosConverted() error {
	var faults []error
	for _, name := range ScenarioNames() {
		if _, exempt := homeCPEExempt[name]; exempt {
			continue
		}
		declared := scenarios[name]
		for index := range declared.nodes {
			if declared.nodes[index].name != nodeCPE {
				continue
			}
			if err := runsConversion(name, &declared, &declared.nodes[index]); err != nil {
				faults = append(faults, err)
			}
		}
	}
	return errors.Join(faults...)
}

// runsConversion judges one scenario's ze-cpe node.
func runsConversion(name string, declared *scenario, cpe *node) error {
	if cpe.config != "" {
		return fmt.Errorf("scenario %s runs ze-cpe from its own %s, not the converted configuration; declare its CPE with convertedCPE()", name, cpe.config)
	}
	if declared.prepare == nil {
		return fmt.Errorf("scenario %s has no prepare step, so nothing renders the conversion its ze-cpe mounts", name)
	}
	for _, file := range cpe.prepared {
		if file.source == renderedCPEConfig && file.target == zeMountedConfig {
			return nil
		}
	}
	return fmt.Errorf("scenario %s's ze-cpe mounts no rendered %s at %s", name, renderedCPEConfig, zeMountedConfig)
}
