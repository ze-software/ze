// Design: docs/architecture/container/runtime.md -- the container config and its one rule set
// Related: container.go -- the engine entry, which hands this parser to the config transaction
// Related: register.go -- the registration, whose static verifier is this parser

package container

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The words of the container config root, as the plugin's YANG module spells
// them (yang/ze-container-conf.yang).
const (
	configRoot   = "container"
	listInstance = "instance"
	leafEnabled  = "enabled"
)

// enabledDefault is the YANG default of the enabled leaf.
const enabledDefault = true

// instanceNameLengthMax is the longest container name the schema accepts.
const instanceNameLengthMax = 63

// instanceConfig is one container as the configuration states it. Only
// parseContainerSections builds one, so a value of this type holds a name the
// name rule accepted. The zero value is not a valid container.
type instanceConfig struct {
	name    string
	enabled bool
	// stopTimeout is the time a stop waits, after the stop signal, before the
	// kill: the stop-timeout leaf, or its default.
	stopTimeout time.Duration
	// stopSignal is the stop-signal leaf, or stopSignalFromImage when the
	// config names none.
	stopSignal stopSignal
}

// containerConfig is the whole container root: its instances, sorted by name,
// and each config sibling's parsed part, in registration order
// (sibling.go). The zero value is the config with no container and every
// sibling unconfigured; a removed root parses to no container and each
// sibling's parse of the empty root.
type containerConfig struct {
	instances []instanceConfig
	siblings  []siblingValue
}

// parseContainerSections reads and judges the container root of one delivery.
// It is the one rule set of the plugin: the static verifier that `ze config
// validate` runs, the reload's verify and the boot configure all call it, so
// what validate accepts is what starts. After the runtime's own nodes it runs
// the parse of each registered config sibling, which judges the sibling's
// nodes: the runtime never reads them.
//
// The second result, present, is false only when the delivery carries no
// container section. A section with an empty body is a removed root: it is
// present, with no instance, and installing it stops every container. Reading
// it as "nothing changed" would leave the containers of a deleted config
// running.
//
// It changes no state.
func parseContainerSections(sections []sdk.ConfigSection) (containerConfig, bool, error) {
	return parseSectionsWith(sections, registeredSiblings.configs)
}

// parseSectionsWith is parseContainerSections over the config siblings
// siblings. A sibling's refusal is answered as it is: it is the sibling's
// operator error.
func parseSectionsWith(sections []sdk.ConfigSection, siblings []configSibling) (containerConfig, bool, error) {
	for _, section := range sections {
		if section.Root != configRoot {
			continue
		}
		parsed, err := parseContainerConfig(section.Data)
		if err != nil {
			return containerConfig{}, true, err
		}
		for index := range siblings {
			value, refusal := siblings[index].parse(sections)
			if refusal != nil {
				return containerConfig{}, true, refusal
			}
			parsed.siblings = append(parsed.siblings, siblingValue{name: siblings[index].name, value: value})
		}
		return parsed, true, nil
	}
	return containerConfig{}, false, nil
}

// parseContainerConfig reads the instances of one delivered container
// section. A section arrives wrapped in its root, and a removed root arrives
// as an empty object, which holds no instance.
func parseContainerConfig(data string) (containerConfig, error) {
	var delivered map[string]any
	if err := json.Unmarshal([]byte(data), &delivered); err != nil {
		return containerConfig{}, fmt.Errorf("container config: the delivered section is not a JSON object: %w", err)
	}
	// A nil body (a removed root) indexes to nil, and no entry is read.
	body := configvalue.Section(configRoot, delivered)
	entries := configvalue.ListEntries(body[listInstance])

	cfg := containerConfig{instances: make([]instanceConfig, 0, len(entries))}
	for _, entry := range entries {
		instance, err := parseInstance(entry)
		if err != nil {
			return containerConfig{}, err
		}
		cfg.instances = append(cfg.instances, instance)
	}
	return cfg, nil
}

