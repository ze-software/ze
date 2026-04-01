#!/usr/bin/env python3
"""
UK Network Lab -- Route Injection for Looking Glass Graph Testing

Topology (AS65000):
  Core Ring: Telehouse (th), Leeds (lds), Manchester (man), Birmingham (bhm)
  Edge:      Slough (slo), Bradford (bfd)

  Ring: TH <-> LDS <-> MAN <-> BHM <-> TH
  Slough  -> TH, BHM (customer POP)
  Bradford -> LDS, MAN (customer POP)

External:
  NTT        AS2914  -> th-1  (transit)
  Cogent     AS174   -> man-1 (transit)
  Cloudflare AS13335 -> th-2  (peering)
  Akamai     AS20940 -> man-2 (peering)

Routers (IP addresses):
  th-1  10.0.1.1    th-2  10.0.1.2
  lds-1 10.0.2.1    lds-2 10.0.2.2
  man-1 10.0.3.1    man-2 10.0.3.2
  bhm-1 10.0.4.1    bhm-2 10.0.4.2
  slo-1 10.0.5.1    slo-2 10.0.5.2
  bfd-1 10.0.6.1    bfd-2 10.0.6.2
"""
import sys
import time
from ze_api import API


def dispatch(api, command):
    resp = api._call_engine(
        'ze-plugin-engine:dispatch-command', {'command': command})
    if resp is None:
        return {'status': 'error', 'data': 'engine call returned None'}
    return resp.get('result', {})


def inject(api, peer, prefix, nhop, aspath):
    cmd = f'rib inject {peer} ipv4/unicast {prefix} origin igp nhop {nhop} aspath {aspath}'
    result = dispatch(api, cmd)
    if result.get('status') != 'done':
        print(f'FAIL: {cmd} -> {result}', file=sys.stderr)
        sys.exit(1)


def main():
    api = API()
    api.declare_done()
    api.wait_for_config()
    api.capability_done()
    api.wait_for_registry()
    api.ready()
    time.sleep(1.0)

    # --- Prefix 10.10.1.0/24: dual transit (NTT at th-1, Cogent at man-1) ---
    p1 = '10.10.1.0/24'
    inject(api, '10.0.1.1', p1, '10.0.1.1', '2914,65100')   # th-1 egress (NTT)
    inject(api, '10.0.3.1', p1, '10.0.3.1', '174,65100')    # man-1 egress (Cogent)
    inject(api, '10.0.1.2', p1, '10.0.1.1', '2914,65100')   # th-2 -> th-1
    inject(api, '10.0.4.1', p1, '10.0.1.1', '2914,65100')   # bhm-1 -> th-1
    inject(api, '10.0.4.2', p1, '10.0.1.1', '2914,65100')   # bhm-2 -> th-1
    inject(api, '10.0.5.1', p1, '10.0.1.1', '2914,65100')   # slo-1 -> th-1
    inject(api, '10.0.5.2', p1, '10.0.1.1', '2914,65100')   # slo-2 -> th-1
    inject(api, '10.0.2.1', p1, '10.0.3.1', '174,65100')    # lds-1 -> man-1
    inject(api, '10.0.2.2', p1, '10.0.3.1', '174,65100')    # lds-2 -> man-1
    inject(api, '10.0.3.2', p1, '10.0.3.1', '174,65100')    # man-2 -> man-1
    inject(api, '10.0.6.1', p1, '10.0.3.1', '174,65100')    # bfd-1 -> man-1
    inject(api, '10.0.6.2', p1, '10.0.3.1', '174,65100')    # bfd-2 -> man-1

    # --- Prefix 10.10.2.0/24: peering only (Cloudflare at th-2) ---
    p2 = '10.10.2.0/24'
    inject(api, '10.0.1.2', p2, '10.0.1.2', '13335,65200')  # th-2 egress
    inject(api, '10.0.1.1', p2, '10.0.1.2', '13335,65200')  # th-1 -> th-2
    inject(api, '10.0.2.1', p2, '10.0.1.2', '13335,65200')  # lds-1 -> th-2
    inject(api, '10.0.2.2', p2, '10.0.1.2', '13335,65200')  # lds-2 -> th-2
    inject(api, '10.0.3.1', p2, '10.0.1.2', '13335,65200')  # man-1 -> th-2
    inject(api, '10.0.3.2', p2, '10.0.1.2', '13335,65200')  # man-2 -> th-2
    inject(api, '10.0.4.1', p2, '10.0.1.2', '13335,65200')  # bhm-1 -> th-2
    inject(api, '10.0.4.2', p2, '10.0.1.2', '13335,65200')  # bhm-2 -> th-2
    inject(api, '10.0.5.1', p2, '10.0.1.2', '13335,65200')  # slo-1 -> th-2
    inject(api, '10.0.5.2', p2, '10.0.1.2', '13335,65200')  # slo-2 -> th-2
    inject(api, '10.0.6.1', p2, '10.0.1.2', '13335,65200')  # bfd-1 -> th-2
    inject(api, '10.0.6.2', p2, '10.0.1.2', '13335,65200')  # bfd-2 -> th-2

    # --- Prefix 10.10.3.0/24: all four externals, peering preferred ---
    p3 = '10.10.3.0/24'
    inject(api, '10.0.1.2', p3, '10.0.1.2', '13335,65300')  # th-2 egress (Cloudflare)
    inject(api, '10.0.3.2', p3, '10.0.3.2', '20940,65300')  # man-2 egress (Akamai)
    inject(api, '10.0.1.1', p3, '10.0.1.2', '13335,65300')  # th-1 -> th-2
    inject(api, '10.0.4.1', p3, '10.0.1.2', '13335,65300')  # bhm-1 -> th-2
    inject(api, '10.0.4.2', p3, '10.0.1.2', '13335,65300')  # bhm-2 -> th-2
    inject(api, '10.0.5.1', p3, '10.0.1.2', '13335,65300')  # slo-1 -> th-2
    inject(api, '10.0.5.2', p3, '10.0.1.2', '13335,65300')  # slo-2 -> th-2
    inject(api, '10.0.2.1', p3, '10.0.3.2', '20940,65300')  # lds-1 -> man-2
    inject(api, '10.0.2.2', p3, '10.0.3.2', '20940,65300')  # lds-2 -> man-2
    inject(api, '10.0.3.1', p3, '10.0.3.2', '20940,65300')  # man-1 -> man-2
    inject(api, '10.0.6.1', p3, '10.0.3.2', '20940,65300')  # bfd-1 -> man-2
    inject(api, '10.0.6.2', p3, '10.0.3.2', '20940,65300')  # bfd-2 -> man-2

    print('OK: all 36 routes injected', file=sys.stderr)

    # Stay alive while HTTP checks run. Test runner kills us when done.
    try:
        while True:
            time.sleep(10)
    except (KeyboardInterrupt, BrokenPipeError):
        pass


if __name__ == '__main__':
    main()
