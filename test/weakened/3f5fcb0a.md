| Test | Reason |
|---|---|
| TestRecoveryNativeSentIdentityAndNewerOwner | Renamed TestRecoveryAddPathDepartureWithdrawsEverySentIdentifier. The RIB no longer holds the cut, so it names the reconnected identifier 99 too (spec guards table, MOVES LAYER); the reconnect guard is asserted at the writer by TestAdjOutDepartureOwesOnlyDepartedSession/add-path-reconnect-keeps-newer-identifier, red when the ADD-PATH generation refusal is removed. |
| TestRecoveryMetadataPreservesCutAndRejectsMalformedOwnership | Renamed TestRecoveryMetadataRejectsMalformedOwnership. The cut cases left with the recovery-cut input (spec guards table); every malformed-source case stays, plus an empty source. |
| recovery_test | The two cut-value assertions left with the recovery-cut input; source parsing and every refusal are still asserted. |
