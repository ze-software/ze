#!/bin/bash
set -euo pipefail
cd '/home/thomas/Code/github.com/ze-software/ze/gh-pages'

# ze-commit-script: tmp/commit-408ec569-a-affa48.sh session=408ec569

# Commit a: site: publish bounded VPN kernel proof evidence
# ze-commit-block: tag=a paths='assets/site.css' 'data/release-roadmap.json' 'data/search-index.json' 'llms-full.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/roadmap/index.html' 'project/roadmap/index.md' 'reference/feature-status/index.html' 'reference/feature-status/index.md'
_ze_index="$PWD"/'tmp/commit-408ec569-a-affa48.index'
_ze_message='tmp/commit-msg-408ec569-a-c58326.txt'
if [ ! -f "$_ze_message" ]; then
  echo 'ERROR: commit a (site: publish bounded VPN kernel proof evidence) was not run, and HEAD is unchanged by it.' >&2
  echo "Its message file $_ze_message is gone. A block deletes it once its commit lands, so this block already published on an earlier run of this script, or the file was cleaned up." >&2
  echo 'Check whether it landed with: git log --format='"'"'%h %s'"'"' --fixed-strings --grep='"'"'site: publish bounded VPN kernel proof evidence'"'"'' >&2
  echo "Then prepare whatever is still uncommitted again with ./le commit create, rather than running this script again." >&2
  exit 1
fi
_ze_text=$(git stripspace < "$_ze_message")
if [ -z "$_ze_text" ]; then
  echo 'ERROR: commit a (site: publish bounded VPN kernel proof evidence) was not published, and HEAD is unchanged by it.' >&2
  echo "Its message file $_ze_message holds no message once blank lines are stripped, and git commit refuses an empty message too." >&2
  echo "Write the message into that file, then run this script again." >&2
  exit 1
fi
_ze_status=0
_ze_gpgsign=$(git config --type=bool --get commit.gpgsign) || _ze_status=$?
if [ "$_ze_status" -gt 1 ]; then
  echo 'ERROR: commit a (site: publish bounded VPN kernel proof evidence) was not published, and HEAD is unchanged by it.' >&2
  echo "git config could not read commit.gpgsign as a boolean (its error is above), so whether to sign is unknown; git commit refuses here too." >&2
  echo "Set it with: git config commit.gpgsign true (or false), then run this script again." >&2
  exit 1
fi
_ze_sign=
if [ "$_ze_gpgsign" = true ]; then _ze_sign=1; fi
_ze_parent='beeff4530dc3da64ede69d1e41cb8b1342bc8e93'
_ze_attempt=1
while :; do
_ze_seen=$_ze_parent
_ze_parent=$(git rev-parse --verify 'HEAD^{commit}')
_ze_moved=$(git --literal-pathspecs rev-list "$_ze_seen..$_ze_parent" -- 'assets/site.css' 'data/release-roadmap.json' 'data/search-index.json' 'llms-full.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/roadmap/index.html' 'project/roadmap/index.md' 'reference/feature-status/index.html' 'reference/feature-status/index.md')
_ze_peer=$(printf '%s\n' "$_ze_moved" | grep -v -x -F -f <(printf '%s\n' ${_ze_own:-}) || [ "$?" -eq 1 ])
_ze_names=
if [ -n "$_ze_peer" ]; then
  _ze_names=$(printf '%s\n' "$_ze_peer" | git --literal-pathspecs -c core.quotePath=false diff-tree --stdin -r --no-renames --name-only --no-commit-id -- 'assets/site.css' 'data/release-roadmap.json' 'data/search-index.json' 'llms-full.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/roadmap/index.html' 'project/roadmap/index.md' 'reference/feature-status/index.html' 'reference/feature-status/index.md' | sort -u)
fi
if [ -n "$_ze_peer" ]; then
  echo 'ERROR: commit a (site: publish bounded VPN kernel proof evidence) was not published, and HEAD is unchanged by it.' >&2
  echo "A commit that landed after it was prepared changed paths it carries:" >&2
  printf '%s\n' "$_ze_names" >&2
  echo "Building on that commit would write content prepared before the change over it." >&2
  echo "Read the change with: git log -p $_ze_seen..$_ze_parent -- <path>" >&2
  echo "then fold it into your copy and prepare the commit again with ./le commit create." >&2
  exit 1
