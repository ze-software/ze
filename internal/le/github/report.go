// Design: docs/contributing/github-integration.md -- identity-qualified API evidence.

package github

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/ze-software/ze/internal/core/opererr"
)

const codeInput = "github-report-input"
const codeAPI = "github-report-api"
const reportQualification = "GitHub API observations only; not a native test or release certificate"

// These DTOs preserve GitHub's external JSON keys. They are unvalidated API
// input, never the report's own public schema or proof of an identified run.
type apiRun struct {
	ID             uint64         `json:"id"`
	WorkflowID     uint64         `json:"workflow_id"`
	Name           string         `json:"name"`
	HeadSHA        string         `json:"head_sha"`
	RunAttempt     uint64         `json:"run_attempt"`
	Event          string         `json:"event"`
	HeadBranch     string         `json:"head_branch"`
	Path           string         `json:"path"`
	Status         string         `json:"status"`
	Conclusion     *string        `json:"conclusion"`
	URL            string         `json:"url"`
	HTMLURL        string         `json:"html_url"`
	Repository     *apiRepository `json:"repository"`
	HeadRepository *apiRepository `json:"head_repository"`
}

type apiRepository struct {
	FullName string `json:"full_name"`
}

type apiJob struct {
	ID         uint64    `json:"id"`
	RunID      uint64    `json:"run_id"`
	RunAttempt uint64    `json:"run_attempt"`
	HeadSHA    string    `json:"head_sha"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Conclusion *string   `json:"conclusion"`
	URL        string    `json:"url"`
	HTMLURL    string    `json:"html_url"`
	Steps      []apiStep `json:"steps"`
}

type apiStep struct {
	Number     uint64  `json:"number"`
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	Conclusion *string `json:"conclusion"`
}

// reportRevision reads every bounded run page for one exact full commit ID,
// then the selected attempt's jobs. An API failure preserves qualified partial
// results and returns an error; Complete never means that tests passed.
func (client *Client) reportRevision(ctx context.Context, revision string) (Report, error) {
	report := newReport()
	canonical, err := fullRevision(revision)
	if err != nil {
		report.Diagnostic = codeInput
		return report.failed(err, "./le help github report revision")
	}
	report.Revision = canonical
	read, cancel, err := client.start(ctx)
	if err != nil {
		return report.failed(err, revisionCommand(canonical))
	}
	defer cancel()
	runs, err := read.runs(canonical)
	if err != nil {
		report.Problems = append(report.Problems, err.Error())
	}
	for index := range runs {
		report.addRun(read, runs[index], canonical, 0)
	}
	if len(runs) == 0 {
		report.Problems = append(report.Problems, "no GitHub workflow runs were reported for this revision")
	}
	return report.finish(revisionCommand(canonical))
}

// reportRun reads the run's own identity before using its ID and attempt to
// read jobs. It never substitutes a later attempt or a different head revision.
func (client *Client) reportRun(ctx context.Context, id uint64) (Report, error) {
	report := newReport()
	report.RequestedRunID = id
	if id == 0 {
		report.Diagnostic = codeInput
		return report.failed(fmt.Errorf("%w: run ID must be positive", errInput), "./le help github report run")
	}
	read, cancel, err := client.start(ctx)
	if err != nil {
		return report.failed(err, runCommand(id))
	}
	defer cancel()
	var raw apiRun
	if err := read.get("actions/runs/"+strconv.FormatUint(id, 10), nil, &raw); err != nil {
		return report.failed(err, runCommand(id))
	}
	report.Revision = raw.HeadSHA
	report.addRun(read, raw, "", id)
	return report.finish(runCommand(id))
}

func newReport() Report {
	return Report{Repository: Repository, Runs: []Run{}, Problems: []string{}, Qualification: reportQualification}
}

func revisionCommand(revision string) string { return "./le github report revision " + revision }
func runCommand(id uint64) string            { return "./le github report run " + strconv.FormatUint(id, 10) }

func (report Report) failed(err error, fix string) (Report, error) {
	report.Problems = append(report.Problems, err.Error())
	return report.finish(fix)
}

func (report Report) finish(fix string) (Report, error) {
	if len(report.Problems) == 0 {
		report.Complete = true
		return report, nil
	}
	if report.Diagnostic == "" {
		report.Diagnostic = codeAPI
	}
	report.Complete = false
	return report, opererr.New("GitHub evidence is incomplete",
		strings.Join(report.Problems, "; ")+". Known rows retain their own identities and do not certify the requested revision.", fix)
}

func (read *reach) runs(revision string) ([]apiRun, error) {
	rows := []apiRun{}
	total := -1
	seen := map[uint64]bool{}
	// Every collection is bounded to ten pages. Explicit page numbers avoid
	// following API-provided URLs or a looping Link chain.
	for page := 1; page <= pagesMax; page++ {
		query := pageQuery(page)
		query.Set("head_sha", revision)
		var response struct {
			Total *int     `json:"total_count"`
			Runs  []apiRun `json:"workflow_runs"`
		}
		if err := read.get("actions/runs", query, &response); err != nil {
			return rows, err
		}
		if response.Runs == nil {
			return rows, errors.New("GitHub response omitted its workflow_runs array")
		}
		count, err := collectionCount(response.Total, total, len(response.Runs), len(rows))
		if err != nil {
			return rows, err
		}
		total = count
		for index := range response.Runs {
			if seen[response.Runs[index].ID] {
				return rows, errors.New("GitHub run pagination repeated a run ID")
			}
			seen[response.Runs[index].ID] = true
			rows = append(rows, response.Runs[index])
		}
		if len(rows) == total {
			return rows, nil
		}
	}
	return rows, errors.New("GitHub run pagination exceeded its page bound")
}

func collectionCount(reported *int, previous, pageRows, held int) (int, error) {
	if reported == nil {
		return 0, errors.New("GitHub collection omitted total_count")
	}
	if *reported < 0 {
		return 0, errors.New("GitHub collection stated a negative count")
	}
	if *reported > pagesMax*pageEntries {
		return 0, errors.New("GitHub collection exceeds the bounded page population")
	}
	if previous >= 0 {
		if *reported != previous {
			return 0, errors.New("GitHub collection changed during pagination")
		}
	}
	if pageRows > pageEntries {
		return 0, errors.New("GitHub collection exceeded the requested page size")
	}
	if held+pageRows > *reported {
		return 0, errors.New("GitHub collection returned more rows than its count")
	}
	if pageRows == 0 {
		if held < *reported {
			return 0, errors.New("GitHub collection ended before every reported row was read")
		}
	}
	return *reported, nil
}

func (report *Report) addRun(read *reach, raw apiRun, revision string, requested uint64) {
	run := Run{ID: raw.ID, WorkflowID: raw.WorkflowID, Name: readable(raw.Name), Revision: readable(raw.HeadSHA),
		Attempt: raw.RunAttempt, Event: readable(raw.Event), HeadBranch: readable(raw.HeadBranch), WorkflowPath: readable(raw.Path),
		Status: readable(raw.Status), Conclusion: conclusionText(raw.Conclusion), Jobs: []Job{}}
	if raw.HeadRepository != nil {
		head := readable(raw.HeadRepository.FullName)
		run.HeadRepository = &head
	}
	problems := runIdentity(raw, revision, requested)
	if len(problems) == 0 {
		run.URL = "https://github.com/" + Repository + "/actions/runs/" + strconv.FormatUint(raw.ID, 10)
		problems = append(problems, read.client.stateProblems(raw.Status, raw.Conclusion)...)
		jobs, err := read.jobs(raw)
		run.Jobs = jobs
		if err != nil {
			problems = append(problems, err.Error())
		}
		run.JobsComplete = err == nil
	}
	for _, problem := range problems {
		report.Problems = append(report.Problems, "run "+strconv.FormatUint(raw.ID, 10)+": "+problem)
	}
	report.Runs = append(report.Runs, run)
}

func runIdentity(raw apiRun, revision string, requested uint64) []string {
	problems := []string{}
	if problem := nameProblem("run", raw.Name); problem != "" {
		problems = append(problems, problem)
	}
	if raw.ID == 0 {
		problems = append(problems, "API omitted a positive run ID")
	}
	if requested != 0 {
		if raw.ID != requested {
			problems = append(problems, "API returned a different run ID")
		}
	}
	canonical, err := fullRevision(raw.HeadSHA)
	if err != nil {
		problems = append(problems, "API returned no full head commit ID")
	}
	if revision != "" {
		if canonical != revision {
			problems = append(problems, "API head commit differs from the requested revision")
		}
	}
	if raw.WorkflowID == 0 {
		problems = append(problems, "API omitted the workflow ID")
	}
	if raw.RunAttempt == 0 {
		problems = append(problems, "API omitted the run attempt")
	}
	if raw.Repository != nil {
		if raw.Repository.FullName != Repository {
			problems = append(problems, "API hosting repository differs from "+Repository)
		}
	}
	if raw.URL != "" {
		if raw.URL != apiRoot+"actions/runs/"+strconv.FormatUint(raw.ID, 10) {
			problems = append(problems, "API run URL has a different identity")
		}
	}
	if raw.HTMLURL != "" {
		if raw.HTMLURL != "https://github.com/"+Repository+"/actions/runs/"+strconv.FormatUint(raw.ID, 10) {
			problems = append(problems, "workflow link has a different identity")
		}
	}
	return problems
}

func (read *reach) jobs(run apiRun) ([]Job, error) {
	rows := []Job{}
	total := -1
	seen := map[uint64]bool{}
	path := "actions/runs/" + strconv.FormatUint(run.ID, 10) + "/attempts/" + strconv.FormatUint(run.RunAttempt, 10) + "/jobs"
	for page := 1; page <= pagesMax; page++ {
		var response struct {
			Total *int     `json:"total_count"`
			Jobs  []apiJob `json:"jobs"`
		}
		if err := read.get(path, pageQuery(page), &response); err != nil {
			return rows, err
		}
		if response.Jobs == nil {
			return rows, errors.New("GitHub response omitted its jobs array")
		}
		count, err := collectionCount(response.Total, total, len(response.Jobs), len(rows))
		if err != nil {
			return rows, err
		}
		total = count
		for _, raw := range response.Jobs {
			if seen[raw.ID] {
				return rows, errors.New("GitHub job pagination repeated a job ID")
			}
			seen[raw.ID] = true
			row := Job{ID: raw.ID, RunID: raw.RunID, Attempt: raw.RunAttempt, Revision: readable(raw.HeadSHA),
				Name: readable(raw.Name), Status: readable(raw.Status), Conclusion: conclusionText(raw.Conclusion), Steps: []Step{}}
			rows = append(rows, row)
			job := &rows[len(rows)-1]
			if problem := nameProblem("job", raw.Name); problem != "" {
				return rows, errors.New(problem)
			}
			if raw.ID == 0 {
				return rows, errors.New("API omitted a positive job ID")
			}
			if raw.RunID != run.ID {
				return rows, errors.New("job belongs to a different run")
			}
			if raw.RunAttempt != run.RunAttempt {
				return rows, errors.New("job belongs to a different attempt")
			}
			if raw.HeadSHA != run.HeadSHA {
				return rows, errors.New("job belongs to a different head commit")
			}
			job.URL = "https://github.com/" + Repository + "/actions/runs/" + strconv.FormatUint(run.ID, 10) + "/job/" + strconv.FormatUint(raw.ID, 10)
			if raw.URL != "" {
				if raw.URL != apiRoot+"actions/jobs/"+strconv.FormatUint(raw.ID, 10) {
					return rows, errors.New("job API URL has a different identity")
				}
			}
			if raw.HTMLURL != "" {
				if raw.HTMLURL != job.URL {
					if raw.HTMLURL != "https://github.com/"+Repository+"/runs/"+strconv.FormatUint(raw.ID, 10) {
						return rows, errors.New("job link has a different identity")
					}
				}
			}
			if problems := read.client.stateProblems(raw.Status, raw.Conclusion); len(problems) != 0 {
				return rows, errors.New(strings.Join(problems, "; "))
			}
			if len(raw.Steps) > 1000 {
				return rows, errors.New("job exceeded its step bound")
			}
			steps := map[uint64]bool{}
			for _, step := range raw.Steps {
				job.Steps = append(job.Steps, Step{Number: step.Number, Name: readable(step.Name), Status: readable(step.Status), Conclusion: conclusionText(step.Conclusion)})
				if problem := nameProblem("step", step.Name); problem != "" {
					return rows, errors.New(problem)
				}
				if step.Number == 0 {
					return rows, errors.New("job step omitted its positive number")
				}
				if steps[step.Number] {
					return rows, errors.New("job repeated a step number")
				}
				steps[step.Number] = true
				if problems := read.client.stateProblems(step.Status, step.Conclusion); len(problems) != 0 {
					return rows, errors.New(strings.Join(problems, "; "))
				}
			}
		}
		if len(rows) == total {
			return rows, nil
		}
	}
	return rows, errors.New("GitHub job pagination exceeded its page bound")
}

func (client *Client) stateProblems(status string, conclusion *string) []string {
	class, known := client.states.Status[status]
	if !known {
		return []string{"unknown API status " + strconv.Quote(readable(status))}
	}
	if conclusion == nil {
		if class == stateFinished {
			return []string{"completed API result has no conclusion"}
		}
		return nil
	}
	if class == stateActive {
		return []string{"unfinished API result already carries a conclusion"}
	}
	if !slices.Contains(client.states.Conclusion, *conclusion) {
		return []string{"unknown API conclusion " + strconv.Quote(readable(*conclusion))}
	}
	return nil
}

func conclusionText(value *string) string {
	if value == nil {
		return ""
	}
	return readable(*value)
}

func readable(value string) string {
	if len(value) > 1024 {
		return "[API value exceeds 1024 bytes]"
	}
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return strconv.Quote(value)
	}
	return value
}

func nameProblem(kind, value string) string {
	if value == "" {
		return kind + " omitted its name"
	}
	if len(value) > 1024 {
		return kind + " name exceeded the text bound"
	}
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return kind + " name contains control characters"
	}
	return ""
}
