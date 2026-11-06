# Reboot checkpoint source assets

These are inert text snapshots, not installed production changes. The canonical
inventory checkpoint is committed separately. `manifest.json` names each other
unfinished task-owned source file, its base commit and hashes. Unrelated edits
were excluded. Nothing in this directory is evidence of a test PASS.

On a new machine read the parent handoff first. For each unfinished file, compare
the current canonical bytes with both snapshots. If they match the after hash,
the work is already present. If they match the base hash (or the file is absent
when base_sha256 is null), restore the after bytes through apply_patch in the
main checkout. If neither matches, reconcile the two snapshots with the current
file; never overwrite concurrent work. Preserve each file's intended mode from
the original checkout/private checkpoint where relevant.

Diagnostic overlays contain the original absolute checkout and scratch paths.
Do not execute them unchanged. Reconstruct the original scratch-relative layout
from diagnostics/ using apply_patch, then regenerate each overlay Replace map
with keys rooted at the NEW main checkout and values rooted at its selected
private .go.txt sources. Compare each before snapshot with current canonical
source before using an overlay. The overlay is a Go compiler input in the main
checkout, not a second tree to test. Review refusals and required independent
reviews before running any probe; several drafts remain explicitly refused or
unexecuted. Commands in historical HANDOFF.md files record their original host.

Raw logs, captures, credentials, profiles and test binaries are deliberately
absent. Their private archive location is in the parent handoff. Copy that
archive separately if original proof bytes are needed on the other machine.
