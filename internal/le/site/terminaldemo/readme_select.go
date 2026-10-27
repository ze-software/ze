// Design: docs/contributing/gh-pages.md -- GitHub observations admit README changes.
package siteterminaldemo

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// CarrierObservation records a human-observed result on GitHub's README surface.
// A caller MUST test the exact SHA256 and retain the evidence before selection.
type CarrierObservation struct {
	SHA256      string `json:"sha256"`
	Result      string `json:"result"`
	Evidence    string `json:"evidence"`
	LimitBytes  int64  `json:"limit-bytes"`
	LimitSource string `json:"limit-source"`
	URL         string `json:"url,omitempty"`
}

// CarrierProbe is supplied only after observing the candidates on GitHub.
// Local browser playback and Markdown API HTML alone MUST NOT count as a pass.
type CarrierProbe struct {
	ObservedAt time.Time          `json:"observed-at"`
	SVG        CarrierObservation `json:"svg"`
	Video      CarrierObservation `json:"video"`
	Thumbnail  CarrierObservation `json:"thumbnail"`
}

func (e *Engine) updateReadmeCarrier(output, probePath string, report *CarrierReport) error {
	if err := readCarrierJSON(filepath.Join(output, "carrier.json"), report); err != nil {
		return err
	}
	demo, entry, _, err := e.verifiedHero()
	if err != nil {
		return err
	}
	if report.Demo != demo.ID {
		return fmt.Errorf("carrier names %q, not the selected hero %q", report.Demo, demo.ID)
	}
	if report.CastSHA256 != entry.Assets["cast"].SHA256 {
		return fmt.Errorf("carrier cast changed; generate and probe new candidates")
	}
	if err := verifyCarrierFiles(output, report); err != nil {
		return err
	}
	var probe CarrierProbe
	if err := readCarrierJSON(probePath, &probe); err != nil {
		return err
	}
	choice, err := selectCarrier(report, &probe)
	if err != nil {
		return err
	}
	// Authored metadata still comes from the manifest, never from the probe file.
	report.Title = demo.Title
	report.Description = demo.Description
	report.Destination = "https://ze-software.net/demos/terminal/#" + demo.Anchor
	report.Choice = choice
	report.Observation = "observed"
	report.Probe = &probe
	root, err := os.OpenRoot(e.root)
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck // Publication operations own the verdict.
	readme, err := root.ReadFile("README.md")
	if err != nil {
		return err
	}
	body, err := replaceReadmeHero(string(readme), readmeCarrierMarkdown(report, &probe))
	if err != nil {
		return err
	}
	// Video uses its observed attachment and a text link, not another image
	// whose GitHub probe may have failed. All candidates remain in output.
	if choice != "video" {
		asset := report.Assets[choice]
		name := "docs/demos/" + demo.ID + "-readme" + filepath.Ext(asset.Path)
		if err := publishCarrierImage(output, asset.Path, root, name); err != nil {
			return err
		}
	}
	if err := root.WriteFile("README.md", []byte(body), 0o644); err != nil {
		return err
	}
	return writeCarrierJSON(filepath.Join(output, "carrier.json"), report)
}

