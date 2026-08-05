# ssh

<p>
    <a href="https://github.com/charmbracelet/ssh/releases"><img src="https://img.shields.io/github/release/charmbracelet/ssh.svg" alt="Latest Release"></a>
    <a href="https://pkg.go.dev/charm.land/ssh?tab=doc"><img src="https://godoc.org/github.com/golang/gddo?status.svg" alt="GoDoc"></a>
    <a href="https://github.com/charmbracelet/ssh/actions"><img src="https://github.com/charmbracelet/ssh/actions/workflows/build.yml/badge.svg" alt="Build Status"></a>
    <a href="https://codecov.io/gh/charmbracelet/ssh"><img alt="Codecov branch" src="https://img.shields.io/codecov/c/github/charmbracelet/ssh/master.svg"></a>
</p>

An SSH server library for Go. `ssh` wraps the lower-level
[x/crypto/ssh](https://pkg.go.dev/golang.org/x/crypto/ssh) package with a
higher-level API that feels like [net/http](https://pkg.go.dev/net/http):

```go
package main

import (
	"io"
	"log"

	"charm.land/ssh"
)

func main() {
	ssh.Handle(func(s ssh.Session) {
		io.WriteString(s, "Hello world\n")
	})

	log.Fatal(ssh.ListenAndServe(":2222", nil))
}
```

## Features

* Familiar, `net/http`-inspired API
* Handlers for sessions, channels, port forwarding, and more
* Public key, password, and keyboard-interactive auth
* PTY support across platforms
* Used in production by [Soft Serve](https://github.com/charmbracelet/soft-serve)
  and [Wish](https://github.com/charmbracelet/wish)

## Examples

A bunch of great examples are in the [`_examples`](_examples) directory.

## Usage

[See the GoDoc reference.](https://pkg.go.dev/charm.land/ssh)

## Feedback

We’d love to hear your thoughts on this project. Feel free to drop us a note!

* [Twitter](https://twitter.com/charmcli)
* [The Fediverse](https://mastodon.social/@charmcli)
* [Discord](https://charm.land/chat)

## Acknowledgements

This package was originally forked from [gliderlabs/ssh](https://github.com/gliderlabs/ssh)

## License

[MIT](https://github.com/charmbracelet/ssh/raw/master/LICENSE)

***

Part of [Charm](https://charm.land).

<a href="https://charm.land/"><img alt="The Charm logo" src="https://stuff.charm.sh/charm-banner-softy.jpg" width="400"></a>

Charm热爱开源 • Charm loves open source
