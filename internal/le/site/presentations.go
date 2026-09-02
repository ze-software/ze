// Design: website/AI.md -- presentation artifacts are reproducible Go outputs
package site

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/sourcerewrite"
)

var presentationMIME = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
	".gif": "image/gif", ".svg": "image/svg+xml", ".webp": "image/webp",
	".ico": "image/x-icon", ".css": "text/css", ".js": "application/javascript",
	".woff": "font/woff", ".woff2": "font/woff2",
}

var (
	cssURLPattern      = regexp.MustCompile(`url\(([^)]*)\)`)
	doubleAssetPattern = regexp.MustCompile(`(<(?:img|link)\s[^>]*(?:src|href)=")([^"]+)(")`)
	singleImagePattern = regexp.MustCompile(`(<img\s[^>]*src=')([^']+)(')`)
	iframePattern      = regexp.MustCompile(`<iframe\s[^>]*src="([^"]+)"[^>]*>`)
	embedPattern       = regexp.MustCompile(`<!--\s*embed:\s*(.+?)\s*-->`)
	titlePattern       = regexp.MustCompile(`(?s)<title>(.*?)</title>`)
)

// bundlePresentation writes input beside itself as name-inlined.html.
func bundlePresentation(input string) (string, error) {
	if !strings.HasSuffix(input, ".html") {
		return "", fmt.Errorf("%s does not end with .html", input)
	}
	if strings.HasSuffix(input, "-inlined.html") {
		return "", fmt.Errorf("%s is already an inlined file", input)
	}
	content, err := os.ReadFile(input) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return "", err
	}
	output := strings.TrimSuffix(input, ".html") + "-inlined.html"
	bundled := bundlePresentationHTML(string(content), filepath.Dir(input))
	bundled = markStandalonePresentation(bundled, filepath.Base(input), filepath.Base(output))
	if err := os.WriteFile(output, []byte(bundled), 0o644); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return "", err
	}
	return output, nil
}

func bundlePresentationHTML(content, base string) string {
	replaceAsset := func(match string, groups []string) string {
		path := groups[2]
		if remoteAsset(path) {
			return match
		}
		uri, ok := presentationDataURI(base, path)
		if !ok {
			return match
		}
		return groups[1] + uri + groups[3]
	}
	content = replaceRegexpGroups(doubleAssetPattern, content, replaceAsset)
	content = replaceRegexpGroups(singleImagePattern, content, replaceAsset)
	content = replaceRegexpGroups(iframePattern, content, func(match string, groups []string) string {
		path := groups[1]
		if remoteAsset(path) {
			return match
		}
		data, err := os.ReadFile(filepath.Join(base, filepath.Clean(filepath.FromSlash(path)))) //nolint:gosec // a site build reads the checkout it was pointed at
		if err != nil {
			return match
		}
		escaped := strings.NewReplacer("&", "&amp;", "\"", "&quot;").Replace(string(data))
		return strings.Replace(match, `src="`+path+`"`, `srcdoc="`+escaped+`"`, 1)
	})
	var tags []string
	slidesPath := filepath.Join(base, "slides.md")
	if slides, err := os.ReadFile(slidesPath); err == nil { //nolint:gosec // a site build reads the checkout it was pointed at
		if !strings.Contains(content, `id="embedded-slides"`) {
			tags = append(tags, `<script id="embedded-slides" type="text/plain">`+base64.StdEncoding.EncodeToString(slides)+`</script>`)
		}
		for _, match := range embedPattern.FindAllSubmatch(slides, -1) {
			name := sanitizeEmbedName(string(match[1]))
			data, readErr := os.ReadFile(filepath.Join(base, name)) //nolint:gosec // a site build reads the checkout it was pointed at
			if readErr != nil {
				continue
			}
			id := "embedded-" + strings.ReplaceAll(name, ".", "-")
			tags = append(tags, `<script id="`+id+`" type="text/plain">`+base64.StdEncoding.EncodeToString(data)+`</script>`)
		}
	}
	if len(tags) != 0 {
		content = injectBeforeScript(content, strings.Join(tags, "\n")+"\n")
	}
	screenshots, _ := filepath.Glob(filepath.Join(base, "screenshots", "*"))
	sort.Strings(screenshots)
	var mapBody textbuf.Buffer
	mapBody.Reset()
	for _, path := range screenshots {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		uri, ok := presentationDataURI(filepath.Dir(path), filepath.Base(path))
		if !ok {
			continue
		}
		fmt.Fprintf(&mapBody, "  %q: %q,\n", filepath.Base(path), uri)
	}
	if mapBody.Len() != 0 {
		content = injectBeforeScript(content, "<script>\nvar _inlinedScreenshots = {\n"+mapBody.String()+"};\n</script>\n")
		content = strings.ReplaceAll(content, `'screenshots/' + escapeHtml(imgName) + '.png'`, `(_inlinedScreenshots[escapeHtml(imgName) + '.png'] || 'screenshots/' + escapeHtml(imgName) + '.png')`)
	}
	return content
}

