# Session-scoped binary directory.
#
# Concurrent AI sessions share ONE working tree, so a fixed output path like
# bin/ze means session B's `make ze` overwrites the binary session A is running
# tests against, mid-run. Every canonical binary therefore goes into this
# session's OWN directory, under its BARE name:
# tmp/session/<YYYY-MM-DD>-<session-id>/bin/ze.
#
# Why a directory and not a name suffix: a directory gives each session its own
# namespace. No id can reproduce another binary's name, so no collision guard is
# needed; argv[0] personality dispatch keeps working (cmd/ze/dispatch.go
# binarySuffixRoot reads the segment after the last '-'); and a .ci test can exec
# `ze` by bare name off one PATH entry. Cleanup is one directory the operator
# identifies by date and removes, rather than a glob over a name pattern that
# misses everything it does not match.
#
# `ze` derives its config and database directory from its OWN location
# (internal/core/paths/paths.go, ConfigDirFromBinary -> <prefix>/etc/ze, where
# <prefix> is the parent of bin/), so a session's binary resolves the
# session-local <session-dir>/etc/ze. That isolation is the intent, not an
# accident: <repo>/etc/ze becomes the human's alone.
#
# THE DIRECTORY IS LOOKED UP, NEVER RECOMPUTED. <YYYY-MM-DD>-<sid> is not a pure
# function of the id, so every consumer takes the single directory matching
# tmp/session/????-??-??-<sid>, and names a new one with today's date only on a
# miss. Recomputing from today's date would move a session's directory at
# midnight and orphan the binaries that session is running. make, Go
# (internal/test/sessionpath) and shell (scripts/dev/session-scratch.sh) each
# implement this rule, and TestMakeAndGoAgreeOnBinDir
# (scripts/dev/session_bin_dir_test.py) is what stops the three drifting.
#
# Off-session -- a human shell, CI -- ZE_BIN_DIR is bin and every path below is
# exactly the path used before this file existed. Nothing changes for anyone but
# an AI session.
#
# Source of the id: CLAUDE_CODE_SESSION_ID, which the Claude CLI exports into
# every child process (.claude/hooks/lib/session_id.py, source 1 -- "always set
# on current CLIs"). We deliberately do NOT shell out to session_id.py here:
# its last-resort source 4 MINTS an id keyed on the topmost process ancestor
# when none is found, so calling it from a human's `make` would invent a session
# and move that human's binaries. Presence of the exported variable is the
# correct, side-effect-free signal that an AI session is driving this build.

# `?=` so an explicit `make ZE_SESSION_ID=x` (or an id exported by a parent make)
# still wins, while the normal case takes the CLI's variable.
ZE_SESSION_ID ?= $(CLAUDE_CODE_SESSION_ID)

