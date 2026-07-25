#!/usr/bin/env python3
"""Move command YANG files from component/ to plugins/.

Phase 2 of spec-yang-rename-ownership: move -cmd.yang files to plugin
directories so removing a plugin removes its command surface.

Usage:
    python3 scripts/dev/move_cmd_yang_to_plugins.py              # dry run
    python3 scripts/dev/move_cmd_yang_to_plugins.py --apply      # apply

Pattern:
    - component/<name>/yang/ze-<x>-cmd.yang -> plugins/<name>-cmd/yang/
    - If plugins/<name>/ already exists, merge there instead of creating -cmd
    - Keep -conf.yang and -api.yang in component/<name>/yang/
    - Self-containment tests move with the cmd YANG
    - Codegen regenerates after moves
"""

import os
import re
import shutil
import sys
from pathlib import Path


def find_project_root():
    d = Path(__file__).resolve()
    while d != d.parent:
        if (d / "go.mod").exists():
            return d
        d = d.parent
    raise RuntimeError("go.mod not found")


ROOT = find_project_root()
INTERNAL = ROOT / "internal"
MODULE = "github.com/ze-software/ze"

# Subsystems to KEEP in component/ (intrinsic to component).
KEEP_IN_COMPONENT = {
    "iface",  # core network operations, 4 cmd YANG files
    "firewall",  # core infrastructure
    "ike",  # paired with ipsec security stack
    "doctor",  # infrastructure diagnostics
}

# Subsystems to move. Map: component name -> plugin target name.
# If a plugin already exists at plugins/<target>/, merge there.
MOVE_MAP = {
    "aaa": "aaa-cmd",
    "bfd": "bfd",  # existing plugins/bfd/
    "gnmi": "gnmi-cmd",
    "l2tp": "l2tp-cmd",
    "ldp": "ldp-cmd",
    "mpls": "mpls-cmd",
    "ping": "ping-cmd",
    "pki": "pki-cmd",
    "pppoe": "pppoe-cmd",
    "resolve": "resolve-cmd",
    "rsvpte": "rsvpte-cmd",
    "storage": "storage-cmd",
    "subscriber": "subscriber-cmd",
    "traceroute": "traceroute-cmd",
    "traffic": "traffic-cmd",
    "flowexport": "flowexport-cmd",
    "config/archive": "config-archive-cmd",
}

# BGP sub-plugins: already in the bgp plugin tree, skip.
# Central verb anchors (cmd/show, cmd/clear, etc.): keep in component/cmd/.


def discover_cmd_yang(component_name):
    """Find all -cmd.yang files in a component's yang/ directory."""
    if "/" in component_name:
        yang_dir = INTERNAL / "component" / component_name / "yang"
    else:
        yang_dir = INTERNAL / "component" / component_name / "yang"

    if not yang_dir.exists():
        return []

    return sorted(
        f for f in yang_dir.iterdir() if f.suffix == ".yang" and "-cmd" in f.stem
    )


def discover_cmd_tests(component_name):
    """Find self-containment and cmd schema tests in the component's yang/ dir."""
    if "/" in component_name:
        yang_dir = INTERNAL / "component" / component_name / "yang"
    else:
        yang_dir = INTERNAL / "component" / component_name / "yang"

    if not yang_dir.exists():
        return []

    return sorted(
        f for f in yang_dir.iterdir() if f.suffix == ".go" and "cmd_schema" in f.name
    )


def move_yang_files(component_name, plugin_name, apply):
    """Move cmd YANG files from component to plugin."""
    cmd_yangs = discover_cmd_yang(component_name)
    cmd_tests = discover_cmd_tests(component_name)

    if not cmd_yangs:
        return []

    plugin_yang_dir = INTERNAL / "plugins" / plugin_name / "yang"
    actions = []

    for f in cmd_yangs:
        dest = plugin_yang_dir / f.name
        actions.append(("move", str(f.relative_to(ROOT)), str(dest.relative_to(ROOT))))
        if apply:
            plugin_yang_dir.mkdir(parents=True, exist_ok=True)
            shutil.move(str(f), str(dest))

    for f in cmd_tests:
        dest = plugin_yang_dir / f.name
        if not dest.exists():
            actions.append(
                ("move", str(f.relative_to(ROOT)), str(dest.relative_to(ROOT)))
            )
            if apply:
                plugin_yang_dir.mkdir(parents=True, exist_ok=True)
                shutil.move(str(f), str(dest))

    return actions


def update_self_containment_tests(apply):
    """Update test files that reference old paths in self-containment expected-owner maps."""
    test_dirs = [
        INTERNAL / "component" / "cmd" / "show" / "yang",
        INTERNAL / "component" / "cmd" / "clear" / "yang",
        INTERNAL / "component" / "cmd" / "delete" / "yang",
        INTERNAL / "component" / "cmd" / "monitor" / "yang",
        INTERNAL / "component" / "cmd" / "set" / "yang",
    ]

    updated = []
    for d in test_dirs:
        for f in d.glob("*_test.go"):
            try:
                content = f.read_text()
            except (UnicodeDecodeError, OSError):
                continue

            original = content
            for comp_name, plugin_name in MOVE_MAP.items():
                old_path = f"internal/component/{comp_name}/yang"
                new_path = f"internal/plugins/{plugin_name}/yang"
                content = content.replace(old_path, new_path)

            if content != original:
                if apply:
                    f.write_text(content)
                updated.append(str(f.relative_to(ROOT)))

    return updated


def main():
    apply = "--apply" in sys.argv

    print(f"{'APPLY' if apply else 'DRY RUN'}: move cmd YANG to plugins/")
    print(f"Project root: {ROOT}")
    print()

    # Show what stays
    print("Keeping in component/ (intrinsic):")
    for name in sorted(KEEP_IN_COMPONENT):
        yangs = discover_cmd_yang(name)
        print(f"  {name}: {len(yangs)} cmd YANG files")
    print()

    # Process moves
    all_actions = []
    for comp_name, plugin_name in sorted(MOVE_MAP.items()):
        yangs = discover_cmd_yang(comp_name)
        if not yangs:
            print(f"  SKIP {comp_name}: no cmd YANG found")
            continue

        actions = move_yang_files(comp_name, plugin_name, apply)
        all_actions.extend(actions)

        yang_names = [
            Path(a[1]).name
            for a in actions
            if a[0] == "move" and a[1].endswith(".yang")
        ]
        print(
            f"  {comp_name} -> plugins/{plugin_name}/yang/ ({len(yang_names)} files: {', '.join(yang_names)})"
        )

    print()
    print(f"Total moves: {len(all_actions)}")

    # Update self-containment test references
    updated_tests = update_self_containment_tests(apply)
    if updated_tests:
        print(f"\nUpdated self-containment tests: {len(updated_tests)}")
        for t in updated_tests:
            print(f"  {t}")

    if not apply:
        print()
        print(
            "Dry run. To apply: python3 scripts/dev/move_cmd_yang_to_plugins.py --apply"
        )
        print(
            "After applying, run: go run scripts/codegen/yang_glue.go && go run scripts/codegen/plugin_imports.go"
        )

    return 0


if __name__ == "__main__":
    sys.exit(main())
