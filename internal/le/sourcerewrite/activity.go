package sourcerewrite

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	defaultActivityOutput = "tmp/code-activity.html"
	defaultActivityRef    = "HEAD"
	defaultActivityServe  = "127.0.0.1:8000"
)

// ActivityDaysDefault is the window every activity surface opens on: a year
// ending on the day it is measured. It is exported because a caller that wants
// that window states it rather than repeating the number.
const ActivityDaysDefault = 365

// dayLayout formats a calendar day. Every bound this file gives git is a day
// with a fixed time appended, so the time never enters the layout.
const dayLayout = "2006-01-02"

var activityCodeExtensions = map[string]bool{
	".awk": true, ".bash": true, ".c": true, ".cc": true, ".cfg": true,
	".ci": true, ".conf": true, ".cpp": true, ".css": true, ".et": true,
	".go": true, ".h": true, ".hpp": true, ".html": true, ".js": true,
	".json": true, ".jsx": true, ".lua": true, ".mk": true, ".pl": true,
	".proto": true, ".py": true, ".rb": true, ".rs": true, ".scss": true,
	".sh": true, ".sql": true, ".tmpl": true, ".toml": true, ".tpl": true,
	".ts": true, ".tsx": true, ".yang": true, ".yaml": true, ".yml": true,
	".zsh": true,
}

var activityCodeNames = map[string]bool{
	"Dockerfile": true, "GNUmakefile": true, "Makefile": true, "go.mod": true, "go.sum": true,
}

var defaultActivityExcludes = []string{
	"tmp/*", "vendor/*", "*/vendor/*", "pages/activity.html",
	"pages/code-activity.html", "tmp/code-activity.html",
}

// ActivityOptions selects the history, files, and output used by the dashboard.
type ActivityOptions struct {
	Repo       string
	Days       int
	Output     string
	Ref        string
	AllRefs    bool
	AllFiles   bool
	Extensions map[string]bool
	Excludes   []string
	Author     string
	Open       bool
}

// activityReport names the static dashboard written by writeActivity.
type activityReport struct {
	File string `json:"file"`
}

func (r activityReport) Text() string {
	return "wrote " + r.File + "\n"
}

type activityTotals struct {
	Additions map[time.Time]int
	Commits   map[time.Time]int
}

// ActivityGoBucket counts one class of Go file: how many files it holds, and
// how their lines divide between code, blank and comment.
type ActivityGoBucket struct {
	Files        int
	TotalLines   int
	CodeLines    int
	BlankLines   int
	CommentLines int
}

// ActivityGo is a checkout's tracked Go source in four buckets. Total counts
// every first-party file, and Code and Tests divide that same population, so
// Code plus Tests equals Total and Vendor stands outside all three.
type ActivityGo struct {
	Total         ActivityGoBucket
	Code          ActivityGoBucket
	Tests         ActivityGoBucket
	Vendor        ActivityGoBucket
	VendorModules int
}

// defaultActivityOptions returns the legacy producer's defaults for root.
func defaultActivityOptions(root string) ActivityOptions {
	extensions := make(map[string]bool, len(activityCodeExtensions))
	for extension := range activityCodeExtensions {
		extensions[extension] = true
	}
	return ActivityOptions{
		Repo: root, Days: ActivityDaysDefault, Output: defaultActivityOutput, Ref: defaultActivityRef,
		Extensions: extensions, Excludes: append([]string(nil), defaultActivityExcludes...),
	}
}

// parseExtensions parses the producer's comma-separated extension grammar.
func parseExtensions(raw string) (map[string]bool, error) {
	extensions := make(map[string]bool)
	for item := range strings.SplitSeq(raw, ",") {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		if !strings.HasPrefix(item, ".") {
			item = "." + item
		}
		extensions[item] = true
	}
	if len(extensions) == 0 {
		return nil, fmt.Errorf("at least one extension is required")
	}
	return extensions, nil
}

