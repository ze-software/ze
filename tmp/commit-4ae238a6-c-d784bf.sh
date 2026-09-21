#!/bin/bash
set -euo pipefail
cd '/Users/thomas/Code/github.com/ze-software/ze/gh-pages'

# ze-commit-script: tmp/commit-4ae238a6-c-d784bf.sh session=4ae238a6

# Commit c: site: publish the week of 2026-09-14 and who implements each RFC
# ze-commit-block: tag=c paths='changes/feed.xml' 'compare/bgp/index.html' 'compare/bgp/index.md' 'data/changes.json' 'data/milestones.json' 'data/rfc-compliance.json' 'data/rfc-requirements.json' 'data/search-index.json' 'data/site-facts.json' 'index.html' 'index.md' 'llms-full.txt' 'llms.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/changes/feed.xml' 'project/changes/index.html' 'project/changes/index.md' 'project/milestones/index.html' 'project/milestones/index.md' 'quality/health/index.html' 'quality/health/index.md' 'quality/rfc-compliance/index.html' 'quality/rfc-compliance/index.md' 'quality/rfc-compliance/rfc2003/index.html' 'quality/rfc-compliance/rfc2003/index.md' 'quality/rfc-compliance/rfc2473/index.html' 'quality/rfc-compliance/rfc2473/index.md' 'quality/rfc-compliance/rfc2784/index.html' 'quality/rfc-compliance/rfc2784/index.md' 'quality/rfc-compliance/rfc2890/index.html' 'quality/rfc-compliance/rfc2890/index.md' 'quality/rfc-compliance/rfc3031/index.html' 'quality/rfc-compliance/rfc3031/index.md' 'quality/rfc-compliance/rfc4862/index.html' 'quality/rfc-compliance/rfc4862/index.md' 'quality/rfc-compliance/rfc6071/index.html' 'quality/rfc-compliance/rfc6071/index.md' 'quality/rfc-compliance/rfc6482/index.html' 'quality/rfc-compliance/rfc6482/index.md' 'quality/rfc-compliance/rfc7854/index.html' 'quality/rfc-compliance/rfc7854/index.md' 'quality/rfc-compliance/rfc9319/index.html' 'quality/rfc-compliance/rfc9319/index.md' 'quality/rfc-compliance/rfc9384/index.html' 'quality/rfc-compliance/rfc9384/index.md' 'quality/rfc-compliance/rfc9582/index.html' 'quality/rfc-compliance/rfc9582/index.md' 'sitemap.xml' 'talks/linx-2026-06/activity.html' 'talks/linx-2026-06/index-inlined.html' 'changes/discord/2026-09-14-weekly.md' 'project/changes/2026-09-14/index.html' 'project/changes/2026-09-14/index.md'
_ze_index="$PWD"/'tmp/commit-4ae238a6-c-d784bf.index'
rm -f "$_ze_index"
GIT_INDEX_FILE="$_ze_index" git read-tree HEAD
GIT_INDEX_FILE="$_ze_index" git update-index --index-info <<'ZE_INDEX_INFO'
100644 a2440ccb21659461578ec671875a00146a6aac61 0	changes/discord/2026-09-14-weekly.md
100644 111470ef211e90773aeee202533781305077cd03 0	changes/feed.xml
100644 33de5e179e6074e2d6f3f82795ffef13832df79b 0	compare/bgp/index.html
100644 e9492735416c6c7e5e790534a5ff9a95bf3ad623 0	compare/bgp/index.md
100644 96f49a7a412787abda6b7cc699a59589a2724953 0	data/changes.json
100644 8759302a7b0b9b7f9989daa3156f5a7df4f3c884 0	data/milestones.json
100644 0dde80ba6e30bd015a857bf8ce8779bf19923c23 0	data/rfc-compliance.json
100644 537947d3be11c9a718eb2ad21350773bffd780dc 0	data/rfc-requirements.json
100644 4fd1a227871cae30c20c2f0213de08b0db5c81b0 0	data/search-index.json
100644 fc3afb05e18cd0ca99e2b9fd79a55d79bff09a39 0	data/site-facts.json
100644 44cbbd3ff1c18fb6106a37898520e9557a7ab8fb 0	index.html
100644 624d3c534639e344beade529edc06e68f5e133e5 0	index.md
100644 c045960e7a28caaf299f87cb1b90f3494d59f3ef 0	llms-full.txt
100644 457926725fd71a65c10d0007f4197cfa091416a5 0	llms.txt
100644 9053a8906bb2cbac52e12dabab4448adda868252 0	project/activity/index.html
100644 0b032453ebdef504ea302a2729f6299b13246f26 0	project/activity/index.md
100644 2e1f86193c4b5265aed851b5a45bf0b8c8226f4d 0	project/changes/2026-09-14/index.html
100644 e5f35d3e519fecd4ed34f1c9aaf79f940ad6b9d7 0	project/changes/2026-09-14/index.md
100644 111470ef211e90773aeee202533781305077cd03 0	project/changes/feed.xml
100644 d20a509103e587215abcfe7cee7c487bdcc97e8c 0	project/changes/index.html
100644 097b765d1ca7fe2f38dd8b3ccc86ad32b0dd12b5 0	project/changes/index.md
100644 71bcca6aa5d2c535e6dcd0688102acb686ea7b69 0	project/milestones/index.html
100644 993fa7bbc613ae49125055ccfcb58cf678fe71d0 0	project/milestones/index.md
100644 7a2d2a45cc27cb60f6e642851a5c49bee091ce83 0	quality/health/index.html
100644 fd1a08933c2757b803f5fe0994b53d6637955ed4 0	quality/health/index.md
100644 0c0af283a6f84fbb7ec2b2df7bd3e3cae54ede1b 0	quality/rfc-compliance/index.html
100644 f20275c58987a6430af84819a1935d6b51b78269 0	quality/rfc-compliance/index.md
100644 1938f6a08e1715cf9c4a394caa37d033bed8e896 0	quality/rfc-compliance/rfc2003/index.html
100644 e835981eec1fb950a40ade6552c14ecffb4d3fd8 0	quality/rfc-compliance/rfc2003/index.md
100644 5157674b756cca719034a4aa25ef952fe783ebba 0	quality/rfc-compliance/rfc2473/index.html
100644 9a8ee311ea641cd7aff06a56e1238fbd8f76e001 0	quality/rfc-compliance/rfc2473/index.md
100644 329422c0ddd7631e50d7a8fe7d4944ad71621833 0	quality/rfc-compliance/rfc2784/index.html
100644 91f95fb4430e15d114af124451f35983990a1afa 0	quality/rfc-compliance/rfc2784/index.md
100644 2730c9bacdd062d6023b408e1066e72a05846df7 0	quality/rfc-compliance/rfc2890/index.html
100644 a5bd117ef53bf96b5d953bb0e939ce604f5ffbc9 0	quality/rfc-compliance/rfc2890/index.md
100644 d4cf3a9fbd1bfaf9983f88f3fe56d2a697181bf0 0	quality/rfc-compliance/rfc3031/index.html
100644 7b75ee8f25ad56d2eb4241367ec9ef0812ff04ce 0	quality/rfc-compliance/rfc3031/index.md
100644 ac72e4d3f1ac646be8746519f006ebb6dfbdc4e1 0	quality/rfc-compliance/rfc4862/index.html
100644 195374ebd216ef1694ce70df93c911c446d74c24 0	quality/rfc-compliance/rfc4862/index.md
100644 347f38ca138fe29be768215bc449f184bc4068c3 0	quality/rfc-compliance/rfc6071/index.html
100644 b483eaf1138539f71cd2cf7b5f3eecd223dadeb8 0	quality/rfc-compliance/rfc6071/index.md
100644 14594a6cff10a598c08160d06ac0d4678126a61f 0	quality/rfc-compliance/rfc6482/index.html
100644 64ab7b30139ccd8983f4ea978be2b89d1bf575f2 0	quality/rfc-compliance/rfc6482/index.md
100644 be61c47c16d9ebcbaff4b45c7fe7aaa1d64a1ef4 0	quality/rfc-compliance/rfc7854/index.html
100644 9a063ce48ac5a0659e67f5b001f98068449e2b54 0	quality/rfc-compliance/rfc7854/index.md
100644 1e691a9fe7f4daa3693578b815803acda8b7daa0 0	quality/rfc-compliance/rfc9319/index.html
100644 cf5fdaf9816939108820f40404556c09a3fafc2b 0	quality/rfc-compliance/rfc9319/index.md
100644 9dcba22e5f65a120a8e5543300a410c8f3c47197 0	quality/rfc-compliance/rfc9384/index.html
100644 143d486a27e31c3868c0e1e14bdcefcc6604206b 0	quality/rfc-compliance/rfc9384/index.md
100644 275ee572e37855ea47598345f6add7a2af5e7f90 0	quality/rfc-compliance/rfc9582/index.html
100644 09fa808b04074da753e2435ccac98dc4b7a476f2 0	quality/rfc-compliance/rfc9582/index.md
100644 20d1b6c368cd552c2c9d90e68e007523d96337f5 0	sitemap.xml
100644 46754fcf701aa272cfb88d2a1deb26dcb9316098 0	talks/linx-2026-06/activity.html
100644 087a7c3228f89498ed5b9a3ffe2dd28857f32d49 0	talks/linx-2026-06/index-inlined.html
ZE_INDEX_INFO
rm -f "$_ze_index.now"
GIT_INDEX_FILE="$_ze_index.now" git add -f -- 'changes/feed.xml' 'compare/bgp/index.html' 'compare/bgp/index.md' 'data/changes.json' 'data/milestones.json' 'data/rfc-compliance.json' 'data/rfc-requirements.json' 'data/search-index.json' 'data/site-facts.json' 'index.html' 'index.md' 'llms-full.txt' 'llms.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/changes/feed.xml' 'project/changes/index.html' 'project/changes/index.md' 'project/milestones/index.html' 'project/milestones/index.md' 'quality/health/index.html' 'quality/health/index.md' 'quality/rfc-compliance/index.html' 'quality/rfc-compliance/index.md' 'quality/rfc-compliance/rfc2003/index.html' 'quality/rfc-compliance/rfc2003/index.md' 'quality/rfc-compliance/rfc2473/index.html' 'quality/rfc-compliance/rfc2473/index.md' 'quality/rfc-compliance/rfc2784/index.html' 'quality/rfc-compliance/rfc2784/index.md' 'quality/rfc-compliance/rfc2890/index.html' 'quality/rfc-compliance/rfc2890/index.md' 'quality/rfc-compliance/rfc3031/index.html' 'quality/rfc-compliance/rfc3031/index.md' 'quality/rfc-compliance/rfc4862/index.html' 'quality/rfc-compliance/rfc4862/index.md' 'quality/rfc-compliance/rfc6071/index.html' 'quality/rfc-compliance/rfc6071/index.md' 'quality/rfc-compliance/rfc6482/index.html' 'quality/rfc-compliance/rfc6482/index.md' 'quality/rfc-compliance/rfc7854/index.html' 'quality/rfc-compliance/rfc7854/index.md' 'quality/rfc-compliance/rfc9319/index.html' 'quality/rfc-compliance/rfc9319/index.md' 'quality/rfc-compliance/rfc9384/index.html' 'quality/rfc-compliance/rfc9384/index.md' 'quality/rfc-compliance/rfc9582/index.html' 'quality/rfc-compliance/rfc9582/index.md' 'sitemap.xml' 'talks/linx-2026-06/activity.html' 'talks/linx-2026-06/index-inlined.html' 'changes/discord/2026-09-14-weekly.md' 'project/changes/2026-09-14/index.html' 'project/changes/2026-09-14/index.md' 2>/dev/null || true
_ze_drift=$({ GIT_INDEX_FILE="$_ze_index.now" git -c core.quotePath=false ls-files -s -- 'changes/feed.xml' 'compare/bgp/index.html' 'compare/bgp/index.md' 'data/changes.json' 'data/milestones.json' 'data/rfc-compliance.json' 'data/rfc-requirements.json' 'data/search-index.json' 'data/site-facts.json' 'index.html' 'index.md' 'llms-full.txt' 'llms.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/changes/feed.xml' 'project/changes/index.html' 'project/changes/index.md' 'project/milestones/index.html' 'project/milestones/index.md' 'quality/health/index.html' 'quality/health/index.md' 'quality/rfc-compliance/index.html' 'quality/rfc-compliance/index.md' 'quality/rfc-compliance/rfc2003/index.html' 'quality/rfc-compliance/rfc2003/index.md' 'quality/rfc-compliance/rfc2473/index.html' 'quality/rfc-compliance/rfc2473/index.md' 'quality/rfc-compliance/rfc2784/index.html' 'quality/rfc-compliance/rfc2784/index.md' 'quality/rfc-compliance/rfc2890/index.html' 'quality/rfc-compliance/rfc2890/index.md' 'quality/rfc-compliance/rfc3031/index.html' 'quality/rfc-compliance/rfc3031/index.md' 'quality/rfc-compliance/rfc4862/index.html' 'quality/rfc-compliance/rfc4862/index.md' 'quality/rfc-compliance/rfc6071/index.html' 'quality/rfc-compliance/rfc6071/index.md' 'quality/rfc-compliance/rfc6482/index.html' 'quality/rfc-compliance/rfc6482/index.md' 'quality/rfc-compliance/rfc7854/index.html' 'quality/rfc-compliance/rfc7854/index.md' 'quality/rfc-compliance/rfc9319/index.html' 'quality/rfc-compliance/rfc9319/index.md' 'quality/rfc-compliance/rfc9384/index.html' 'quality/rfc-compliance/rfc9384/index.md' 'quality/rfc-compliance/rfc9582/index.html' 'quality/rfc-compliance/rfc9582/index.md' 'sitemap.xml' 'talks/linx-2026-06/activity.html' 'talks/linx-2026-06/index-inlined.html' 'changes/discord/2026-09-14-weekly.md' 'project/changes/2026-09-14/index.html' 'project/changes/2026-09-14/index.md'; GIT_INDEX_FILE="$_ze_index" git -c core.quotePath=false ls-files -s -- 'changes/feed.xml' 'compare/bgp/index.html' 'compare/bgp/index.md' 'data/changes.json' 'data/milestones.json' 'data/rfc-compliance.json' 'data/rfc-requirements.json' 'data/search-index.json' 'data/site-facts.json' 'index.html' 'index.md' 'llms-full.txt' 'llms.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/changes/feed.xml' 'project/changes/index.html' 'project/changes/index.md' 'project/milestones/index.html' 'project/milestones/index.md' 'quality/health/index.html' 'quality/health/index.md' 'quality/rfc-compliance/index.html' 'quality/rfc-compliance/index.md' 'quality/rfc-compliance/rfc2003/index.html' 'quality/rfc-compliance/rfc2003/index.md' 'quality/rfc-compliance/rfc2473/index.html' 'quality/rfc-compliance/rfc2473/index.md' 'quality/rfc-compliance/rfc2784/index.html' 'quality/rfc-compliance/rfc2784/index.md' 'quality/rfc-compliance/rfc2890/index.html' 'quality/rfc-compliance/rfc2890/index.md' 'quality/rfc-compliance/rfc3031/index.html' 'quality/rfc-compliance/rfc3031/index.md' 'quality/rfc-compliance/rfc4862/index.html' 'quality/rfc-compliance/rfc4862/index.md' 'quality/rfc-compliance/rfc6071/index.html' 'quality/rfc-compliance/rfc6071/index.md' 'quality/rfc-compliance/rfc6482/index.html' 'quality/rfc-compliance/rfc6482/index.md' 'quality/rfc-compliance/rfc7854/index.html' 'quality/rfc-compliance/rfc7854/index.md' 'quality/rfc-compliance/rfc9319/index.html' 'quality/rfc-compliance/rfc9319/index.md' 'quality/rfc-compliance/rfc9384/index.html' 'quality/rfc-compliance/rfc9384/index.md' 'quality/rfc-compliance/rfc9582/index.html' 'quality/rfc-compliance/rfc9582/index.md' 'sitemap.xml' 'talks/linx-2026-06/activity.html' 'talks/linx-2026-06/index-inlined.html' 'changes/discord/2026-09-14-weekly.md' 'project/changes/2026-09-14/index.html' 'project/changes/2026-09-14/index.md'; } | sort | uniq -u | cut -f2- | sort -u)
rm -f "$_ze_index.now"
if [ -n "$_ze_drift" ]; then
  echo "NOTE: these paths changed on disk after this commit was prepared." >&2
  echo "The commit carries the prepared content; the difference stays in the working tree." >&2
  echo "$_ze_drift" >&2
