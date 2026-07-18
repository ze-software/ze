#!/usr/bin/env python3

import importlib.util
import pathlib
import tempfile
import unittest
from unittest import mock


HERE = pathlib.Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location("terminal_render", HERE / "render.py")
terminal_render = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(terminal_render)


class SourceDigestTest(unittest.TestCase):
    def test_ignores_page_metadata_but_tracks_render_inputs(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            demo_root = root / "demos" / "terminal"
            source_dir = demo_root / "example"
            source_dir.mkdir(parents=True)
            shared = demo_root / "common.tape"
            shared.write_text("Set Width 1200\n")
            source = source_dir / "demo.tape"
            source.write_text("Type 'ze show version'\n")
            demo = {
                "id": "example",
                "source": "example/demo.tape",
                "page": "guide/example.md",
                "anchor": "first",
                "title": "First title",
            }

            with mock.patch.multiple(
                terminal_render,
                ROOT=root,
                DEMO_ROOT=demo_root,
                SHARED_SOURCE_PATHS=(shared,),
            ):
                baseline = terminal_render.source_digest(demo)

                page_only = dict(
                    demo,
                    page="guide/elsewhere.md",
                    anchor="second",
                    title="Second title",
                )
                self.assertEqual(baseline, terminal_render.source_digest(page_only))

                source.write_text("Type 'ze show health'\n")
                self.assertNotEqual(baseline, terminal_render.source_digest(demo))

                source.write_text("Type 'ze show version'\n")
                privileged = dict(demo, privileged=True)
                self.assertNotEqual(baseline, terminal_render.source_digest(privileged))

                realtime = dict(demo, realtime=True)
                self.assertNotEqual(baseline, terminal_render.source_digest(realtime))


class RenderAccelerationTest(unittest.TestCase):
    def test_shortens_capture_delays_without_changing_presentation_timing(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            demo_root = root / "demos" / "terminal"
            source = demo_root / "example" / "demo.tape"
            source.parent.mkdir(parents=True)
            source.write_text(
                "Output artifacts/example.webm\n"
                "Source common.tape\n"
                "Sleep 20s\n"
                'Type "ze show version"\n'
            )
            with mock.patch.multiple(
                terminal_render,
                ROOT=root,
                DEMO_ROOT=demo_root,
            ):
                generated = terminal_render.accelerated_terminal_tape(
                    {"id": "example", "source": "example/demo.tape"}
                )
                rendered = generated.read_text()
            self.assertIn("Set TypingSpeed 25ms\n", rendered)
            self.assertIn("Sleep 4000ms\n", rendered)
            self.assertNotIn("PlaybackSpeed", rendered)
            self.assertIn("Sleep 20s\n", source.read_text())

    def test_expands_capture_timestamps_and_dimensions(self):
        with tempfile.TemporaryDirectory() as tmp:
            capture = pathlib.Path(tmp) / "example.webm"
            capture.write_bytes(b"compressed")

            def write_expanded(command, check):
                self.assertTrue(check)
                self.assertEqual(command[command.index("-itsscale") + 1], "5")
                self.assertEqual(command[command.index("-c:v") + 1], "libvpx-vp9")
                self.assertIn("scale=1680:1008:flags=lanczos", command)
                pathlib.Path(command[-1]).write_bytes(b"expanded")

            with mock.patch.object(
                terminal_render.subprocess, "run", side_effect=write_expanded
            ):
                terminal_render.expand_timeline(capture, terminal_render.RENDER_SPEEDUP)
            self.assertEqual(capture.read_bytes(), b"expanded")
            self.assertFalse((pathlib.Path(tmp) / "example.fast.webm").exists())

    def test_realtime_capture_keeps_original_timestamps(self):
        with tempfile.TemporaryDirectory() as tmp:
            capture = pathlib.Path(tmp) / "example.webm"
            capture.write_bytes(b"compressed")

            def write_expanded(command, check):
                self.assertTrue(check)
                self.assertEqual(command[command.index("-itsscale") + 1], "1")
                self.assertEqual(command[command.index("-c:v") + 1], "libvpx-vp9")
                self.assertIn("scale=1680:1008:flags=lanczos", command)
                pathlib.Path(command[-1]).write_bytes(b"expanded")

            with mock.patch.object(
                terminal_render.subprocess, "run", side_effect=write_expanded
            ):
                terminal_render.expand_timeline(capture, 1)
            self.assertEqual(capture.read_bytes(), b"expanded")


class RenderTapeSelectionTest(unittest.TestCase):
    def _write_source(self, tmp):
        root = pathlib.Path(tmp)
        demo_root = root / "demos" / "terminal"
        source = demo_root / "example" / "demo.tape"
        source.parent.mkdir(parents=True)
        source.write_text(
            "Output artifacts/example.webm\n"
            "Source common.tape\n"
            "Sleep 20s\n"
            'Type "ze show version"\n'
        )
        return root, demo_root, source

    def test_normal_demo_uses_accelerated_tape(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, demo_root, source = self._write_source(tmp)
            demo = {
                "id": "example",
                "source": "example/demo.tape",
                "kind": "terminal",
            }
            with mock.patch.multiple(
                terminal_render, ROOT=root, DEMO_ROOT=demo_root
            ):
                self.assertEqual(terminal_render.capture_speedup(demo), 5)
                tape = terminal_render.render_tape(demo)
                self.assertNotEqual(tape, source)
                rendered = tape.read_text()
            self.assertIn("Set TypingSpeed 25ms\n", rendered)
            self.assertIn("Sleep 4000ms\n", rendered)

    def test_realtime_demo_uses_original_tape_unchanged(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, demo_root, source = self._write_source(tmp)
            demo = {
                "id": "example",
                "source": "example/demo.tape",
                "kind": "terminal",
                "realtime": True,
            }
            with mock.patch.multiple(
                terminal_render, ROOT=root, DEMO_ROOT=demo_root
            ):
                self.assertEqual(terminal_render.capture_speedup(demo), 1)
                tape = terminal_render.render_tape(demo)
                self.assertEqual(tape, source)
                rendered = tape.read_text()
            self.assertNotIn("Set TypingSpeed 25ms", rendered)
            self.assertIn("Sleep 20s\n", rendered)
            self.assertEqual(rendered, source.read_text())


if __name__ == "__main__":
    unittest.main()
