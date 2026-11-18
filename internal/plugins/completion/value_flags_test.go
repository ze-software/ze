// Design: docs/architecture/cli/command-completion.md -- offline flag values

package completion

import (
	"bytes"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
)

// TestInitCertificateValueFlags checks the real init registration, including
// the neighboring switch and file flags, before a shell consumes its hints.
func TestInitCertificateValueFlags(t *testing.T) {
	flags := registry.CommandFlags("init")
	for name, want := range map[string]string{
		"--web-cert":      "text",
		"--web-cert-name": "text",
		"--force":         registry.FlagValueNone,
		"--from":          registry.FlagValueFile,
	} {
		index := slices.IndexFunc(flags, func(flag registry.FlagSpec) bool { return flag.Name == name })
		if index < 0 {
			t.Fatalf("init registration has no %s", name)
		}
		if got := flags[index].ValueHint; got != want {
			t.Errorf("init %s value hint = %q, want %q", name, got, want)
		}
	}
}

// TestFlagInventoryAfterCertificateValues proves that completed values do not
// become command-path words. Both separate and equals forms must return flags.
func TestFlagInventoryAfterCertificateValues(t *testing.T) {
	for _, words := range [][]string{
		{"init", "--web-cert", "0.0.0.0:8080"},
		{"init", "--web-cert-name", "router.example.net"},
		{"init", "--web-cert=0.0.0.0:8080"},
		{"init", "--web-cert-name=router.example.net"},
		{"init", "--force", "--web-cert", "[2001:db8::1]:443", "--web-cert-name", "router.example.net"},
	} {
		t.Run(strings.Join(words, " "), func(t *testing.T) {
			var output bytes.Buffer
			if code := writeFlags(&output, words); code != 0 {
				t.Fatalf("flags %v: exit %d", words, code)
			}
			if !strings.Contains(output.String(), "--force\t") {
				t.Fatalf("flags after completed value missing --force: %q", output.String())
			}
		})
	}
}

// TestRootWordsAfterFlagValue asks the existing root-word producer to resume
// its registered verbs after a value which happens to spell a real verb.
func TestRootWordsAfterFlagValue(t *testing.T) {
	prior := registry.CommandFlags("secret")
	t.Cleanup(func() { registry.RegisterCommandFlags("secret", prior) })
	registry.RegisterCommandFlags("secret", []registry.FlagSpec{{Name: "--label", ValueHint: "text"}})
	for _, words := range [][]string{
		{"secret", "--label", "decode"},
		{"secret", "--label=decode"},
	} {
		var output bytes.Buffer
		if code := writeWords(&output, words); code != 0 {
			t.Fatalf("words %v: exit %d", words, code)
		}
		for _, verb := range []string{"decode", "encode"} {
			if !strings.Contains(output.String(), verb+"\t") {
				t.Errorf("words %v missing %s after flag value: %q", words, verb, output.String())
			}
		}
	}
}

// TestCompletionFlagValueHelper serves the real completion command from this
// test executable. The native shells below use it as their ze command.
func TestCompletionFlagValueHelper(t *testing.T) {
	index := slices.Index(os.Args, "--completion-value-helper")
	if index < 0 {
		return
	}
	args := os.Args[index+1:]
	if len(args) == 0 {
		t.Fatal("helper received no command")
	}
	if args[0] != "completion" {
		t.Fatalf("helper received unexpected command %v", args)
	}
	registry.RegisterCommandFlags("secret", []registry.FlagSpec{{Name: "--label", ValueHint: "text"}})
	if code := Run(args[1:]); code != 0 {
		t.Fatalf("completion %v: exit %d", args[1:], code)
	}
}

// TestBashCertificateValueCompletion executes the generated bash function with
// real completion queries, rather than checking its source for a marker.
func TestBashCertificateValueCompletion(t *testing.T) {
	// The executable path is a positional parameter, not shell-quoted source.
	query := `
test_binary=$1
ze() { "$test_binary" -test.run='^TestCompletionFlagValueHelper$' -- --completion-value-helper "$@" | sed '/^PASS$/d'; }
probe() {
    local label=$1
    shift
    COMP_WORDS=(ze "$@")
    COMP_CWORD=$((${#COMP_WORDS[@]} - 1))
    COMPREPLY=()
    _ze
    printf '%s\t%s\n' "$label" "${COMPREPLY[*]}"
}
probe address-empty init --web-cert ''
probe address-dash init --web-cert --web
probe name-empty init --web-cert-name ''
probe name-dash init --web-cert-name --web
probe next-name init --web-cert 0.0.0.0:8080 --web-cert-n
probe next-force init --web-cert-name router.example.net --for
probe equals-next init --web-cert=0.0.0.0:8080 --web-cert-n
probe boolean init --force --web-cert-n
probe address-equals-empty init --web-cert=
probe name-equals-text init --web-cert-name=router
probe ipv6-next init --web-cert '[2001:db8::1]:443' --web-cert-n
probe empty-value-next init --web-cert '' --for
probe boolean-equals init --force=true --web-cert-n
probe root-value secret --label decode ''
probe root-equals secret --label=decode ''
`
	checkCertificateShell(t, "bash", bashScript()+query)
}

