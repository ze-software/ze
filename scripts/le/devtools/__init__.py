"""Libraries the `setup` subprogram is built from.

Split by concern, mirroring the sections the shell-era script had marked with
comment banners, so that each one is small enough to read in a sitting and each
can be tested without running the others:

    tools     what a Ze workflow needs, and how each is installed
    probes    whether a tool is present, which is not the same as on PATH
    servers   whether a language server ANSWERS, which is not the same as present
    system    machine state that is not a binary: sysctls, groups, addresses
    install   the package managers, and vendoring Go dependencies
"""

from __future__ import annotations