func resolveActivityOptions(options ActivityOptions) (ActivityOptions, error) {
	if options.Days <= 0 {
		return options, fmt.Errorf("--days must be positive")
	}
	repo, err := filepath.Abs(options.Repo)
	if err != nil {
		return options, err
	}
	rootResult, err := runGit(repo, false, "rev-parse", "--show-toplevel")
	if err != nil {
		return options, fmt.Errorf("not inside a git repository: %s", repo)
	}
	options.Repo = strings.TrimSpace(rootResult)
	if !filepath.IsAbs(options.Output) {
		options.Output = filepath.Join(options.Repo, filepath.FromSlash(options.Output))
	}
	if relative, relativeErr := filepath.Rel(options.Repo, options.Output); relativeErr == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		options.Excludes = append(options.Excludes, filepath.ToSlash(relative))
	}
	return options, nil
}

// writeActivity renders and writes one self-contained activity dashboard.
func writeActivity(options ActivityOptions) (activityReport, error) {
	resolved, err := resolveActivityOptions(options)
	if err != nil {
		return activityReport{}, err
	}
	body, err := renderActivityPage(resolved, time.Now())
	if err != nil {
		return activityReport{}, err
	}
	if err := os.MkdirAll(filepath.Dir(resolved.Output), 0o750); err != nil {
		return activityReport{}, err
	}
	if err := os.WriteFile(resolved.Output, []byte(body), 0o600); err != nil {
		return activityReport{}, err
	}
	if options.Open {
		fileURL := (&url.URL{Scheme: "file", Path: resolved.Output}).String()
		if err := openActivityURL(fileURL); err != nil {
			return activityReport{}, err
		}
	}
	return activityReport{File: resolved.Output}, nil
}

// serveActivity serves a dashboard that is recomputed for every request.
func serveActivity(options ActivityOptions, address string) error {
	resolved, err := resolveActivityOptions(options)
	if err != nil {
		return err
	}
	host, port, err := parseActivityAddress(address)
	if err != nil {
		return err
	}
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return err
	}
	defer func() {
		_ = listener.Close()
	}()
	url := "http://" + net.JoinHostPort(host, strconv.Itoa(port)) + "/"
	_, _ = fmt.Fprintln(os.Stdout, "serving dynamic activity page at "+url)
	if options.Open {
		if err := openActivityURL(url); err != nil {
			return err
		}
	}
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/" && request.URL.Path != "/activity.html" {
			http.NotFound(writer, request)
			return
		}
		body, renderErr := renderActivityPage(resolved, time.Now())
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		if renderErr != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(writer, "<pre>%s</pre>", html.EscapeString(renderErr.Error()))
			return
		}
		writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = writer.Write([]byte(body))
	})
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	serveResult := make(chan error, 1)
	go func() {
		serveResult <- server.Serve(listener)
	}()
	select {
	case serveErr := <-serveResult:
		signal.Stop(interrupt)
		if errors.Is(serveErr, http.ErrServerClosed) {
			return nil
		}
		return serveErr
	case <-interrupt:
		signal.Stop(interrupt)
		_, _ = fmt.Fprintln(os.Stdout, "\nstopped")
		return server.Close()
	}
}

func parseActivityAddress(value string) (string, int, error) {
	separator := strings.LastIndexByte(value, ':')
	if separator < 0 {
		return value, 8000, nil
	}
	host, rawPort := value[:separator], value[separator+1:]
	if host == "" {
		host = "127.0.0.1"
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port: %s", rawPort)
	}
	return host, port, nil
}

func runGit(repo string, check bool, arguments ...string) (string, error) {
	argv := make([]string, 0, 4+len(arguments))
	argv = append(argv, "-c", "core.quotePath=false", "-C", repo)
	argv = append(argv, arguments...)
	command := exec.CommandContext(context.Background(), "git", argv...) //nolint:gosec // argv is a direct native git invocation; no shell interprets it
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err != nil && check {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("git command failed: git %s\n%s", strings.Join(argv, " "), detail)
	}
	if err != nil {
		return stdout.String(), err
	}
	return stdout.String(), nil
}

