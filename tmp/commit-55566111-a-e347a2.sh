#!/bin/bash
set -euo pipefail
cd '/home/thomas/Code/github.com/ze-software/ze/gh-pages'

# ze-commit-script: tmp/commit-55566111-a-e347a2.sh session=55566111

# Commit a: site: slower hero recording, RFC lists filtered per profile
# ze-commit-block: tag=a paths='assets/demos/manifest.json' 'assets/demos/showcase.cast' 'assets/demos/showcase.txt' 'data/release-roadmap.json' 'data/rfc-compliance.json' 'data/rfc-requirements.json' 'data/search-index.json' 'data/site-facts.json' 'demos/terminal/index.html' 'demos/terminal/index.md' 'features/dns-resolver/index.html' 'features/dns-resolver/index.md' 'guides/config-editor/index.html' 'guides/config-editor/index.md' 'index.html' 'llms-full.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/roadmap/index.html' 'project/roadmap/index.md' 'quality/health/index.html' 'quality/health/index.md' 'quality/rfc-compliance/index.html' 'quality/rfc-compliance/index.md' 'quality/rfc-compliance/profiles/dns/index.html' 'quality/rfc-compliance/profiles/dns/index.md' 'quality/rfc-compliance/profiles/fttx-edge/index.html' 'quality/rfc-compliance/profiles/fttx-edge/index.md' 'quality/rfc-compliance/profiles/index.html' 'quality/rfc-compliance/profiles/index.md' 'quality/rfc-compliance/rfc1035/index.html' 'quality/rfc-compliance/rfc1035/index.md' 'quality/rfc-compliance/rfc2181/index.html' 'quality/rfc-compliance/rfc2181/index.md' 'reference/rfcs/index.html' 'reference/rfcs/index.md' 'talks/linx-2026-06/activity.html' 'talks/linx-2026-06/index-inlined.html'
_ze_index="$PWD"/'tmp/commit-55566111-a-e347a2.index'
_ze_message='tmp/commit-msg-55566111-a-012288.txt'
if [ ! -f "$_ze_message" ]; then
  echo 'ERROR: commit a (site: slower hero recording, RFC lists filtered per profile) was not run, and HEAD is unchanged by it.' >&2
  echo "Its message file $_ze_message is gone. A block deletes it once its commit lands, so this block already published on an earlier run of this script, or the file was cleaned up." >&2
  echo 'Check whether it landed with: git log --format='"'"'%h %s'"'"' --fixed-strings --grep='"'"'site: slower hero recording, RFC lists filtered per profile'"'"'' >&2
  echo "Then prepare whatever is still uncommitted again with ./le commit create, rather than running this script again." >&2
  exit 1
fi
_ze_text=$(git stripspace < "$_ze_message")
if [ -z "$_ze_text" ]; then
  echo 'ERROR: commit a (site: slower hero recording, RFC lists filtered per profile) was not published, and HEAD is unchanged by it.' >&2
  echo "Its message file $_ze_message holds no message once blank lines are stripped, and git commit refuses an empty message too." >&2
  echo "Write the message into that file, then run this script again." >&2
  exit 1
fi
_ze_status=0
_ze_gpgsign=$(git config --type=bool --get commit.gpgsign) || _ze_status=$?
if [ "$_ze_status" -gt 1 ]; then
  echo 'ERROR: commit a (site: slower hero recording, RFC lists filtered per profile) was not published, and HEAD is unchanged by it.' >&2
  echo "git config could not read commit.gpgsign as a boolean (its error is above), so whether to sign is unknown; git commit refuses here too." >&2
  echo "Set it with: git config commit.gpgsign true (or false), then run this script again." >&2
  exit 1
