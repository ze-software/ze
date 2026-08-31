// Design: website/AI.md -- presentation updates are native, reproducible site actions
package site

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/sourcerewrite"
)

const talkStatsTimeout = 30 * time.Second

var (
	coauthoredPattern   = regexp.MustCompile(`\*\*[0-9,]+ co-authored commits\*\*`)
	pluginsPattern      = regexp.MustCompile(`\*\*[0-9,]+ plugins\*\*`)
	configNodesPattern  = regexp.MustCompile(`\*\*[0-9,]+ config nodes\*\*`)
	yangSchemasPattern  = regexp.MustCompile(`across [0-9,]+ YANG schemas`)
	rationalePattern    = regexp.MustCompile(`[0-9,]+ rationale files`)
	functionalPattern   = regexp.MustCompile(`\*\*[0-9,]+ functional tests\*\*`)
	interopPattern      = regexp.MustCompile(`[0-9,]+ interop scenarios`)
	learnedPattern      = regexp.MustCompile(`[0-9,]+ learned summaries`)
	goSizePattern       = regexp.MustCompile(`- Only \*\*\d[^*]*\*\*[^\n]* of Go code`)
	vendorSizePattern   = regexp.MustCompile(`- Only \*\*[^*]+\*\* of vendored code`)
	activityEmbedMarker = []byte(`<!-- embed: activity.html -->`)
)

// talkUpdateOptions controls one source or staged presentation refresh.
type talkUpdateOptions struct {
	Repository string
	Directory  string
	BundleOnly bool
	Today      time.Time
}

// talkUpdateReport names every presentation artifact changed by updateTalk.
type talkUpdateReport struct {
	Talk     string `json:"talk"`
	Slides   string `json:"slides,omitempty"`
	Activity string `json:"activity,omitempty"`
	Bundle   string `json:"bundle"`
}

// updateTalk refreshes live statistics and activity used by a deck, then writes
// its standalone bundle. Decks without slides retain the former bundle-only
// update behavior.
func updateTalk(options talkUpdateOptions) (talkUpdateReport, error) {
	directory, err := filepath.Abs(options.Directory)
	if err != nil {
		return talkUpdateReport{}, err
	}
	input := filepath.Join(directory, "index.html")
	if info, statErr := os.Stat(input); statErr != nil || !info.Mode().IsRegular() {
		if statErr == nil {
			statErr = fmt.Errorf("not a regular file")
		}
		return talkUpdateReport{}, fmt.Errorf("talk entry point %s: %w", input, statErr)
	}
	report := talkUpdateReport{Talk: filepath.Base(directory)}
	if !options.BundleOnly {
		slides := filepath.Join(directory, "slides.md")
		content, readErr := os.ReadFile(slides) //nolint:gosec // a site build reads the checkout it was pointed at
		switch {
		case readErr == nil:
			if options.Repository == "" {
				return talkUpdateReport{}, fmt.Errorf("repository is required to refresh talk slides")
			}
			stats, statsErr := collectTalkStats(options.Repository)
			if statsErr != nil {
				return talkUpdateReport{}, statsErr
			}
			content = updateTalkSlides(content, stats)
			if writeErr := os.WriteFile(slides, content, 0o644); writeErr != nil { //nolint:gosec // a tracked slide deck in the checkout keeps the tree's own 0644
				return talkUpdateReport{}, writeErr
			}
			report.Slides = slides
			if bytes.Contains(content, activityEmbedMarker) {
				activity := filepath.Join(directory, "activity.html")
				if renderErr := renderActivity(ActivityOptions{
					Repository: options.Repository,
					Output:     activity,
					Days:       sourcerewrite.ActivityDaysDefault,
					Today:      options.Today,
				}); renderErr != nil {
					return talkUpdateReport{}, renderErr
				}
				report.Activity = activity
			}
		case os.IsNotExist(readErr):
		default:
			return talkUpdateReport{}, readErr
		}
	}
	report.Bundle, err = bundlePresentation(input)
	if err != nil {
		return talkUpdateReport{}, err
	}
	return report, nil
}

// refreshTalks updates every authored deck in a staged website tree.
func refreshTalks(repository, talksDirectory string) ([]talkUpdateReport, error) {
	entries, err := os.ReadDir(talksDirectory)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	reports := make([]talkUpdateReport, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		directory := filepath.Join(talksDirectory, entry.Name())
		if _, statErr := os.Stat(filepath.Join(directory, "index.html")); os.IsNotExist(statErr) {
			continue
		} else if statErr != nil {
			return nil, statErr
		}
		options := talkUpdateOptions{
			Repository: repository,
			Directory:  directory,
			BundleOnly: true,
		}
		activity := ""
		if entry.Name() == "linx-2026-06" {
			// The published deck is a snapshot of the day it was presented.
			activity = filepath.Join(directory, "activity.html")
			if renderErr := renderActivity(ActivityOptions{
				Repository: repository,
				Output:     activity,
				Days:       sourcerewrite.ActivityDaysDefault,
				Today:      time.Date(2026, time.June, 11, 0, 0, 0, 0, time.UTC),
			}); renderErr != nil {
				return nil, fmt.Errorf("update talk %s activity: %w", entry.Name(), renderErr)
			}
		}
		report, updateErr := updateTalk(options)
		if updateErr != nil {
			return nil, fmt.Errorf("update talk %s: %w", entry.Name(), updateErr)
		}
		report.Activity = activity
		reports = append(reports, report)
	}
	return reports, nil
}

