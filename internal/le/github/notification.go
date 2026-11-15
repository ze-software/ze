// Design: docs/contributing/github-integration.md -- previews bind trusted evidence, exact templates and destination identities.
// Related: notification_delivery.go -- approved delivery and its receipts.
package github

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const notificationTextMax = 2000

// Exact words at GitHub, receipt and presentation boundaries. Internal
// transitions and evidence verdicts use their private numeric enums.
const (
	notificationWireFailure   = "failure"
	notificationWireRecovery  = "recovery"
	notificationWireBlocked   = "blocked"
	notificationWireCompleted = "completed"
	notificationWireSuccess   = "success"
	notificationWireTimedOut  = "timed_out"
	notificationWireQuiet     = "quiet"
	notificationWireDelivered = "delivered"
)

// notificationDestination is a validated presentation label and optional
// Discord identities. Empty identities permit preview only. Its zero value is
// invalid. No webhook token or URL belongs in this value or any receipt.
type notificationDestination struct{ label, channel, webhook string }

func newNotificationDestination(label, channel, webhook string) (notificationDestination, error) {
	if label == "" {
		return notificationDestination{}, errors.New("notification destination label is empty")
	}
	if len(label) > 80 {
		return notificationDestination{}, errors.New("notification destination label exceeds 80 bytes")
	}
	for _, r := range label {
		if notificationASCIIAlphanumeric(r) {
			continue
		}
		switch r {
		case '-', '_':
			continue
		}
		return notificationDestination{}, errors.New("notification destination label must contain only letters, digits, hyphens and underscores")
	}
	if channel == "" {
		if webhook != "" {
			return notificationDestination{}, errors.New("webhook identity requires a channel identity")
		}
	} else {
		if err := notificationSnowflake(channel); err != nil {
			return notificationDestination{}, fmt.Errorf("notification channel identity is invalid: %w", err)
		}
		if err := notificationSnowflake(webhook); err != nil {
			return notificationDestination{}, fmt.Errorf("notification webhook identity is invalid: %w", err)
		}
	}
	return notificationDestination{label: label, channel: channel, webhook: webhook}, nil
}

// notificationASCIIAlphanumeric keeps label and credential acceptance on the
// same exact ASCII ranges without accepting other Unicode letters or digits.
func notificationASCIIAlphanumeric(value rune) bool {
	if value >= 'a' {
		if value <= 'z' {
			return true
		}
	}
	if value >= 'A' {
		if value <= 'Z' {
			return true
		}
	}
	if value >= '0' {
		if value <= '9' {
			return true
		}
	}
	return false
}

func notificationSnowflake(value string) error {
	if value == "" {
		return errors.New("identity is empty")
	}
	if len(value) > 20 {
		return errors.New("identity exceeds the 20-digit bound")
	}
	if value[0] == '0' {
		return errors.New("identity must be positive without leading zeroes")
	}
	_, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		// NumError.Num is untrusted response data. Preserve its parse cause
		// without allowing a malformed identity to disclose credential bytes.
		if number, ok := errors.AsType[*strconv.NumError](err); ok {
			return fmt.Errorf("identity is not an unsigned decimal integer: %w", number.Err)
		}
		return fmt.Errorf("identity could not be parsed: %w", err)
	}
	return nil
}

// notificationTemplate is immutable after validation. Its zero value is invalid.
// Tokens are a closed set; an unsupported token cannot silently disappear.
type notificationTemplate struct{ failure, recovery string }

func newNotificationTemplate(failure, recovery string) (notificationTemplate, error) {
	for _, text := range []string{failure, recovery} {
		if text == "" {
			return notificationTemplate{}, errors.New("notification template is empty")
		}
		if len(text) > notificationTextMax {
			return notificationTemplate{}, errors.New("notification template exceeds the message bound")
		}
		if !utf8.ValidString(text) {
			return notificationTemplate{}, errors.New("notification template is not valid UTF-8")
		}
		for _, r := range text {
			if r == '\n' {
				continue
			}
			if unicode.IsControl(r) {
				return notificationTemplate{}, errors.New("notification template contains a control character")
			}
		}
		plain := strings.NewReplacer("{workflow}", "x", "{revision}", "x", "{run_url}", "x", "{attempt}", "x").Replace(text)
		if strings.ContainsAny(plain, "{}") {
			return notificationTemplate{}, errors.New("notification template contains an unknown token")
		}
	}
	return notificationTemplate{failure: failure, recovery: recovery}, nil
}