func publishCarrierImage(source, name string, target *os.Root, destination string) error {
	root, err := os.OpenRoot(source)
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck // The copy result owns the verdict.
	input, err := root.Open(name)
	if err != nil {
		return err
	}
	defer input.Close() //nolint:errcheck // The copy result owns the verdict.
	output, err := target.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func selectCarrier(report *CarrierReport, probe *CarrierProbe) (string, error) {
	if probe.ObservedAt.IsZero() {
		return "", fmt.Errorf("GitHub probe needs an observed-at timestamp, not an assumed rendering result")
	}
	for _, candidate := range []struct {
		kind        string
		observation *CarrierObservation
	}{
		{"svg", &probe.SVG}, {"video", &probe.Video}, {"thumbnail", &probe.Thumbnail},
	} {
		observation := candidate.observation
		asset := report.Assets[candidate.kind]
		if observation.SHA256 != asset.SHA256 {
			return "", fmt.Errorf("%s probe does not cover the measured candidate", candidate.kind)
		}
		if strings.TrimSpace(observation.Evidence) == "" {
			return "", fmt.Errorf("%s probe needs retained evidence of GitHub rendering, animation, legibility and destination", candidate.kind)
		}
		if observation.LimitBytes <= 0 {
			return "", fmt.Errorf("%s probe needs the applicable measured upload limit", candidate.kind)
		}
		if err := httpsURL(observation.LimitSource); err != nil {
			return "", fmt.Errorf("%s limit source: %w", candidate.kind, err)
		}
		switch observation.Result {
		case "pass":
			if asset.Bytes > observation.LimitBytes {
				return "", fmt.Errorf("%s exceeds the probe's applicable limit", candidate.kind)
			}
		case "fail":
		default:
			return "", fmt.Errorf("%s GitHub result is unobserved: supply pass or fail with evidence", candidate.kind)
		}
	}
	if probe.SVG.Result == "pass" {
		return "svg", nil
	}
	if probe.Video.Result == "pass" {
		if err := githubVideoURL(probe.Video.URL); err != nil {
			return "", err
		}
		return "video", nil
	}
	if probe.Thumbnail.Result != "pass" {
		return "", fmt.Errorf("neither inline carrier works and the thumbnail has not passed its GitHub probe")
	}
	return "thumbnail", nil
}

func httpsURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("expected an HTTPS URL, got %q", value)
	}
	if parsed.Host == "" {
		return fmt.Errorf("HTTPS URL has no host")
	}
	if strings.ContainsAny(value, "\r\n<>\" ") {
		return fmt.Errorf("URL contains unsafe Markdown characters")
	}
	return nil
}

func githubVideoURL(value string) error {
	if err := httpsURL(value); err != nil {
		return err
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return err
	}
	if parsed.Host != "github.com" {
		return fmt.Errorf("video requires the GitHub attachment URL observed by the probe")
	}
	if !strings.HasPrefix(parsed.Path, "/user-attachments/assets/") {
		return fmt.Errorf("video URL is not a GitHub user attachment")
	}
	if parsed.User != nil {
		return fmt.Errorf("video URL must not carry credentials")
	}
	return nil
}

func readmeCarrierMarkdown(report *CarrierReport, probe *CarrierProbe) string {
	var buffer textbuf.Buffer
	if report.Choice == "video" {
		buffer.Str(probe.Video.URL).Str("\n\n")
	} else {
		extension := ".png"
		if report.Choice == "svg" {
			extension = ".svg"
		}
		title := strings.NewReplacer("\\", "\\\\", "[", "\\[", "]", "\\]", "\n", " ", "\r", " ").Replace(report.Title)
		buffer.Str("[![").Str(title).Str("](docs/demos/").Str(report.Demo).Str("-readme").Str(extension).
			Str(")](").Str(report.Destination).Str(")\n\n")
	}
	buffer.Str(report.Description).
		Str("\n\n[Watch the recording and read its transcript](").Str(report.Destination).Str(").\n")
	return buffer.String()
}

func replaceReadmeHero(readme, replacement string) (string, error) {
	const start = "<!-- terminal-demo-hero:start -->"
	const end = "<!-- terminal-demo-hero:end -->"
	if strings.Count(readme, start) != 1 {
		return "", fmt.Errorf("README must contain exactly one hero start marker")
	}
	if strings.Count(readme, end) != 1 {
		return "", fmt.Errorf("README must contain exactly one hero end marker")
	}
	begin := strings.Index(readme, start) + len(start)
	finish := strings.Index(readme, end)
	if finish < begin {
		return "", fmt.Errorf("README hero markers are reversed")
	}
	return readme[:begin] + "\n" + replacement + readme[finish:], nil
}