fi
rm -f "$_ze_index"
GIT_INDEX_FILE="$_ze_index" git read-tree "$_ze_parent"
GIT_INDEX_FILE="$_ze_index" git update-index --index-info <<'ZE_INDEX_INFO'
100644 a9858c5076bda7703e0c8a51dcb6ab94f3b07086 0	assets/site.css
100644 f944447eb325c9f3a46c3dacd61b8fc18d44cf60 0	data/release-roadmap.json
100644 cfca7908debd79e4551da9f9f930c4bc8bea7f4f 0	data/search-index.json
100644 316526fd6b0eb168ad121d9377a51e34528933f0 0	llms-full.txt
100644 63f3237d79412fdce688dfdad2540cd370f1ff6d 0	project/activity/index.html
100644 3690a6dc6cd903960d0cfdf5084dd49b5bc4e0ee 0	project/activity/index.md
100644 78f71136f43713c39975cbb30c375bebc44f9ca2 0	project/roadmap/index.html
100644 20193bd23eedfcfc4231607256341d4d6427c9b7 0	project/roadmap/index.md
100644 ee2dd56111dedc7f945ba7d76d18f05ea72baff4 0	reference/feature-status/index.html
100644 114acb70a29932b4f9e802e009545c82531fbcd2 0	reference/feature-status/index.md
ZE_INDEX_INFO
_ze_lost=$(git cat-file --batch-check='%(objectname)' <<'ZE_INDEX_INFO' | sed -n 's/ missing$//p'
a9858c5076bda7703e0c8a51dcb6ab94f3b07086
f944447eb325c9f3a46c3dacd61b8fc18d44cf60
cfca7908debd79e4551da9f9f930c4bc8bea7f4f
316526fd6b0eb168ad121d9377a51e34528933f0
63f3237d79412fdce688dfdad2540cd370f1ff6d
3690a6dc6cd903960d0cfdf5084dd49b5bc4e0ee
78f71136f43713c39975cbb30c375bebc44f9ca2
20193bd23eedfcfc4231607256341d4d6427c9b7
ee2dd56111dedc7f945ba7d76d18f05ea72baff4
114acb70a29932b4f9e802e009545c82531fbcd2
ZE_INDEX_INFO
)
if [ -n "$_ze_lost" ]; then
  echo 'ERROR: commit a (site: publish bounded VPN kernel proof evidence) was not published, and HEAD is unchanged by it.' >&2
  echo "The content prepared for these paths is gone from the object store, collected by a git gc:" >&2
  GIT_INDEX_FILE="$_ze_index" git --literal-pathspecs -c core.quotePath=false ls-files -s -- 'assets/site.css' 'data/release-roadmap.json' 'data/search-index.json' 'llms-full.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/roadmap/index.html' 'project/roadmap/index.md' 'reference/feature-status/index.html' 'reference/feature-status/index.md' | grep -F "$_ze_lost" | while read -r _ze_mode _ze_object _ze_stage _ze_path; do echo "invalid object $_ze_mode $_ze_object for '$_ze_path'"; done >&2
  echo "The working tree still holds each file. Prepare the commit again with ./le commit create." >&2
  exit 1
fi
_ze_tree=$(GIT_INDEX_FILE="$_ze_index" git write-tree)
_ze_status=0
_ze_commit=$(printf '%s\n' "$_ze_text" | git commit-tree ${_ze_sign:+-S} -p "$_ze_parent" "$_ze_tree") || _ze_status=$?
if [ "$_ze_status" -ne 0 ]; then
  echo 'ERROR: commit a (site: publish bounded VPN kernel proof evidence) was not published, and HEAD is unchanged by it.' >&2
  echo "git commit-tree could not build the commit (its error is above). When commit.gpgsign is true, signing is the usual cause:" >&2
  echo "make gpg, or gpg.program, sign with user.signingKey, then run this script again." >&2
  exit 1
fi
_ze_changed=$(git diff-tree -r -z --no-renames --name-only "$_ze_parent" "$_ze_commit" | tr '\0' '\n')
if [ -z "$_ze_changed" ]; then
  echo 'ERROR: commit a (site: publish bounded VPN kernel proof evidence) was not published, and HEAD is unchanged by it.' >&2
  echo "HEAD already holds the prepared content of every path it names, so it has nothing to commit." >&2
  echo "Drop this block, or prepare the commit again from your working tree with ./le commit create." >&2
  exit 1
fi
_ze_stray=$(printf '%s\n' "$_ze_changed" | grep -v -x -F -e 'assets/site.css' -e 'data/release-roadmap.json' -e 'data/search-index.json' -e 'llms-full.txt' -e 'project/activity/index.html' -e 'project/activity/index.md' -e 'project/roadmap/index.html' -e 'project/roadmap/index.md' -e 'reference/feature-status/index.html' -e 'reference/feature-status/index.md' || [ "$?" -eq 1 ])
if [ -n "$_ze_stray" ]; then
  echo 'ERROR: commit a (site: publish bounded VPN kernel proof evidence) was not published, and HEAD is unchanged by it.' >&2
  echo "Its tree changes paths the commit does not name, against its parent $_ze_parent:" >&2
  printf '%s\n' "$_ze_stray" >&2
  echo "Publishing it would revert or add work this commit does not own. This is a defect in the" >&2
  echo "generated script: report it with this script's path, then prepare the commit again with ./le commit create." >&2
  exit 1
fi
if _ze_swap=$(git update-ref -m 'commit: site: publish bounded VPN kernel proof evidence' HEAD "$_ze_commit" "$_ze_parent" 2>&1); then
  _ze_own="${_ze_own:-} $_ze_commit"
  break
