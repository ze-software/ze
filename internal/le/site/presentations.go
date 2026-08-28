// Design: website/AI.md -- presentation artifacts are reproducible Go outputs
package site

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
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
	var mapBody strings.Builder
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
	var out strings.Builder
	offset := 0
	for _, indexes := range matches {
		out.WriteString(content[offset:indexes[0]])
		groups := make([]string, len(indexes)/2)
		for index := range groups {
			start, end := indexes[index*2], indexes[index*2+1]
			if start >= 0 {
				groups[index] = content[start:end]
			}
		}
		out.WriteString(replace(groups[0], groups))
		offset = indexes[1]
	}
	out.WriteString(content[offset:])
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

// ActivityOptions controls the deterministic repository-activity page.
type ActivityOptions struct {
	Repository, Ref, Output string
	Days                    int
	Today                   time.Time
	Compact                 bool
}

// renderActivity collects daily additions and commits from git and writes HTML.
func renderActivity(options ActivityOptions) error {
	if options.Days <= 0 {
		return fmt.Errorf("days must be positive")
	}
	if options.Ref == "" {
		options.Ref = "HEAD"
	}
	if options.Today.IsZero() {
		options.Today = time.Now().UTC()
	}
	start := options.Today.AddDate(0, 0, 1-options.Days).Format("2006-01-02")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "-c", "core.quotePath=false", "-C", options.Repository, "log", options.Ref, "--since="+start, "--date=short", "--format=@@%ad", "--numstat") //nolint:gosec // fixed git verbs over the repository and ref this build was pointed at
	raw, err := command.Output()
	if err != nil {
		return fmt.Errorf("collect activity: %w", err)
	}
	additions, commits := map[string]int{}, map[string]int{}
	day := ""
	for line := range strings.SplitSeq(string(raw), "\n") {
		if rest, ok := strings.CutPrefix(line, "@@"); ok {
			day = rest
			commits[day]++
			continue
		}
		fields := strings.Fields(line)
		if day == "" || len(fields) < 3 {
			continue
		}
		value, parseErr := strconv.Atoi(fields[0])
		if parseErr == nil {
			additions[day] += value
		}
	}
	var days []string
	for value := range commits {
		days = append(days, value)
	}
	sort.Strings(days)
	var out bytes.Buffer
	out.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>Code activity</title></head><body><main><h1>Code activity</h1><table><thead><tr><th>Date</th><th>Commits</th><th>Lines added</th></tr></thead><tbody>")
	for _, value := range days {
		fmt.Fprintf(&out, "<tr><td>%s</td><td>%d</td><td>%d</td></tr>", html.EscapeString(value), commits[value], additions[value])
	}
	out.WriteString("</tbody></table></main></body></html>\n")
	if err := os.MkdirAll(filepath.Dir(options.Output), 0o755); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return err
	}
	return os.WriteFile(options.Output, out.Bytes(), 0o644) //nolint:gosec // published web content: a web server, often another account, serves these bytes
}