fi
_ze_sign=
if [ "$_ze_gpgsign" = true ]; then _ze_sign=1; fi
_ze_parent='9814e321df0e63d09e578af4e2d06e8e8ec2b7fe'
_ze_attempt=1
while :; do
_ze_seen=$_ze_parent
_ze_parent=$(git rev-parse --verify 'HEAD^{commit}')
_ze_moved=$(git --literal-pathspecs rev-list "$_ze_seen..$_ze_parent" -- 'assets/demos/manifest.json' 'assets/demos/showcase.cast' 'assets/demos/showcase.txt' 'data/release-roadmap.json' 'data/rfc-compliance.json' 'data/rfc-requirements.json' 'data/search-index.json' 'data/site-facts.json' 'demos/terminal/index.html' 'demos/terminal/index.md' 'features/dns-resolver/index.html' 'features/dns-resolver/index.md' 'guides/config-editor/index.html' 'guides/config-editor/index.md' 'index.html' 'llms-full.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/roadmap/index.html' 'project/roadmap/index.md' 'quality/health/index.html' 'quality/health/index.md' 'quality/rfc-compliance/index.html' 'quality/rfc-compliance/index.md' 'quality/rfc-compliance/profiles/dns/index.html' 'quality/rfc-compliance/profiles/dns/index.md' 'quality/rfc-compliance/profiles/fttx-edge/index.html' 'quality/rfc-compliance/profiles/fttx-edge/index.md' 'quality/rfc-compliance/profiles/index.html' 'quality/rfc-compliance/profiles/index.md' 'quality/rfc-compliance/rfc1035/index.html' 'quality/rfc-compliance/rfc1035/index.md' 'quality/rfc-compliance/rfc2181/index.html' 'quality/rfc-compliance/rfc2181/index.md' 'reference/rfcs/index.html' 'reference/rfcs/index.md' 'talks/linx-2026-06/activity.html' 'talks/linx-2026-06/index-inlined.html')
_ze_peer=$(printf '%s\n' "$_ze_moved" | grep -v -x -F -f <(printf '%s\n' ${_ze_own:-}) || [ "$?" -eq 1 ])
_ze_names=
if [ -n "$_ze_peer" ]; then
  _ze_names=$(printf '%s\n' "$_ze_peer" | git --literal-pathspecs -c core.quotePath=false diff-tree --stdin -r --no-renames --name-only --no-commit-id -- 'assets/demos/manifest.json' 'assets/demos/showcase.cast' 'assets/demos/showcase.txt' 'data/release-roadmap.json' 'data/rfc-compliance.json' 'data/rfc-requirements.json' 'data/search-index.json' 'data/site-facts.json' 'demos/terminal/index.html' 'demos/terminal/index.md' 'features/dns-resolver/index.html' 'features/dns-resolver/index.md' 'guides/config-editor/index.html' 'guides/config-editor/index.md' 'index.html' 'llms-full.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/roadmap/index.html' 'project/roadmap/index.md' 'quality/health/index.html' 'quality/health/index.md' 'quality/rfc-compliance/index.html' 'quality/rfc-compliance/index.md' 'quality/rfc-compliance/profiles/dns/index.html' 'quality/rfc-compliance/profiles/dns/index.md' 'quality/rfc-compliance/profiles/fttx-edge/index.html' 'quality/rfc-compliance/profiles/fttx-edge/index.md' 'quality/rfc-compliance/profiles/index.html' 'quality/rfc-compliance/profiles/index.md' 'quality/rfc-compliance/rfc1035/index.html' 'quality/rfc-compliance/rfc1035/index.md' 'quality/rfc-compliance/rfc2181/index.html' 'quality/rfc-compliance/rfc2181/index.md' 'reference/rfcs/index.html' 'reference/rfcs/index.md' 'talks/linx-2026-06/activity.html' 'talks/linx-2026-06/index-inlined.html' | sort -u)