func collectActivity(options ActivityOptions, today time.Time) (activityTotals, error) {
	today = dateOnly(today)
	start := today.AddDate(0, 0, -(options.Days - 1))
	// today closes the window as well as opening it. Bounded by --since alone,
	// a measurement pinned to a past day still read every commit made after it:
	// those commits fall outside the drawn grid, but they set the heat scale
	// every cell inside it is colored against, so one large drop made after a
	// frozen talk date flattened the year that deck was showing.
	//
	// The clock of each bound is appended rather than formatted, because a Go
	// layout spells 23:59:59 as 15:04:05: a literal "23:59:59" in the layout
	// reads as a day and a 12-hour clock, and git answers nothing for it.
	arguments := []string{"log", "--date=short", "--pretty=format:@@@%ad", "--numstat",
		"--since=" + start.Format(dayLayout) + "T00:00:00Z",
		"--until=" + today.Format(dayLayout) + "T23:59:59Z"}
	if options.Author != "" {
		arguments = append(arguments, "--author="+options.Author)
	}
	if options.AllRefs {
		arguments = append(arguments, "--all")
	} else {
		arguments = append(arguments, options.Ref)
	}
	arguments = append(arguments, "--")
	output, err := runGit(options.Repo, true, arguments...)
	if err != nil {
		return activityTotals{}, err
	}
	totals := activityTotals{Additions: make(map[time.Time]int), Commits: make(map[time.Time]int)}
	var current time.Time
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, "@@@") {
			current, err = time.Parse("2006-01-02", line[3:])
			if err != nil {
				return activityTotals{}, err
			}
			totals.Commits[current]++
			continue
		}
		if current.IsZero() || line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 || !allDigits(parts[0]) {
			continue
		}
		if !isActivitySourcePath(parts[2], options) {
			continue
		}
		added, _ := strconv.Atoi(parts[0])
		totals.Additions[current] += added
	}
	return totals, nil
}

func firstActivityCommit(options ActivityOptions) (*time.Time, error) {
	arguments := []string{"log", "--date=short", "--pretty=format:%ad"}
	if options.AllRefs {
		arguments = append(arguments, "--all")
	} else {
		arguments = append(arguments, options.Ref)
	}
	arguments = append(arguments, "--")
	output, err := runGit(options.Repo, true, arguments...)
	if err != nil {
		return nil, err
	}
	var earliest *time.Time
	for line := range strings.FieldsSeq(output) {
		day, parseErr := time.Parse("2006-01-02", line)
		if parseErr != nil {
			return nil, parseErr
		}
		if earliest == nil || day.Before(*earliest) {
			copy := day
			earliest = &copy
		}
	}
	return earliest, nil
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func numstatTargetPath(path string) string {
	path = strings.TrimSpace(path)
	if !strings.Contains(path, "=>") {
		return path
	}
	if close := strings.IndexByte(path, '}'); close >= 0 && strings.Contains(path[:close], "{") {
		prefix, suffix := path[:close], path[close+1:]
		if _, target, ok := strings.Cut(prefix, "=>"); ok {
			return strings.TrimSpace(target) + suffix
		}
	}
	_, target, _ := strings.Cut(path, "=>")
	return strings.TrimSpace(target)
}

func isActivitySourcePath(path string, options ActivityOptions) bool {
	path = numstatTargetPath(path)
	for _, pattern := range options.Excludes {
		if fnmatch(pattern, path) {
			return false
		}
	}
	if options.AllFiles {
		return true
	}
	return activityCodeNames[filepath.Base(path)] || options.Extensions[strings.ToLower(filepath.Ext(path))]
}

func fnmatch(pattern, value string) bool {
	var expression textbuf.Buffer
	expression.Reset().Byte('^')
	for index := 0; index < len(pattern); index++ {
		switch pattern[index] {
		case '*':
			expression.Str(".*")
		case '?':
			expression.Byte('.')
		case '[':
			end := strings.IndexByte(pattern[index+1:], ']')
			if end < 0 {
				expression.Str(`\[`)
				continue
			}
			end += index + 1
			class := pattern[index+1 : end]
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			}
			expression.Byte('[').Str(class).Byte(']')
			index = end
		default:
			expression.Str(regexp.QuoteMeta(string(pattern[index])))
		}
	}
	expression.Byte('$')
	matched, err := regexp.MatchString(expression.String(), value)
	return err == nil && matched
}

