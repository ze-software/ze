#!/bin/bash
# PostToolUse hook on Read: refresh a freshness marker whenever implementation
# source is read. "Implementation source" is the file types a spec can
# legitimately be ABOUT, and the KIND is the file's EXTENSION and nothing else:
# .go, .py, .sh, .yang, and the make wiring (Makefile, *.mk). A spec about a
# Python tool or a shell hook is grounded by reading THAT tool, not by reading
# an unrelated .go file purely to satisfy this gate (T-4).
#
# THE EXTENSION IS THE WHOLE RULE, AND THAT IS LOAD-BEARING (2026-08-08).
# This hook and _SUBJECT_PATTERNS in pretool-writeedit.py are two ends of ONE
# contract: the reader derives the kind a spec is ABOUT from the paths the spec
# lists, and asks for the marker this writer records. When the two ends spell
# the kind differently, reading the very file the spec NAMES writes nothing, and
# the only way past the block is to read an UNRELATED file. That is the T-4 hole
# made compulsory: a gate whose sanctioned exit is reading an unrelated file
# manufactures the evidence it was built to demand.
#
# It was real, not hypothetical. While this writer accepted .py only under
# scripts/ and .claude/hooks/, and .sh only under .claude/hooks/ and scripts/,
# 11 open specs named py subjects (test/interop/interop.py,
# test/ipsec-interop/lab.py, tools/kernel-builder/build.py) and 2 named sh
# subjects (packaging/deb/preinstall.sh) that NO accepted Read could ever cover.
# A directory list is a second thing to keep in sync and it silently drifted.
# An extension has nothing to drift.
#
# TWO markers are written per accepted Read, and the second is what scopes the
# gate rather than relaxing it:
#   tmp/session/.source-read-<SID>          any implementation source was read
#   tmp/session/.source-read-<KIND>-<SID>   source of THAT kind was read
# c_design_without_lsp in pretool-writeedit.py derives the kind a spec is about
# from its own "## Files to Modify" and "## Files to Create" lists, then asks for
# the matching marker. So a Go spec still needs Go read, and a hooks spec is
# satisfied by the hook. Both ends spell the kinds the same way: go, py, sh,
# make, yang.
#
# A READ THAT SHOWED ALMOST NOTHING GROUNDS NOTHING (2026-08-08).
# The gate is strict about WHICH file was read, so it must not be trivial about
# HOW MUCH of it was read: Read(file, limit=1) would otherwise clear every spec
# of that kind for the next 30 minutes. A read is accepted when it returned the
# WHOLE file (any size: a 12-line register.go read entire IS the producer), or a
# window of at least MIN_LINES. A read that showed ZERO lines is refused whatever
# else the payload says, and only a payload shape this hook does not RECOGNISE at
# all is accepted unmeasured -- an unfamiliar tool_response must not silently
# disable the evidence path and block every spec write in the session.
#
# ZERO IS NOT UNMEASURABLE (2026-08-08, review round 2). Three real shapes show
# nothing and each one used to write a marker. Counted over the 211 transcripts
# in ~/.claude/projects: 13 file_unchanged, 36 empty files, 65 failed reads.
#   {"type":"file_unchanged","file":{"filePath":...}}   the harness suppresses the
#       body ("Wasted call -- file unchanged since your last Read"), so the read
#       showed nothing NOW while refreshing a 30-minute clearance.
#   {"file":{"content":"","numLines":1,"totalLines":1}}  a zero-byte file. Both
#       counts are 1, so it read as a whole-file read of a one-line file and was
#       accepted at the whole-file exemption. One Read of any empty .py cleared
#       every py spec in the session.
#   "Error: File does not exist. ..."                    a failed Read, whose
#       tool_response is a plain STRING. jq could not index it, the error went to
#       /dev/null, and every count defaulted to unmeasurable.
# The separator is whether the harness sent a shape this hook knows: a `file`
# object, or a string. Either way it TOLD us, and what it told us is zero.
#
# THE HARNESS COUNTS THE TRAILING NEWLINE AS A LINE. `numLines` and `totalLines`
# are both raw `split("\n")` lengths, so a 317-line README reports 318 and a
# 19-line window reports 20 (measured: 118 whole-file reads of files still on
# disk, totalLines == wc -l + 1 in all 118; numLines == raw split length in all
# 5064 records carrying both content and a count). The bar is therefore taken
# from `content` whenever content and numLines agree, which is what makes 19
# lines read as 19. The whole-file test stays in the harness's own units, where
# the offset cancels on both sides.
#
# Companion to mark-lsp-invoked.sh. The c_design_without_lsp check in
# pretool-writeedit.py accepts EITHER marker before allowing a spec/design file
# write: an LSP invocation OR a source Read. Rationale: reading the function that
# PRODUCES a behavior is the verification we actually want before authoring a
# spec that claims something about that behavior (ai/rules/evidence.md,
# "Behavioral claims and recommendations"). Requiring the LSP tool specifically
# false-negatives legitimate investigation done via the Read tool.
#
# Marker content: ISO-8601 timestamp.
# Non-blocking: this hook only records; it never rejects a Read.