// TestZshCertificateValueCompletion exercises the generated zsh args state.
// _arguments supplies that state in an interactive completion session; the
// harness supplies it directly so it needs no terminal or user startup files.
func TestZshCertificateValueCompletion(t *testing.T) {
	query := `
test_binary=$1
ze() { "$test_binary" -test.run='^TestCompletionFlagValueHelper$' -- --completion-value-helper "$@" | sed '/^PASS$/d'; }
compdef() { :; }
_arguments() { state=args; }
_describe() {
    local -a candidates
    candidates=("${(@P)2}")
    local candidate
    for candidate in "${candidates[@]}"; do
        candidate=${candidate%%:*}
        [[ "$candidate" == "${words[CURRENT]}"* ]] && print -r -- "$candidate"
    done
    return 0
}
_files() { print UNEXPECTED-FILE; }
probe() {
    local label=$1
    shift
    local -a words=("$@")
    local CURRENT=${#words}
    local state
    local result=$(_ze)
    printf '%s\t%s\n' "$label" "${result//$'\n'/ }"
}
`
	// compdef must exist before the generated script registers its function.
	text := "compdef() { :; }\n" + zshScript() + query + `
probe address-empty init --web-cert ''
probe address-dash init --web-cert --web
probe name-empty init --web-cert-name ''
probe name-dash init --web-cert-name --web
probe next-name init --web-cert 0.0.0.0:8080 --web-cert-n
probe next-force init --web-cert-name router.example.net --for
probe equals-next init --web-cert=0.0.0.0:8080 --web-cert-n
probe boolean init --force --web-cert-n
probe address-equals-empty init --web-cert=
probe name-equals-text init --web-cert-name=router
probe ipv6-next init --web-cert '[2001:db8::1]:443' --web-cert-n
probe empty-value-next init --web-cert '' --for
probe boolean-equals init --force=true --web-cert-n
probe root-value secret --label decode ''
probe root-equals secret --label=decode ''
`
	checkCertificateShell(t, "zsh", text)
}

// TestFishCertificateValueCompletion asks fish's native complete -C engine to
// run the generated rules, including its commandline context and prefix filter.
func TestFishCertificateValueCompletion(t *testing.T) {
	query := `
set -g test_binary $argv[1]
function ze
    $test_binary -test.run='^TestCompletionFlagValueHelper$' -- --completion-value-helper $argv | string match -v PASS
end
function probe
    set -l result (complete -C $argv[2] | string split -f 1 \t)
    printf '%s\t%s\n' $argv[1] (string join ' ' -- $result)
end
probe address-empty 'ze init --web-cert '
probe address-dash 'ze init --web-cert --web'
probe name-empty 'ze init --web-cert-name '
probe name-dash 'ze init --web-cert-name --web'
probe next-name 'ze init --web-cert 0.0.0.0:8080 --web-cert-n'
probe next-force 'ze init --web-cert-name router.example.net --for'
probe equals-next 'ze init --web-cert=0.0.0.0:8080 --web-cert-n'
probe boolean 'ze init --force --web-cert-n'
probe address-equals-empty 'ze init --web-cert='
probe name-equals-text 'ze init --web-cert-name=router'
probe ipv6-next 'ze init --web-cert "[2001:db8::1]:443" --web-cert-n'
probe empty-value-next 'ze init --web-cert "" --for'
probe boolean-equals 'ze init --force=true --web-cert-n'
probe root-value 'ze secret --label decode '
probe root-equals 'ze secret --label=decode '
`
	checkCertificateShell(t, "fish", fishScript()+query)
}

func checkCertificateShell(t *testing.T, shell, source string) {
	t.Helper()
	program, err := exec.LookPath(shell)
	if err != nil {
		t.Skipf("generated completion acceptance requires %s: %v", shell, err)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"-c", source, "completion-test", binary}
	if shell == "fish" {
		// A completion fixture must not execute the operator's startup files.
		args = []string{"--no-config", "-c", source, binary}
	}
	output, err := exec.CommandContext(t.Context(), program, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s completion: %v\n%s", shell, err, output)
	}
	want := map[string]string{
		"address-empty": "", "address-dash": "", "name-empty": "", "name-dash": "",
		"next-name": "--web-cert-name", "next-force": "--force",
		"equals-next": "--web-cert-name", "boolean": "--web-cert-name",
		"address-equals-empty": "", "name-equals-text": "",
		"ipv6-next": "--web-cert-name", "empty-value-next": "--force",
		"boolean-equals": "--web-cert-name",
		"root-value":     "decode encode", "root-equals": "decode encode",
	}
	for line := range strings.SplitSeq(strings.TrimSuffix(string(output), "\n"), "\n") {
		name, candidates, ok := strings.Cut(line, "\t")
		if !ok {
			t.Errorf("%s emitted a non-record: %q", shell, line)
			continue
		}
		expected, found := want[name]
		if !found {
			t.Errorf("%s emitted an unknown record: %q", shell, line)
			continue
		}
		if candidates != expected {
			t.Errorf("%s %s = %q, want %q", shell, name, candidates, expected)
		}
		delete(want, name)
	}
	if len(want) > 0 {
		t.Errorf("%s omitted completion records: %v", shell, want)
	}
}