fi
if [ -n "$_ze_peer" ]; then
  echo 'ERROR: commit a (site: slower hero recording, RFC lists filtered per profile) was not published, and HEAD is unchanged by it.' >&2
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
100644 028a624326aa3ad0a1e4798d944b6f35de82adf7 0	assets/demos/manifest.json
100644 a46665bb005831cff0e9d7b8d458af7cda6f0a91 0	assets/demos/showcase.cast
100644 442c8b568826c5f6372d4acc1da52765a4426ea2 0	assets/demos/showcase.txt
100644 302aa1bb8c60e7877b80b96d272e833cbac6db5c 0	data/release-roadmap.json
100644 19da0fd314cc345a5ef5a0534f686a1216bef926 0	data/rfc-compliance.json
100644 1476a95f62c0810905f141763acfeec9e19385b0 0	data/rfc-requirements.json
100644 a9d090b68f3b9b7fdfae1bcf529443b0e03ce8d9 0	data/search-index.json
100644 fa44a72ddba3d366e3e5c0d53a645a7dff627bc4 0	data/site-facts.json
100644 5b953c99d99f0bf3955ed52fcb296528f4ab0f44 0	demos/terminal/index.html
100644 3ebf57ad217b0edbf693b021f6bdbd8dde0ff08e 0	demos/terminal/index.md
100644 e0d539c5c76b382015fecda3ebac31b76dbf0060 0	features/dns-resolver/index.html
100644 73497f09eb0d17bcb50b1adee106a0bb64489e7b 0	features/dns-resolver/index.md
100644 3ef1f4f61c4fdb277ed9611956bbb5d187fa0a9e 0	guides/config-editor/index.html
100644 a2cb822e703305b1b9835472abfae62782bf84ce 0	guides/config-editor/index.md
100644 7e0ca0544446e0e899fe599696573467742338d9 0	index.html
100644 853515b05e35ed5a8ca4a037586603f8046741e6 0	llms-full.txt
100644 f50fc36a8ca51ce1df0ceb324dd0c4a85bcfe62a 0	project/activity/index.html
100644 4739db61711353a1f089839d2429fa25eee4cb21 0	project/activity/index.md
100644 cac4a95e686e62c0f742f9b4110d024ec35836b1 0	project/roadmap/index.html
100644 a05afa310807cabe22dc37ab2e16f57034c139b1 0	project/roadmap/index.md
100644 7749f6223f6f780af0b0ceb9b84e905718d88c14 0	quality/health/index.html
100644 ce06895ca20605205033ef43713fa59c4c8443fa 0	quality/health/index.md
100644 66d118138ddf82ab87f493a2fd3088607897d8c7 0	quality/rfc-compliance/index.html
100644 c1bbd0d6a669ba64e99be1616de783248642caf7 0	quality/rfc-compliance/index.md
100644 401da6e1cb47ab1c0f210315f4a94aa0423d22b6 0	quality/rfc-compliance/profiles/dns/index.html
100644 2bd35f5bdcc3e45d1fd97ccea4b1779bf5ae1ed5 0	quality/rfc-compliance/profiles/dns/index.md
100644 53cc7fbe4162a5ca71918d92beb22e062102acb5 0	quality/rfc-compliance/profiles/fttx-edge/index.html
100644 a2f29567e5678f69c034c3e4220916487061aea9 0	quality/rfc-compliance/profiles/fttx-edge/index.md
100644 264d95eedc9d0abab67961aa1889be514a661d1d 0	quality/rfc-compliance/profiles/index.html
100644 f10fa612f9dc7661bd91afd40aa8a9349671904a 0	quality/rfc-compliance/profiles/index.md
100644 1c221c319df83a51dc0161d8530cc53873722028 0	quality/rfc-compliance/rfc1035/index.html
100644 fc5823b236e048f474f72ab29b84cca2e2b37b31 0	quality/rfc-compliance/rfc1035/index.md
100644 8b9124217025a3818bc73be91777b9c346d79b83 0	quality/rfc-compliance/rfc2181/index.html
100644 027bb66ea55eb29c0c964c6007e5e21bf3a32703 0	quality/rfc-compliance/rfc2181/index.md
100644 64c9b2ed456c6624346dfec23bafadcb6ec288fb 0	reference/rfcs/index.html
100644 80cde4ea9baa472f685a2a9c4add88dfd51d5d2c 0	reference/rfcs/index.md
100644 a58c692e31b3ffec76e6a35a212f512ba36b97f9 0	talks/linx-2026-06/activity.html
100644 9f0a1df64229ccd93aaf6329757506bd49ebc340 0	talks/linx-2026-06/index-inlined.html
ZE_INDEX_INFO
_ze_lost=$(git cat-file --batch-check='%(objectname)' <<'ZE_INDEX_INFO' | sed -n 's/ missing$//p'
028a624326aa3ad0a1e4798d944b6f35de82adf7
a46665bb005831cff0e9d7b8d458af7cda6f0a91
442c8b568826c5f6372d4acc1da52765a4426ea2
302aa1bb8c60e7877b80b96d272e833cbac6db5c
19da0fd314cc345a5ef5a0534f686a1216bef926
1476a95f62c0810905f141763acfeec9e19385b0
a9d090b68f3b9b7fdfae1bcf529443b0e03ce8d9
fa44a72ddba3d366e3e5c0d53a645a7dff627bc4
5b953c99d99f0bf3955ed52fcb296528f4ab0f44
3ebf57ad217b0edbf693b021f6bdbd8dde0ff08e
e0d539c5c76b382015fecda3ebac31b76dbf0060
73497f09eb0d17bcb50b1adee106a0bb64489e7b
3ef1f4f61c4fdb277ed9611956bbb5d187fa0a9e
a2cb822e703305b1b9835472abfae62782bf84ce
7e0ca0544446e0e899fe599696573467742338d9
853515b05e35ed5a8ca4a037586603f8046741e6
f50fc36a8ca51ce1df0ceb324dd0c4a85bcfe62a
4739db61711353a1f089839d2429fa25eee4cb21
cac4a95e686e62c0f742f9b4110d024ec35836b1
a05afa310807cabe22dc37ab2e16f57034c139b1
7749f6223f6f780af0b0ceb9b84e905718d88c14
ce06895ca20605205033ef43713fa59c4c8443fa
66d118138ddf82ab87f493a2fd3088607897d8c7
c1bbd0d6a669ba64e99be1616de783248642caf7
401da6e1cb47ab1c0f210315f4a94aa0423d22b6
2bd35f5bdcc3e45d1fd97ccea4b1779bf5ae1ed5
53cc7fbe4162a5ca71918d92beb22e062102acb5
a2f29567e5678f69c034c3e4220916487061aea9
264d95eedc9d0abab67961aa1889be514a661d1d
f10fa612f9dc7661bd91afd40aa8a9349671904a
1c221c319df83a51dc0161d8530cc53873722028
fc5823b236e048f474f72ab29b84cca2e2b37b31
8b9124217025a3818bc73be91777b9c346d79b83
027bb66ea55eb29c0c964c6007e5e21bf3a32703
64c9b2ed456c6624346dfec23bafadcb6ec288fb
80cde4ea9baa472f685a2a9c4add88dfd51d5d2c
a58c692e31b3ffec76e6a35a212f512ba36b97f9
9f0a1df64229ccd93aaf6329757506bd49ebc340
ZE_INDEX_INFO
)
if [ -n "$_ze_lost" ]; then
  echo 'ERROR: commit a (site: slower hero recording, RFC lists filtered per profile) was not published, and HEAD is unchanged by it.' >&2
  echo "The content prepared for these paths is gone from the object store, collected by a git gc:" >&2
  GIT_INDEX_FILE="$_ze_index" git --literal-pathspecs -c core.quotePath=false ls-files -s -- 'assets/demos/manifest.json' 'assets/demos/showcase.cast' 'assets/demos/showcase.txt' 'data/release-roadmap.json' 'data/rfc-compliance.json' 'data/rfc-requirements.json' 'data/search-index.json' 'data/site-facts.json' 'demos/terminal/index.html' 'demos/terminal/index.md' 'features/dns-resolver/index.html' 'features/dns-resolver/index.md' 'guides/config-editor/index.html' 'guides/config-editor/index.md' 'index.html' 'llms-full.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/roadmap/index.html' 'project/roadmap/index.md' 'quality/health/index.html' 'quality/health/index.md' 'quality/rfc-compliance/index.html' 'quality/rfc-compliance/index.md' 'quality/rfc-compliance/profiles/dns/index.html' 'quality/rfc-compliance/profiles/dns/index.md' 'quality/rfc-compliance/profiles/fttx-edge/index.html' 'quality/rfc-compliance/profiles/fttx-edge/index.md' 'quality/rfc-compliance/profiles/index.html' 'quality/rfc-compliance/profiles/index.md' 'quality/rfc-compliance/rfc1035/index.html' 'quality/rfc-compliance/rfc1035/index.md' 'quality/rfc-compliance/rfc2181/index.html' 'quality/rfc-compliance/rfc2181/index.md' 'reference/rfcs/index.html' 'reference/rfcs/index.md' 'talks/linx-2026-06/activity.html' 'talks/linx-2026-06/index-inlined.html' | grep -F "$_ze_lost" | while read -r _ze_mode _ze_object _ze_stage _ze_path; do echo "invalid object $_ze_mode $_ze_object for '$_ze_path'"; done >&2
  echo "The working tree still holds each file. Prepare the commit again with ./le commit create." >&2
  exit 1
