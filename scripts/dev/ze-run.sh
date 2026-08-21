#!/usr/bin/env bash
# ze-run.sh -- admission point for heavy jobs in a shared checkout.
#
# Several Claude sessions, and the subagents they spawn, work one checkout on
# one machine. Every heavy target is sized for the WHOLE box (GO_TEST_PROCS,
# golangci-lint's one worker per core), so two agents starting a heavy job at
# the same moment oversubscribe the machine until it stops responding. That was
# reported on 2026-08-17: three concurrent sessions, and "running the linting by
# hand can cause the machine to freeze".
#
# This is the ONE place that answers "may this job run now". A job either takes
# a free slot and runs, or it waits and says who is holding the slot.
#
#   Usage: scripts/dev/ze-run.sh LABEL CMD [ARGS...]
#
# LABEL becomes a path component under tmp/.ze-jobs/, so it is restricted to
# [A-Za-z0-9_-]. Nothing else is accepted -- a separator or a dot in a label is
# how a job escapes the registry directory.
#
# THE REGISTRY. tmp/.ze-jobs/<label>.<pid>.job holds one FIELD=VALUE line per
# field: LABEL, PID, PGID, TREE (the tree hash the job is judging), KEY and
# PARAMS (the work this job does -- see THE WORK KEY), STARTED, LOG, STATE and
# CMD. Its presence IS the claim on a slot; there is no second
# record of who is running. The job's own exit trap removes it, and a waiter
# reaps an entry whose PID is dead, so a crashed job costs one poll interval
# rather than an operator.
#
# FAIL CLOSED. An entry this script cannot read counts as OCCUPIED. Reading
# "cannot parse" as "nothing is running" would admit every session at once,
# which is the failure the wrapper exists to prevent. Such an entry is
# discarded only once it is older than STALL_SECONDS, which is what keeps the
# registry bounded.
#
# NESTING. A wrapped job may run wrapped stages: `make ze-precommit-verify`
# holds a slot and then runs `make ze-lint`, which is wrapped too. The inner
# job would wait for a slot its own parent holds, and nothing would ever
# release it. So the wrapper exports ZE_RUN_JOB, and a job whose parent entry
# is still present runs inside the parent's slot instead of queueing.
#
# THE LOG. The job's output is teed to tmp/.ze-jobs/<label>.<pid>.log, which is
# what LOG names. The file is real rather than aspirational because a waiter
# has to see progress (AC-8) and because judging a holder by whether its log
# grows is what decides whether its slot may be broken. The cost is that the
# job's stdout is a pipe, so a tool that colours only for a terminal stops
# colouring.
#
# ATTACH AND SHARE. Serialization alone makes eight agents queue for eight runs
# of the same thing. In a shared checkout most askers want the SAME job on the
# SAME tree, so a second asker whose LABEL and TREE both match a RUNNING job
# does not queue for a duplicate: it follows that job's log to its own stdout
# and exits with that job's code. One run answers both.
#
# The job key is (label, tree hash, work key), never the label alone. Attaching
# on the label would certify a session with a run that never saw its code, and
# the Go-commit coverage gate reads exactly that certificate
# (full_verify_coverage in scripts/dev/commit_helper.py). A tree hash that is
# empty or `unknown` matches nothing, another unknown included: an unmeasured
# tree is not a matching tree. A job that waited re-reads the hash when it is
# admitted, so the field says which tree the job IS judging rather than which
# tree its asker saw (plan/spec-shared-machine-job-admission.md, R-2, AC-3 and
# AC-4).
#
# THE WORK KEY. A label names the TARGET, and a parameterised target is many
# jobs under one name. `make ze-unit-pkg-test PKG=./a` and the same target on
# `./b` hand this script the same label and the same command, because PKG
# reaches the `_ze-unit-pkg-test-impl` half alone. Two sessions testing
# different packages therefore matched, and the second reported the first run's
# exit code as its own: on 2026-08-19 a package that was never compiled read as
# passing (plan/journal/stale-artifact-reused.md). The same shape had already
# bitten the appliance kernel, where `ARCH=` was missing from a rebuild trigger
# and an amd64 vmlinuz answered an arm64 build.
#
# So KEY fingerprints what the job DOES: the command, plus the make
# command-line variables the caller typed. That set is DERIVED rather than
# declared -- GNU make puts every `VAR=value` from the command line into
# MAKEFLAGS -- so a target that grows a parameter tomorrow is keyed on it with
# no edit here and none in its recipe. A recipe cannot forget to declare what
# it never declares. The command is in the key too, which is what keys the
# hand-queued route: two agents naming one label for two commands
# (ai/rules/commands.md) are the same defect with no make in it.
#
# Its boundary, stated because a guard that fails open in silence is the thing
# this file exists to refuse: a parameter supplied through the ENVIRONMENT
# (`PKG=./a make ze-unit-pkg-test`) reaches make as a variable but never
# reaches MAKEFLAGS, and no rule, doc or recipe in this repository spells it
# that way. It is indistinguishable from the ambient environment, which every
# session differs in, so keying on that would end sharing altogether. A recipe
# that needs to be keyed on one passes it into the command it hands this
# script, which the key already reads.
#
# THE RESULT. A job records its exit code in tmp/.ze-jobs/<label>.<pid>.rc
# BEFORE it removes its entry, so an attacher that watches the entry disappear
# always finds a code waiting for it. There is no default and no zero: a job
# killed outright leaves no record, and the attacher then reports nothing at
# all. It returns to the admission queue and runs the job itself, because a
# verdict it did not observe is worse than the work it avoided.
#
# LIVENESS, NOT AGE. A slot is broken in two cases and no other: the holder's
# process is GONE, or the holder is alive and its log has not grown for
# STALL_SECONDS. Elapsed time is not one of them.
#
# verify-lock.sh judged by age, and this script inherited that rule: a holder
# past 1800 seconds had its process group killed, on a threshold whose comment
# justified it with "ze-precommit-verify targets ~2 min". The recorded history
# in tmp/.ze-verify-duration.txt says 12m45s, and a full run under load on
# 2026-08-17 took over 20 minutes, ze-lint alone about 18 of them. So under
# exactly the contention this wrapper exists to manage, the first waiter killed
# the legitimate run, ran slowly itself, and was killed in turn by the next
# waiter. Age cannot tell "hung" from "slow because the box is oversubscribed";
# a growing log can, and it stays true however long the job has been running
# (plan/spec-shared-machine-job-admission.md, R-1 and AC-6).
#
# A holder whose log cannot be read is NOT killed. Fail closed points one way
# here: an unjudgeable holder keeps its slot, because admitting a second job
# oversubscribes the box and killing on a guess destroys a run. Two targets are
# never signalled: process group 0, and this script's own group.
#
# The simpler shape that was looked for first: plain flock over more targets,
# with no registry. It gives no way to see who is holding the slot, no way to
# reap a crashed holder without an operator, and nowhere to record the tree
# hash that later lets a second asker share a running job's result.