func presentationDataURI(base, name string) (string, bool) {
	path := filepath.Join(base, filepath.Clean(filepath.FromSlash(name)))
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	mime, ok := presentationMIME[strings.ToLower(filepath.Ext(path))]
	if !ok {
		return "", false
	}
	data, err := os.ReadFile(path) //nolint:gosec // a site build reads the checkout it was pointed at
	if err != nil {
		return "", false
	}
	if filepath.Ext(path) == ".css" {
		data = inlinePresentationCSS(data, filepath.Dir(path))
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), true
}

func inlinePresentationCSS(content []byte, base string) []byte {
	return []byte(replaceRegexpGroups(cssURLPattern, string(content), func(match string, groups []string) string {
		raw := strings.TrimSpace(groups[1])
		quote := ""
		name := raw
		if len(raw) >= 2 && ((raw[0] == '\'' && raw[len(raw)-1] == '\'') || (raw[0] == '"' && raw[len(raw)-1] == '"')) {
			quote, name = raw[:1], raw[1:len(raw)-1]
		}
		if remoteAsset(name) || strings.HasPrefix(name, "#") || strings.HasPrefix(name, "/") {
			return match
		}
		uri, ok := presentationDataURI(base, name)
		if !ok {
			return match
		}
		return "url(" + quote + uri + quote + ")"
	}))
}

func replaceRegexpGroups(pattern *regexp.Regexp, content string, replace func(string, []string) string) string {
	matches := pattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return content
	}
	var out textbuf.Buffer
	out.Reset()
	offset := 0
	for _, indexes := range matches {
		out.Str(content[offset:indexes[0]])
		groups := make([]string, len(indexes)/2)
		for index := range groups {
			start, end := indexes[index*2], indexes[index*2+1]
			if start >= 0 {
				groups[index] = content[start:end]
			}
		}
		out.Str(replace(groups[0], groups))
		offset = indexes[1]
	}
	out.Str(content[offset:])
	return out.String()
}

func remoteAsset(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "data:")
}
func sanitizeEmbedName(name string) string {
	return strings.ToLower(regexp.MustCompile(`[^a-zA-Z0-9._-]+`).ReplaceAllString(name, "-"))
}
func injectBeforeScript(content, insertion string) string {
	if index := strings.Index(content, "<script>"); index >= 0 {
		return content[:index] + insertion + content[index:]
	}
	return strings.Replace(content, "</body>", insertion+"</body>", 1)
}

func markStandalonePresentation(content, input, output string) string {
	if match := titlePattern.FindStringSubmatch(content); len(match) != 0 {
		title := strings.TrimSpace(match[1])
		if !strings.Contains(title, "standalone HTML deck") {
			title = regexp.MustCompile(`\s+-\s+[^<]* slides$`).ReplaceAllString(title, "") + " - standalone HTML deck"
		}
		content = titlePattern.ReplaceAllString(content, "<title>"+title+"</title>")
	}
	content = strings.Replace(content, input, output, 1)
	return content
}

// ActivityOptions controls the deterministic repository-activity page a talk
// deck embeds.
type ActivityOptions struct {
	Repository, Ref, Output string
	Days                    int
	Today                   time.Time
}

// renderActivity writes the calendar heatmap one talk deck embeds in an iframe.
//
// The deck inlines this file as an iframe srcdoc, where no link to the site's
// assets resolves, so the document carries its own stylesheet inside its head.
// That stylesheet is the deck's, not the website's: dark, and sized in
// viewport units so the whole widget lands inside one slide. The published
// /project/activity/ page draws the same measurement light and full size, and
// neither rendering reads the other's rules.
//
// Today closes the window as well as opening it, so a deck frozen at the day it
// was presented shows that day's year and not the months since.
func renderActivity(options ActivityOptions) error {
	if options.Days <= 0 {
		return fmt.Errorf("days must be positive")
	}
	if options.Today.IsZero() {
		options.Today = time.Now().UTC()
	}
	window, err := sourcerewrite.MeasureActivity(options.Repository, options.Days, options.Ref, options.Today)
	if err != nil {
		return fmt.Errorf("measure activity: %w", err)
	}

	var page textbuf.Buffer
	page.Reset().Str("<!doctype html>\n<html lang=\"en\">\n    <head>\n")
	page.Str("        <meta charset=\"utf-8\" />\n")
	page.Str("        <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\" />\n")
	page.Str("        <title>Development activity</title>\n")
	page.Str(activitySlideStyle)
	page.Str("    </head>\n    <body>\n")
	page.Str(activityBody(&window, activitySurfaceSlide))
	page.Str("    </body>\n</html>\n")

	if err := os.MkdirAll(filepath.Dir(options.Output), 0o755); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return err
	}
	return os.WriteFile(options.Output, []byte(page.String()), 0o644) //nolint:gosec // published web content: a web server, often another account, serves these bytes
}