// candidateNotificationTemplate supplies unapproved text for review artifacts.
// Its presence grants no permission to send a message.
func candidateNotificationTemplate() (notificationTemplate, error) {
	return newNotificationTemplate("{workflow} failed at {revision} (attempt {attempt}).\nRun and logs: {run_url}",
		"{workflow} recovered at {revision} (attempt {attempt}) after its delivered failure.\nRun and evidence: {run_url}")
}

type notificationVerdict uint8

const (
	notificationUnspecified notificationVerdict = iota
	notificationUnjudged
	notificationFailed
	notificationPassed
)

// notificationCandidate is rendered review text, never a delivery receipt.
type notificationCandidate struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// notificationPreview is safe to retain as an artifact. ApprovalDigest binds
// the template bytes and destination identities, without containing a token.
type notificationPreview struct {
	DeliveryState      string                  `json:"delivery-state"`
	Repository         string                  `json:"repository"`
	Destination        string                  `json:"destination"`
	ChannelID          string                  `json:"channel-id"`
	WebhookID          string                  `json:"webhook-id"`
	ApprovalDigest     string                  `json:"approval-digest"`
	Run                *Run                    `json:"run"`
	Complete           bool                    `json:"complete"`
	Candidates         []notificationCandidate `json:"candidates"`
	Problems           []string                `json:"problems"`
	verdict            notificationVerdict
	fingerprint, scope string
	event              *notificationEvent
}

// notificationEvent is the immutable authority behind the presentation DTO.
// buildNotificationPreview owns its allocation and never exposes its pointer.
// Changing a preview's exported display fields cannot change what is sent.
type notificationEvent struct {
	runID, attempt, workflowID                       uint64
	workflowPath, binding, failureText, recoveryText string
	destination                                      notificationDestination
}

// buildNotificationPreview renders both candidate transitions from one
// identified trusted run. An incomplete or active result stays unjudged.
func buildNotificationPreview(report Report, template notificationTemplate, destination notificationDestination) (notificationPreview, error) {
	preview := notificationPreview{DeliveryState: "preview-only", Repository: Repository,
		Destination: destination.label, ChannelID: destination.channel, WebhookID: destination.webhook,
		Complete: report.Complete, Problems: slices.Clone(report.Problems), verdict: notificationUnjudged}
	if destination.label == "" {
		return preview, errors.New("notification destination was not constructed")
	}
	if template.failure == "" {
		return preview, errors.New("notification template was not constructed")
	}
	if report.Repository != Repository {
		return preview, errors.New("notification evidence names another repository")
	}
	if len(report.Runs) != 1 {
		return preview, errors.New("notification preview requires exactly one identified run")
	}
	run := report.Runs[0]
	if err := notificationTrust(run); err != nil {
		return preview, err
	}
	if report.RequestedRunID != run.ID {
		return preview, errors.New("notification run does not match the requested identity")
	}
	preview.Run = &run
	binding, err := notificationTemplateBinding(template, destination)
	if err != nil {
		return preview, err
	}
	preview.ApprovalDigest = binding
	preview.event = &notificationEvent{runID: run.ID, attempt: run.Attempt, workflowID: run.WorkflowID,
		workflowPath: run.WorkflowPath, binding: preview.ApprovalDigest, destination: destination}
	replace := strings.NewReplacer("{workflow}", run.Name, "{revision}", run.Revision,
		"{run_url}", run.URL, "{attempt}", strconv.FormatUint(run.Attempt, 10))
	for _, candidate := range []notificationCandidate{{Kind: notificationWireFailure, Text: template.failure}, {Kind: notificationWireRecovery, Text: template.recovery}} {
		candidate.Text = replace.Replace(candidate.Text)
		if len(candidate.Text) > notificationTextMax {
			return preview, errors.New("rendered notification exceeds the message bound")
		}
		preview.Candidates = append(preview.Candidates, candidate)
	}
	preview.event.failureText = preview.Candidates[0].Text
	preview.event.recoveryText = preview.Candidates[1].Text
	if report.Complete {
		preview.verdict, preview.scope, preview.fingerprint = notificationResult(run)
	}
	return preview, nil
}

