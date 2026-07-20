# RFC Summaries Before Design

**When:** when a spec lists RFC summaries in its Required Reading section
**Severity:** blocking

## Directives

When a spec lists RFC summaries in its Required Reading section,
read ALL of them before making any design recommendations or protocol claims.

## Why

Training knowledge of RFCs is unreliable: drafts change between versions,
details get conflated across similar RFCs, and wire format specifics (field
offsets, PDU sizes, flag positions) are frequently wrong from memory.
The RFC summaries in `rfc/short/` are the verified source of truth.

## Mechanical Rule

1. Spec lists RFC summaries under Required Reading -> read every one that exists
2. If a summary is marked "MUST CREATE" -> create it BEFORE design work
3. Only after all summaries are read and annotated: make design recommendations
4. Never cite RFC section numbers, PDU formats, or protocol semantics from memory
   when a summary exists (or should exist) in the repo

## Banned Reasoning

| Excuse | Reality |
|--------|---------|
| "I know this RFC from training" | Training conflates draft versions. Read the summary. |
| "The design doesn't depend on wire format details" | You don't know that until you've read it. |
| "I'll read it later when implementing" | Design decisions made without RFC constraints get reworked. |
| "The spec already describes the algorithm" | The spec may be wrong. The RFC summary is the authority. |