fi
_ze_tree=$(GIT_INDEX_FILE="$_ze_index" git write-tree)
_ze_status=0
_ze_commit=$(printf '%s\n' "$_ze_text" | git commit-tree ${_ze_sign:+-S} -p "$_ze_parent" "$_ze_tree") || _ze_status=$?
if [ "$_ze_status" -ne 0 ]; then
  echo 'ERROR: commit a (site: slower hero recording, RFC lists filtered per profile) was not published, and HEAD is unchanged by it.' >&2
  echo "git commit-tree could not build the commit (its error is above). When commit.gpgsign is true, signing is the usual cause:" >&2
  echo "make gpg, or gpg.program, sign with user.signingKey, then run this script again." >&2
  exit 1
fi
_ze_changed=$(git diff-tree -r -z --no-renames --name-only "$_ze_parent" "$_ze_commit" | tr '\0' '\n')
if [ -z "$_ze_changed" ]; then
  echo 'ERROR: commit a (site: slower hero recording, RFC lists filtered per profile) was not published, and HEAD is unchanged by it.' >&2
  echo "HEAD already holds the prepared content of every path it names, so it has nothing to commit." >&2
  echo "Drop this block, or prepare the commit again from your working tree with ./le commit create." >&2
  exit 1
fi
_ze_stray=$(printf '%s\n' "$_ze_changed" | grep -v -x -F -e 'assets/demos/manifest.json' -e 'assets/demos/showcase.cast' -e 'assets/demos/showcase.txt' -e 'data/release-roadmap.json' -e 'data/rfc-compliance.json' -e 'data/rfc-requirements.json' -e 'data/search-index.json' -e 'data/site-facts.json' -e 'demos/terminal/index.html' -e 'demos/terminal/index.md' -e 'features/dns-resolver/index.html' -e 'features/dns-resolver/index.md' -e 'guides/config-editor/index.html' -e 'guides/config-editor/index.md' -e 'index.html' -e 'llms-full.txt' -e 'project/activity/index.html' -e 'project/activity/index.md' -e 'project/roadmap/index.html' -e 'project/roadmap/index.md' -e 'quality/health/index.html' -e 'quality/health/index.md' -e 'quality/rfc-compliance/index.html' -e 'quality/rfc-compliance/index.md' -e 'quality/rfc-compliance/profiles/dns/index.html' -e 'quality/rfc-compliance/profiles/dns/index.md' -e 'quality/rfc-compliance/profiles/fttx-edge/index.html' -e 'quality/rfc-compliance/profiles/fttx-edge/index.md' -e 'quality/rfc-compliance/profiles/index.html' -e 'quality/rfc-compliance/profiles/index.md' -e 'quality/rfc-compliance/rfc1035/index.html' -e 'quality/rfc-compliance/rfc1035/index.md' -e 'quality/rfc-compliance/rfc2181/index.html' -e 'quality/rfc-compliance/rfc2181/index.md' -e 'reference/rfcs/index.html' -e 'reference/rfcs/index.md' -e 'talks/linx-2026-06/activity.html' -e 'talks/linx-2026-06/index-inlined.html' || [ "$?" -eq 1 ])
if [ -n "$_ze_stray" ]; then
  echo 'ERROR: commit a (site: slower hero recording, RFC lists filtered per profile) was not published, and HEAD is unchanged by it.' >&2
  echo "Its tree changes paths the commit does not name, against its parent $_ze_parent:" >&2
  printf '%s\n' "$_ze_stray" >&2
  echo "Publishing it would revert or add work this commit does not own. This is a defect in the" >&2
  echo "generated script: report it with this script's path, then prepare the commit again with ./le commit create." >&2
  exit 1