set -e

JOBS_DIR="tmp/.ze-jobs"
REGISTRY_LOCK="$JOBS_DIR/.registry.lock"
# The documented view of the current holder: ai/rules/git-safety.md and
# ai/rules/commands.md both tell readers this file names it. It is written from
# the registry entry, by the entry's own writer, and removed with it.
OWNER_FILE="tmp/.ze-verify.lock.owner"
DURATION_FILE="tmp/.ze-verify-duration.txt"

# How long a live holder may produce NOTHING before its slot is broken. It is a
# silence budget, not a run-time budget: a job that keeps writing is never
# broken, whatever its elapsed time. The default is above the longest silent
# stretch measured in this repository (ze-lint, about 18 minutes of a 20 minute
# verify on 2026-08-17), so a real run has headroom and a wedged one is still
# reclaimed within the hour.
#
# ZE_VERIFY_MAX_LOCK_AGE is honoured as the older spelling: ai/rules/git-safety.md
# tells readers to raise it, and that instruction must keep working. It now buys
# a longer SILENCE, which a healthy slow job no longer needs.
STALL_MIN=60
STALL_MAX=3600
STALL_SECONDS="${ZE_JOB_STALL_SECONDS:-${ZE_VERIFY_MAX_LOCK_AGE:-1800}}"

