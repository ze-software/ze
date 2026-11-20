# ExaBGP -- the licence of the formats and tables the bridge ports

**Nothing in this directory is compiled.** It holds the licence of ExaBGP,
because the ExaBGP bridge compiled into `ze` ports ExaBGP's API encoders and
transcribes tables from its source, and the licence asks that a binary carry
its notice.

| Field | Value |
|-------|-------|
| Upstream | [github.com/Exa-Networks/exabgp](https://github.com/Exa-Networks/exabgp), file `LICENCE.txt` at the repository root |
| `LICENSE` | Byte for byte ExaBGP's `LICENCE.txt`, kept under the name the other licence copies in Ze use. Its first line: "Python BGP route injector" |
| Version | The bridge cites ExaBGP `main`, at commit `f9bfb853b924e97deba646b0bd50693ab07c4f3e` where it names one (`internal/exabgp/bridge/bridge_version.go`, `APIVersion`) |
| Read from | The local checkout of ExaBGP `main`, on 2026-11-20 |
| Confirmed against | `https://raw.githubusercontent.com/Exa-Networks/exabgp/main/LICENCE.txt` and the file at commit `f9bfb853b`: both identical. The `LICENCE.txt` of the 5.0 and 4.2 lines and `internal/exabgp/bridge/testdata/exabgp-old/LICENCE-ExaBGP.txt` are identical to it too (SHA-256 `19002288545f4fcb273332f0daef938a5c72c638af2de28ab440cc30ad987632`) |
| Copyright | Every holder the file lists, from "Copyright (c) 2009-2020, Exa Networks Limited" to "Copyright (c) 2020-2020, Hiroki Shirokura" |

## What Ze derived

`internal/exabgp/bridge` translates between Ze and a script written for
ExaBGP's process API. Two kinds of code in it come from ExaBGP's source:

- **Ported formats.** The text event encoder is ExaBGP's own,
  ported from `Response.Text` in `src/exabgp/reactor/api/response/text.py`
  (`bridge_event_text.go`; `docs/architecture/exabgp-bridge.md`, "The text
  encoder is ExaBGP's own, ported"). The JSON encoder writes the format of
  `src/exabgp/reactor/api/response/json.py` (`bridge_event.go`,
  `bridge_event_json.go`).
- **Transcribed tables.** Tables ExaBGP holds are written as Go values, such
  as `flowICMPTypeNames`, "ExaBGP main's own table, transcribed from
  ICMPType.names (src/exabgp/protocol/ip/icmp.py)" (`bridge_event.go`).

Each file that takes something from ExaBGP names the ExaBGP source path in its
comments. `grep -rl 'src/exabgp/' internal/exabgp/bridge --include='*.go'`
lists them. `internal/exabgp/migration` cites ExaBGP's configuration parser
the same way, to state what ExaBGP accepts.
