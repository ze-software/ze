"""Resolve the Ze binary used to generate website command data."""

import pathlib
import subprocess


def resolve(main_repo):
    """Return the production Ze binary for the current build session."""
    main_repo = pathlib.Path(main_repo).resolve()
    result = subprocess.run(
        ["make", "-s", "ze-session-binary-path"],
        cwd=main_repo,
        capture_output=True,
        text=True,
    )
    if result.returncode == 0 and result.stdout.strip():
        path = pathlib.Path(result.stdout.strip())
        return path if path.is_absolute() else (main_repo / path).resolve()
    return main_repo / "bin" / "ze"
