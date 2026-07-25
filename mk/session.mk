# Session-scoped binary names.
#
# Concurrent AI sessions share ONE working tree, so a fixed output path like
# bin/ze means session B's `make ze` overwrites the binary session A is running
# tests against, mid-run. Every canonical binary therefore carries this session's
# id as a NAME suffix: bin/ze-<session-id>.
#
# Why a suffix and not a per-session directory: `ze` derives its config and
# database directory from its OWN location (internal/core/paths/paths.go,
# ConfigDirFromBinary -> <prefix>/etc/ze, where <prefix> is the parent of bin/).
# Moving the binary to tmp/s/<id>/bin/ would silently repoint the daemon at an
# empty, session-local etc/ze that SessionEnd then deletes, while the live
# database in <repo>/etc/ze went unused. A suffix keeps the binary in bin/, so
# config/DB resolution is byte-for-byte what it is today.
#
# TEST binaries take the opposite trade-off on purpose: they live in a private
# bin/ subdir (mk/test-functional.mk, internal/test/sessionpath) because .ci
# tests exec them by BARE name and an isolated etc/ze is desirable there.
#
# Off-session -- a human shell, CI -- ZE_BIN_SUFFIX is empty and every path below
# is exactly the path used before this file existed. Nothing changes for anyone
# but an AI session.
#
# Source of the id: CLAUDE_CODE_SESSION_ID, which the Claude CLI exports into
# every child process (.claude/hooks/lib/session_id.py, source 1 -- "always set
# on current CLIs"). We deliberately do NOT shell out to session_id.py here:
# its last-resort source 4 MINTS an id keyed on the topmost process ancestor
# when none is found, so calling it from a human's `make` would invent a session
# and suffix that human's binaries. Presence of the exported variable is the
# correct, side-effect-free signal that an AI session is driving this build.

# `?=` so an explicit `make ZE_SESSION_ID=x` (or an id exported by a parent make)
# still wins, while the normal case takes the CLI's variable.
ZE_SESSION_ID ?= $(CLAUDE_CODE_SESSION_ID)

# Reject an id that is unusable as a filename component before it reaches a path.
# Whitespace shows up as a word count other than 1; '/' would escape bin/; '.'
# and '..' would alias it. Anything rejected falls through to the shared paths,
# so a malformed id degrades to today's behavior rather than to a bad path.
#
# The validation deliberately runs on the RESOLVED id, not on
# CLAUDE_CODE_SESSION_ID: a command-line assignment overrides every file
# assignment in make, so validating only the environment source would leave
# `make ze ZE_SESSION_ID=../../etc` writing outside bin/.
# The charset check MUST match Go's sidSafe (internal/test/sessionpath, itself a
# mirror of _SID_SAFE_RE in .claude/hooks/lib/session_id.py). A weaker check here
# is not cosmetic: make would build bin/ze-a+b while Go's ID() rejected the same
# id and looked in the shared bin/, so the build and the test runner would
# disagree about which artifacts belong to this session -- exactly the drift the
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
# character -- $ ` \ " -- is literal, so a quote-free id cannot inject shell.
# The guard wraps the assignment rather than following it, because `:=` is
# expanded at parse time and would otherwise run the command before any check.
ZE_SESSION_BAD := contains-a-quote
ifeq ($(findstring ',$(ZE_SESSION_ID)),)
ZE_SESSION_BAD := $(shell printf '%s' '$(ZE_SESSION_ID)' | tr -d 'A-Za-z0-9._-')
endif

# The canonical binary base names. ONE list: the ZEBIN_* paths below are built
# from it, and so is the collision check, so a new binary cannot be added without
# the guard learning about it.
ZE_BIN_NAMES := ze ze-appliance ze-setup ze-stripped ze-test ze-chaos ze-analyze ze-perf

ZE_SESSION_SAFE :=
ifeq ($(words $(ZE_SESSION_ID)),1)
  ifeq ($(findstring /,$(ZE_SESSION_ID)),)
    ifeq ($(ZE_SESSION_BAD),)
      ifneq ($(ZE_SESSION_ID),.)
        ifneq ($(ZE_SESSION_ID),..)
          # A suffix that reproduces another binary's name would make `make ze`
          # write OVER it: ZE_SESSION_ID=test turns bin/ze-<id> into bin/ze-test,
          # the real test-runner binary. Reject rather than overwrite.
          ifeq ($(filter ze-$(ZE_SESSION_ID),$(ZE_BIN_NAMES)),)
            ZE_SESSION_SAFE := $(ZE_SESSION_ID)
          endif
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

ifeq ($(ZE_SESSION_ID),)
ZE_BIN_SUFFIX :=
else
ZE_BIN_SUFFIX := -$(ZE_SESSION_ID)
endif

# The canonical binaries. Every recipe and every reference uses these variables
# rather than a literal bin/<name>, so the suffix cannot be applied in one place
# and forgotten in another (a build writing bin/ze-<id> while a test target
# depends on bin/ze would rebuild forever).
ZEBIN_ZE        := bin/ze$(ZE_BIN_SUFFIX)
ZEBIN_APPLIANCE := bin/ze-appliance$(ZE_BIN_SUFFIX)
ZEBIN_SETUP     := bin/ze-setup$(ZE_BIN_SUFFIX)
ZEBIN_STRIPPED  := bin/ze-stripped$(ZE_BIN_SUFFIX)
ZEBIN_TEST      := bin/ze-test$(ZE_BIN_SUFFIX)
ZEBIN_CHAOS     := bin/ze-chaos$(ZE_BIN_SUFFIX)
ZEBIN_ANALYZE   := bin/ze-analyze$(ZE_BIN_SUFFIX)
ZEBIN_PERF      := bin/ze-perf$(ZE_BIN_SUFFIX)

# Root for throwaway scratch that make targets create (isolated test binary sets,
# and anything else a target wants swept with the session). Off-session this is
# plain tmp/, exactly as before. Under a session it is the session's own
# directory, which .claude/hooks/session-end-scratch.sh already removes wholesale
# at SessionEnd -- so scratch that used to outlive its owner now cannot.
ifeq ($(ZE_SESSION_ID),)
ZE_SCRATCH_DIR := tmp
else
ZE_SCRATCH_DIR := tmp/s/$(ZE_SESSION_ID)
endif

# Print the path of this session's ze binary. Scripts, docs and agents that used
# to hardcode `bin/ze` ask for it here instead, so they keep working whether or
# not a session is active:  $(make ze-path) show version
.PHONY: ze-path
ze-path:
	@printf '%s\n' '$(ZEBIN_ZE)'