# Reject an id that is unusable as a filename component before it reaches a path.
# Whitespace shows up as a word count other than 1; '/' would escape the session
# root; '.' and '..' would alias it. Anything rejected falls through to the
# shared paths, so a malformed id degrades to today's behavior rather than to a
# bad path. The id is now a path COMPONENT rather than a name suffix, so this
# check is what keeps a hostile id inside tmp/session/.
#
# The validation deliberately runs on the RESOLVED id, not on
# CLAUDE_CODE_SESSION_ID: a command-line assignment overrides every file
# assignment in make, so validating only the environment source would leave
# `make ze ZE_SESSION_ID=../../etc` writing outside the session root.
# The charset check MUST match Go's sidSafe (internal/test/sessionpath, itself a
# mirror of _SID_SAFE_RE in .claude/hooks/lib/session_id.py). A weaker check here
# is not cosmetic: make would build into a directory Go's ID() rejected while Go
# looked in the shared bin/, so the build and the test runner would disagree
# about which artifacts belong to this session -- exactly the drift the
# single-resolver design exists to prevent.
#
# `tr -d` deletes every accepted character; a non-empty remainder means the id
# carries something Go would reject. One `tr` per make run, no side effects --
# unlike calling session_id.py, which MINTS an id (session_id.py:278-286) and
# would invent a session for a human's `make`.
#
# The id must be INTERPOLATED into the command: passing it through an exported
# variable and reading $$VAR inside the shell was tried and silently validated
# nothing, because make's `export` reaches recipe environments, not $(shell)
# calls expanded during parsing (verified: the remainder came back empty for an
# id that plainly had one).
#
# Interpolation is made safe by refusing, in pure make, the ONE character that
# can terminate the single-quoted shell literal below. Inside '...' every other
# character -- $ ` \ " -- is literal, so a quote-free id cannot inject SHELL.
# The guard wraps the assignment rather than following it, because `:=` is
# expanded at parse time and would otherwise run the command before any check.
#
# It bounds the shell layer only, and MAKE's own layer sits above it. ZE_SESSION_ID
# is recursively expanded, so `$(...)` inside the incoming value is expanded by
# make at the first reference -- which is this guard. Verified:
# CLAUDE_CODE_SESSION_ID='$(shell touch X)' created X at parse time while the id
# was still rejected and the path still resolved to bin/. The PATH is safe either
# way, which is what this file owes. Anyone who can set that variable in your
# environment can already run commands as you, so make's layer crosses no
# privilege boundary and is not guarded here -- but do not read the paragraph
# above as covering it.
ZE_SESSION_BAD := contains-a-quote
ifeq ($(findstring ',$(ZE_SESSION_ID)),)
ZE_SESSION_BAD := $(shell printf '%s' '$(ZE_SESSION_ID)' | tr -d 'A-Za-z0-9._-')
endif

ZE_SESSION_SAFE :=
ifeq ($(words $(ZE_SESSION_ID)),1)
  ifeq ($(findstring /,$(ZE_SESSION_ID)),)
    ifeq ($(ZE_SESSION_BAD),)
      ifneq ($(ZE_SESSION_ID),.)
        ifneq ($(ZE_SESSION_ID),..)
          ZE_SESSION_SAFE := $(ZE_SESSION_ID)
        endif
      endif
    endif
  endif
endif

# Exported so the Go side (internal/test/sessionpath) reads the SAME id this
# file used, instead of re-deriving one and disagreeing about which artifacts
# belong to this session. The exported value is the VALIDATED one, so Go and
# make cannot disagree about a rejected id either.
#
# `override` is required, not stylistic: a command-line assignment outranks every
# makefile assignment, so a plain `ZE_SESSION_ID :=` here would be discarded and
# `make ze ZE_SESSION_ID=../../etc` would still reach the -o path.
override ZE_SESSION_ID := $(ZE_SESSION_SAFE)
export ZE_SESSION_ID

# The ONE root for per-session state: this session's dated directory sits beside
# the flat marker files the hooks write (.claude/hooks/lib/state-file.sh).
ZE_SESSION_ROOT := tmp/session

# ZE_SCRATCH_DIR is this session's own directory -- binaries, isolated test
# binary sets, and anything else a target wants kept with the session. Off-session
# it is plain tmp/, exactly as before.
#
# The dated name is resolved by the glob-then-create rule in the header. Nothing
# is created here: the recipes that write into the directory mkdir it, exactly as
# they did for bin/, so asking for a path never mints a directory.
#
# $(wildcard) rather than a shell glob costs no fork, and the id reaching it is
# already refused unless it matches [A-Za-z0-9._-]+, which carries no glob
# metacharacter. $(firstword) makes a tree that somehow holds two dated
# directories for one id resolve deterministically to the older of them.
#
# The TRAILING SLASH in the pattern is what restricts the match to DIRECTORIES,
# which is make's own idiom for it and is the rule the shell copy spells `[ -d ]`
# and the python copy spells isdir. $(patsubst) takes the slash back off, because
# every consumer wants the plain directory name. Without it a regular file of the
# dated shape would be make's answer and nobody else's.
ifeq ($(ZE_SESSION_ID),)
ZE_SCRATCH_DIR := tmp
ZE_BIN_DIR := bin
else
ZE_SCRATCH_DIR := $(patsubst %/,%,$(firstword $(wildcard $(ZE_SESSION_ROOT)/????-??-??-$(ZE_SESSION_ID)/)))
ifeq ($(ZE_SCRATCH_DIR),)
ZE_SCRATCH_DIR := $(ZE_SESSION_ROOT)/$(shell date +%Y-%m-%d)-$(ZE_SESSION_ID)
endif
ZE_BIN_DIR := $(ZE_SCRATCH_DIR)/bin
endif

# The canonical binaries, every one a BARE name under $(ZE_BIN_DIR). Every recipe
# and every reference uses these variables rather than a literal bin/<name>, so
# the directory cannot be applied in one place and forgotten in another (a build
# writing the session directory while a test target depends on bin/ze would
# rebuild forever). The final element of $(ZE_BIN_DIR) is always `bin`, or ze
# cannot resolve a config directory at all (internal/core/paths/paths.go
# isBinDir).
ZEBIN_ZE        := $(ZE_BIN_DIR)/ze
ZEBIN_APPLIANCE := $(ZE_BIN_DIR)/ze-appliance
ZEBIN_SETUP     := $(ZE_BIN_DIR)/ze-setup
ZEBIN_STRIPPED  := $(ZE_BIN_DIR)/ze-stripped
ZEBIN_TEST      := $(ZE_BIN_DIR)/ze-test
ZEBIN_CHAOS     := $(ZE_BIN_DIR)/ze-chaos
ZEBIN_ANALYZE   := $(ZE_BIN_DIR)/ze-analyze
ZEBIN_PERF      := $(ZE_BIN_DIR)/ze-perf

# Seeding this session's store. The binaries above resolve their config and
# database directory from their own location (internal/core/paths/paths.go
# ConfigDirFromBinary), so a session's ze reads $(ZE_SCRATCH_DIR)/etc/ze rather
# than the repository's etc/ze. Isolation is the intent; an EMPTY store is not.
# internal/component/config/storage/blob.go NewBlob creates the blob and returns
# a nil error when it is absent, so an unseeded session is silently empty rather
# than red. The first ze_core binary this session builds therefore seeds it, and
# every failure of that seeding is loud (scripts/dev/session-seed-store.sh).
#
# Called with the binary the recipe just built, because ze, ze-appliance and
# ze-stripped are equal seeders: each links internal/core/resolve (the silent
# path), internal/component/ssh (the host key) and internal/plugins/init
# (`ze init`, registered under //go:build ze_core). Seeding from the binary in
# hand is what stops `make ze-stripped` leaving an empty store, and it asks no
# recipe to build a binary nobody wanted. ze-setup, ze-test, ze-chaos,
# ze-analyze and ze-perf link no init and reach no silent path, so they do not
# call this.
#
# Off-session the macro expands to NOTHING, and a recipe line that expands to
# nothing is neither printed nor executed, so a human's `make ze` stays the
# command it always was -- `make -n ze` included.
ifeq ($(ZE_SESSION_ID),)
ZE_SEED_SESSION_STORE =
else
ZE_SEED_SESSION_STORE = @scripts/dev/session-seed-store.sh $(1)
endif

# Print the path of this session's ze binary. Scripts, docs and agents that used
# to hardcode `bin/ze` ask for it here instead, so they keep working whether or
# not a session is active:  $(make ze-path) show version
.PHONY: ze-path
ze-path:
	@printf '%s\n' '$(ZEBIN_ZE)'
