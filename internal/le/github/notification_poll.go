// Design: docs/contributing/github-notifier.md -- one bounded poll shares the durable one-shot delivery path.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

const notificationPollWorkflowsMax = 6

type notificationPollFile struct {
	Version           uint8    `json:"version"`
	StartRunID        uint64   `json:"start-run-id"`
	ActivationConfigs []string `json:"activation-configs"`
}

// notificationPoll is validated operator authority. Its zero value is invalid;
// loadNotificationPoll alone supplies its immutable workflow scopes.
type notificationPoll struct {
	start  uint64
	scopes []notificationPollScope
}

type notificationPollScope struct {
	activation             notificationActivation
	workflowID             uint64
	workflowPath           string
	scanRunID, scanAttempt uint64
}

type notificationPollReport struct {
	State      string                       `json:"state"`
	StartRunID uint64                       `json:"start-run-id"`
	Results    []notificationDeliveryReport `json:"results"`
	Problems   []string                     `json:"problems"`
}

func notificationPollActions(transport http.RoundTripper) []leaction.Action {
	return []leaction.Action{{Verb: "poll", Summary: "catch up approved workflow notifications once",
		Why:        "Read bounded stable main-branch runs for each approved workflow, then process them oldest first. Existing private receipts govern delivery and independent scan progress. This action installs no service and initializes no state.",
		Parameters: []leaction.Parameter{{Keyword: "config", Kind: leaction.KindPath, Value: "private-file", Requirement: leaction.Required, Help: "the private poll file naming initialized approvals"}},
		AnswerArgs: func(args leaction.Arguments) (any, int) {
			poll, err := loadNotificationPoll(args.One("config"))
			if err != nil {
				return notificationPollError(notificationPollReport{}, err)
			}
			client, err := NewClient(transport)
			if err != nil {
				return notificationPollError(notificationPollReport{}, err)
			}
			report, err := poll.run(context.Background(), client, transport)
			if err != nil {
				return notificationPollError(report, err)
			}
			return report, 0
		}}}
}

func notificationPollError(report notificationPollReport, err error) (any, int) {
	report.State = "incomplete"
	leaction.ReportError(opererr.Wrap(err, "GitHub notification polling could not finish",
		"Keep private receipts and resolve the reported workflow or API problem before the next scheduled poll.", "./le help github poll"))
	return report, 1
}