// notificationTemplateBinding computes an identity for comparison with the
// operator's supplied approval. Computing it never issues that approval.
func notificationTemplateBinding(template notificationTemplate, destination notificationDestination) (string, error) {
	body, err := json.Marshal(struct {
		Version                                          int
		Failure, Recovery, Destination, Channel, Webhook string
	}{1, template.failure, template.recovery, destination.label, destination.channel, destination.webhook})
	if err != nil {
		return "", err
	}
	return notificationDigest(body), nil
}

func notificationTrust(run Run) error {
	if run.ID == 0 {
		return errors.New("notification run identity is missing")
	}
	if run.WorkflowID == 0 {
		return errors.New("notification workflow identity is missing")
	}
	if run.Attempt == 0 {
		return errors.New("notification attempt identity is missing")
	}
	if run.HeadRepository == nil {
		return errors.New("notification run has no head repository identity")
	}
	if *run.HeadRepository != Repository {
		return errors.New("notification run was not produced by the fixed repository")
	}
	if run.HeadBranch != mainBranch {
		return errors.New("notification run is not on the trusted main branch")
	}
	switch run.Event {
	case "push", "schedule", "workflow_dispatch":
	default:
		return errors.New("notification run event cannot authorize trusted maintainer evidence")
	}
	switch run.WorkflowPath {
	case ".github/workflows/verify.yml", ".github/workflows/evidence-nightly.yml", ".github/workflows/qemu-nightly.yml",
		".github/workflows/perf-nightly.yml", ".github/workflows/govulncheck.yml", ".github/workflows/codeql.yml":
	default:
		return errors.New("notification workflow is outside the reviewed evidence set")
	}
	if _, err := fullRevision(run.Revision); err != nil {
		return err
	}
	if run.URL != "https://github.com/"+Repository+"/actions/runs/"+strconv.FormatUint(run.ID, 10) {
		return errors.New("notification run link does not match its canonical identity")
	}
	if run.Name == "" {
		return errors.New("notification workflow name is empty")
	}
	if len(run.Name) > 160 {
		return errors.New("notification workflow name exceeds its bound")
	}
	return nil
}

func notificationDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func notificationResult(run Run) (notificationVerdict, string, string) {
	if run.Status != notificationWireCompleted {
		return notificationUnjudged, "", ""
	}
	if !run.JobsComplete {
		return notificationUnjudged, "", ""
	}
	if len(run.Jobs) == 0 {
		return notificationUnjudged, "", ""
	}
	var scope, failures []string
	allPassed := true
	for _, job := range run.Jobs {
		if job.Status != notificationWireCompleted {
			return notificationUnjudged, "", ""
		}
		scope = append(scope, "job:"+notificationListDigest([]string{job.Name}))
		if job.Conclusion != notificationWireSuccess {
			allPassed = false
		}
		// GitHub can mark a job successful after a continued-on-error step.
		// Recovery requires actual successful step evidence for every job.
		if len(job.Steps) == 0 {
			allPassed = false
		}
		for _, step := range job.Steps {
			scope = append(scope, "step:"+notificationListDigest([]string{job.Name, strconv.FormatUint(step.Number, 10), step.Name}))
			if step.Status != notificationWireCompleted {
				allPassed = false
			}
			if step.Conclusion != notificationWireSuccess {
				allPassed = false
			}
		}
		switch job.Conclusion {
		case notificationWireFailure, notificationWireTimedOut:
			failures = append(failures, notificationListDigest([]string{"job", job.Name, job.Conclusion}))
			for _, step := range job.Steps {
				switch step.Conclusion {
				case notificationWireFailure, notificationWireTimedOut:
					failures = append(failures, notificationListDigest([]string{"step", job.Name, strconv.FormatUint(step.Number, 10), step.Name, step.Conclusion}))
				}
			}
		}
	}
	slices.Sort(scope)
	slices.Sort(failures)
	scopeDigest := notificationListDigest(scope)
	switch run.Conclusion {
	case notificationWireSuccess:
		if !allPassed {
			return notificationUnjudged, "", ""
		}
		return notificationPassed, scopeDigest, ""
	case notificationWireFailure, notificationWireTimedOut:
		if len(failures) == 0 {
			return notificationUnjudged, "", ""
		}
		return notificationFailed, scopeDigest, notificationListDigest(append(failures, scopeDigest, run.Conclusion))
	default:
		return notificationUnjudged, "", ""
	}
}

func notificationListDigest(values []string) string {
	parts := make([]string, 0, 3*len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(len(value)), ":", value)
	}
	return notificationDigest([]byte(strings.Join(parts, "")))
}