# How many admitted jobs run at once. The Makefile derives it beside the
# per-job ceiling it depends on and exports it; reading it here keeps the two
# from drifting. The default of one is for a caller that arrives without make.
SLOTS_MIN=1
SLOTS="${ZE_RUN_SLOTS:-1}"
# How often a waiting job re-checks the registry.
POLL_SECONDS=2
# How often a waiting job repeats its banner. A 20 minute wait at the poll
# interval would otherwise put 600 identical lines into an agent's context.
BANNER_SECONDS=30
# How long an attacher waits for the result of the job it followed, after that
# job's output has ended. The job writes the result before it drops its entry,
# so this covers only the moment between its last log line and its exit trap.
ATTACH_RESULT_WAIT=10

# ---- helpers ---------------------------------------------------------------

_usage() {
    echo "usage: $0 LABEL CMD [ARGS...]" >&2
    exit 2
}

# _field FILE NAME -- the value of one FIELD=VALUE line, empty when absent.
# NAME rather than KEY, because KEY is now the name of one of those fields.
_field() {
    awk -v k="$2" 'index($0, k "=") == 1 { print substr($0, length(k) + 2); exit }' \
        "$1" 2>/dev/null || true
}

_numeric() {
    case "${1:-}" in
        '' | *[!0-9]*) return 1 ;;
    esac
    return 0
}

# _alive PID -- true when the process is still running a job.
#
# The process table is asked, because the two cheaper questions each answer a
# different one. `kill -0` asks whether we may SIGNAL the process, so another
# user's job reads as absent while it holds a slot. /proc answers correctly and
# exists on Linux alone, so a macOS session got the `kill -0` answer for every
# process. `ps -o state=` is the one question with the one meaning, and this
# script already depends on `ps -o` for MY_PGID.
#
# A ZOMBIE counts as gone. It has already exited, holds no CPU and no memory,
# and writes nothing more, but `kill -0` succeeds on it and the process table
# still lists it for as long as its parent takes to reap it. Reading that as
# "alive" makes a finished job hold its slot, and makes a follower wait on a run
# that ended. Linux prints the state as one letter and macOS appends flags to
# it, so the STATE is the first letter in both.
_alive() {
    _numeric "$1" || return 1
    [ "$1" -gt 0 ] || return 1
    case "$(ps -o state= -p "$1" 2>/dev/null | tr -d '[:space:]')" in
        '') return 1 ;;
        Z*) return 1 ;;
    esac
    return 0
}

_mtime() {
    stat -c %Y "$1" 2>/dev/null || stat -f %m "$1" 2>/dev/null || echo 0
}

# The ceiling on SLOTS. More slots than cores is not admission.
_cores() {
    nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4
}

# The fingerprint of the tree this job is about to judge: the same one
# verify-status.sh writes, so a later step can tell whether a running job has
# already seen the asker's code.
_tree_hash() {
    "$(dirname "$0")/verify-status.sh" tree_hash 2>/dev/null || echo unknown
}

# The parameters this job was given: the make command-line variable
# definitions, one per line. GNU make writes them into MAKEFLAGS behind a `--`
# separator, after the flags, so `-j4` and `--jobserver-auth` stay out by
# construction -- neither changes a verdict, and jobserver-auth differs on
# every invocation.
#
# Sorted, because make lists the definitions in the caller's own order: two
# sessions typing `PKG=./a RUN=X` and `RUN=X PKG=./a` ask for one thing, so the
# key is over the SET.
#
# Four names are dropped, and all four are this script's OWN: they choose how a
# job is ADMITTED and can change nothing about what it judges. The Makefile
# documents `make <target> ZE_RUN_SLOTS=1` and ai/rules/git-safety.md documents
# raising the stall window, so a session following either must still share a
# running job rather than pay for a second one. MAY_ATTACH is dropped for a
# second reason as well: it says this job will not take another's verdict, and
# keying on it would silently make it say that nobody may take THIS job's
# verdict either.
_job_params() {
    case "${MAKEFLAGS:-}" in
        *' -- '*) ;;
        *) return 0 ;;
    esac
    printf '%s\n' "${MAKEFLAGS#* -- }" \
        | tr '[:blank:]' '\n' \
        | grep -v -e '^$' \
            -e '^ZE_RUN_SLOTS=' -e '^ZE_JOB_STALL_SECONDS=' \
            -e '^ZE_VERIFY_MAX_LOCK_AGE=' -e '^MAY_ATTACH=' \
        | LC_ALL=C sort
}

