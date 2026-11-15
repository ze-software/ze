// Design: docs/contributing/github-integration.md -- output is evidence, not a certificate.

package github

// Repository is the only repository this development integration reads.
const Repository = "ze-software/ze"

// mainBranch is the fixed branch used by trusted evidence and local handoffs.
const mainBranch = "main"

// Report is an external presentation document. Complete means every requested
// API result was read and understood; it does not mean the tests passed.
type Report struct {
	Repository     string   `json:"repository"`
	Revision       string   `json:"revision"`
	RequestedRunID uint64   `json:"requested-run-id"`
	Complete       bool     `json:"complete"`
	Runs           []Run    `json:"runs"`
	Problems       []string `json:"problems"`
	Diagnostic     string   `json:"diagnostic"`
	Qualification  string   `json:"qualification"`
}

// Run preserves GitHub's workflow identity and its current attempt.
type Run struct {
	ID             uint64  `json:"id"`
	WorkflowID     uint64  `json:"workflow-id"`
	Name           string  `json:"name"`
	Revision       string  `json:"revision"`
	Attempt        uint64  `json:"attempt"`
	Event          string  `json:"event"`
	HeadBranch     string  `json:"head-branch"`
	WorkflowPath   string  `json:"workflow-path"`
	HeadRepository *string `json:"head-repository"`
	Status         string  `json:"status"`
	Conclusion     string  `json:"conclusion"`
	URL            string  `json:"url"`
	Jobs           []Job   `json:"jobs"`
	JobsComplete   bool    `json:"jobs-complete"`
}

// Job is one identified API job, including steps that did not complete.
type Job struct {
	ID         uint64 `json:"id"`
	RunID      uint64 `json:"run-id"`
	Attempt    uint64 `json:"attempt"`
	Revision   string `json:"revision"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	URL        string `json:"url"`
	Steps      []Step `json:"steps"`
}

// Step preserves the API's step number, name and verdict separately.
type Step struct {
	Number     uint64 `json:"number"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

// PagesDeployment identifies the newest github-pages deployment and its newest
// status. An empty or failed status is never replaced by an older success.
type PagesDeployment struct {
	ID             uint64 `json:"id"`
	StatusID       uint64 `json:"status-id"`
	SHA            string `json:"sha"`
	Ref            string `json:"ref"`
	Environment    string `json:"environment"`
	Status         string `json:"status"`
	EnvironmentURL string `json:"environment-url"`
	LogURL         string `json:"log-url"`
}