fi
if _ze_swap=$(git update-ref -m 'commit: site: slower hero recording, RFC lists filtered per profile' HEAD "$_ze_commit" "$_ze_parent" 2>&1); then
  _ze_own="${_ze_own:-} $_ze_commit"
  break
fi
_ze_now=$(git rev-parse --verify 'HEAD^{commit}')
_ze_lock=$(printf '%s\n' "$_ze_swap" | sed -n "s/.*Unable to create '\(.*\.lock\)'.*/\1/p")
if [ "$_ze_now" = "$_ze_parent" ]; then
  echo 'ERROR: commit a (site: slower hero recording, RFC lists filtered per profile) was not published, and HEAD is unchanged by it.' >&2
  echo "git update-ref refused to move HEAD, which still names the parent the commit was built on:" >&2
  printf '%s\n' "$_ze_swap" >&2
  if [ -n "$_ze_lock" ]; then echo "Another git process holds $_ze_lock, or one that stopped left it behind. Run this script again once the other git command finishes; remove $_ze_lock only when no git process is running."; else echo "Fix what git names (a permission, for example), then run this script again."; fi >&2
  exit 1
fi
if [ "$_ze_attempt" -ge 5 ]; then
  echo 'ERROR: commit a (site: slower hero recording, RFC lists filtered per profile) was not published, and HEAD is unchanged by it.' >&2
  echo "HEAD moved under it 5 times in a row while other commits landed." >&2
  echo "Run this script again once the other sessions' commits have landed." >&2
  exit 1
