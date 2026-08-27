#!/usr/bin/env -S uv run --with pytest python3

import importlib.util
import pathlib
import sys


HERE = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
SPEC = importlib.util.spec_from_file_location(
    "render_cli_catalog", HERE / "render-cli-catalog.py"
)
render_cli_catalog = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(render_cli_catalog)


def command(**updates):
    value = {
        "path": "show bgp rib",
        "mode": "read-only",
        "description": "Show routes.",
        "pipes": [],
        "pipe-aliases": [],
        "operators": [],
    }
    value.update(updates)
    return value


def test_render_row_includes_command_pipes_aliases_and_operators():
    rendered = render_cli_catalog.render_row(
        command(
            pipes=[
                {
                    "name": "community",
                    "description": "Filter by community",
                    "takes-arg": True,
                }
            ],
            **{
                "pipe-aliases": [
                    {
                        "name": "summary",
                        "description": "Aggregate fields",
                        "expansion": "display router-id local-as",
                    }
                ],
                "operators": [
                    {
                        "name": "json",
                        "class": "global",
                        "available": "always",
                        "description": "JSON output",
                    },
                    {
                        "name": "match",
                        "class": "data",
                        "available": "with-rows",
                        "description": "Filter rows",
                    },
                ],
            },
        )
    )

    assert "1 command pipe" in rendered
    assert "1 alias" in rendered
    assert "2 operators" in rendered
    assert "community &lt;value&gt;" in rendered
    assert "display router-id local-as" in rendered
    assert "Always" in rendered
    assert "With rows" in rendered
    assert "json" in rendered
    assert "match" in rendered

def test_render_row_preserves_stream_and_local_surface_qualifiers():
    rendered = render_cli_catalog.render_row(
        command(
            operators=[
                {
                    "name": "log",
                    "class": "stream",
                    "available": "when-streaming",
                    "description": "Append updates",
                },
                {
                    "name": "save",
                    "class": "global",
                    "available": "always",
                    "local-only": True,
                    "description": "Write the answer",
                },
            ]
        )
    )

    assert "While streaming" in rendered
    assert "Local process only" in rendered
    assert "log" in rendered
    assert "save" in rendered
    markdown = render_cli_catalog.markdown_pipe_details(
        command(
            operators=[
                {
                    "name": "save",
                    "available": "always",
                    "local-only": True,
                }
            ]
        )
    )
    assert "Local process only: `save`" in markdown
    guide = render_cli_catalog.render_pipe_guide(
        [
            command(
                operators=[
                    {
                        "name": "save",
                        "class": "global",
                        "available": "always",
                        "local-only": True,
                        "description": "Write the answer",
                    }
                ]
            )
        ]
    )
    assert "Local process only" in guide

def test_render_pipe_contract_preserves_independent_dimensions_exactly():
    value = command(
        **{
            "answer-shape": "tab",
            "address-fields": ["address", "peer"],
            "operators": [
                {
                    "name": "save",
                    "class": "global",
                    "available": "always",
                    "local-only": True,
                    "description": "Write the answer",
                }
            ],
        }
    )

    assert render_cli_catalog.render_pipe_details(value) == (
        '<details class="cli-pipes"><summary>1 operator · answer: tab · 2 address fields</summary>'
        '<div class="cli-pipe-detail"><p><span>Answer shape</span><code>tab</code></p>'
        '<p><span>Address fields</span><code>address · peer</code></p>'
        '<p><span>Always</span><code>save</code></p>'
        '<p><span>Local process only</span><code>save</code></p></div></details>'
    )
    assert render_cli_catalog.markdown_pipe_details(value) == (
        "Answer shape: `tab`<br>Address fields: `address`, `peer`"
        "<br>Always: `save`<br>Local process only: `save`"
    )
    assert render_cli_catalog.operator_catalog([value]) == [
        {
            "name": "save",
            "class": "global",
            "description": "Write the answer",
            "available": ["always", "local-only"],
        }
    ]


def test_render_row_marks_commands_without_pipe_support():
    rendered = render_cli_catalog.render_row(command())

    assert '<span class="cli-pipe-none">None</span>' in rendered


def test_operator_guide_deduplicates_and_merges_availability():
    operators = [
        {
            "name": "match",
            "class": "data",
            "available": "with-rows",
            "description": "Filter rows",
        }
    ]
    always = [dict(operators[0], available="always")]

    rendered = render_cli_catalog.render_pipe_guide(
        [command(operators=operators), command(path="show bgp", operators=always)]
    )

    assert rendered.count("<code>match</code>") == 1
    assert "Always, With rows" in rendered
    assert "Filter rows" in rendered


def test_markdown_mirror_lists_pipe_capabilities():
    commands = [
        command(
            pipes=[{"name": "prefix", "takes-arg": True}],
            operators=[
                {
                    "name": "json",
                    "class": "global",
                    "available": "always",
                    "description": "JSON output",
                }
            ],
        )
    ]
    groups = [("show bgp", commands)]

    rendered = render_cli_catalog.render_markdown(commands, groups)

    assert "| Command | Mode | Description | Pipes |" in rendered
    assert "Command: `prefix <value>`" in rendered
    assert "Always: `json`" in rendered
