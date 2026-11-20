// Design: docs/architecture/third-party-credits.md -- which modules a shipped flavor links
// Overview: credits.go -- the declaration and the record this feeds

package credits

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/deps"
	"github.com/ze-software/ze/internal/le/gitenv"
	repocompiles "github.com/ze-software/ze/internal/le/repo/compiles"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

// flavorLinks is the set of third-party modules one shipped flavor links on
// one operating system.
type flavorLinks struct {
	// Flavor is the matrix row and the system, "distro/linux".
	Flavor string `json:"flavor"`
	// Modules are the module paths, sorted, Ze's own module excluded.
	Modules []string `json:"modules"`
}

// unpinnedSystems are the operating systems a flavor with no pinned GOOS is
// asked about: production is Linux, and the owner's development binary is
// built on macOS, and each links modules the other does not.
var unpinnedSystems = []string{"darwin", "linux"}

// goListTimeout bounds one `go list -deps` run: it loads the package graph of
// one flavor, a few seconds warm.
const goListTimeout = 2 * time.Minute

// goListOutputMax bounds what one run may answer: about 100 module paths.
const goListOutputMax = 1 << 20

// packageTemplate prints one line per package the anchor links: its import
// path, its module (empty for a standard-library package or one of Ze's
// own), and its own load error, quoted, or nothing. Run under -e, so a load
// error is printed here rather than ending the run, and judged by
// linkedModules.
const packageTemplate = "{{.ImportPath}}\t{{with .Module}}{{if not .Main}}{{.Path}}{{end}}{{end}}\t" +
	"{{with .Error}}{{printf \"%q\" .Err}}{{end}}"

// linkedByFlavor asks the go command, without compiling, which modules each
// row of the build matrix links: the flavor's tags plus the feature tags when
// the row takes them, under its pinned system or under each unpinned one.
func linkedByFlavor(root string) ([]flavorLinks, error) {
	features, err := repofeaturetags.DaemonTags(root)
	if err != nil {
		return nil, errors.New("read the feature tags: " + err.Error())
	}
	goMod, err := readRel(root, "go.mod")
	if err != nil {
		return nil, err
	}
	module, err := deps.ModulePath(string(goMod))
	if err != nil {
		return nil, err
	}
	generator := module + "/" + creditsPackage
	var answer []flavorLinks
	for _, flavor := range repocompiles.Flavors() {
		tags := slices.Clone(flavor.Tags)
		if flavor.Features {
			tags = append(tags, features...)
		}
		systems := unpinnedSystems
		if flavor.GOOS != "" {
			systems = []string{flavor.GOOS}
		}
		for _, system := range systems {
			name := flavor.Name + "/" + system
			modules, err := goListModules(root, flavor.Anchor, tags, system, generator)
			if err != nil {
				return nil, errors.New("list the modules flavor " + name + " links: " + err.Error())
			}
			answer = append(answer, flavorLinks{Flavor: name, Modules: modules})
		}
	}
	return answer, nil
}

// goListModules runs `go list -e -deps` for one anchor and answers the
// modules it names, sorted and once each, judged by linkedModules. Both
// bounds are stated: a run that stops answering is killed, and one that
// answers too much is refused. generator is the import path of the package
// that embeds the data update writes.
func goListModules(root, anchor string, tags []string, system, generator string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), goListTimeout)
	defer cancel()
	// The arguments are literals of this package and the matrix's own anchor
	// and tags; le is a build-host tool no network peer reaches.
	command := exec.CommandContext(ctx, "go", "list", "-e", "-mod=vendor", "-deps", //nolint:gosec // literal command; anchor and tags come from the build matrix
		"-tags", strings.Join(tags, ","), "-f", packageTemplate, anchor)
	command.Dir = root
	command.Env = append(gitenv.ReadOnly(os.Environ()), "CGO_ENABLED=0", "GOWORK=off", "GOOS="+system)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	pipe, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := command.Start(); err != nil {
		return nil, errors.New("run go list: " + err.Error())
	}
	raw, readErr := io.ReadAll(io.LimitReader(pipe, goListOutputMax+1))
	waitErr := command.Wait()
	if readErr != nil {
		return nil, errors.New("read what go list answered: " + readErr.Error())
	}
	if waitErr != nil {
		return nil, errors.New("go list failed: " + waitErr.Error() + ": " + strings.TrimSpace(stderr.String()))
	}
	if len(raw) > goListOutputMax {
		return nil, errors.New("go list answered more than 1 MiB of package lines")
	}
	return linkedModules(raw, generator)
}

// linkedModules reads what packageTemplate printed: the modules named, sorted
// and once each. A package that could not be loaded is refused, naming it
// and its error, because a package go list could not load may hide imports,
// and a linked set short of a module is the failure the check exists to
// catch. One error is accepted: the generator package's embed pattern of the
// data directory finding nothing, which is the state before the first
// update writes it. An embed pattern is resolved after the package's imports
// are read, so that package's imports, and the modules below it, are whole.
// Under -deps every package the anchor reaches has its own line, so judging
// each package's own error judges every error the load met.
func linkedModules(raw []byte, generator string) ([]string, error) {
	var modules []string
	for line := range strings.SplitSeq(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			return nil, errors.New("go list answered a line its template does not write: " + line)
		}
		pkg, module, quoted := fields[0], fields[1], fields[2]
		if quoted != "" {
			loadError, err := strconv.Unquote(quoted)
			if err != nil {
				return nil, errors.New("go list answered an error its template does not quote: " + line)
			}
			if !dataNotYetWritten(pkg, loadError, generator) {
				return nil, errors.New("go list cannot load " + pkg + ": " + loadError)
			}
		}
		if module == "" {
			continue
		}
		if slices.Contains(modules, module) {
			continue
		}
		modules = append(modules, module)
	}
	slices.Sort(modules)
	return modules, nil
}

// dataNotYetWritten answers whether a load error is the generator package's
// embed pattern of the data directory matching nothing: "pattern data: no
// matching files found", or a directory with no embeddable file. The same
// error from any other package, or another pattern, is not accepted.
func dataNotYetWritten(pkg, loadError, generator string) bool {
	if pkg != generator {
		return false
	}
	return strings.HasPrefix(loadError, "pattern "+path.Base(DataRel)+": ")
}

// shipped answers every module any flavor links, sorted, with the flavors
// that link each.
func shipped(links []flavorLinks) map[string][]string {
	byModule := make(map[string][]string)
	for _, flavor := range links {
		for _, module := range flavor.Modules {
			if slices.Contains(byModule[module], flavor.Flavor) {
				continue
			}
			byModule[module] = append(byModule[module], flavor.Flavor)
		}
	}
	for module := range byModule {
		slices.Sort(byModule[module])
	}
	return byModule
}
