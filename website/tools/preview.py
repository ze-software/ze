#!/usr/bin/env python3
"""Build and serve the website artifact for local preview."""

import argparse
import functools
import http.server
import pathlib
import subprocess
import webbrowser

import sitepaths

SOURCE_ROOT = sitepaths.SOURCE_ROOT
OUTPUT_ROOT = sitepaths.OUTPUT_ROOT
MAIN_REPO = sitepaths.MAIN_REPO


def parse_args():
    parser = argparse.ArgumentParser(
        description="Build the website, serve the Pages artifact, and open it in a browser."
    )
    parser.add_argument(
        "--port", type=int, default=8765, help="local port (default: 8765)"
    )
    parser.add_argument(
        "--no-build",
        action="store_true",
        help="serve the existing Pages artifact without rebuilding it",
    )
    parser.add_argument(
        "--no-open",
        action="store_true",
        help="do not open the preview in the default browser",
    )
    return parser.parse_args()

def build():
    subprocess.run(["make", "-C", str(MAIN_REPO), "ze-build"], check=True)
    subprocess.run([str(SOURCE_ROOT / "update-website.sh")], check=True)


def main():
    args = parse_args()
    if not args.no_build:
        build()

    if not (OUTPUT_ROOT / "index.html").is_file():
        raise SystemExit("website artifact not found; run without --no-build")

    handler = functools.partial(
        http.server.SimpleHTTPRequestHandler, directory=str(OUTPUT_ROOT)
    )
    try:
        server = http.server.ThreadingHTTPServer(("127.0.0.1", args.port), handler)
    except OSError as exc:
        raise SystemExit(
            "cannot start preview on port %d: %s" % (args.port, exc)
        ) from exc

    url = "http://127.0.0.1:%d/" % args.port
    print("serving %s at %s" % (OUTPUT_ROOT, url), flush=True)
    print("press Ctrl-C to stop", flush=True)
    if not args.no_open:
        webbrowser.open(url)

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nstopping preview")
    finally:
        server.server_close()


if __name__ == "__main__":
    main()