# The hasher, resolved once. sha256sum ships with GNU coreutils and shasum with
# macOS; a machine with neither gets no key at all rather than a weaker one.
if command -v sha256sum >/dev/null 2>&1; then
    HASH_CMD="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
    HASH_CMD="shasum -a 256"
else
    HASH_CMD=""
fi

# _job_key CMD... -- the fingerprint of what this job DOES. See THE WORK KEY at
# the top of this file.
#
# `unknown` when the machine has no hasher, and `unknown` matches nothing --
# the rule TREE already follows. An unmeasured job is not a matching job, so it
# queues and runs its own work.
_job_key() {
    if [ -z "$HASH_CMD" ]; then
        echo unknown
        return 0
    fi
    { printf 'CMD=%s\n' "$*"; _job_params; } | $HASH_CMD | awk '{ print $1 }'
}

# _break_stalled ENTRY LABEL PID PGID LOG STATIC ELAPSED -- kill a holder that
# is alive but has stopped producing output, then drop its entry.
#
# The kill prints the evidence that justified it: which file stopped growing and
# for how long. Elapsed time is reported as context, never as the reason -- an
# operator reading "20 minutes elapsed" cannot tell whether the kill was right,
# and reading "no output for 31 minutes" can.
_break_stalled() {
    _entry="$1"; _label="$2"; _pid="$3"; _pgid="$4"; _log="$5"; _static="$6"; _elapsed="$7"
    _target=""
    if _numeric "$_pgid" && [ "$_pgid" != "0" ] && [ "$_pgid" != "$MY_PGID" ]; then
        _target="-$_pgid"
    elif _numeric "$_pid" && [ "$_pid" != "$$" ]; then
        _target="$_pid"
    fi
    if [ -z "$_target" ]; then
        # Nothing safe to signal: leave the entry alone rather than kill this
        # session's own process group.
        return 0
    fi
    printf '\033[31m[%s] breaking STALLED job: %s (pid %s, pgid %s)\n' \
        "$LABEL" "$_label" "$_pid" "$_pgid" >&2
    printf '  evidence: %s has not grown for %ds (stall window %ds); the job had been running %ds\033[0m\n' \
        "$_log" "$_static" "$STALL_SECONDS" "$_elapsed" >&2
    kill -TERM -- "$_target" 2>/dev/null || true
    for _ in 1 2 3; do
        _alive "$_pid" || break
        sleep 1
    done
    kill -KILL -- "$_target" 2>/dev/null || true
    rm -f "$_entry" "${_entry%.job}.log"
}