fi
_ze_attempt=$((_ze_attempt + 1))
echo "NOTE: HEAD moved while commit a was being published; building it again on the new HEAD." >&2
done
git -c core.quotePath=false show --stat --format='[%h] %s' "$_ze_commit"
rm -f "$_ze_index.now"
GIT_INDEX_FILE="$_ze_index.now" git add -f -- 'assets/demos/manifest.json' 'assets/demos/showcase.cast' 'assets/demos/showcase.txt' 'data/release-roadmap.json' 'data/rfc-compliance.json' 'data/rfc-requirements.json' 'data/search-index.json' 'data/site-facts.json' 'demos/terminal/index.html' 'demos/terminal/index.md' 'features/dns-resolver/index.html' 'features/dns-resolver/index.md' 'guides/config-editor/index.html' 'guides/config-editor/index.md' 'index.html' 'llms-full.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/roadmap/index.html' 'project/roadmap/index.md' 'quality/health/index.html' 'quality/health/index.md' 'quality/rfc-compliance/index.html' 'quality/rfc-compliance/index.md' 'quality/rfc-compliance/profiles/dns/index.html' 'quality/rfc-compliance/profiles/dns/index.md' 'quality/rfc-compliance/profiles/fttx-edge/index.html' 'quality/rfc-compliance/profiles/fttx-edge/index.md' 'quality/rfc-compliance/profiles/index.html' 'quality/rfc-compliance/profiles/index.md' 'quality/rfc-compliance/rfc1035/index.html' 'quality/rfc-compliance/rfc1035/index.md' 'quality/rfc-compliance/rfc2181/index.html' 'quality/rfc-compliance/rfc2181/index.md' 'reference/rfcs/index.html' 'reference/rfcs/index.md' 'talks/linx-2026-06/activity.html' 'talks/linx-2026-06/index-inlined.html' 2>/dev/null || true
_ze_drift=$({ GIT_INDEX_FILE="$_ze_index.now" git -c core.quotePath=false ls-files -s -- 'assets/demos/manifest.json' 'assets/demos/showcase.cast' 'assets/demos/showcase.txt' 'data/release-roadmap.json' 'data/rfc-compliance.json' 'data/rfc-requirements.json' 'data/search-index.json' 'data/site-facts.json' 'demos/terminal/index.html' 'demos/terminal/index.md' 'features/dns-resolver/index.html' 'features/dns-resolver/index.md' 'guides/config-editor/index.html' 'guides/config-editor/index.md' 'index.html' 'llms-full.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/roadmap/index.html' 'project/roadmap/index.md' 'quality/health/index.html' 'quality/health/index.md' 'quality/rfc-compliance/index.html' 'quality/rfc-compliance/index.md' 'quality/rfc-compliance/profiles/dns/index.html' 'quality/rfc-compliance/profiles/dns/index.md' 'quality/rfc-compliance/profiles/fttx-edge/index.html' 'quality/rfc-compliance/profiles/fttx-edge/index.md' 'quality/rfc-compliance/profiles/index.html' 'quality/rfc-compliance/profiles/index.md' 'quality/rfc-compliance/rfc1035/index.html' 'quality/rfc-compliance/rfc1035/index.md' 'quality/rfc-compliance/rfc2181/index.html' 'quality/rfc-compliance/rfc2181/index.md' 'reference/rfcs/index.html' 'reference/rfcs/index.md' 'talks/linx-2026-06/activity.html' 'talks/linx-2026-06/index-inlined.html'; GIT_INDEX_FILE="$_ze_index" git -c core.quotePath=false ls-files -s -- 'assets/demos/manifest.json' 'assets/demos/showcase.cast' 'assets/demos/showcase.txt' 'data/release-roadmap.json' 'data/rfc-compliance.json' 'data/rfc-requirements.json' 'data/search-index.json' 'data/site-facts.json' 'demos/terminal/index.html' 'demos/terminal/index.md' 'features/dns-resolver/index.html' 'features/dns-resolver/index.md' 'guides/config-editor/index.html' 'guides/config-editor/index.md' 'index.html' 'llms-full.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/roadmap/index.html' 'project/roadmap/index.md' 'quality/health/index.html' 'quality/health/index.md' 'quality/rfc-compliance/index.html' 'quality/rfc-compliance/index.md' 'quality/rfc-compliance/profiles/dns/index.html' 'quality/rfc-compliance/profiles/dns/index.md' 'quality/rfc-compliance/profiles/fttx-edge/index.html' 'quality/rfc-compliance/profiles/fttx-edge/index.md' 'quality/rfc-compliance/profiles/index.html' 'quality/rfc-compliance/profiles/index.md' 'quality/rfc-compliance/rfc1035/index.html' 'quality/rfc-compliance/rfc1035/index.md' 'quality/rfc-compliance/rfc2181/index.html' 'quality/rfc-compliance/rfc2181/index.md' 'reference/rfcs/index.html' 'reference/rfcs/index.md' 'talks/linx-2026-06/activity.html' 'talks/linx-2026-06/index-inlined.html'; } | sort | uniq -u | cut -f2- | sort -u)
rm -f "$_ze_index.now"
if [ -n "$_ze_drift" ]; then
  echo "NOTE: these paths changed on disk after this commit was prepared." >&2
  echo "The commit carries the prepared content; the difference stays in the working tree." >&2
  echo "$_ze_drift" >&2
