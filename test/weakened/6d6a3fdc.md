| Test | Reason |
|---|---|
| TestRecoveryLabeledTextPrefixMatchesNativeSentStack | Renamed to TestRecoveryLabeledKeyMatchesNativeSentStack with the RecoveryRequest.PrefixOnly field it drove: nothing produced that field, so the branch is deleted. The test now asks with a one-label native key against the route sent with a two-label stack and still requires the sent identity and a WITHDRAW answer. |
