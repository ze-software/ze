#!/bin/bash
set -euo pipefail
cd '/home/thomas/Code/github.com/ze-software/ze/gh-pages'

# ze-commit-script: tmp/commit-55566111-push-6db728.sh session=55566111

# ze-commit-push: Thomas: "git push", answering whether to push the site live
# Push dfa6a74afca8eb19c76423a79752fc3207827181 to origin refs/heads/gh-pages. This script makes no commit.
git push 'origin' 'dfa6a74afca8eb19c76423a79752fc3207827181:refs/heads/gh-pages'
