#!/usr/bin/env python3
"""Exercise the real Onix X11 window. Run under xvfb-run; needs Pillow and libXtst."""
import ctypes
import ctypes.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time

from PIL import ImageGrab

ROOT = Path(__file__).resolve().parents[1]
(ROOT / "artifacts").mkdir(exist_ok=True)
x11 = ctypes.CDLL(ctypes.util.find_library("X11"))
xtst = ctypes.CDLL(ctypes.util.find_library("Xtst"))
x11.XOpenDisplay.argtypes = [ctypes.c_char_p]
x11.XOpenDisplay.restype = ctypes.c_void_p
x11.XKeysymToKeycode.argtypes = [ctypes.c_void_p, ctypes.c_ulong]
x11.XKeysymToKeycode.restype = ctypes.c_ubyte
x11.XDefaultRootWindow.argtypes = [ctypes.c_void_p]
x11.XDefaultRootWindow.restype = ctypes.c_ulong
x11.XQueryTree.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.POINTER(ctypes.c_ulong), ctypes.POINTER(ctypes.c_ulong), ctypes.POINTER(ctypes.POINTER(ctypes.c_ulong)), ctypes.POINTER(ctypes.c_uint)]
x11.XSetInputFocus.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.c_int, ctypes.c_ulong]
x11.XFlush.argtypes = [ctypes.c_void_p]
x11.XFree.argtypes = [ctypes.c_void_p]
x11.XCloseDisplay.argtypes = [ctypes.c_void_p]
x11.XResizeWindow.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.c_uint, ctypes.c_uint]
x11.XMoveWindow.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.c_int, ctypes.c_int]
xtst.XTestFakeKeyEvent.argtypes = [ctypes.c_void_p, ctypes.c_uint, ctypes.c_int, ctypes.c_ulong]
xtst.XTestFakeMotionEvent.argtypes = [ctypes.c_void_p, ctypes.c_int, ctypes.c_int, ctypes.c_int, ctypes.c_ulong]
xtst.XTestFakeButtonEvent.argtypes = [ctypes.c_void_p, ctypes.c_uint, ctypes.c_int, ctypes.c_ulong]
display = x11.XOpenDisplay(None)
assert display, "Run this script with xvfb-run -a"


def key(sym, down=True):
    code = x11.XKeysymToKeycode(display, sym)
    assert code, f"Unmapped key {sym}"
    xtst.XTestFakeKeyEvent(display, code, int(down), 0)
    x11.XFlush(display)


def chord(letter, modifiers=(0xFFE3, 0xFFE1)):
    for mod in modifiers:
        key(mod)
    key(ord(letter)); key(ord(letter), False)
    for mod in reversed(modifiers):
        key(mod, False)
    time.sleep(0.15)


def press(sym):
    key(sym); key(sym, False)
    time.sleep(0.1)


def click(x, y, button=1):
    xtst.XTestFakeMotionEvent(display, -1, x, y, 0)
    xtst.XTestFakeButtonEvent(display, button, 1, 0)
    xtst.XTestFakeButtonEvent(display, button, 0, 0)
    x11.XFlush(display)
    time.sleep(0.15)


def type_text(value, submit=True):
    for char in value:
        shifted = char.isupper() or char in '~!@#$%^&*()_+{}|:"<>?'
        if shifted:
            key(0xFFE1)
        key(ord(char)); key(ord(char), False)
        if shifted:
            key(0xFFE1, False)
        time.sleep(0.008)
    if submit:
        press(0xFF0D)


def wait_for(condition, description):
    deadline = time.monotonic() + 8
    while time.monotonic() < deadline:
        if condition():
            return
        time.sleep(0.05)
    raise AssertionError(description)


def children(pid):
    path = Path(f"/proc/{pid}/task/{pid}/children")
    return path.read_text().split() if path.exists() else []