fi
GIT_INDEX_FILE="$_ze_index" git commit -F 'tmp/commit-msg-4ae238a6-c-4d494c.txt'
# Point the shared index at what was committed. Nothing else in it is touched.
git ls-tree HEAD -- 'changes/feed.xml' 'compare/bgp/index.html' 'compare/bgp/index.md' 'data/changes.json' 'data/milestones.json' 'data/rfc-compliance.json' 'data/rfc-requirements.json' 'data/search-index.json' 'data/site-facts.json' 'index.html' 'index.md' 'llms-full.txt' 'llms.txt' 'project/activity/index.html' 'project/activity/index.md' 'project/changes/feed.xml' 'project/changes/index.html' 'project/changes/index.md' 'project/milestones/index.html' 'project/milestones/index.md' 'quality/health/index.html' 'quality/health/index.md' 'quality/rfc-compliance/index.html' 'quality/rfc-compliance/index.md' 'quality/rfc-compliance/rfc2003/index.html' 'quality/rfc-compliance/rfc2003/index.md' 'quality/rfc-compliance/rfc2473/index.html' 'quality/rfc-compliance/rfc2473/index.md' 'quality/rfc-compliance/rfc2784/index.html' 'quality/rfc-compliance/rfc2784/index.md' 'quality/rfc-compliance/rfc2890/index.html' 'quality/rfc-compliance/rfc2890/index.md' 'quality/rfc-compliance/rfc3031/index.html' 'quality/rfc-compliance/rfc3031/index.md' 'quality/rfc-compliance/rfc4862/index.html' 'quality/rfc-compliance/rfc4862/index.md' 'quality/rfc-compliance/rfc6071/index.html' 'quality/rfc-compliance/rfc6071/index.md' 'quality/rfc-compliance/rfc6482/index.html' 'quality/rfc-compliance/rfc6482/index.md' 'quality/rfc-compliance/rfc7854/index.html' 'quality/rfc-compliance/rfc7854/index.md' 'quality/rfc-compliance/rfc9319/index.html' 'quality/rfc-compliance/rfc9319/index.md' 'quality/rfc-compliance/rfc9384/index.html' 'quality/rfc-compliance/rfc9384/index.md' 'quality/rfc-compliance/rfc9582/index.html' 'quality/rfc-compliance/rfc9582/index.md' 'sitemap.xml' 'talks/linx-2026-06/activity.html' 'talks/linx-2026-06/index-inlined.html' 'changes/discord/2026-09-14-weekly.md' 'project/changes/2026-09-14/index.html' 'project/changes/2026-09-14/index.md' | git update-index --index-info
rm -f "$_ze_index"
