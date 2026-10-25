// Design: docs/architecture/core-design.md -- the weekly update's command
// Detail: poster.go -- what the parsed options drive
//
// answer.go is the boundary between the operator and the publication: it reads
// the grammar, fills every seam a publisher has from this machine, and answers the
// engine.
//
// The command has ONE action, publishing the weekly update, declared in the
// action table below, so every word after it is a modifier and a keyword always comes before a value
// (ai/rules/cli.md). A post file named `force.md` therefore cannot be read as
// the force keyword.
//
//	le weekly                                       what would be published
//	le weekly confirm                               publish it
//	le weekly source website/changes/posts/2026-08-10.md
//	le weekly source <path> confirm resume-from 3   finish a half-sent post
//	le weekly channel ze-test confirm force date-stamp
//
// The bare command publishes nothing. That is the default because a message
// cannot be taken back: it is in a public channel the moment it lands, so an
// operator reads the plan and then confirms it.
package weekly

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// PostsDirRel is where the approved weekly sources live, relative to the
// checkout. It is the committed, published copy of the text, which is what
// makes it the one source a publication reads.
const PostsDirRel = "website/changes/posts"

// ArchiveDirRel is where a published week is recorded, relative to the
// checkout. The archive is the duplicate-publication guard, so the producer and
// the weekly skill MUST read and write this one directory.
const ArchiveDirRel = "website/changes/discord"

// channels are the Discord channels this tool will post to. ze-test is the
// rehearsal channel, and anything else is a typo -- which here means a message
// in the wrong public place.
var channels = [...]string{"ze-news", "ze-test"}

// updateVerb is the one action: publishing the weekly update.
const updateVerb = "update"

// The keywords the update takes.
const (
	sourceKeyword      = "source"
	dirKeyword         = "dir"
	channelKeyword     = "channel"
	confirmKeyword     = "confirm"
	forceKeyword       = "force"
	resumeFromKeyword  = "resume-from"
	dateStampKeyword   = "date-stamp"
	noDateStampKeyword = "no-date-stamp"
)

