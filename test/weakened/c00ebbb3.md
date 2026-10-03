| Test | Reason |
|---|---|
| expectNoPDU | Deleted the helper that treated every read error, including EOF, as successful silence. All callers now use expectSilence, which requires an actual timeout and rejects closed transports or other errors. No no-response assertion was dropped; successful session and MaxPDU cases now also traverse live establishment. |