# _attach ENTRY PID LOG -- share a running job instead of running a second copy
# of it. The job's log is replayed from its first line to this job's stdout, so
# the asker sees the whole run and not the part that was left when it arrived.
#
# Returns 0 with ATTACH_RC set to the code that job recorded. Returns 1 when it
# left no record, which is the one case the caller has to decide for itself:
# nothing was observed, so nothing may be reported.
_attach() {
    _entry="$1"; _pid="$2"; _log="$3"
    _result="${_entry%.job}.rc"
    printf '\033[36m[%s] attaching to the %s already running for this tree (pid %s): one run answers both\033[0m\n' \
        "$LABEL" "$LABEL" "$_pid" >&2

    # The log is followed on a descriptor THIS job holds open, never by a tool
    # told to watch a path. `cat` copies what has arrived and returns at end of
    # file; the descriptor keeps its position, so the next round continues where
    # this one stopped and no byte is printed twice.
    #
    # Holding the descriptor is what makes the LAST bytes readable. A job
    # records its result and then unlinks its entry and its log in one `rm`, so
    # a follower that reopens the path finds nothing and loses the end of the
    # run. The open descriptor keeps the file until this loop closes it, and the
    # copy taken after the entry is gone is therefore the whole tail of the run
    # rather than a race against the holder's exit trap.
    #
    # `tail -f --pid=` did this until 2026-08-21. `--pid` is GNU coreutils only:
    # BSD tail refused the option, the follower exited before it copied a byte,
    # and the discarded diagnostic made the sharer print nothing and say
    # nothing. Every other external call in this file already carries a
    # portable form, and this one no longer needs one -- the shell can hold a
    # descriptor itself.
    #
    # The open IS the test, and it carries no `2>/dev/null`: `exec` without a
    # command applies its redirections to THIS SHELL, so silencing the open
    # would silence every later banner this job writes, the one saying why it
    # stopped sharing included. A name that cannot be opened leaves this job
    # waiting for the holder without replaying it, which is what a job whose
    # entry named no log always did, and the shell says which name failed.
    _following=0
    if [ -n "$_log" ] && exec 8< "$_log"; then
        _following=1
    fi
    while :; do
        if [ "$_following" = "1" ]; then
            cat <&8
        fi
        # The entry going is the job ending: it is removed after the result is
        # recorded. A killed job leaves its entry behind, so the process is
        # asked too.
        if [ ! -f "$_entry" ] || ! _alive "$_pid"; then
            if [ "$_following" = "1" ]; then
                cat <&8
            fi
            break
        fi
        sleep "$POLL_SECONDS"
    done
    if [ "$_following" = "1" ]; then
        exec 8<&-
    fi

    _waited=0
    while [ ! -f "$_result" ] && [ "$_waited" -lt "$ATTACH_RESULT_WAIT" ]; do
        sleep 1
        _waited=$(( _waited + 1 ))
    done
    if [ -f "$_result" ]; then
        ATTACH_RC=$(cat "$_result" 2>/dev/null || echo 1)
        # An unreadable result is a failure, never a pass: the shared run's
        # verdict is the whole point of attaching to it.
        _numeric "$ATTACH_RC" || ATTACH_RC=1
        printf '\033[36m[%s] the shared %s finished with exit %s\033[0m\n' \
            "$LABEL" "$LABEL" "$ATTACH_RC" >&2
        return 0
    fi
    printf '\033[33m[%s] the job we followed (pid %s) ended without recording a result; nothing was observed, so back to the queue\033[0m\n' \
        "$LABEL" "$_pid" >&2
    return 1
}

_write_entry() {
    {
        printf 'LABEL=%s\n' "$LABEL"
        printf 'PID=%s\n' "$$"
        printf 'PGID=%s\n' "$MY_PGID"
        printf 'TREE=%s\n' "$TREE"
        # KEY decides whether a later asker may share this run. PARAMS is the
        # readable half, for the operator asking why two jobs did not share; it
        # decides nothing.
        printf 'KEY=%s\n' "$KEY"
        printf 'PARAMS=%s\n' "$PARAMS"
        printf 'STARTED=%s\n' "$START"
        printf 'LOG=%s\n' "$LOG"
        printf 'STATE=running\n'
        printf 'CMD=%s\n' "$*"
    } > "$ENTRY"
    : > "$LOG"
    cp "$ENTRY" "$OWNER_FILE"
}

