| Test | Reason |
|---|---|
| TestScriptVersionResetsOnStart | Moved, not weakened: the state it tested (detect, hold, reset to the setting, format per version and encoder) moved from bridgerun's script into bridge.ScriptVersion, and the test moved with it to internal/exabgp/bridge/bridge_script_version_test.go with every assertion, now calling Detect, Reset, Current and Format on the shared type. |
| version_test | The assertion count fell only by TestScriptVersionResetsOnStart, moved above; TestScriptHeldToDetectedVersion and TestFleetRefusesAnInvalidAPIVersionSetting stay here unchanged. |
