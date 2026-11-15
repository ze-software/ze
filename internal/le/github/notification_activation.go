// Design: docs/contributing/github-integration.md -- a private operator-provisioned file supplies exact notification approval.
// Related: notification_activation_actions.go -- explicit initialization and one-shot delivery.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"unicode/utf8"
)

const notificationConfigBytesMax = 64 << 10

// notificationActivation is immutable operator authority after private-file
// validation. Its zero value is invalid. It contains no webhook credential.
// It is safe for concurrent use; the receipt store serializes delivery.
type notificationActivation struct {
	template                       notificationTemplate
	destination                    notificationDestination
	approval                       notificationApproval
	credentialFile, stateDirectory string
}

// notificationActivationFile is untrusted boundary data until its file and
// exact binding are checked. It never issues or computes an approval field.
type notificationActivationFile struct {
	Version          uint8  `json:"version"`
	Destination      string `json:"destination"`
	ChannelID        string `json:"channel-id"`
	WebhookID        string `json:"webhook-id"`
	FailureTemplate  string `json:"failure-template"`
	RecoveryTemplate string `json:"recovery-template"`
	ApprovalDigest   string `json:"approval-digest"`
	CredentialFile   string `json:"credential-file"`
	StateDirectory   string `json:"state-directory"`
}

func loadNotificationActivation(path string) (notificationActivation, error) {
	body, err := readNotificationPrivateFile(path, notificationConfigBytesMax)
	if err != nil {
		return notificationActivation{}, err
	}
	if !utf8.Valid(body) {
		return notificationActivation{}, errors.New("notification configuration is not valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var raw notificationActivationFile
	if err := decoder.Decode(&raw); err != nil {
		return notificationActivation{}, errors.New("notification configuration is not a recognized complete JSON document")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return notificationActivation{}, errors.New("notification configuration contains trailing or unreadable data")
	}
	if raw.Version != 1 {
		return notificationActivation{}, errors.New("notification configuration version is unknown")
	}
	template, err := newNotificationTemplate(raw.FailureTemplate, raw.RecoveryTemplate)
	if err != nil {
		return notificationActivation{}, err
	}
	destination, err := newNotificationDestination(raw.Destination, raw.ChannelID, raw.WebhookID)
	if err != nil {
		return notificationActivation{}, err
	}
	if destination.channel == "" {
		return notificationActivation{}, errors.New("notification configuration needs the approved channel and webhook identities")
	}
	if err := notificationHash(raw.ApprovalDigest); err != nil {
		return notificationActivation{}, errors.New("notification configuration has no valid exact approval digest")
	}
	binding, err := notificationTemplateBinding(template, destination)
	if err != nil {
		return notificationActivation{}, err
	}
	if raw.ApprovalDigest != binding {
		return notificationActivation{}, errors.New("notification configuration differs from its exact template and destination approval")
	}
	if err := notificationPrivatePath(raw.CredentialFile); err != nil {
		return notificationActivation{}, errors.New("notification credential-file must be an absolute clean file path")
	}
	if err := notificationPrivatePath(raw.StateDirectory); err != nil {
		return notificationActivation{}, errors.New("notification state-directory must be an absolute clean directory path")
	}
	state, err := os.Lstat(raw.StateDirectory)
	if err != nil {
		return notificationActivation{}, errors.New("notification state-directory must already exist")
	}
	if !state.IsDir() {
		return notificationActivation{}, errors.New("notification state-directory must be a directory without a symbolic link")
	}
	if err := notificationPrivateOwner(state, 0o700); err != nil {
		return notificationActivation{}, err
	}
	return notificationActivation{template: template, destination: destination,
		approval:       notificationApproval{BindingDigest: raw.ApprovalDigest},
		credentialFile: raw.CredentialFile, stateDirectory: raw.StateDirectory}, nil
}

func notificationPrivatePath(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("notification private path is not absolute")
	}
	if filepath.Clean(path) != path {
		return errors.New("notification private path contains an alias")
	}
	return nil
}

// readNotificationPrivateFile trusts only a regular 0600 file owned by this
// process's effective user in an existing owned 0700 directory. Directory and
// file aliases refuse, and the opened directory must match the observed one.
// Read errors intentionally omit path values and file contents.
func readNotificationPrivateFile(path string, limit int64) (body []byte, result error) {
	if err := notificationPrivatePath(path); err != nil {
		return nil, err
	}
	directory := filepath.Dir(path)
	before, err := os.Lstat(directory)
	if err != nil {
		return nil, errors.New("notification private file directory could not be read")
	}
	if !before.IsDir() {
		return nil, errors.New("notification private file directory must not be a symbolic link")
	}
	if err := notificationPrivateOwner(before, 0o700); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, errors.New("notification private file directory could not be opened")
	}
	defer func() { result = errors.Join(result, root.Close()) }()
	opened, err := root.Open(".")
	if err != nil {
		return nil, errors.New("notification private directory identity could not be read")
	}
	info, statErr := opened.Stat()
	closeErr := opened.Close()
	if statErr != nil {
		return nil, errors.New("notification private directory identity could not be read")
	}
	if closeErr != nil {
		return nil, errors.New("notification private directory handle could not be closed")
	}
	if !os.SameFile(before, info) {
		return nil, errors.New("notification private directory changed while opening it")
	}
	// A substituted FIFO must not block before its descriptor can be checked.
	file, err := notificationRootFile(root, filepath.Base(path), os.O_RDONLY, 0)
	if err != nil {
		return nil, errors.New("notification private file could not be opened without following a symbolic link")
	}
	defer func() { result = errors.Join(result, file.Close()) }()
	info, err = file.Stat()
	if err != nil {
		return nil, errors.New("notification private file identity could not be read")
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("notification private file must be a regular file")
	}
	if err := notificationPrivateOwner(info, 0o600); err != nil {
		return nil, err
	}
	body, err = io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, errors.New("notification private file could not be completely read")
	}
	if int64(len(body)) > limit {
		return nil, errors.New("notification private file exceeds its byte bound")
	}
	return body, nil
}

