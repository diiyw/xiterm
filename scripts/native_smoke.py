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
xtst.XTestFakeKeyEvent.argtypes = [ctypes.c_void_p, ctypes.c_uint, ctypes.c_int, ctypes.c_ulong]
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


def type_text(value):
    for char in value:
        shifted = char.isupper() or char in '~!@#$%^&*()_+{}|:"<>?'
        if shifted:
            key(0xFFE1)
        key(ord(char)); key(ord(char), False)
        if shifted:
            key(0xFFE1, False)
        time.sleep(0.008)
    key(0xFF0D); key(0xFF0D, False)


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
        "profiles": [{"name": "Smoke", "shell": "/bin/sh", "args": ["-i"], "directory": str(temp), "env": ["PS1=xiterm $ "]}],
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
        x11.XSetInputFocus(display, windows[-1], 1, 0)
        x11.XFlush(display)
        wait_for(lambda: len(children(process.pid)) == 1, "Initial shell did not start")
        time.sleep(0.5)
        type_text(f"printf keyboard-ok > {marker}")
        wait_for(lambda: marker.exists(), "Keyboard text and Enter did not reach the PTY")
        assert marker.read_text() == "keyboard-ok"
        type_text("printf '\\033[32mXiTerm native terminal\\033[0m\\n'; pwd; uname -s")
        type_text("printf '\\357\\202\\233  \\356\\202\\260  \\357\\204\\240  Nerd Font icons\\n'")
        chord("d")
        wait_for(lambda: len(children(process.pid)) == 2, "Column split did not create a second shell")
        chord("e")
        wait_for(lambda: len(children(process.pid)) == 3, "Nested row split did not create a third shell")
        type_text("printf 'independent shell session\\n'; stty size")
        chord("t")
        wait_for(lambda: len(children(process.pid)) == 4, "New tab did not create an independent shell")
        chord("w")
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
            chord("w")
        process.wait(timeout=8)
        assert process.returncode == 0, process.stderr.read().decode()
        wait_for(lambda: all(not Path(f"/proc/{pid}").exists() for pid in child_pids), "Shell processes survived window close")
        print(f"PASS: native keyboard, PTY execution, nested splits, tabs, search, save, shell cleanup; screenshot {output}")
    finally:
        if process.poll() is None:
            process.terminate()
            process.wait(timeout=5)
        x11.XCloseDisplay(display)
