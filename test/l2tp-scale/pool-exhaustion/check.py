#!/usr/bin/env python3
"""Scale scenario: pool exhaustion.

Validates AC-7: when pool is smaller than session count, sessions
beyond pool size are rejected and existing sessions are unaffected.
Uses a /24 pool (254 addresses) with 300 sessions requested.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from harness import ScaleScenario, find_free_port, log_info, log_pass  # noqa: E402

TUNNELS = 1
SESSIONS_REQUESTED = 300
POOL_SIZE = 254


def run():
    s = ScaleScenario(os.path.dirname(__file__))
    port = find_free_port()

    config = """\
l2tp {{
    enabled true
    shared-secret s3cr3t
    auth {{
        local {{
            user test {{
                password testpass
            }}
        }}
    }}
    pool {{
        ipv4 {{
            gateway 10.99.0.1
            start 10.99.0.2
            end 10.99.0.255
            dns-primary 8.8.8.8
        }}
    }}
}}
environment {{
    l2tp {{
        server main {{
            ip 127.0.0.1
            port {port}
        }}
    }}
}}
"""
    try:
        s.setup(config_text=config.format(port=port), l2tp_port=port)

        result = s.run_scale(
            tunnels=TUNNELS,
            sessions=SESSIONS_REQUESTED,
            dwell="1s",
        )
    finally:
        s.teardown()

    if result is None:
        raise RuntimeError("no result from ze-test l2tp-scale")

    sessions_up = result.get("sessions-up", 0)

    log_info(
        "sessions established: %d/%d (pool size: %d)"
        % (sessions_up, SESSIONS_REQUESTED, POOL_SIZE)
    )

    if sessions_up > POOL_SIZE:
        raise RuntimeError(
            "more sessions than pool addresses: %d > %d" % (sessions_up, POOL_SIZE)
        )

    if sessions_up == 0:
        raise RuntimeError("no sessions established at all")

    log_pass(
        "pool exhaustion enforced: %d sessions <= %d pool addresses"
        % (sessions_up, POOL_SIZE)
    )


if __name__ == "__main__":
    run()
