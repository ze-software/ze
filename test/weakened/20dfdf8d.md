# Test weakenings this commit accepts

| Test | Reason |
|------|--------|
| recordingFirewall | The call-order recorder was replaced by sweepBackend using the real firewall registry and final backend state. The three inherited sweep tests retain name/owner, both-family claim-and-withdraw and failed-reconcile cleanup coverage; production SDK startup tests add configured-backend and reload discrimination. Current local race PASS and configured-backend overlay RED are recorded in the spec. |