func collectGoStats(options ActivityOptions) (ActivityGo, error) {
	output, err := runGit(options.Repo, true, "ls-files", "--", "*.go")
	if err != nil {
		return ActivityGo{}, err
	}
	var stats ActivityGo
	for relative := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		if relative == "" {
			continue
		}
		path := filepath.Join(options.Repo, filepath.FromSlash(relative))
		info, statErr := os.Stat(path)
		if statErr != nil || !info.Mode().IsRegular() {
			continue
		}
		if strings.HasPrefix(relative, "vendor/") {
			if err := addGoFile(&stats.Vendor, path); err != nil {
				return ActivityGo{}, err
			}
			continue
		}
		excluded := false
		for _, pattern := range options.Excludes {
			if fnmatch(pattern, relative) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}
		if err := addGoFile(&stats.Total, path); err != nil {
			return ActivityGo{}, err
		}
		bucket := &stats.Code
		if strings.HasSuffix(relative, "_test.go") {
			bucket = &stats.Tests
		}
		if err := addGoFile(bucket, path); err != nil {
			return ActivityGo{}, err
		}
	}
	modules, readErr := os.ReadFile(filepath.Join(options.Repo, "vendor", "modules.txt"))
	if readErr == nil {
		for line := range strings.SplitSeq(string(modules), "\n") {
			if strings.HasPrefix(line, "# ") && !strings.HasPrefix(line, "##") {
				stats.VendorModules++
			}
		}
	} else if !os.IsNotExist(readErr) {
		return ActivityGo{}, readErr
	}
	return stats, nil
}

func addGoFile(bucket *ActivityGoBucket, path string) error {
	raw, err := os.ReadFile(path) //nolint:gosec // a rewrite tool reads the path the operator named
	if err != nil {
		return err
	}
	text := string(raw)
	if !utf8.Valid(raw) {
		text = strings.ToValidUTF8(text, "\uFFFD")
	}
	bucket.Files++
	inBlock := false
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(nil, len(text)+1)
	for scanner.Scan() {
		bucket.TotalLines++
		kind, nextBlock := classifyGoLine(scanner.Text(), inBlock)
		inBlock = nextBlock
		switch kind {
		case "blank":
			bucket.BlankLines++
		case lineComment:
			bucket.CommentLines++
		default:
			bucket.CodeLines++
		}
	}
	return scanner.Err()
}

func classifyGoLine(line string, inBlock bool) (string, bool) {
	cursor := strings.TrimSpace(line)
	if cursor == "" {
		return "blank", inBlock
	}
	for {
		if inBlock {
			end := strings.Index(cursor, "*/")
			if end < 0 {
				return lineComment, true
			}
			cursor = strings.TrimSpace(cursor[end+2:])
			inBlock = false
			if cursor == "" {
				return lineComment, false
			}
			continue
		}
		if strings.HasPrefix(cursor, "//") {
			return lineComment, false
		}
		if strings.HasPrefix(cursor, "/*") {
			end := strings.Index(cursor[2:], "*/")
			if end < 0 {
				return lineComment, true
			}
			cursor = strings.TrimSpace(cursor[end+4:])
			if cursor == "" {
				return lineComment, false
			}
			continue
		}
		return "code", false
	}
}

func renderActivityPage(options ActivityOptions, generated time.Time) (string, error) {
	window, err := measureActivity(options, generated)
	if err != nil {
		return "", err
	}
	lineSummary := summarizeActivity(window.Lines, "Total added lines", "Days with added lines", "Peak line day", "Top Added-Line Days", "Added lines", "Line thresholds")
	commitSummary := summarizeActivity(window.Commits, "Total commits", "Days with commits", "Peak commit day", "Top Commit Days", "Commits", "Commit thresholds")
	summaryJSON, _ := json.Marshal(map[string]activitySummary{"lines": lineSummary, "commits": commitSummary})

	filterLabel := "source files"
	if options.AllFiles {
		filterLabel = "all files"
	}
	author := options.Author
	if author == "" {
		author = "all authors"
	}
	return renderActivityHTML(&activityPageData{
		Repo: html.EscapeString(options.Repo), Ref: html.EscapeString(activityGitLabel(options)),
		Range:     window.Start.Format("2006-01-02") + " to " + window.End.Format("2006-01-02"),
		Generated: generated.Format("2006-01-02 15:04:05"), Filter: filterLabel, Author: html.EscapeString(author),
		WeekCount: window.Weeks, MonthLabels: window.MonthLabels, Cells: window.Cells, SummaryJSON: string(summaryJSON),
		LineSummary: lineSummary, CommitSummary: commitSummary,
		LineTop:   renderTopDays(window.Lines.daily, "No added source lines in this range."),
		CommitTop: renderTopDays(window.Commits.daily, "No commits in this range."), Stats: window.Go,
	}), nil
}