// actions is the whole command surface, declared once, so the listing, the
// usage, the help pages and the parse read one grammar (internal/le/le/action).
var actions = leaction.New("weekly", leaction.Action{
	Verb:    updateVerb,
	Summary: "publish the weekly update to Discord",
	Why: "publish the weekly update to Discord; without confirm it answers what would be published" +
		" and posts nothing",
	Writes: true,
	Parameters: []leaction.Parameter{
		{Keyword: sourceKeyword, Kind: leaction.KindPath, Value: "path", Requirement: leaction.Optional,
			Help: "the one post to publish"},
		{Keyword: dirKeyword, Kind: leaction.KindPath, Value: "path", Requirement: leaction.Optional,
			Help: "the posts directory to sweep; default " + PostsDirRel + ""},
		{Keyword: channelKeyword, Kind: leaction.KindEnum, Value: "channel", Values: channels[:],
			ValueHelp: map[string]string{
				"ze-news": "the public news channel; the default",
				"ze-test": "the rehearsal channel, to check a post first",
			},
			Requirement: leaction.Optional, Help: "the channel to post to; ze-test is the rehearsal channel"},
		{Keyword: confirmKeyword, Kind: leaction.KindSwitch, Help: "publish, rather than answer the plan"},
		{Keyword: forceKeyword, Kind: leaction.KindSwitch,
			Help: "publish the post named by source even if already archived"},
		{Keyword: resumeFromKeyword, Kind: leaction.KindUint, Value: "n", Requirement: leaction.Optional,
			Help: "the 1-based message of the source post to resume from"},
		{Keyword: dateStampKeyword, Kind: leaction.KindSwitch, Help: "stamp the post with its date"},
		{Keyword: noDateStampKeyword, Kind: leaction.KindSwitch, Help: "post without the date stamp"},
	},
	AnswerArgs: answerUpdate,
})

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le weekly` command. A line that does not open with the verb
// is the update, as callers have always typed it (D-P3-bare in
// spec-le-cli-usability), so the verb is supplied here and the table reads the
// rest of the line: bare `le weekly` still publishes nothing and answers the
// plan. A refusal the table's parser makes answers 2 (D-P3-stray); one the
// body makes keeps the 1 it answered before.
func Answer(args []string) (any, int) {
	if len(args) > 0 && actions.Holds(args[0]) {
		return actions.Answer(args)
	}
	return actions.Answer(append([]string{updateVerb}, args...))
}

// answerUpdate publishes, or plans, the update one parsed line names.
func answerUpdate(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err) //nolint:errcheck // CLI output
		return nil, 1
	}

	opts, err := optionsOf(args, filepath.Join(root, PostsDirRel))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err) //nolint:errcheck // CLI output
		return nil, 1
	}

	discordSh := resolveDiscordSh()
	poster := &publisher{
		Channel:    opts.Channel,
		Send:       execSender(discordSh),
		DiscordSh:  discordSh,
		Sleep:      time.Sleep,
		Today:      today(),
		ArchiveDir: filepath.Join(root, ArchiveDirRel),
		// Progress is the running commentary of a publication that takes
		// minutes. It is not the answer, so it goes to stderr and leaves stdout
		// to the payload (ai/rules/cli.md).
		Progress: os.Stderr,
	}

	report, err := poster.Run(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err) //nolint:errcheck // CLI output
		return report, 1
	}
	return report, 0
}

// today answers the current day with its clock removed, so every date decision
// this tool makes is a whole-day one and a run at 23:59 differs from a run at
// 00:01 by a date rather than by an hour.
func today() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// optionsOf turns the parsed keywords into the options, and refuses a value
// the table's parser does not judge: a channel outside the closed set, a
// resume point that is not a 1-based number, and two keywords that contradict
// each other.
func optionsOf(args leaction.Arguments, defaultDir string) (Options, error) {
	opts := Options{
		Channel: channels[0],
		Source:  args.One(sourceKeyword),
		Dir:     args.One(dirKeyword),
		Confirm: args.Has(confirmKeyword),
		Force:   args.Has(forceKeyword),
	}
	if args.Has(dateStampKeyword) && args.Has(noDateStampKeyword) {
		return opts, errors.New("date-stamp and no-date-stamp: say one of them")
	}
	if args.Has(dateStampKeyword) {
		opts.Stamp = StampOn
	}
	if args.Has(noDateStampKeyword) {
		opts.Stamp = StampOff
	}
	if args.Has(channelKeyword) {
		value := args.One(channelKeyword)
		if !known(value) {
			var tb textbuf.Buffer
			return opts, errors.New(tb.Str("unknown channel ").Quoted(value).
				Str("; say ze-news or ze-test").String())
		}
		opts.Channel = value
	}
	if args.Has(resumeFromKeyword) {
		value := args.One(resumeFromKeyword)
		number, err := strconv.Atoi(value)
		if err != nil {
			var tb textbuf.Buffer
			return opts, errors.New(tb.Str("resume-from takes a number, got ").Quoted(value).String())
		}
		if number < 1 {
			return opts, errors.New("resume-from takes a 1-based message number")
		}
		opts.ResumeFrom = number
	}

	err := opts.settle(defaultDir)
	return opts, err
}

// settle decides what the words mean together, and fills the default a sweep
// reads.
func (o *Options) settle(defaultDir string) error {
	if o.Source != "" && o.Dir != "" {
		return errors.New("source names one post and dir names a sweep: say one of them")
	}
	if o.Source != "" {
		return nil
	}

	// force and resume-from are decisions about ONE post. A sweep that forced
	// would publish every unfinished week it found, and a resume point names a
	// message of a specific post.
	if o.Force {
		return errors.New("force is a decision about one post: name it with source")
	}
	if o.ResumeFrom != 0 {
		return errors.New("resume-from names a message of one post: name it with source")
	}
	if o.Dir == "" {
		o.Dir = defaultDir
	}
	return nil
}

// known reports whether a name is one of the channels this tool posts to.
func known(channel string) bool {
	for _, allowed := range channels {
		if channel == allowed {
			return true
		}
	}
	return false
}
