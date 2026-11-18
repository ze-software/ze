// Design: docs/architecture/doctor-and-health-checks.md -- resource-owned warnings
// Related: report.go -- ordinary warning admission and the bounded store
package report

import (
	"errors"
	"time"
	"unicode/utf8"
)

// ErrResourceWarningOwned refuses ordinary replacement or clearance of a
// resource-owned subject. The original resource's episode must settle it.
var ErrResourceWarningOwned = errors.New("warning belongs to a retained resource: complete its original recovery and clear that episode through its owner")

type resourceWarningKey struct{ source, subject string }

const resourceWarningLatestSeparator = "; latest: "

// ResourceWarningCauseMax reserves enough of MessageMax for both the original
// and latest actual cause. Counts belong to the domain owner, not this text.
const ResourceWarningCauseMax = (MessageMax - len(resourceWarningLatestSeparator)) / 2

// FitResourceWarningCause bounds an actual cause at a UTF-8 rune boundary.
// Producers MUST bound each original/latest cause before retaining an episode.
func FitResourceWarningCause(message string) string {
	if len(message) <= ResourceWarningCauseMax {
		return message
	}
	cut := ResourceWarningCauseMax
	for cut > 0 {
		if utf8.RuneStart(message[cut]) {
			break
		}
		cut--
	}
	return message[:cut]
}

// ResourceWarning reserves one warning slot before a native mutation can
// require it. It is a daemon-local capability, never a wire value. Its owner
// MUST retain it through process loss and clear only after genuine recovery.
// NewResourceWarning constructs it; zero/nil refuses. Safe for concurrent use.
type ResourceWarning struct {
	store   *store
	key     resourceWarningKey
	episode *ResourceWarningEpisode
	closed  bool
}

// ResourceWarningEpisode retains the first and latest actual failure cause,
// original Fix, and idempotent publish/clear capability. A cleared episode
// cannot affect a later one. Only
// ResourceWarning.NewEpisode constructs it; zero/nil refuses. Safe for
// concurrent use through its original store's lock.
type ResourceWarningEpisode struct {
	owner      *ResourceWarning
	issue      Issue
	firstCause string
	published  bool
	cleared    bool
}

// NewResourceWarning reserves a bounded slot and exclusive custody of source
// and subject. Callers MUST derive them from their original resource/recipient,
// not an untrusted report DTO. It refuses before native IO when the reservation
// cap is full. Ordinary warnings keep their existing eviction behavior in the
// remaining capacity. A healthy owner calls Close without raising or clearing.
func NewResourceWarning(source, subject string) (*ResourceWarning, error) {
	if !validFields(source, "reserved", subject, "", "", nil) {
		return nil, errors.New("resource warning source or subject is invalid: retain a bounded original resource identity before native work")
	}
	s := pkg.Load()
	s.mu.Lock()
	defer s.mu.Unlock()
	key := resourceWarningKey{source: source, subject: subject}
	if s.resources[key] != nil {
		return nil, ErrResourceWarningOwned
	}
	if len(s.resources) >= s.warningCap {
		return nil, errors.New("resource warning reservation is full: export a larger ze.report.warnings.max before admitting native work")
	}
	for ordinary := range s.warnings {
		if ordinary.source != source {
			continue
		}
		if ordinary.subject == subject {
			return nil, errors.New("resource warning subject is already published without custody: retain a fresh diagnostic episode before native work")
		}
	}
	owner := &ResourceWarning{store: s, key: key}
	s.resources[key] = owner
	if s.warningSlotsLocked() > s.warningCap {
		s.evictOldestWarning()
	}
	return owner, nil
}

// NewEpisode retains the original failure before publication. The caller MUST
// keep and retry this same episode after an unknown publish outcome. It MUST
// NOT repeat any native mutation to retry a diagnostic. At most one pending or
// published episode belongs to a reservation at once.
// Optional detail follows RaiseWarningFix: only the first map is used and
// shallow-cloned. Mutable values inside that map MUST NOT change afterward.
func (owner *ResourceWarning) NewEpisode(code, message, fix string, detail ...map[string]any) (*ResourceWarningEpisode, error) {
	if owner == nil {
		return nil, errors.New("resource warning owner is absent")
	}
	if owner.store == nil {
		return nil, errors.New("resource warning owner is uninitialized")
	}
	fields := firstDetail(detail)
	if !validFields(owner.key.source, code, owner.key.subject, message, fix, fields) {
		return nil, errors.New("resource warning fields exceed their bounds: shorten the original explanation before publication")
	}
	if len(message) > ResourceWarningCauseMax {
		return nil, errors.New("resource warning original cause exceeds its reserved share: bound it with FitResourceWarningCause before publication")
	}
	s := owner.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if owner.closed {
		return nil, errors.New("resource warning owner is closed")
	}
	if s.resources[owner.key] != owner {
		return nil, errors.New("resource warning reservation belongs to another owner lifetime")
	}
	if owner.episode != nil {
		return nil, errors.New("resource warning already has an unresolved episode: retain and finish its original recovery")
	}
	now := time.Now()
	episode := &ResourceWarningEpisode{owner: owner, firstCause: message, issue: Issue{Source: owner.key.source, Code: code, Severity: SeverityWarning, Subject: owner.key.subject, Message: message, Fix: fix, Raised: now, Updated: now, Detail: copyDetail(fields)}}
	owner.episode = episode
	return episode, nil
}