with tempfile.TemporaryDirectory(prefix="xiterm-smoke-") as temp:
    temp = Path(temp)
    marker = temp / "keyboard-ok"
    config = temp / "config.json"
    config.write_text(json.dumps({
        "default_profile": "Smoke",
        "width": 1000, "height": 650,
        "profiles": [{"name": "Smoke", "shell": "/bin/bash", "args": ["--noprofile", "--norc", "-i"], "directory": str(temp), "env": ["PS1=xiterm $ ", f"HISTFILE={temp / 'history'}"]}],
    }))
    process = subprocess.Popen([str(ROOT / "bin/xiterm"), "-config", str(config)], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    try:
        root = x11.XDefaultRootWindow(display)
        windows = []
        def find_window():
            parent, root_return = ctypes.c_ulong(), ctypes.c_ulong()
            array, count = ctypes.POINTER(ctypes.c_ulong)(), ctypes.c_uint()
            x11.XQueryTree(display, root, ctypes.byref(root_return), ctypes.byref(parent), ctypes.byref(array), ctypes.byref(count))
            windows[:] = [array[i] for i in range(count.value)]
            if array:
                x11.XFree(array)
            return bool(windows)
        wait_for(find_window, "Native window did not open")
        x11.XMoveWindow(display, windows[-1], 0, 0)
        x11.XSetInputFocus(display, windows[-1], 1, 0)
        x11.XFlush(display)
        wait_for(lambda: len(children(process.pid)) == 1, "Initial shell did not start")
        time.sleep(0.5)
        type_text(f"printf keyboard-ok > {marker}")
        wait_for(lambda: marker.exists(), "Keyboard text and Enter did not reach the PTY")
        assert marker.read_text() == "keyboard-ok"
        # Plain letters, especially c/d/t after a shortcut, must remain text.
        chord("t")
        wait_for(lambda: len(children(process.pid)) == 2, "Shortcut did not open a tab")
        type_text("exit")
        wait_for(lambda: len(children(process.pid)) == 1, "exit did not remove the tab")
        directory = temp / "directory-ok"
        type_text(f"cd /; pwd > {directory}")
        wait_for(directory.exists, "cd was intercepted by an application shortcut")
        assert directory.read_text().strip() == "/"
        assert len(children(process.pid)) == 1, "Plain cd created a tab or split"
        type_text(f"cd {temp}")
        # History previews do not execute until Enter; Down restores a draft.
        history_marker = temp / "history-ok"
        type_text(f"printf recalled > {history_marker}")
        wait_for(history_marker.exists, "History setup failed")
        history_marker.unlink()
        press(0xFF52)  # Up
        assert not history_marker.exists(), "Up executed history without confirmation"
        press(0xFF54)  # Down: return to the empty draft.
        press(0xFF0D)
        assert not history_marker.exists(), "Down did not restore the draft"
        press(0xFF52); press(0xFF0D)
        wait_for(history_marker.exists, "Up/Enter did not recall and execute history")
        draft_marker = temp / "draft-ok"
        type_text(f"printf draft > {draft_marker}", submit=False)
        press(0xFF52); press(0xFF54); press(0xFF0D)
        wait_for(draft_marker.exists, "Down did not restore the unsubmitted command")
        # stty reads the actual PTY dimensions, not the rendered scene size.
        def terminal_size(name, width, height):
            x11.XResizeWindow(display, windows[-1], width, height)
            x11.XFlush(display)
            time.sleep(0.35)
            path = temp / name
            type_text(f"stty size > {path}")
            wait_for(path.exists, "PTY dimensions were not reported")
            return tuple(map(int, path.read_text().split()))
        base = terminal_size("base-size", 1000, 650)
        grown = terminal_size("grown-size", 1200, 780)
        shrunk = terminal_size("shrunk-size", 800, 520)
        assert grown[0] > base[0] and grown[1] > base[1], (base, grown)
        assert shrunk[0] < base[0] and shrunk[1] < base[1], (base, shrunk)
        terminal_size("restored-size", 1200, 780)
        # Menu opens on right click and supports both keyboard and mouse.
        click(350, 100, 3)
        press(0xFF54); press(0xFF0D)
        wait_for(lambda: len(children(process.pid)) == 2, "Context menu New tab failed")
        type_text("exit")
        wait_for(lambda: len(children(process.pid)) == 1, "Menu tab did not close on exit")
        click(350, 100, 3)
        ImageGrab.grab(xdisplay=os.environ["DISPLAY"]).save(ROOT / "artifacts/context-menu.png")
        click(390, 100 + 6 + 32 + 16)  # Split columns.
        wait_for(lambda: len(children(process.pid)) == 2, "Context menu Split columns failed")
        type_text("exit")
        wait_for(lambda: len(children(process.pid)) == 1, "Exited split pane was retained")
        type_text("printf '\\033[32mXiTerm native terminal\\033[0m\\n'; pwd; uname -s")
        type_text("printf '\\357\\202\\233  \\356\\202\\260  \\357\\204\\240  Nerd Font icons\\n'")
        chord("d")
        wait_for(lambda: len(children(process.pid)) == 2, "Column split did not create a second shell")
        chord("e")
        wait_for(lambda: len(children(process.pid)) == 3, "Nested row split did not create a third shell")
        type_text("printf 'independent shell session\\n'; stty size")
        chord("t")
        wait_for(lambda: len(children(process.pid)) == 4, "New tab did not create an independent shell")
        type_text("exit")
        wait_for(lambda: len(children(process.pid)) == 3, "Closing a tab did not reap its shell")
        time.sleep(0.4)
        output = ROOT / "artifacts/native-smoke.png"
        output.parent.mkdir(exist_ok=True)
        screenshot = ImageGrab.grab(xdisplay=os.environ["DISPLAY"]).convert("RGB")
        screenshot.save(output)
        # Inspect a region that contains text in the lower-right split; this
        # catches incorrect rendering transforms that process checks miss.
        ink = sum(1 for r, g, b in screenshot.crop((680, 460, 1200, 620)).getdata()
                  if r > 110 and g > 110 and b > 110)
        assert ink > 300, "Terminal glyphs are missing from the split pane"
        chord("f")
        type_text("independent")
        key(0xFF1B); key(0xFF1B, False)
        chord("s")
        wait_for(lambda: json.loads(config.read_text()).get("font_size") == 15, "Preferences were not saved")
        child_pids = children(process.pid)
        for _ in range(3):
            type_text("exit")
        process.wait(timeout=8)
        assert process.returncode == 0, process.stderr.read().decode()
        wait_for(lambda: all(not Path(f"/proc/{pid}").exists() for pid in child_pids), "Shell processes survived window close")
        print(f"PASS: cd input, Up/Down history/drafts, real PTY grow/shrink {base}/{grown}/{shrunk}, SVG context menu, nested splits, tabs, search, save, exit/last-tab cleanup; screenshot {output}")
    finally:
        if process.poll() is None:
            ImageGrab.grab(xdisplay=os.environ["DISPLAY"]).save(ROOT / "artifacts/native-failure.png")
            process.terminate()
            process.wait(timeout=5)
        x11.XCloseDisplay(display)