# _scan_and_claim CMD... -- runs under the registry lock. Reaps dead entries,
# counts the live ones, and either shares an equivalent job, claims a slot, or
# reports the holder. Prints one tab-separated line:
#   ATTACH<TAB>entry<TAB>pid<TAB>log
#   CLAIMED
#   BUSY<TAB>label<TAB>pid<TAB>elapsed
_scan_and_claim() {
    now=$(date +%s)
    occupied=0
    holder_label=""
    holder_pid="?"
    holder_elapsed=0
    attach_entry=""
    attach_pid=""
    attach_log=""

    for entry in "$JOBS_DIR"/*.job; do
        [ -e "$entry" ] || continue
        pid=$(_field "$entry" PID)
        label=$(_field "$entry" LABEL)
        started=$(_field "$entry" STARTED)

        if [ ! -r "$entry" ] || ! _numeric "$pid"; then
            # Fail closed: an entry we cannot read is a job we cannot prove is
            # gone. It has no readable LOG to judge, so this one case still goes
            # by age, and it only DROPS the entry -- nothing is signalled. The
            # window is shared so the registry stays bounded by one number.
            age=$(( now - $(_mtime "$entry") ))
            if [ "$age" -gt "$STALL_SECONDS" ]; then
                rm -f "$entry" "${entry%.job}.log"
                continue
            fi
            occupied=$(( occupied + 1 ))
            if [ -z "$holder_label" ]; then
                holder_label="unreadable entry ${entry##*/}"
                holder_elapsed="$age"
            fi
            continue
        fi

        if ! _alive "$pid"; then
            rm -f "$entry" "${entry%.job}.log"
            continue
        fi

        _numeric "$started" || started="$now"
        elapsed=$(( now - started ))

        # The holder is alive. Whether it is WORKING is answered by its log, not
        # by the clock: tee updates the file's mtime on every write, so a log
        # that grew inside the stall window is a job still producing. A holder
        # whose log is unreadable, absent, or has an unusable mtime is left
        # alone -- see LIVENESS, NOT AGE at the top of this file.
        log=$(_field "$entry" LOG)
        log_mtime=0
        if [ -n "$log" ] && [ -f "$log" ]; then
            log_mtime=$(_mtime "$log")
        fi
        if _numeric "$log_mtime" && [ "$log_mtime" -gt 0 ]; then
            static=$(( now - log_mtime ))
            if [ "$static" -gt "$STALL_SECONDS" ]; then
                _break_stalled "$entry" "$label" "$pid" \
                    "$(_field "$entry" PGID)" "$log" "$static" "$elapsed"
                continue
            fi
        fi

        # ATTACH AND SHARE: same work, same tree, still running. The tree hash
        # says the run has seen this asker's code and the work key says it is
        # doing this asker's job, so a value nobody could measure matches
        # nothing -- including another job that could not measure one either.
        if [ "${MAY_ATTACH:-1}" = "1" ] && [ -z "$attach_entry" ] \
            && [ "$label" = "$LABEL" ] \
            && [ -n "$TREE" ] && [ "$TREE" != "unknown" ] \
            && [ "$(_field "$entry" TREE)" = "$TREE" ] \
            && [ -n "$KEY" ] && [ "$KEY" != "unknown" ] \
            && [ "$(_field "$entry" KEY)" = "$KEY" ] \
            && [ "$(_field "$entry" STATE)" = "running" ]; then
            attach_entry="$entry"
            attach_pid="$pid"
            attach_log="$log"
        fi

        occupied=$(( occupied + 1 ))
        if [ -z "$holder_label" ]; then
            holder_label="$label"
            holder_pid="$pid"
            holder_elapsed="$elapsed"
        fi
    done

    # A result is read by an attacher moments after it is written, and nothing
    # deletes it for its reader, so age is what bounds this half of the
    # registry. The window is the same one the entries use.
    for result in "$JOBS_DIR"/*.rc; do
        [ -e "$result" ] || continue
        if [ "$(( now - $(_mtime "$result") ))" -gt "$STALL_SECONDS" ]; then
            rm -f "$result"
        fi
    done

    if [ -n "$attach_entry" ]; then
        printf 'ATTACH\t%s\t%s\t%s\n' "$attach_entry" "$attach_pid" "$attach_log"
        return 0
    fi

    if [ "$occupied" -ge "$SLOTS" ]; then
        printf 'BUSY\t%s\t%s\t%s\n' "$holder_label" "$holder_pid" "$holder_elapsed"
        return 0
    fi

    # TREE is what a later asker attaches on, so it has to name the tree this
    # job is about to judge. A job that queued behind a 20 minute holder asked
    # about a tree that may have moved since, so the hash is taken again here,
    # at the moment of admission, and only when the job waited.
    if [ "${TREE_STALE:-0}" = "1" ]; then
        TREE=$(_tree_hash)
    fi
    START="$now"
    _write_entry "$@"
    printf 'CLAIMED\n'
}

_release() {
    _rc=$?
    _end=$(date +%s)
    printf '%s\t%s\t%s\n' "$LABEL" "$(( _end - START ))" \
        "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$DURATION_FILE"
    # The result before the entry, never after: an attacher watching this entry
    # disappear must find the code already written rather than a race it can
    # lose. Attachers may be several, and none of them owns the file, so the
    # registry scan retires it by age.
    printf '%s\n' "$_rc" > "${ENTRY%.job}.rc"
    rm -f "$ENTRY" "$LOG" "$OWNER_FILE"
}

_report_previous() {
    [ -f "$DURATION_FILE" ] || return 0
    _prev=$(awk -F'\t' -v l="$LABEL" '$1==l{s=$2}END{if(s)print s}' "$DURATION_FILE")
    [ -n "$_prev" ] || return 0
    printf '[%s] previous run took %dm%ds (%ds)\n' \
        "$LABEL" $((_prev/60)) $((_prev%60)) "$_prev"
}

# ---- arguments -------------------------------------------------------------

LABEL="${1:-}"
shift || true
[ -n "$LABEL" ] && [ "$#" -gt 0 ] || _usage

case "$LABEL" in
    *[!A-Za-z0-9_-]*)
        echo "error: label '$LABEL' is not a path component ([A-Za-z0-9_-] only)" >&2
        exit 2
        ;;
esac

# ---- make's no-execute modes: never take a slot ----------------------------
#
# A wrapped recipe reads `ze-run.sh <label> $(MAKE) ... _<name>-impl`, and GNU
# make EXECUTES a recipe line containing $(MAKE) even under -n, -t and -q: that
# is how recursive make participates in those modes. So `make -n ze-lint` really
# starts this script. Without this guard it queues for a slot and, because the
# stage sub-make does nothing and writes no log, it is never seen to make
# progress -- `make -n` hangs until the stall window expires.
#
# Admission is skipped rather than refused. This script records no verdict, so
# running the command through costs nothing: the child make prints its recipes
# and exits. scripts/status/verify_run.go REFUSES under the same modes instead,
# because it writes a verify record that a no-execute run would forge.
_make_dry_run() {
    _flags="${MAKEFLAGS%% *}"
    case "$_flags" in
        -*|*=*|"") return 1 ;;
        *[ntq]*) return 0 ;;
        *) return 1 ;;
    esac
}

if _make_dry_run; then
    exec "$@"
fi

# The stall window is enforced by killing a process GROUP, so a value outside
# the range it was designed for is refused rather than clamped or accepted. Too
# small kills a healthy job between two log lines; too large leaves a wedged one
# holding the only slot for hours. Neither is a policy this script picks for a
# caller who typed something else.
if ! _numeric "$STALL_SECONDS" \
    || [ "$STALL_SECONDS" -lt "$STALL_MIN" ] || [ "$STALL_SECONDS" -gt "$STALL_MAX" ]; then
    echo "error: stall window '$STALL_SECONDS' is out of range ($STALL_MIN..$STALL_MAX seconds)" >&2
    echo "       set ZE_JOB_STALL_SECONDS. It budgets SILENCE, not run time: a job that keeps" >&2
    echo "       writing to its log is never broken, however long it runs." >&2
    exit 2
fi

# Refused rather than clamped, like the stall window above. Zero queues every
# job for ever; more slots than cores admits every asker, which is no admission.
SLOTS_MAX=$(_cores)
if ! _numeric "$SLOTS" \
    || [ "$SLOTS" -lt "$SLOTS_MIN" ] || [ "$SLOTS" -gt "$SLOTS_MAX" ]; then
    echo "error: slot count '$SLOTS' is out of range ($SLOTS_MIN..$SLOTS_MAX)" >&2
    echo "       set ZE_RUN_SLOTS. The Makefile derives it from GO_TEST_PROCS;" >&2
    echo "       take it down with \`make <target> ZE_RUN_SLOTS=1\`." >&2
    exit 2
fi

# ---- nested job: run inside the parent's slot -------------------------------

if [ -n "${ZE_RUN_JOB:-}" ] && [ -f "$ZE_RUN_JOB" ]; then
    printf '[%s] running inside %s\n' "$LABEL" "$(_field "$ZE_RUN_JOB" LABEL)" >&2
    exec "$@"
fi

if ! command -v flock >/dev/null 2>&1; then
    echo "error: flock required (Linux util-linux package)" >&2
    exit 1
fi

mkdir -p "$JOBS_DIR"
MY_PGID=$(ps -o pgid= -p $$ | tr -d ' ')
TREE=$(_tree_hash)
# The tree moves while a job waits, so TREE is re-read at admission. What this
# job DOES cannot move: the command and the parameters were fixed by the caller
# before this script started, so KEY is measured once, here.
KEY=$(_job_key "$@")
PARAMS=$(_job_params | tr '\n' ' ')
PARAMS="${PARAMS% }"
# This job's own entry, should it get a slot. The name carries the PID so two
# jobs with the same label can coexist once more than one slot exists.
ENTRY="$JOBS_DIR/$LABEL.$$.job"
LOG="$JOBS_DIR/$LABEL.$$.log"

# ---- admission -------------------------------------------------------------

last_banner=0
# Sharing is offered once. A queue that keeps re-attaching to jobs that die
# without a result would follow one corpse after another and never run
# anything, so an attach that observed nothing sends this job back to the
# ordinary queue for good.
#
# The caller may decline sharing outright, and the ENVIRONMENT is where it says
# so: `MAY_ATTACH=0` makes this job queue for its own run rather than take
# another's verdict. That is the route ai/rules and plan/journal name for a
# caller who wants its own answer, so it must be honoured rather than
# overwritten -- a plain `MAY_ATTACH=1` here destroyed the inherited value, and
# the opt-out did nothing at all. Any value other than `1` declines, which is
# the safe direction: a typo costs a duplicate run, never a borrowed verdict.
#
# It governs THIS job's asking, and nothing else. A job that declined to attach
# still runs its own work honestly, so a later asker with the same key may
# still share its result -- which is why _job_params drops the name from the
# key rather than letting it split one.
MAY_ATTACH="${MAY_ATTACH:-1}"
# Whether this job's tree hash predates its admission (see _scan_and_claim).
TREE_STALE=0
while :; do
    exec 9>"$REGISTRY_LOCK"
    if flock -w 10 9; then
        outcome=$(_scan_and_claim "$@")
    else
        # Nobody could be asked, so nobody is admitted.
        outcome=$(printf 'BUSY\tregistry lock\t?\t0')
    fi
    exec 9>&-

    # ATTACH carries entry, pid, log; BUSY carries label, pid, elapsed.
    IFS=$'\t' read -r state first second third <<< "$outcome"
    if [ "$state" = "ATTACH" ]; then
        if _attach "$first" "$second" "$third"; then
            exit "$ATTACH_RC"
        fi
        MAY_ATTACH=0
        TREE_STALE=1
        continue
    fi
    [ "$state" = "BUSY" ] || break

    TREE_STALE=1
    now=$(date +%s)
    if [ "$last_banner" = "0" ] || [ $(( now - last_banner )) -ge "$BANNER_SECONDS" ]; then
        printf '\033[33m[%s] waiting: %s running (pid %s, %ds elapsed)...\033[0m\n' \
            "$LABEL" "$first" "$second" "$third" >&2
        last_banner="$now"
    fi
    sleep "$POLL_SECONDS"
done

# ---- run -------------------------------------------------------------------

# _scan_and_claim ran in a command substitution, so the START it recorded did
# not survive it. The entry it wrote is the record, so read it back from there.
START=$(_field "$ENTRY" STARTED)

trap '_release' EXIT INT TERM
_report_previous

# Absolute, because a stage may run from a directory other than the checkout
# root and the registry paths are root-relative. A nested job that cannot see
# its parent's entry queues, and a job queueing behind its own parent never
# starts.
export ZE_RUN_JOB="$PWD/$ENTRY"
set +e
"$@" 2>&1 | tee "$LOG"
rc=${PIPESTATUS[0]}
set -e
exit "$rc"