// Publish is idempotent for this exact episode and preserves its original cause,
// fix and first timestamp. Capacity was reserved before native IO, so publication
// cannot fail because unrelated ordinary warnings filled the bus.
func (episode *ResourceWarningEpisode) Publish() error {
	if episode == nil {
		return errors.New("resource warning episode is absent")
	}
	owner := episode.owner
	if owner == nil {
		return errors.New("resource warning episode is uninitialized")
	}
	if owner.store == nil {
		return errors.New("resource warning episode has no original store")
	}
	s := owner.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if episode.cleared {
		return errors.New("resource warning episode was already cleared")
	}
	if owner.closed {
		return errors.New("resource warning original owner is closed")
	}
	if s.resources[owner.key] != owner {
		return errors.New("resource warning reservation belongs to another owner lifetime")
	}
	if owner.episode != episode {
		return errors.New("resource warning episode belongs to another owner lifetime")
	}
	key := warningKey{source: owner.key.source, code: episode.issue.Code, subject: owner.key.subject}
	s.warnings[key] = &episode.issue
	delete(s.evicted, key)
	episode.published = true
	return nil
}

// Update retains the latest actual cause beside the immutable first cause and
// original fix. Only this episode's capability can update it. The native route
// MUST verify that exact episode and deduplicate its report-only update serial;
// a diagnostic retry MUST NOT repeat native IO or change domain cleanup counts.
func (episode *ResourceWarningEpisode) Update(message string) error {
	if episode == nil {
		return errors.New("resource warning episode is absent")
	}
	owner := episode.owner
	if owner == nil {
		return errors.New("resource warning episode is uninitialized")
	}
	if owner.store == nil {
		return errors.New("resource warning episode has no original store")
	}
	if len(message) > ResourceWarningCauseMax {
		return errors.New("resource warning latest cause exceeds its reserved share: bound it with FitResourceWarningCause before publication")
	}
	s := owner.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if episode.cleared {
		return errors.New("resource warning episode was already cleared")
	}
	if owner.episode != episode {
		return errors.New("resource warning update belongs to another owner lifetime")
	}
	if s.resources[owner.key] != owner {
		return errors.New("resource warning reservation belongs to another owner lifetime")
	}
	episode.issue.Message = episode.firstCause
	if message != episode.firstCause {
		episode.issue.Message += resourceWarningLatestSeparator + message
	}
	episode.issue.Updated = time.Now()
	return nil
}

// Clear MUST be called only after the original resource's actual recovery
// proof or ACK. The bus cannot establish that domain fact. Repeating Clear
// acknowledges this exact old episode and never clears a new failure.
func (episode *ResourceWarningEpisode) Clear() error {
	if episode == nil {
		return errors.New("resource warning episode is absent")
	}
	owner := episode.owner
	if owner == nil {
		return errors.New("resource warning episode is uninitialized")
	}
	if owner.store == nil {
		return errors.New("resource warning episode has no original store")
	}
	s := owner.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if episode.cleared {
		return nil
	}
	if s.resources[owner.key] != owner {
		return errors.New("resource warning reservation belongs to another owner lifetime")
	}
	if owner.episode != episode {
		return errors.New("resource warning clear belongs to another owner lifetime")
	}
	key := warningKey{source: owner.key.source, code: episode.issue.Code, subject: owner.key.subject}
	delete(s.warnings, key)
	delete(s.evicted, key)
	episode.cleared = true
	owner.episode = nil
	return nil
}

// Close releases only an unused or genuinely cleared reservation. An unresolved
// episode refuses, including one whose Publish answer was lost. Safe to retry.
func (owner *ResourceWarning) Close() error {
	if owner == nil {
		return errors.New("resource warning owner is absent")
	}
	if owner.store == nil {
		return errors.New("resource warning owner is uninitialized")
	}
	s := owner.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if owner.closed {
		return nil
	}
	if owner.episode != nil {
		return errors.New("resource warning recovery is unconfirmed: retain its original reservation and episode")
	}
	if s.resources[owner.key] != owner {
		return errors.New("resource warning reservation belongs to another owner lifetime")
	}
	delete(s.resources, owner.key)
	owner.closed = true
	return nil
}

// Both helpers require s.mu. Resources reserve a slot even before publication.
func (s *store) resourceOwnsLocked(key warningKey) bool {
	return s.resources[resourceWarningKey{source: key.source, subject: key.subject}] != nil
}
func (s *store) warningSlotsLocked() int {
	slots := len(s.warnings) + len(s.resources)
	for _, owner := range s.resources {
		if owner.episode == nil {
			continue
		}
		if owner.episode.published {
			slots--
		}
	}
	return slots
}

// TryRaiseWarningFix is the acknowledged transport boundary for an ordinary
// warning. A protected resource subject returns a refusal; legacy void callers
// still preserve that subject through store.raiseWarning's same ownership gate.
func TryRaiseWarningFix(source, code, subject, message, fix string) error {
	if !validFields(source, code, subject, message, fix, nil) {
		return errors.New("warning fields exceed their bounds: shorten the report before publication")
	}
	s := pkg.Load()
	key := warningKey{source: source, code: code, subject: subject}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.resourceOwnsLocked(key) {
		return ErrResourceWarningOwned
	}
	s.raiseWarningLocked(key, message, fix, nil)
	return nil
}

// TryClearWarning is the acknowledged ordinary clear boundary. It never
// returns success for a retained resource warning it could not clear.
func TryClearWarning(source, code, subject string) error {
	if source == "" {
		return errors.New("warning source is absent")
	}
	if code == "" {
		return errors.New("warning code is absent")
	}
	if subject == "" {
		return errors.New("warning subject is absent")
	}
	s := pkg.Load()
	key := warningKey{source: source, code: code, subject: subject}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.resourceOwnsLocked(key) {
		return ErrResourceWarningOwned
	}
	delete(s.warnings, key)
	delete(s.evicted, key)
	return nil
}