// parseInstance reads one entry of the instance list. The schema refuses the
// same values when the config is parsed; this is the second check of the
// pair, for a delivery that did not pass that parser.
func parseInstance(entry configvalue.ListEntry) (instanceConfig, error) {
	if err := checkInstanceName(entry.Key); err != nil {
		return instanceConfig{}, err
	}
	enabled := enabledDefault
	if raw, present := entry.Fields[leafEnabled]; present {
		value, ok := configvalue.Bool(raw)
		if !ok {
			return instanceConfig{}, opererr.New(
				"container "+entry.Key+": enabled is neither true nor false",
				"the leaf states whether the container runs, so ze refuses a value it cannot read rather than pick one",
				"set container instance "+entry.Key+" enabled true",
			)
		}
		enabled = value
	}
	stopTimeout, err := parseStopTimeout(entry)
	if err != nil {
		return instanceConfig{}, err
	}
	signal := stopSignalFromImage
	if raw, present := entry.Fields[leafStopSignal]; present {
		word, isWord := raw.(string)
		if !isWord {
			return instanceConfig{}, refuseStopSignal(entry.Key)
		}
		value, known := parseStopSignal(word)
		if !known {
			return instanceConfig{}, refuseStopSignal(entry.Key)
		}
		signal = value
	}
	return instanceConfig{name: entry.Key, enabled: enabled, stopTimeout: stopTimeout, stopSignal: signal}, nil
}

// parseStopTimeout reads the stop-timeout leaf of one entry, or its default
// when the leaf is absent. The schema refuses a value outside 1 to 30; this is
// the second check of the pair.
func parseStopTimeout(entry configvalue.ListEntry) (time.Duration, error) {
	raw, present := entry.Fields[leafStopTimeout]
	if !present {
		return stopTimeoutSecondsDefault * time.Second, nil
	}
	seconds, read := configvalue.Int(raw)
	if !read {
		return 0, refuseStopTimeout(entry.Key, "it is not a whole number")
	}
	if seconds < stopTimeoutSecondsMin {
		return 0, refuseStopTimeout(entry.Key, "it is below 1")
	}
	if seconds > stopTimeoutSecondsMax {
		return 0, refuseStopTimeout(entry.Key, "it is above 30")
	}
	return time.Duration(seconds) * time.Second, nil
}

func refuseStopSignal(name string) error {
	return opererr.New(
		"container "+name+": stop-signal is not one of term, int, quit, hup, usr1 and usr2",
		"the leaf names the signal a stop sends, so ze refuses a value it cannot read rather than send another",
		setInstanceStatement(name, leafStopSignal, "term"),
	)
}

func refuseStopTimeout(name, defect string) error {
	return opererr.New(
		"container "+name+": stop-timeout is not a number of seconds from 1 to 30: "+defect,
		"a daemon stop gives the containers 30 seconds to obey their stop signals, and a stop needs at least one",
		setInstanceStatement(name, leafStopTimeout, "SECONDS"),
	)
}

// checkInstanceName refuses a name outside the form the schema states:
// 1 to 63 characters, lowercase letters, digits and interior hyphens. The
// name becomes a control group, a mount point and a state key, so a name
// with a separator or a dot never reaches those.
func checkInstanceName(name string) error {
	if name == "" {
		return refuseInstanceName(name, "it is empty")
	}
	if len(name) > instanceNameLengthMax {
		return refuseInstanceName(name, "it is longer than 63 characters")
	}
	if name[0] == '-' {
		return refuseInstanceName(name, "it starts with a hyphen")
	}
	if name[len(name)-1] == '-' {
		return refuseInstanceName(name, "it ends with a hyphen")
	}
	// Bounded by instanceNameLengthMax, checked above.
	for i := range len(name) {
		if !isInstanceNameByte(name[i]) {
			return refuseInstanceName(name, "it holds a character other than a lowercase letter, a digit or a hyphen")
		}
	}
	return nil
}

func isInstanceNameByte(c byte) bool {
	if c >= 'a' && c <= 'z' {
		return true
	}
	if c >= '0' && c <= '9' {
		return true
	}
	return c == '-'
}

func refuseInstanceName(name, defect string) error {
	var problem textbuf.Buffer
	problem.Str("container name ").Quoted(name).Str(" is not a name ze accepts: ").Str(defect)
	return opererr.New(
		problem.String(),
		"the name becomes the container's control group, its mount point and its host name, which hold lowercase letters, digits and hyphens only",
		"set container instance NAME with a name of 1 to 63 lowercase letters, digits and interior hyphens",
	)
}