func loadNotificationPoll(path string) (notificationPoll, error) {
	body, err := readNotificationPrivateFile(path, notificationConfigBytesMax)
	if err != nil {
		return notificationPoll{}, err
	}
	if !utf8.Valid(body) {
		return notificationPoll{}, errors.New("notification poll configuration is not valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var raw notificationPollFile
	if err := decoder.Decode(&raw); err != nil {
		return notificationPoll{}, errors.New("notification poll configuration is not recognized JSON")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return notificationPoll{}, errors.New("notification poll configuration contains trailing data")
	}
	if raw.Version != 1 {
		return notificationPoll{}, errors.New("notification poll configuration version is unknown")
	}
	if raw.StartRunID == 0 {
		return notificationPoll{}, errors.New("notification poll start-run-id must be positive and explicit")
	}
	if len(raw.ActivationConfigs) == 0 {
		return notificationPoll{}, errors.New("notification poll configuration names no workflow approval files")
	}
	if len(raw.ActivationConfigs) > notificationPollWorkflowsMax {
		return notificationPoll{}, errors.New("notification poll configuration exceeds six workflows")
	}
	poll := notificationPoll{start: raw.StartRunID, scopes: make([]notificationPollScope, 0, len(raw.ActivationConfigs))}
	paths := map[string]bool{}
	workflows := map[uint64]bool{}
	stores := map[string]bool{}
	for _, config := range raw.ActivationConfigs {
		if paths[config] {
			return notificationPoll{}, errors.New("notification poll configuration repeats an approval file")
		}
		paths[config] = true
		activation, err := loadNotificationActivation(config)
		if err != nil {
			return notificationPoll{}, err
		}
		if stores[activation.stateDirectory] {
			return notificationPoll{}, errors.New("notification workflows must not share a receipt directory")
		}
		stores[activation.stateDirectory] = true
		scope, err := notificationPollLoadScope(activation)
		if err != nil {
			return notificationPoll{}, err
		}
		if workflows[scope.workflowID] {
			return notificationPoll{}, errors.New("notification poll repeats a workflow identity")
		}
		workflows[scope.workflowID] = true
		poll.scopes = append(poll.scopes, scope)
	}
	return poll, nil
}

func notificationPollLoadScope(activation notificationActivation) (scope notificationPollScope, result error) {
	store, err := newFileNotificationStore(activation.stateDirectory)
	if err != nil {
		return scope, err
	}
	defer func() { result = errors.Join(result, store.Close()) }()
	result = store.withLock(context.Background(), func(ledger notificationLedger) error {
		ready, err := notificationPollReady(ledger, activation)
		if err != nil {
			return err
		}
		scope = notificationPollScope{activation: activation, workflowID: ready.state.workflowID,
			workflowPath: ready.state.workflowPath, scanRunID: ready.state.scanRunID, scanAttempt: ready.state.scanAttempt}
		return nil
	})
	return scope, result
}

func notificationPollReady(ledger notificationLedger, activation notificationActivation) (notificationReadyReceipt, error) {
	raw, err := ledger.load()
	if err != nil {
		return notificationReadyReceipt{}, err
	}
	state, err := decodeNotificationReceipt(raw)
	if err != nil {
		return notificationReadyReceipt{}, err
	}
	ready, ok := state.(notificationReadyReceipt)
	if !ok {
		return notificationReadyReceipt{}, errors.New("notification workflow has an unresolved delivery; reconcile it before polling")
	}
	if ready.state.binding != activation.approval.BindingDigest {
		return notificationReadyReceipt{}, errors.New("notification poll approval differs from its receipt binding")
	}
	return ready, nil
}

func (poll notificationPoll) run(ctx context.Context, client *Client, transport http.RoundTripper) (notificationPollReport, error) {
	report := notificationPollReport{State: "complete", StartRunID: poll.start, Results: []notificationDeliveryReport{}, Problems: []string{}}
	read, cancel, err := client.start(ctx)
	if err != nil {
		return report, err
	}
	defer cancel()
	runs, err := poll.discover(read)
	if err != nil {
		return report, err
	}
	// Discovery is complete before this loop. Each workflow's oldest unresolved
	// event stops only that workflow; all work shares read's operation budget.
	for scopeIndex := range poll.scopes {
		scope := &poll.scopes[scopeIndex]
		for runIndex := range runs {
			run := &runs[runIndex]
			if run.WorkflowID != scope.workflowID {
				continue
			}
			if run.ID < poll.start {
				continue
			}
			if notificationPollScanned(run.ID, run.RunAttempt, scope.scanRunID, scope.scanAttempt) {
				continue
			}
			result, err := scope.process(read, *run, transport)
			report.Results = append(report.Results, result)
			if err != nil {
				report.Problems = append(report.Problems, scope.workflowPath+": "+err.Error())
				break
			}
		}
	}
	if len(report.Problems) != 0 {
		return report, errors.New("one or more workflows could not complete notification catch-up")
	}
	return report, nil
}

func notificationPollScanned(id, attempt, scanID, scanAttempt uint64) bool {
	if id < scanID {
		return true
	}
	if id == scanID {
		return attempt <= scanAttempt
	}
	return false
}

func (scope notificationPollScope) process(read *reach, discovered apiRun, transport http.RoundTripper) (report notificationDeliveryReport, result error) {
	report = notificationDeliveryReport{State: notificationWireBlocked, Destination: scope.activation.destination.label,
		RunID: discovered.ID, Attempt: discovered.RunAttempt}
	store, err := newFileNotificationStore(scope.activation.stateDirectory)
	if err != nil {
		return report, err
	}
	defer func() { result = errors.Join(result, store.Close()) }()
	result = store.withLock(read.ctx, func(ledger notificationLedger) error {
		ready, err := notificationPollReady(ledger, scope.activation)
		if err != nil {
			return err
		}
		if ready.state.workflowID != scope.workflowID {
			return errors.New("notification workflow changed after discovery")
		}
		if ready.state.workflowPath != scope.workflowPath {
			return errors.New("notification workflow path changed after discovery")
		}
		if ready.state.scanRunID < scope.scanRunID {
			return errors.New("notification scan history moved backwards after discovery")
		}
		if notificationPollScanned(discovered.ID, discovered.RunAttempt, ready.state.scanRunID, ready.state.scanAttempt) {
			report.State = notificationWireQuiet
			return nil
		}
		preview, err := scope.readPreview(read, discovered)
		if err != nil {
			return err
		}
		if err := notificationPollTerminal(preview); err != nil {
			return err
		}
		if preview.verdict == notificationUnjudged {
			report.State, report.Reason = notificationWireQuiet, "completed nonactionable result; delivered failure history retained"
			return ready.scan(ledger, discovered.ID, discovered.RunAttempt)
		}
		credential, err := readNotificationPrivateFile(scope.activation.credentialFile, 2048)
		if err != nil {
			return err
		}
		sender, err := newDiscordNotificationSender(string(credential), scope.activation.destination, transport)
		if err != nil {
			return err
		}
		report, err = deliverNotification(read.ctx, notificationLockedStore{ledger: ledger}, preview, scope.activation.approval, sender)
		if err != nil {
			return err
		}
		if report.State != notificationWireQuiet {
			if report.State != notificationWireDelivered {
				return errors.New("notification delivery was refused; its event will not be resent automatically")
			}
		}
		ready, err = notificationPollReady(ledger, scope.activation)
		if err != nil {
			return err
		}
		return ready.scan(ledger, discovered.ID, discovered.RunAttempt)
	})
	if result != nil {
		report.Reason = result.Error()
	}
	return report, result
}

// notificationLockedStore borrows an existing exclusive ledger. Callers MUST
// retain its file-store lock until delivery and scan persistence both return.
type notificationLockedStore struct{ ledger notificationLedger }

func (store notificationLockedStore) withLock(ctx context.Context, use func(notificationLedger) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return use(store.ledger)
}

func (ready notificationReadyReceipt) scan(ledger notificationLedger, id, attempt uint64) error {
	if notificationPollScanned(id, attempt, ready.state.scanRunID, ready.state.scanAttempt) {
		return errors.New("notification scan would repeat or reverse history")
	}
	state := ready.state
	state.scanRunID, state.scanAttempt = id, attempt
	return ledger.save(notificationReadyReceipt{state: state}.encode())
}

func (scope notificationPollScope) readPreview(read *reach, discovered apiRun) (notificationPreview, error) {
	var raw apiRun
	if err := read.get("actions/runs/"+strconv.FormatUint(discovered.ID, 10), nil, &raw); err != nil {
		return notificationPreview{}, err
	}
	if raw.RunAttempt != discovered.RunAttempt {
		return notificationPreview{}, errors.New("notification run attempt changed after discovery")
	}
	if raw.WorkflowID != discovered.WorkflowID {
		return notificationPreview{}, errors.New("notification workflow identity changed after discovery")
	}
	if raw.Path != discovered.Path {
		return notificationPreview{}, errors.New("notification workflow path changed after discovery")
	}
	if raw.HeadSHA != discovered.HeadSHA {
		return notificationPreview{}, errors.New("notification source revision changed after discovery")
	}
	report := newReport()
	report.RequestedRunID = discovered.ID
	report.addRun(read, raw, "", discovered.ID)
	report, err := report.finish("./le help github poll")
	if err != nil {
		return notificationPreview{}, err
	}
	preview, err := buildNotificationPreview(report, scope.activation.template, scope.activation.destination)
	if err != nil {
		return notificationPreview{}, err
	}
	if err := checkNotificationApproval(preview, scope.activation.approval); err != nil {
		return notificationPreview{}, err
	}
	return preview, nil
}

func notificationPollTerminal(preview notificationPreview) error {
	if !preview.Complete {
		return errors.New("notification run evidence is incomplete")
	}
	if preview.Run.Status != notificationWireCompleted {
		return errors.New("notification earlier run is still active")
	}
	for _, job := range preview.Run.Jobs {
		if job.Status != notificationWireCompleted {
			return errors.New("notification earlier job is still active")
		}
		for _, step := range job.Steps {
			if step.Status != notificationWireCompleted {
				return errors.New("notification earlier step is still active")
			}
		}
	}
	if preview.verdict == notificationUnjudged {
		switch preview.Run.Conclusion {
		case notificationWireFailure, notificationWireTimedOut:
			return errors.New("notification failure has insufficient job evidence")
		}
	}
	return nil
}

// discover reads each workflow's bounded suffix independently. A quiet
// workflow never inherits the unrelated repository population as a backlog.
// Every needed snapshot recheck completes before any sender is constructed.
func (poll notificationPoll) discover(read *reach) ([]apiRun, error) {
	rows := []apiRun{}
	snapshots := []notificationPollSnapshot{}
	identities := map[uint64]bool{}
	for scopeIndex := range poll.scopes {
		scope := &poll.scopes[scopeIndex]
		listed, first, err := scope.discover(read, max(poll.start, scope.scanRunID))
		if err != nil {
			return nil, err
		}
		for index := range listed {
			raw := &listed[index]
			if identities[raw.ID] {
				return nil, errors.New("notification workflow feeds repeat a run identity")
			}
			identities[raw.ID] = true
		}
		rows = append(rows, listed...)
		for index := range listed {
			raw := &listed[index]
			if raw.ID < poll.start {
				continue
			}
			if notificationPollScanned(raw.ID, raw.RunAttempt, scope.scanRunID, scope.scanAttempt) {
				continue
			}
			snapshots = append(snapshots, notificationPollSnapshot{workflowID: scope.workflowID, page: first})
			break
		}
	}
	for _, snapshot := range snapshots {
		check, err := notificationPollReadPage(read, snapshot.workflowID, 1)
		if err != nil {
			return nil, err
		}
		before, err := json.Marshal(snapshot.page)
		if err != nil {
			return nil, err
		}
		after, err := json.Marshal(check)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(before, after) {
			return nil, errors.New("notification run listing changed before delivery")
		}
	}
	return poll.selectRuns(rows)
}

type notificationPollSnapshot struct {
	workflowID uint64
	page       notificationPollPage
}

func (scope notificationPollScope) discover(read *reach, floor uint64) ([]apiRun, notificationPollPage, error) {
	rows := []apiRun{}
	seen := map[uint64]bool{}
	total, held := -1, 0
	var first notificationPollPage
	for page := 1; page <= pagesMax; page++ {
		response, err := notificationPollReadPage(read, scope.workflowID, page)
		if err != nil {
			return nil, first, err
		}
		if response.Total == nil {
			return nil, first, errors.New("notification run listing omitted total_count")
		}
		if *response.Total < 0 {
			return nil, first, errors.New("notification run listing has a negative count")
		}
		if total >= 0 {
			if *response.Total != total {
				return nil, first, errors.New("notification run listing changed during pagination")
			}
		}
		total = *response.Total
		if response.Runs == nil {
			return nil, first, errors.New("notification run listing omitted its runs")
		}
		if len(response.Runs) > pageEntries {
			return nil, first, errors.New("notification run page exceeds its bound")
		}
		held += len(response.Runs)
		if held > total {
			return nil, first, errors.New("notification run listing exceeds its count")
		}
		if page == 1 {
			first = response
		}
		covered := false
		for index := range response.Runs {
			raw := &response.Runs[index]
			if seen[raw.ID] {
				return nil, first, errors.New("notification run listing repeats an identity")
			}
			seen[raw.ID] = true
			if len(rows) != 0 {
				if raw.ID >= rows[len(rows)-1].ID {
					return nil, first, errors.New("notification run listing is not newest first")
				}
			}
			if problems := runIdentity(*raw, "", 0); len(problems) != 0 {
				return nil, first, errors.New("notification run listing contains an invalid identity")
			}
			if problems := read.client.stateProblems(raw.Status, raw.Conclusion); len(problems) != 0 {
				return nil, first, errors.New("notification run listing contains an unknown or inconsistent state")
			}
			if raw.HeadBranch != mainBranch {
				return nil, first, errors.New("notification run listing returned another branch")
			}
			if raw.WorkflowID != scope.workflowID {
				return nil, first, errors.New("notification workflow feed returned another workflow identity")
			}
			if raw.Path != scope.workflowPath {
				return nil, first, errors.New("notification workflow feed returned another workflow path")
			}
			rows = append(rows, *raw)
			if raw.ID <= floor {
				covered = true
			}
		}
		if covered {
			return rows, first, nil
		}
		if held == total {
			return rows, first, nil
		}
		if len(response.Runs) == 0 {
			return nil, first, errors.New("notification run listing ended before its scan boundary")
		}
	}
	return nil, first, errors.New("notification catch-up exceeded its page bound; no messages were sent")
}

type notificationPollPage struct {
	Total *int     `json:"total_count"`
	Runs  []apiRun `json:"workflow_runs"`
}

func notificationPollReadPage(read *reach, workflowID uint64, page int) (notificationPollPage, error) {
	query := pageQuery(page)
	query.Set("branch", mainBranch)
	var response notificationPollPage
	err := read.get("actions/workflows/"+strconv.FormatUint(workflowID, 10)+"/runs", query, &response)
	return response, err
}

func (poll notificationPoll) selectRuns(rows []apiRun) ([]apiRun, error) {
	selected := []apiRun{}
	for rowIndex := range rows {
		raw := &rows[rowIndex]
		for scopeIndex := range poll.scopes {
			scope := &poll.scopes[scopeIndex]
			if raw.WorkflowID != scope.workflowID {
				if raw.Path == scope.workflowPath {
					return nil, errors.New("notification run listing replaced an approved workflow identity")
				}
				continue
			}
			if raw.Path != scope.workflowPath {
				return nil, errors.New("notification run listing changed an approved workflow path")
			}
			switch raw.Event {
			case "push", "schedule", "workflow_dispatch":
			default:
				continue
			}
			report := Run{ID: raw.ID, WorkflowID: raw.WorkflowID, Name: raw.Name, Revision: raw.HeadSHA,
				Attempt: raw.RunAttempt, HeadBranch: raw.HeadBranch, WorkflowPath: raw.Path, Event: raw.Event,
				URL: fmt.Sprintf("https://github.com/%s/actions/runs/%d", Repository, raw.ID)}
			if raw.HeadRepository != nil {
				report.HeadRepository = &raw.HeadRepository.FullName
			}
			if err := notificationTrust(report); err != nil {
				return nil, err
			}
			selected = append(selected, *raw)
		}
	}
	slices.Reverse(selected)
	return selected, nil
}
