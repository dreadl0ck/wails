#!/usr/bin/env python3
"""Run under xvfb-run + dbus-run-session. Requires Gtk3 introspection and xdotool."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time


def receiver(output):
    import gi
    gi.require_version("Gtk", "3.0")
    from gi.repository import Gtk, Gdk
    window = Gtk.Window(title="Wails file drag receiver")
    window.set_default_size(300, 220)
    window.add(Gtk.Label(label="Drop files here"))
    window.drag_dest_set(Gtk.DestDefaults.ALL, [Gtk.TargetEntry.new("text/uri-list", 0, 0)], Gdk.DragAction.COPY)
    def dropped(widget, context, x, y, selection, info, timestamp):
        Path(output).write_text(json.dumps(selection.get_uris()))
        Gtk.drag_finish(context, True, False, timestamp)
    window.connect("drag-data-received", dropped)
    window.show_all()
    Gtk.main()


def wait(check, label):
    deadline = time.monotonic() + 15
    while time.monotonic() < deadline:
        value = check()
        if value: return value
        time.sleep(0.05)
    raise RuntimeError("Timed out waiting for " + label)


def xd(*args):
    return subprocess.check_output(["xdotool", *map(str, args)], text=True).strip()


def find_window(title):
    result = subprocess.run(["xdotool", "search", "--onlyvisible", "--name", "^" + title + "$"], capture_output=True, text=True)
    return result.stdout.splitlines()[0] if result.returncode == 0 and result.stdout.strip() else None


def run(binary):
    with tempfile.TemporaryDirectory(prefix="wails-file-drag-") as root:
        root = Path(root)
        (root / "another folder").mkdir()
        files = [root / "sample #1 ü.txt", root / "another folder" / "second.txt"]
        for file in files: file.write_text("native file drag fixture")
        output, log = root / "received.json", root / "source.log"
        env = {**os.environ, "GDK_BACKEND": "x11", "WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS": "1", "LIBGL_ALWAYS_SOFTWARE": "1"}
        with log.open("w") as logs:
            target = subprocess.Popen([sys.executable, __file__, "--receiver", str(output)], env=env)
            source = subprocess.Popen([binary, *map(str, files)], stdout=logs, stderr=logs, env=env)
            try:
                src = wait(lambda: find_window("Wails file drag source"), "source window")
                dst = wait(lambda: find_window("Wails file drag receiver"), "receiver window")
                wait(lambda: "file-drag-out ready" in log.read_text(), "frontend runtime")
                xd("windowmove", "--sync", src, 30, 30)
                xd("windowmove", "--sync", dst, 450, 30)
                xd("mouseup", 1)
                xd("windowfocus", "--sync", src)
                wait(lambda: "trace: armed" in log.read_text(), "initial armed source")
                xd("mousemove", "--sync", "--window", src, 120, 120)
                xd("click", 1)
                wait(lambda: "trace: click" in log.read_text(), "ordinary click while armed")
                def drag(cancel=False):
                    starts = log.read_text().count("Starting native file drag-out")
                    xd("windowfocus", "--sync", src)
                    xd("mousemove", "--sync", 350, 300)
                    xd("mousemove", "--sync", "--window", src, 120, 120)
                    wait(lambda: log.read_text().count("trace: armed") > log.read_text().count("file-drag-out ended"), "armed source")
                    xd("mousemove", "--sync", 350, 300)
                    xd("mousemove", "--sync", "--window", src, 120, 120)
                    xd("mousedown", 1)
                    time.sleep(0.05)
                    for x in (150, 160, 170, 180):
                        xd("mousemove", "--sync", "--window", src, x, 120)
                        time.sleep(0.05)
                    wait(lambda: log.read_text().count("Starting native file drag-out") > starts, "native drag start")
                    if cancel: xd("key", "Escape")
                    else:
                        for x in range(200, 601, 40):
                            xd("mousemove", "--sync", x, 150)
                            time.sleep(0.05)
                    xd("mouseup", 1)
                drag()
                wait(output.exists, "native URI transfer")
                expected = [file.as_uri() for file in files]
                if json.loads(output.read_text()) != expected: raise RuntimeError(f"Native file paths changed during transfer: {output.read_text()} != {expected}")
                wait(lambda: "performed:true" in log.read_text(), "successful completion")
                output.unlink()
                drag(cancel=True)
                wait(lambda: "performed:false" in log.read_text(), "cancellation callback")
                if output.exists(): raise RuntimeError("Cancelled drag delivered files")
                drag()
                wait(output.exists, "drag after cancellation")
                if json.loads(output.read_text()) != expected: raise RuntimeError("Second transfer changed the file paths")
                wait(lambda: log.read_text().count("file-drag-out ended: map[performed:true]") >= 2, "final native completion")
                print("PASS: ordinary click, multi-file native transfer, Unicode/spaces, cancellation, and reuse")
            except Exception:
                print(log.read_text(), file=sys.stderr)
                raise
            finally:
                xd("mouseup", 1)
                xd("keyup", "Escape")
                for process in (source, target):
                    process.terminate()
                    try: process.wait(timeout=5)
                    except subprocess.TimeoutExpired: process.kill(); process.wait()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", nargs="?")
    parser.add_argument("--receiver")
    args = parser.parse_args()
    if args.receiver: receiver(args.receiver)
    elif args.binary: run(str(Path(args.binary).resolve()))
    else: parser.error("a probe binary is required")