type talkStats struct {
	Coauthored, Plugins, YANGNodes, YANGFiles int
	Rationale, Functional, Interop, Learned   int
	GoLines                                   int64
	VendorBytes                               int64
}

func collectTalkStats(repository string) (talkStats, error) {
	root, err := filepath.Abs(repository)
	if err != nil {
		return talkStats{}, err
	}
	stats := talkStats{}
	ctx, cancel := context.WithTimeout(context.Background(), talkStatsTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "-C", root, "log", "--format=%H", "--grep=Co-Authored-By") //nolint:gosec // fixed git verbs over the checkout root
	output, err := command.Output()
	if err != nil {
		return talkStats{}, fmt.Errorf("count co-authored commits: %w", err)
	}
	stats.Coauthored = countNonEmptyLines(output)
	lineBuffer := make([]byte, 32*1024)

	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		relative = filepath.ToSlash(relative)
		if entry.IsDir() {
			if relative == gitMetadataDir || entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			if filepath.Dir(relative) == "internal/plugins" {
				stats.Plugins++
			}
			if filepath.Dir(relative) == "test/interop/scenarios" {
				stats.Interop++
			}
			return nil
		}
		name := entry.Name()
		switch {
		case strings.HasSuffix(name, ".yang"):
			stats.YANGFiles++
			nodes, countErr := countYANGNodes(path)
			if countErr != nil {
				return countErr
			}
			stats.YANGNodes += nodes
		case strings.HasSuffix(name, ".ci") && pathUnder(relative, "test"):
			stats.Functional++
		case strings.HasSuffix(name, ".md") && pathUnder(relative, "ai/rationale"):
			stats.Rationale++
		case strings.HasSuffix(name, ".md") && pathUnder(relative, "plan/learned"):
			stats.Learned++
		case strings.HasSuffix(name, ".go") && (pathUnder(relative, "internal") || pathUnder(relative, "cmd")):
			lines, countErr := countFileLines(path, lineBuffer)
			if countErr != nil {
				return countErr
			}
			stats.GoLines += lines
		}
		if filepath.Dir(relative) == "test/interop/scenarios" {
			stats.Interop++
		}
		return nil
	})
	if err != nil {
		return talkStats{}, err
	}
	stats.VendorBytes, err = directoryBytes(filepath.Join(root, "vendor"))
	if err != nil {
		return talkStats{}, err
	}
	return stats, nil
}

func updateTalkSlides(content []byte, stats talkStats) []byte {
	text := string(content)
	text = coauthoredPattern.ReplaceAllString(text, "**"+formatCount(stats.Coauthored)+" co-authored commits**")
	text = pluginsPattern.ReplaceAllString(text, "**"+formatCount(stats.Plugins)+" plugins**")
	text = configNodesPattern.ReplaceAllString(text, "**"+formatCount(stats.YANGNodes)+" config nodes**")
	text = yangSchemasPattern.ReplaceAllString(text, "across "+formatCount(stats.YANGFiles)+" YANG schemas")
	text = rationalePattern.ReplaceAllString(text, formatCount(stats.Rationale)+" rationale files")
	text = functionalPattern.ReplaceAllString(text, "**"+formatCount(stats.Functional)+" functional tests**")
	text = interopPattern.ReplaceAllString(text, formatCount(stats.Interop)+" interop scenarios")
	text = learnedPattern.ReplaceAllString(text, formatCount(stats.Learned)+" learned summaries")
	text = goSizePattern.ReplaceAllString(text, "- Only **"+formatCount(int(stats.GoLines/1000))+"k lines** of Go code")
	text = vendorSizePattern.ReplaceAllString(text, "- Only **"+formatByteSize(stats.VendorBytes)+"** of vendored code")
	return []byte(text)
}

func pathUnder(path, directory string) bool {
	return path == directory || strings.HasPrefix(path, directory+"/")
}

func countYANGNodes(path string) (int, error) {
	file, err := os.Open(path) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return 0, err
	}
	defer func() { _ = file.Close() }() // read-only: a close error says nothing the read did not
	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "leaf", "leaf-list", "container", "list":
			count++
		}
	}
	return count, scanner.Err()
}

func countFileLines(path string, buffer []byte) (int64, error) {
	file, err := os.Open(path) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return 0, err
	}
	defer func() { _ = file.Close() }() // read-only: a close error says nothing the read did not
	var count int64
	for {
		n, readErr := file.Read(buffer)
		count += int64(bytes.Count(buffer[:n], []byte{'\n'}))
		if readErr == io.EOF {
			return count, nil
		}
		if readErr != nil {
			return 0, readErr
		}
	}
}

func directoryBytes(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) && path == root {
				return nil
			}
			return walkErr
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}

func countNonEmptyLines(content []byte) int {
	count := 0
	for line := range bytes.SplitSeq(content, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) != 0 {
			count++
		}
	}
	return count
}

func formatCount(value int) string {
	raw := strconv.Itoa(value)
	for position := len(raw) - 3; position > 0; position -= 3 {
		raw = raw[:position] + "," + raw[position:]
	}
	return raw
}

func formatByteSize(value int64) string {
	const (
		kib = int64(1024)
		mib = 1024 * kib
		gib = 1024 * mib
	)
	switch {
	case value >= gib:
		return strconv.FormatInt((value+gib-1)/gib, 10) + "G"
	case value >= mib:
		return strconv.FormatInt((value+mib-1)/mib, 10) + "M"
	case value >= kib:
		return strconv.FormatInt((value+kib-1)/kib, 10) + "K"
	default:
		return strconv.FormatInt(value, 10)
	}
}
