| Test | Reason |
|---|---|
| FuzzDecode | owner 2026-10-18 (rows RFC9568-7.2-5, RFC9568-5.2.8-2): VRRPv3 IPv4 receive no longer dual-accepts, so the Advertisement.MsgOnlyChecksum flag the dropped assertion checked no longer exists. The fuzz target now drives both checksum modes and round-trips every accepted packet, not only the unflagged ones. |
| TestRoundTrip | owner 2026-10-18 (rows RFC9568-7.2-5, RFC9568-5.2.8-2): the dropped assertion checked the removed MsgOnlyChecksum flag. The round trip still decodes field-equal under the group's single checksum form. |
| TestDecodeVerifiesChecksumBothFamilies | owner 2026-10-18 (rows RFC9568-7.2-5, RFC9568-5.2.8-2): the dropped assertion checked the removed MsgOnlyChecksum flag. Accepting only the configured form is proven by TestDecodeV3IPv4ChecksumOnlyConfiguredForm (packet_test.go), both modes and both polarities. |
| TestDecodeGoldenV2 | owner 2026-10-18 (rows RFC9568-7.2-5, RFC9568-5.2.8-2): the dropped assertion checked that v2 did not set the removed MsgOnlyChecksum flag. |
| TestDecodeGoldenV3IPv4 | owner 2026-10-18 (rows RFC9568-7.2-5, RFC9568-5.2.8-2): the dropped assertion checked the removed MsgOnlyChecksum flag; the golden is now the RFC 9568 message-only form under the default mode. |
| TestDecodeV3IPv4MsgOnlyChecksumCompat | Deleted. owner 2026-10-18 (rows RFC9568-7.2-5, RFC9568-5.2.8-2): it pinned the dual-accept this change removes. TestDecodeV3IPv4ChecksumOnlyConfiguredForm (packet_test.go) replaces it: each mode accepts its own form and refuses the other with ErrChecksum, and the unspecified mode refuses both. |
| TestErrorReasonMapping | owner 2026-10-18 (rows RFC9568-7.2-5, RFC9568-5.2.8-2): the checksum-rfc9568-message-only label is removed with the dual-accept, so its two assertions went with it. |
