# No Backwards Compatibility

**ZeBGP has never been released. There are no users.**

Therefore:
- NO backwards compatibility code
- NO backwards compatibility comments
- NO legacy shims or fallbacks
- NO "for compatibility with older versions" logic

If something needs to change, just change it. Delete the old code. There is no one to break.

This applies to:
- API protocols
- Config syntax
- Wire formats
- CLI commands
- Python libraries
- Everything