type activitySummary struct {
	TotalLabel     string `json:"totalLabel"`
	TotalValue     string `json:"totalValue"`
	ActiveLabel    string `json:"activeLabel"`
	ActiveValue    string `json:"activeValue"`
	PeakLabel      string `json:"peakLabel"`
	PeakValue      string `json:"peakValue"`
	TopHeading     string `json:"topHeading"`
	TopColumn      string `json:"topColumn"`
	ThresholdLabel string `json:"thresholdLabel"`
	ThresholdValue string `json:"thresholdValue"`
}

type activityPageData struct {
	Repo, Ref, Range, Generated, Filter, Author string
	WeekCount                                   int
	MonthLabels, Cells, SummaryJSON             string
	LineSummary, CommitSummary                  activitySummary
	LineTop, CommitTop                          string
	Stats                                       ActivityGo
}

// summarizeActivity labels one measured series for the standalone dashboard,
// which names the peak day inside its own label because it shows no date column
// beside it.
func summarizeActivity(series ActivitySeries, totalLabel, activeLabel, peakPrefix, topHeading, topColumn, thresholdLabel string) activitySummary {
	thresholdText := make([]string, len(series.Thresholds))
	for index, value := range series.Thresholds {
		thresholdText[index] = displayNumber(value)
	}
	return activitySummary{
		TotalLabel: totalLabel, TotalValue: displayNumber(series.Total),
		ActiveLabel: activeLabel, ActiveValue: displayNumber(series.ActiveDays),
		PeakLabel:  peakPrefix + " (" + series.PeakDay.Format("2006-01-02") + ")",
		PeakValue:  displayNumber(series.Peak),
		TopHeading: topHeading, TopColumn: topColumn,
		ThresholdLabel: thresholdLabel, ThresholdValue: strings.Join(thresholdText, ", "),
	}
}

func activityThresholds(values map[time.Time]int) []int {
	maximum := 0
	for _, value := range values {
		maximum = max(maximum, value)
	}
	if maximum <= 1 {
		return []int{0, 0, 0, 0}
	}
	thresholds := make([]int, 4)
	for step := range 4 {
		thresholds[step] = min(maximum-1, max(1, (maximum*(step+1)+4)/5))
	}
	return thresholds
}

func activityLevel(value int, thresholds []int) int {
	if value <= 0 {
		return 0
	}
	for index, threshold := range thresholds {
		if value <= threshold {
			return index + 1
		}
	}
	return 5
}

func dateOnly(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func sundayBefore(day time.Time) time.Time {
	return day.AddDate(0, 0, -int(day.Weekday()))
}

func saturdayAfter(day time.Time) time.Time {
	return day.AddDate(0, 0, 6-int(day.Weekday()))
}

func weeksBetween(start, end time.Time) [][]time.Time {
	weeks := make([][]time.Time, 0, int(end.Sub(start).Hours()/24/7)+1)
	for day := start; !day.After(end); day = day.AddDate(0, 0, 7) {
		week := make([]time.Time, 7)
		for offset := range 7 {
			week[offset] = day.AddDate(0, 0, offset)
		}
		weeks = append(weeks, week)
	}
	return weeks
}

func monthLabels(weeks [][]time.Time) string {
	seen := make(map[string]bool)
	lastColumn := -10
	var out textbuf.Buffer
	out.Reset()
	for column, week := range weeks {
		label := ""
		for _, day := range week {
			key := day.Format("2006-01")
			if day.Day() == 1 && !seen[key] {
				if column+1-lastColumn >= 4 {
					label, lastColumn = day.Format("Jan"), column+1
				}
				seen[key] = true
				break
			}
		}
		if column == 0 && label == "" {
			label, lastColumn = week[0].Format("Jan"), 1
			seen[week[0].Format("2006-01")] = true
		}
		out.Str(`<span class="month-label" style="grid-column:`).Int(int64(column + 1)).
			Str(`">`).Str(label).Str(`</span>` + "\n")
	}
	return out.String()
}

func renderTopDays(values map[time.Time]int, empty string) string {
	type row struct {
		day   time.Time
		value int
	}
	rows := make([]row, 0, len(values))
	for day, value := range values {
		rows = append(rows, row{day, value})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].value == rows[j].value {
			return rows[i].day.After(rows[j].day)
		}
		return rows[i].value > rows[j].value
	})
	if len(rows) == 0 {
		return `<tr><td colspan="2">` + html.EscapeString(empty) + `</td></tr>`
	}
	rows = rows[:min(10, len(rows))]
	var out textbuf.Buffer
	out.Reset()
	for _, item := range rows {
		out.Str("<tr><td>").Str(item.day.Format("Mon 02 Jan 2006")).
			Str("</td><td>").Str(displayNumber(item.value)).Str("</td></tr>\n")
	}
	return out.String()
}

