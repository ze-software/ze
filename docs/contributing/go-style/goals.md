# Ze Go Style: the design goals

The detail behind the design goals in [the Go style core](../ze-go-style.md),
which states them and their order.

| # | Goal | The question it asks |
|---|------|----------------------|
| 1 | Safety | What happens when this input is hostile, this peer is dead, or this buffer is full? |
| 2 | Performance | What does this cost for each UPDATE, for each peer, for each second? |
| 3 | Developer experience | Can the next reader change this code without a second file open? |

The history and the reasoning behind the standard are in
[`ze-go-style-background.md`](../ze-go-style-background.md).
[`writing-style.md`](../writing-style.md) is the standard for every word.
