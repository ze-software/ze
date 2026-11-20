# filippo.io/mlockexe

Package mlockexe locks the running executable's file-backed pages in memory,
so they can't be evicted under memory pressure.

When a process hits a memory cgroup limit or the system runs out of memory,
the kernel can repeatedly evict and re-fault the executable's own text,
causing reclaim thrashing. Locking the executable in memory prevents this.

The simplest way to use it is to call `OnFault` early during startup:

```go
if _, err := mlockexe.OnFault(); err != nil {
	slog.Warn("mlockexe: failed to lock executable in memory", "err", err)
}
```

Locking requires Linux procfs to be available at `/proc`, and `RLIMIT_MEMLOCK`
to accommodate the whole size of the executable. Note that the systemd default
(`LimitMEMLOCK=`, usually 8 MiB) is smaller than most Go binaries, so units will
generally need e.g.

```ini
LimitMEMLOCK=256M
```

See the [package documentation](https://pkg.go.dev/filippo.io/mlockexe) for
details.

## License

This work is marked CC0 1.0 Universal. To view a copy of this mark, visit
[creativecommons.org](https://creativecommons.org/publicdomain/zero/1.0/).

Alternatively, you may use this source code under the terms of the 0BSD license
that can be found in the LICENSE file.
In short, you can do whatever you want with this code.