func displayNumber(value int) string {
	text := strconv.Itoa(value)
	for index := len(text) - 3; index > 0; index -= 3 {
		text = text[:index] + "," + text[index:]
	}
	return text
}

func activityGitLabel(options ActivityOptions) string {
	if options.AllRefs {
		return "all refs"
	}
	branch, branchErr := runGit(options.Repo, false, "rev-parse", "--abbrev-ref", options.Ref)
	commit, commitErr := runGit(options.Repo, false, "rev-parse", "--short", options.Ref)
	if branchErr == nil && commitErr == nil {
		return strings.TrimSpace(branch) + " @ " + strings.TrimSpace(commit)
	}
	return options.Ref
}

func openActivityURL(url string) error {
	var name string
	var arguments []string
	switch runtime.GOOS {
	case "darwin":
		name, arguments = "open", []string{url}
	case "windows":
		name, arguments = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		name, arguments = "xdg-open", []string{url}
	}
	return exec.CommandContext(context.Background(), name, arguments...).Start() //nolint:gosec // fixed platform opener, with the URL as one uninterpreted argv
}

func renderGoCards(stats ActivityGoBucket) string {
	return fmt.Sprintf(`<div class="stat"><span>Files</span><strong>%s</strong></div>
<div class="stat"><span>Total lines</span><strong>%s</strong></div>
<div class="stat"><span>Code lines</span><strong>%s</strong></div>
<div class="stat"><span>Blank lines</span><strong>%s</strong></div>
<div class="stat"><span>Comment lines</span><strong>%s</strong></div>`, displayNumber(stats.Files), displayNumber(stats.TotalLines), displayNumber(stats.CodeLines), displayNumber(stats.BlankLines), displayNumber(stats.CommentLines))
}

func renderGoBucket(title string, stats ActivityGoBucket, note string) string {
	return `<div class="go-bucket"><h3>` + html.EscapeString(title) + `</h3><div class="go-stats">` + renderGoCards(stats) + `</div><p class="note">` + html.EscapeString(note) + `</p></div>`
}

