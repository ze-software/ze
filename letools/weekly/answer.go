// Design: docs/architecture/core-design.md -- the weekly update's command
// Detail: poster.go -- what the parsed options drive
//
// answer.go is the boundary between the operator and the publication: it reads
// the grammar, fills every seam a Poster has from this machine, and answers the
// engine.
//
// The command has ONE action, publishing the weekly update, so every word after
// it is a modifier and a keyword always comes before a value
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
	"github.com/ze-software/ze/letools/lepath"
)

// PostsDirRel is where the approved weekly sources live, relative to the
// checkout. It is the committed, published copy of the text, which is what
// makes it the one source a publication reads.
const PostsDirRel = "website/changes/posts"

// ArchiveDirRel is where a published week is recorded, relative to the
// checkout.
//
// The archive is what marks a week as published, so this path MUST stay the one
// the Python tool writes for as long as both implementations can run: two
// implementations reading two directories would publish the same week twice.
// The migration that removes scripts/ owes this path a new home, and moving it
// before that migration is what would cause the duplicate.
const ArchiveDirRel = "scripts/zeledon/weekly"

// channels are the Discord channels this tool will post to. ze-test is the
// rehearsal channel, and anything else is a typo -- which here means a message
// in the wrong public place.
var channels = [...]string{"ze-news", "ze-test"}

const usage = "usage: le weekly [source <path>] [dir <path>] [channel ze-news|ze-test] " +
	"[confirm] [force] [resume-from <n>] [date-stamp|no-date-stamp]"

// Answer is the `le weekly` command.
func Answer(args []string) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err) //nolint:errcheck // CLI output
		return nil, 1
	}

	opts, err := parseOptions(args, filepath.Join(root, PostsDirRel))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err) //nolint:errcheck // CLI output
		fmt.Fprintln(os.Stderr, usage)             //nolint:errcheck // CLI output
		return nil, 1
	}

	discordSh := FindDiscordSh()
	poster := &Poster{
		Channel:    opts.Channel,
		Send:       ExecSender(discordSh),
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

// parseOptions reads the operator's words.
//
// defaultDir is the posts directory a sweep reads when the operator names none.
// It is a parameter rather than a lookup, so the grammar is driven by a test
// with no checkout standing in for one.
//
// The bound is the argument count: each keyword consumes at most one word
// beyond itself.
func parseOptions(args []string, defaultDir string) (Options, error) {
	opts := Options{Channel: channels[0]}
	stampSeen := false

	for index := 0; index < len(args); index++ {
		word := args[index]
		switch word {
		case "confirm":
			opts.Confirm = true
			continue
		case "force":
			opts.Force = true
			continue
		case "date-stamp", "no-date-stamp":
			if stampSeen {
				return opts, errors.New("date-stamp and no-date-stamp: say one of them")
			}
			stampSeen = true
			opts.Stamp = StampOn
			if word == "no-date-stamp" {
				opts.Stamp = StampOff
			}
			continue
		}

		// The keyword is checked BEFORE its value is demanded. Otherwise a
		// bare path -- the mistake this grammar exists to catch -- is reported
		// as a keyword missing its value, which tells the operator nothing
		// about what is actually wrong.
		if !takesValue(word) {
			return opts, unknownKeyword(word)
		}
		value, err := valueFor(args, index)
		if err != nil {
			return opts, err
		}
		index++

		if err := opts.set(word, value); err != nil {
			return opts, err
		}
	}

	err := opts.settle(defaultDir)
	return opts, err
}

// set reads one keyword and its value.
func (o *Options) set(keyword, value string) error {
	switch keyword {
	case "source":
		o.Source = value
	case "dir":
		o.Dir = value
	case "channel":
		if !known(value) {
			var tb textbuf.Buffer
			return errors.New(tb.Str("unknown channel ").Quoted(value).
				Str("; say ze-news or ze-test").String())
		}
		o.Channel = value
	case "resume-from":
		number, err := strconv.Atoi(value)
		if err != nil {
			var tb textbuf.Buffer
			return errors.New(tb.Str("resume-from takes a number, got ").Quoted(value).String())
		}
		if number < 1 {
			return errors.New("resume-from takes a 1-based message number")
		}
		o.ResumeFrom = number
	default:
		return unknownKeyword(keyword)
	}
	return nil
}

// valued are the keywords that take the word after them. A word outside this
// list and outside the bare keywords above is not part of the grammar.
var valued = [...]string{"source", "dir", "channel", "resume-from"}

// takesValue reports whether a keyword is followed by its value.
func takesValue(word string) bool {
	for _, keyword := range valued {
		if word == keyword {
			return true
		}
	}
	return false
}

// unknownKeyword says the word is not in the grammar, and where a path goes.
func unknownKeyword(word string) error {
	var tb textbuf.Buffer
	return errors.New(tb.Str("unknown keyword ").Quoted(word).
		Str("; a path goes after source or dir").String())
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

// valueFor answers the word after a keyword, or says which keyword is missing
// one.
func valueFor(args []string, index int) (string, error) {
	if index+1 >= len(args) {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str(args[index]).Str(" takes a value").String())
	}
	return args[index+1], nil
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