fi
rm -f "$_ze_message"
# Point the shared index at what was committed. Nothing else in it is touched.
_ze_waited=0
until _ze_said=$({ git ls-tree HEAD -- 'assets/demos/manifest.json' 'assets/demos/showcase.cast' 'assets/demos/showcase.txt' 'data/release-roadmap.json' 'data/rfc-compliance.json' 'data/rfc-requirements.json' 'data/search-index.json' 'data/site-facts.json' 'demos/terminal/index.html' 'demos/terminal/index.md' 'features/dns-resolver/index.html' 'features/dns-resolver/index.md' 'guides/config-editor/index.html' 'guides/config-editor/index.md' 'index.html' 'llms-full.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/roadmap/index.html' 'project/roadmap/index.md' 'quality/health/index.html' 'quality/health/index.md' 'quality/rfc-compliance/index.html' 'quality/rfc-compliance/index.md' 'quality/rfc-compliance/profiles/dns/index.html' 'quality/rfc-compliance/profiles/dns/index.md' 'quality/rfc-compliance/profiles/fttx-edge/index.html' 'quality/rfc-compliance/profiles/fttx-edge/index.md' 'quality/rfc-compliance/profiles/index.html' 'quality/rfc-compliance/profiles/index.md' 'quality/rfc-compliance/rfc1035/index.html' 'quality/rfc-compliance/rfc1035/index.md' 'quality/rfc-compliance/rfc2181/index.html' 'quality/rfc-compliance/rfc2181/index.md' 'reference/rfcs/index.html' 'reference/rfcs/index.md' 'talks/linx-2026-06/activity.html' 'talks/linx-2026-06/index-inlined.html' | git update-index --index-info; } 2>&1); do
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
  printf '%s\n' 'assets/demos/manifest.json' 'assets/demos/showcase.cast' 'assets/demos/showcase.txt' 'data/release-roadmap.json' 'data/rfc-compliance.json' 'data/rfc-requirements.json' 'data/search-index.json' 'data/site-facts.json' 'demos/terminal/index.html' 'demos/terminal/index.md' 'features/dns-resolver/index.html' 'features/dns-resolver/index.md' 'guides/config-editor/index.html' 'guides/config-editor/index.md' 'index.html' 'llms-full.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/roadmap/index.html' 'project/roadmap/index.md' 'quality/health/index.html' 'quality/health/index.md' 'quality/rfc-compliance/index.html' 'quality/rfc-compliance/index.md' 'quality/rfc-compliance/profiles/dns/index.html' 'quality/rfc-compliance/profiles/dns/index.md' 'quality/rfc-compliance/profiles/fttx-edge/index.html' 'quality/rfc-compliance/profiles/fttx-edge/index.md' 'quality/rfc-compliance/profiles/index.html' 'quality/rfc-compliance/profiles/index.md' 'quality/rfc-compliance/rfc1035/index.html' 'quality/rfc-compliance/rfc1035/index.md' 'quality/rfc-compliance/rfc2181/index.html' 'quality/rfc-compliance/rfc2181/index.md' 'reference/rfcs/index.html' 'reference/rfcs/index.md' 'talks/linx-2026-06/activity.html' 'talks/linx-2026-06/index-inlined.html' >&2
  echo "Any later commit in this script did not run." >&2
  printf 'Run this from %s once .git/index.lock is released:\n' "$PWD" >&2
  echo 'git ls-tree HEAD -- '"'"'assets/demos/manifest.json'"'"' '"'"'assets/demos/showcase.cast'"'"' '"'"'assets/demos/showcase.txt'"'"' '"'"'data/release-roadmap.json'"'"' '"'"'data/rfc-compliance.json'"'"' '"'"'data/rfc-requirements.json'"'"' '"'"'data/search-index.json'"'"' '"'"'data/site-facts.json'"'"' '"'"'demos/terminal/index.html'"'"' '"'"'demos/terminal/index.md'"'"' '"'"'features/dns-resolver/index.html'"'"' '"'"'features/dns-resolver/index.md'"'"' '"'"'guides/config-editor/index.html'"'"' '"'"'guides/config-editor/index.md'"'"' '"'"'index.html'"'"' '"'"'llms-full.txt'"'"' '"'"'project/activity/index.html'"'"' '"'"'project/activity/index.md'"'"' '"'"'project/roadmap/index.html'"'"' '"'"'project/roadmap/index.md'"'"' '"'"'quality/health/index.html'"'"' '"'"'quality/health/index.md'"'"' '"'"'quality/rfc-compliance/index.html'"'"' '"'"'quality/rfc-compliance/index.md'"'"' '"'"'quality/rfc-compliance/profiles/dns/index.html'"'"' '"'"'quality/rfc-compliance/profiles/dns/index.md'"'"' '"'"'quality/rfc-compliance/profiles/fttx-edge/index.html'"'"' '"'"'quality/rfc-compliance/profiles/fttx-edge/index.md'"'"' '"'"'quality/rfc-compliance/profiles/index.html'"'"' '"'"'quality/rfc-compliance/profiles/index.md'"'"' '"'"'quality/rfc-compliance/rfc1035/index.html'"'"' '"'"'quality/rfc-compliance/rfc1035/index.md'"'"' '"'"'quality/rfc-compliance/rfc2181/index.html'"'"' '"'"'quality/rfc-compliance/rfc2181/index.md'"'"' '"'"'reference/rfcs/index.html'"'"' '"'"'reference/rfcs/index.md'"'"' '"'"'talks/linx-2026-06/activity.html'"'"' '"'"'talks/linx-2026-06/index-inlined.html'"'"' | git update-index --index-info' >&2
  echo 'rm -f '"'"'tmp/commit-55566111-a-e347a2.index'"'"'' >&2
  exit 1
done
rm -f 'tmp/commit-55566111-a-e347a2.index'
