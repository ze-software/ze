// Design: docs/architecture/appliance/docker-host.md -- the unit files the supervisor reads
// Related: supervise.go -- runs what a unit file names

package dockerhost

import (
	"bufio"
	"bytes"
	"fmt"
	"path"
	"strings"
)

// unitDir is where Lima's `lima-guestagent install-systemd` writes its unit.
const unitDir = "/etc/systemd/system"

// readUnitFile reads /etc/systemd/system/<name> and answers the [Service]
// section's ExecStart and Restart. It reads only what the units Lima writes
// use (lima-guestagent.TEMPLATE.service); a unit with no ExecStart is
// refused rather than started as nothing.
func readUnitFile(root, name string) (UnitSpec, error) {
	if path.Base(name) != name {
		return UnitSpec{}, fmt.Errorf("unit name %q holds a path", name)
	}
	data, err := readBounded(guest(root, unitDir+"/"+name))
	if err != nil {
		return UnitSpec{}, err
	}
	var execStart string
	restart := RestartNo
	section := ""
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			section = line
			continue
		}
		if section != "[Service]" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "ExecStart":
			execStart = strings.TrimSpace(value)
		case "Restart":
			restart, err = parseRestart(strings.TrimSpace(value))
			if err != nil {
				return UnitSpec{}, fmt.Errorf("unit %s: %w", name, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return UnitSpec{}, fmt.Errorf("read unit %s: %w", name, err)
	}
	if execStart == "" {
		return UnitSpec{}, fmt.Errorf("unit %s has no ExecStart in [Service]", name)
	}
	// systemd's ExecStart prefixes: '-' ignores the exit status, '@' sets
	// argv[0], '+', '!' and ':' change privileges or expansion. Lima writes
	// none; a unit using one is refused rather than run differently.
	if strings.ContainsAny(execStart[:1], "-@+!:") {
		return UnitSpec{}, fmt.Errorf("unit %s: ExecStart prefix %q is not supported by this guest", name, execStart[:1])
	}
	expanded, err := expandSpecifiers(execStart, root, name)
	if err != nil {
		return UnitSpec{}, fmt.Errorf("unit %s: %w", name, err)
	}
	argv, err := splitCommandLine(expanded)
	if err != nil {
		return UnitSpec{}, fmt.Errorf("unit %s: %w", name, err)
	}
	return UnitSpec{Name: name, Argv: argv, Restart: restart}, nil
}

// parseRestart maps Restart= to a policy; the values the guest does not
// implement are refused by name.
func parseRestart(value string) (RestartPolicy, error) {
	switch value {
	case "no":
		return RestartNo, nil
	case "on-failure":
		return RestartOnFailure, nil
	case "always":
		return RestartAlways, nil
	}
	return RestartUnspecified, fmt.Errorf("unit option Restart=%s is not supported by this guest; use no, on-failure or always", value)
}

// expandSpecifiers expands the systemd specifiers Lima's unit uses: %t the
// runtime directory (/run for a system unit), %N the unit name without its
// suffix, %n the full unit name, and %% a percent sign. Any other specifier
// is refused rather than passed through as text.
func expandSpecifiers(text, root, name string) (string, error) {
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		if text[i] != '%' {
			out.WriteByte(text[i])
			continue
		}
		if i+1 == len(text) {
			return "", fmt.Errorf("ExecStart ends with a lone %%")
		}
		i++
		switch text[i] {
		case 't':
			out.WriteString(guest(root, "/run"))
		case 'N':
			out.WriteString(strings.TrimSuffix(name, path.Ext(name)))
		case 'n':
			out.WriteString(name)
		case '%':
			out.WriteByte('%')
		default:
			return "", fmt.Errorf("ExecStart specifier %%%c is not supported by this guest (%%t, %%N, %%n, %%%%)", text[i])
		}
	}
	return out.String(), nil
}

// splitCommandLine splits ExecStart into words the way systemd does for the
// forms Lima writes: whitespace separates words, and single or double quotes
// group text anywhere in a word and are removed. A backslash escapes the next
// byte inside double quotes and outside quotes.
func splitCommandLine(text string) ([]string, error) {
	var words []string
	var word strings.Builder
	inWord := false
	var quote byte
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
				continue
			}
			if ch == '\\' && quote == '"' && i+1 < len(text) {
				i++
				ch = text[i]
			}
			word.WriteByte(ch)
			continue
		}
		if ch == ' ' || ch == '\t' {
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
			continue
		}
		inWord = true
		if ch == '\'' || ch == '"' {
			quote = ch
			continue
		}
		if ch == '\\' && i+1 < len(text) {
			i++
			ch = text[i]
		}
		word.WriteByte(ch)
	}
	if quote != 0 {
		return nil, fmt.Errorf("ExecStart has an unterminated %c quote", quote)
	}
	if inWord {
		words = append(words, word.String())
	}
	if len(words) == 0 {
		return nil, fmt.Errorf("ExecStart names no command")
	}
	if !path.IsAbs(words[0]) {
		return nil, fmt.Errorf("ExecStart command %q is not an absolute path", words[0])
	}
	return words, nil
}
