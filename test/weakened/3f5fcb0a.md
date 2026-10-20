| Test | Reason |
|---|---|
| TestFleetRenderUnspecifiedPreservesJSON | Replaced, not weakened: it held the render path for bridge.EncoderUnspecified, which this commit deletes (the fan-out now renders per bridge.Format, chosen by the script API version and encoder; FormatUnspecified is a BUG panic and Fleet.Start still refuses a script with no encoder). TestFleetRenderOncePerFormat in the same file renders v6 JSON, v4 JSON and v6 JSON again, asserting each envelope and that the cached line is reused. |