func notificationPrivateOwner(info os.FileInfo, permissions os.FileMode) error {
	if info.Mode().Perm() != permissions {
		return errors.New("notification private file or directory has unsafe permissions; use 0600 files and 0700 directories")
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("notification private file owner could not be established")
	}
	if int(owner.Uid) != os.Geteuid() {
		return errors.New("notification private file or directory is owned by another user")
	}
	return nil
}

func (activation notificationActivation) preview(ctx context.Context, read notificationRunReader, id uint64) (notificationPreview, error) {
	if id == 0 {
		return notificationPreview{}, errors.New("notification run identity must be positive")
	}
	report, err := read(ctx, id)
	if err != nil {
		return notificationPreview{}, err
	}
	preview, err := buildNotificationPreview(report, activation.template, activation.destination)
	if err != nil {
		return notificationPreview{}, err
	}
	if err := checkNotificationApproval(preview, activation.approval); err != nil {
		return notificationPreview{}, err
	}
	return preview, nil
}

func (activation notificationActivation) initialize(ctx context.Context, read notificationRunReader, id uint64) (report notificationDeliveryReport, result error) {
	preview, err := activation.preview(ctx, read, id)
	if err != nil {
		return report, err
	}
	receipt, err := newNotificationReceipt(preview, activation.approval)
	if err != nil {
		return report, err
	}
	store, err := newFileNotificationStore(activation.stateDirectory)
	if err != nil {
		return report, err
	}
	defer func() { result = errors.Join(result, store.Close()) }()
	if err := store.initialize(ctx, receipt); err != nil {
		return report, err
	}
	return notificationDeliveryReport{State: "initialized", Destination: activation.destination.label,
		RunID: id, Attempt: preview.event.attempt, Reason: "explicit empty receipt initialized"}, nil
}

func (activation notificationActivation) deliver(ctx context.Context, read notificationRunReader, id uint64, transport http.RoundTripper) (report notificationDeliveryReport, result error) {
	preview, err := activation.preview(ctx, read, id)
	if err != nil {
		return report, err
	}
	store, err := newFileNotificationStore(activation.stateDirectory)
	if err != nil {
		return report, err
	}
	defer func() { result = errors.Join(result, store.Close()) }()
	credential, err := readNotificationPrivateFile(activation.credentialFile, 2048)
	if err != nil {
		return report, err
	}
	sender, err := newDiscordNotificationSender(string(credential), activation.destination, transport)
	if err != nil {
		return report, err
	}
	return deliverNotification(ctx, store, preview, activation.approval, sender)
}