fi
_ze_now=$(git rev-parse --verify 'HEAD^{commit}')
_ze_lock=$(printf '%s\n' "$_ze_swap" | sed -n "s/.*Unable to create '\(.*\.lock\)'.*/\1/p")
if [ "$_ze_now" = "$_ze_parent" ]; then
  echo 'ERROR: commit a (site: publish bounded VPN kernel proof evidence) was not published, and HEAD is unchanged by it.' >&2
  echo "git update-ref refused to move HEAD, which still names the parent the commit was built on:" >&2
  printf '%s\n' "$_ze_swap" >&2
  if [ -n "$_ze_lock" ]; then echo "Another git process holds $_ze_lock, or one that stopped left it behind. Run this script again once the other git command finishes; remove $_ze_lock only when no git process is running."; else echo "Fix what git names (a permission, for example), then run this script again."; fi >&2
  exit 1
fi
if [ "$_ze_attempt" -ge 5 ]; then
  echo 'ERROR: commit a (site: publish bounded VPN kernel proof evidence) was not published, and HEAD is unchanged by it.' >&2
  echo "HEAD moved under it 5 times in a row while other commits landed." >&2
  echo "Run this script again once the other sessions' commits have landed." >&2
  exit 1
fi
_ze_attempt=$((_ze_attempt + 1))
echo "NOTE: HEAD moved while commit a was being published; building it again on the new HEAD." >&2
done
git -c core.quotePath=false show --stat --format='[%h] %s' "$_ze_commit"
rm -f "$_ze_index.now"
GIT_INDEX_FILE="$_ze_index.now" git add -f -- 'assets/site.css' 'data/release-roadmap.json' 'data/search-index.json' 'llms-full.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/roadmap/index.html' 'project/roadmap/index.md' 'reference/feature-status/index.html' 'reference/feature-status/index.md' 2>/dev/null || true
_ze_drift=$({ GIT_INDEX_FILE="$_ze_index.now" git -c core.quotePath=false ls-files -s -- 'assets/site.css' 'data/release-roadmap.json' 'data/search-index.json' 'llms-full.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/roadmap/index.html' 'project/roadmap/index.md' 'reference/feature-status/index.html' 'reference/feature-status/index.md'; GIT_INDEX_FILE="$_ze_index" git -c core.quotePath=false ls-files -s -- 'assets/site.css' 'data/release-roadmap.json' 'data/search-index.json' 'llms-full.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/roadmap/index.html' 'project/roadmap/index.md' 'reference/feature-status/index.html' 'reference/feature-status/index.md'; } | sort | uniq -u | cut -f2- | sort -u)
rm -f "$_ze_index.now"
if [ -n "$_ze_drift" ]; then
  echo "NOTE: these paths changed on disk after this commit was prepared." >&2
  echo "The commit carries the prepared content; the difference stays in the working tree." >&2
  echo "$_ze_drift" >&2
fi
rm -f "$_ze_message"
# Point the shared index at what was committed. Nothing else in it is touched.
_ze_waited=0
until _ze_said=$({ git ls-tree HEAD -- 'assets/site.css' 'data/release-roadmap.json' 'data/search-index.json' 'llms-full.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/roadmap/index.html' 'project/roadmap/index.md' 'reference/feature-status/index.html' 'reference/feature-status/index.md' | git update-index --index-info; } 2>&1); do
  case "$_ze_said" in
    *index.lock*)
      if [ "$_ze_waited" -lt 30 ]; then
        sleep 1
        _ze_waited=$((_ze_waited + 1))
        continue
      fi
      ;;
  esac
  echo "ERROR: commit $_ze_commit landed, but the shared index could not be pointed at it." >&2
  echo "git said: $_ze_said" >&2
  echo "These paths read as staged changes in every session until the repair runs:" >&2
  printf '%s\n' 'assets/site.css' 'data/release-roadmap.json' 'data/search-index.json' 'llms-full.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/roadmap/index.html' 'project/roadmap/index.md' 'reference/feature-status/index.html' 'reference/feature-status/index.md' >&2
  echo "Any later commit in this script did not run." >&2
  printf 'Run this from %s once .git/index.lock is released:\n' "$PWD" >&2
  echo 'git ls-tree HEAD -- '"'"'assets/site.css'"'"' '"'"'data/release-roadmap.json'"'"' '"'"'data/search-index.json'"'"' '"'"'llms-full.txt'"'"' '"'"'project/activity/index.html'"'"' '"'"'project/activity/index.md'"'"' '"'"'project/roadmap/index.html'"'"' '"'"'project/roadmap/index.md'"'"' '"'"'reference/feature-status/index.html'"'"' '"'"'reference/feature-status/index.md'"'"' | git update-index --index-info' >&2
  echo 'rm -f '"'"'tmp/commit-408ec569-a-affa48.index'"'"'' >&2
  exit 1
done
rm -f 'tmp/commit-408ec569-a-affa48.index'