# With neither route to the project root there is no tree to record against, and
# a marker written under an unknown cwd is worse than none: it would satisfy a
# gate reading somewhere else. Record nothing, and still never reject the Read.
cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.." || exit 0

source .claude/hooks/lib/session-id.sh
SID=$(_session_id)

INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Implementation source a spec can be ABOUT counts as "investigated the
# producer", and the extension names the KIND. Docs, specs, config, JSON, and
# unrelated markdown carry no kind and do not satisfy the gate. Keep this list
# and _SUBJECT_PATTERNS in pretool-writeedit.py identical, extension for
# extension: the design-gate fixtures drive both ends over one fixture project
# so a divergence is a red test rather than a silent hole.
case "$FILE_PATH" in
    *.go)
        KIND="go" ;;
    *.py)
        KIND="py" ;;
    *.sh)
        KIND="sh" ;;
    */Makefile|Makefile|*.mk)
        KIND="make" ;;
    *.yang)
        KIND="yang" ;;
    *)
        exit 0 ;;
esac

# How much of the file the Read actually showed.
#   SHOWN       lines of TEXT the response carried; -1 when nothing measured it
#   RAW_LINES   the same count in the harness's units (trailing newline counted)
#   TOTAL_LINES the file's length, in those same units
#   LIMIT       the window the REQUEST asked for
#   KNOWN_SHAPE 1 when the payload is one this hook recognises, so an unmeasured
#               SHOWN means the read showed nothing rather than "no idea"
MIN_LINES=20
read -r SHOWN RAW_LINES TOTAL_LINES LIMIT KNOWN_SHAPE <<EOF
$(printf '%s' "$INPUT" | jq -r '
  def num(x): if (x|type) == "number" then (x | floor) else -1 end;
  # The harness splits on "\n" and keeps the empty tail, and JS gives [""] for
  # the empty string, so an empty file counts 1. Match that convention exactly:
  # it is what tells an INTACT content field from one the payload dropped.
  def raw($c): if $c == "" then 1 else ($c | split("\n") | length) end;
  # Lines carrying text: the raw count without the phantom tail, and 0 for an
  # empty file. Preferred over numLines when the two agree; when they disagree,
  # content did not survive the payload and numLines is all there is. That is
  # not hypothetical either: 6 records in the same corpus carry an emptied
  # content beside a numLines of 12 or 94, and reading those as zero would
  # refuse a read that really did show 94 lines.
  def shown($f):
    ($f.content) as $c | ($f.numLines) as $n
    | if ($c | type) == "string" and (($n | type) != "number" or raw($c) == $n)
      then (if $c == "" then 0
            else (raw($c) - (if ($c | endswith("\n")) then 1 else 0 end)) end)
      else num($n) end;
  (if (.tool_response | type) == "object" then .tool_response else null end) as $r
  | (if ($r.file? | type) == "object" then $r.file else null end) as $f
  | [ (if $f == null then -1 else shown($f) end),
      (if $f == null then -1
       elif ($f.numLines | type) == "number" then ($f.numLines | floor)
       elif ($f.content | type) == "string" then raw($f.content)
       else -1 end),
      (if $f == null then -1 else num($f.totalLines) end),
      num(.tool_input.limit),
      (if $f != null or (.tool_response | type) == "string" then 1 else 0 end) ]
  | @tsv' 2>/dev/null)
EOF
SHOWN=${SHOWN:--1}
RAW_LINES=${RAW_LINES:--1}
TOTAL_LINES=${TOTAL_LINES:--1}
LIMIT=${LIMIT:--1}
KNOWN_SHAPE=${KNOWN_SHAPE:-0}

if [ "$SHOWN" -eq 0 ]; then
    # The response measured, and it measured nothing: an empty file, a window
    # past the end, or a read the harness answered without a body. Zero lines
    # ground zero claims, and the whole-file exemption below must not rescue it.
    exit 0
elif [ "$SHOWN" -gt 0 ]; then
    # A read that reached the end of the file is a whole-file read whatever its
    # size, so only a WINDOW is held to the line bar.
    if [ "$TOTAL_LINES" -lt 0 ] || [ "$RAW_LINES" -lt "$TOTAL_LINES" ]; then
        [ "$SHOWN" -lt "$MIN_LINES" ] && exit 0
    fi
elif [ "$KNOWN_SHAPE" -eq 1 ]; then
    # A shape this hook knows -- a file object, or the string a failed Read
    # returns -- that carried no lines at all. file_unchanged is the live case:
    # the body was suppressed, so this Read showed nothing.
    exit 0
elif [ "$LIMIT" -ge 0 ] && [ "$LIMIT" -lt "$MIN_LINES" ]; then
    # An unrecognised payload, but the request itself capped the window below the
    # bar. Read(file, limit=1) cannot have grounded a claim about a producer.
    exit 0
fi

mkdir -p tmp/session
NOW=$(date -Iseconds)
printf '%s\n' "$NOW" > "tmp/session/.source-read-${SID}"
printf '%s\n' "$NOW" > "tmp/session/.source-read-${KIND}-${SID}"

exit 0