func renderActivityHTML(data *activityPageData) string {
	goBuckets := renderGoBucket("All First-Party Go", data.Stats.Total, "Tracked .go files outside vendor/.") +
		renderGoBucket("Production Go", data.Stats.Code, "First-party tracked .go files excluding _test.go.") +
		renderGoBucket("Go Tests", data.Stats.Tests, "First-party tracked _test.go files.") +
		`<div class="go-bucket"><h3>Vendored Dependencies</h3><div class="go-stats">` + renderGoCards(data.Stats.Vendor) + `<div class="stat"><span>Modules</span><strong>` + displayNumber(data.Stats.VendorModules) + `</strong></div></div><p class="note">Tracked vendored .go files under vendor/. Module count comes from vendor/modules.txt.</p></div>`
	return fmt.Sprintf(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width, initial-scale=1"/><title>Source Line Activity</title>
<style>:root{color-scheme:dark;--bg:#071118;--panel:#0d1a24;--text:#e8f7fb;--muted:#8fb7c3;--cell:12px;--gap:3px;--level-0:#14232c;--level-1:#143d45;--level-2:#1d6869;--level-3:#299e91;--level-4:#48d2b2;--level-5:#bef86a}*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--text);font:14px system-ui,sans-serif}.page{max-width:1280px;margin:auto;padding:32px}.eyebrow,.note{color:var(--muted)}h1{font-size:34px}.panel,.go-bucket{background:var(--panel);border:1px solid #29404d;border-radius:14px;padding:20px;margin:16px 0}.toolbar{display:flex;gap:8px}.metric-button{padding:8px 14px}.metric-button.active{background:#48d2b2;color:#071118}.chart-wrap{overflow-x:auto}.chart{display:grid;grid-template-columns:repeat(%d,var(--cell));grid-template-rows:20px repeat(7,var(--cell));gap:var(--gap);min-width:calc(%d * (var(--cell) + var(--gap)))}.month-label{grid-row:1;color:var(--muted)}.day-cell{width:var(--cell);height:var(--cell);background:var(--level-0);border-radius:2px}.day-cell[data-level="1"]{background:var(--level-1)}.day-cell[data-level="2"]{background:var(--level-2)}.day-cell[data-level="3"]{background:var(--level-3)}.day-cell[data-level="4"]{background:var(--level-4)}.day-cell[data-level="5"]{background:var(--level-5)}.outside,.pre-repo{opacity:.25}.summary,.go-stats{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:10px}.stat{background:#10232e;padding:12px;border-radius:8px}.stat span{display:block;color:var(--muted)}.stat strong{font-size:22px}table{width:100%%;border-collapse:collapse}td,th{text-align:left;padding:8px;border-bottom:1px solid #29404d}</style></head>
<body><main class="page"><p class="eyebrow">Git activity and current Go source inventory</p><h1>Source Line Activity</h1><p>%s · %s · %s</p><p class="note">%s · %s · generated %s</p>
<section class="panel"><div class="toolbar"><button class="metric-button active" data-metric="lines">Added lines</button><button class="metric-button" data-metric="commits">Commits</button></div><div class="summary"><div class="stat"><span id="total-label">%s</span><strong id="total-value">%s</strong></div><div class="stat"><span id="active-label">%s</span><strong id="active-value">%s</strong></div><div class="stat"><span id="peak-label">%s</span><strong id="peak-value">%s</strong></div></div><div class="chart-wrap"><div class="chart">%s%s</div></div><p class="note"><span id="threshold-label">%s</span>: <span id="threshold-value">%s</span></p></section>
<section class="panel"><h2 id="top-heading">%s</h2><table><thead><tr><th>Date</th><th id="top-column">%s</th></tr></thead><tbody id="top-body">%s</tbody></table></section><section class="panel"><h2>Current Go Source Inventory</h2>%s</section></main>
<script>const summaries=%s;const topRows={lines:%q,commits:%q};const cells=[...document.querySelectorAll('.day-cell')];for(const button of document.querySelectorAll('.metric-button'))button.addEventListener('click',()=>{const metric=button.dataset.metric;document.querySelectorAll('.metric-button').forEach(b=>b.classList.toggle('active',b===button));for(const cell of cells){cell.dataset.level=cell.dataset[metric+'Level'];cell.setAttribute('aria-label',cell.dataset.dateLabel+': '+cell.dataset[metric+'Display']+' '+(metric==='lines'?'lines added':'commits'))}const s=summaries[metric];for(const key of ['total','active','peak','threshold']){document.getElementById(key+'-label').textContent=s[key+'Label'];document.getElementById(key+'-value').textContent=s[key+'Value']}document.getElementById('top-heading').textContent=s.topHeading;document.getElementById('top-column').textContent=s.topColumn;document.getElementById('top-body').innerHTML=topRows[metric]});</script></body></html>
`, data.WeekCount, data.WeekCount, data.Repo, data.Ref, data.Range, data.Filter, data.Author, data.Generated,
		data.LineSummary.TotalLabel, data.LineSummary.TotalValue, data.LineSummary.ActiveLabel, data.LineSummary.ActiveValue, data.LineSummary.PeakLabel, data.LineSummary.PeakValue,
		data.MonthLabels, data.Cells, data.LineSummary.ThresholdLabel, data.LineSummary.ThresholdValue, data.LineSummary.TopHeading, data.LineSummary.TopColumn, data.LineTop,
		goBuckets, data.SummaryJSON, data.LineTop, data.CommitTop)
}

// The Go line class this counter reports, and the parameter value name the
// command grammar prints.
const (
	lineComment = "comment"
	valuePath   = "path"
)
