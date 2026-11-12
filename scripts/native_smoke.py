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


class XWindowAttributes(ctypes.Structure):
    _fields_ = [
        ("x", ctypes.c_int), ("y", ctypes.c_int),
        ("width", ctypes.c_int), ("height", ctypes.c_int),
        ("border_width", ctypes.c_int), ("depth", ctypes.c_int),
        ("visual", ctypes.c_void_p), ("root", ctypes.c_ulong),
        ("window_class", ctypes.c_int), ("bit_gravity", ctypes.c_int),
        ("win_gravity", ctypes.c_int), ("backing_store", ctypes.c_int),
        ("backing_planes", ctypes.c_ulong), ("backing_pixel", ctypes.c_ulong),
        ("save_under", ctypes.c_int), ("colormap", ctypes.c_ulong),
        ("map_installed", ctypes.c_int), ("map_state", ctypes.c_int),
        ("all_event_masks", ctypes.c_long), ("your_event_mask", ctypes.c_long),
        ("do_not_propagate_mask", ctypes.c_long), ("override_redirect", ctypes.c_int),
        ("screen", ctypes.c_void_p),
    ]


x11.XGetWindowAttributes.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.POINTER(XWindowAttributes)]
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


def drag_tab(start_x, end_x):
    xtst.XTestFakeMotionEvent(display, -1, start_x, 20, 0)
    xtst.XTestFakeButtonEvent(display, 1, 1, 0)
    x11.XFlush(display)
    time.sleep(0.1)
    for step in range(1, 13):
        x = round(start_x + (end_x - start_x) * step / 12)
        xtst.XTestFakeMotionEvent(display, -1, x, 20, 0)
        x11.XFlush(display)
        time.sleep(0.025)
    xtst.XTestFakeButtonEvent(display, 1, 0, 0)
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
            windows.clear()
            for index in range(count.value):
                attrs = XWindowAttributes()
                if x11.XGetWindowAttributes(display, array[index], ctypes.byref(attrs)) and attrs.map_state == 2:
                    windows.append(array[index])
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
        # Home/End must edit the draft in Readline's application cursor mode.
        navigation_marker = temp / "home-end-ok"
        type_text("printf home", submit=False)
        press(0xFF50)  # Home
        type_text("true; ", submit=False)
        press(0xFF57)  # End
        type_text(f"end > {navigation_marker}")
        wait_for(navigation_marker.exists, "Home/End did not edit the command")
        assert navigation_marker.read_text() == "homeend"
        # A full-screen raw application receives navigation instead of local
        # history scrolling. Check the actual PTY bytes, including DECCKM.
        recorder = temp / "navigation.py"
        raw_ready, raw_result = temp / "raw-ready", temp / "raw-result"
        recorder.write_text(
            "import os, pathlib, termios, tty\n"
            "old = termios.tcgetattr(0)\n"
            "try:\n"
            "    tty.setraw(0)\n"
            "    os.write(1, b'\\x1b[?1049h\\x1b[?1h')\n"
            f"    pathlib.Path({str(raw_ready)!r}).touch()\n"
            "    data = b''\n"
            "    while len(data) < 14:\n"
            "        data += os.read(0, 14 - len(data))\n"
            f"    pathlib.Path({str(raw_result)!r}).write_bytes(data)\n"
            "finally:\n"
            "    os.write(1, b'\\x1b[?1l\\x1b[?1049l')\n"
            "    termios.tcsetattr(0, termios.TCSANOW, old)\n"
        )
        type_text(f"python3 {recorder}")
        wait_for(raw_ready.exists, "Raw navigation recorder did not start")
        for sym in (0xFF50, 0xFF57, 0xFF55, 0xFF56):
            press(sym)
        wait_for(raw_result.exists, "Full-screen navigation did not reach the PTY")
        assert raw_result.read_bytes() == b"\x1bOH\x1bOF\x1b[5~\x1b[6~", raw_result.read_bytes()
        time.sleep(0.2)
        # A continuously repainting raw TUI must receive every keystroke
        # promptly, including while its output queue stays backpressured.
        busy_script = temp / "busy-input.py"
        busy_ready, busy_result = temp / "busy-ready", temp / "busy-result"
        typed = "frequentlyrefreshingterminalinput" * 3
        busy_script.write_text(
            "import json, os, pathlib, termios, threading, time, tty\n"
            "old = termios.tcgetattr(0)\n"
            "stopped = threading.Event()\n"
            "def repaint():\n"
            "    cols, rows = os.get_terminal_size(1)\n"
            "    line = (b'frequently refreshing application ' * cols)[:cols - 1]\n"
            "    frame = b'\\x1b[H' + (b'\\x1b[32m' + line + b'\\x1b[0m\\r\\n') * (rows - 2)\n"
            "    while not stopped.is_set():\n"
            "        os.write(1, frame)\n"
            "try:\n"
            "    tty.setraw(0)\n"
            "    os.write(1, b'\\x1b[?1049h\\x1b[?25l')\n"
            "    writer = threading.Thread(target=repaint, daemon=True)\n"
            "    writer.start()\n"
            f"    pathlib.Path({str(busy_ready)!r}).touch()\n"
            "    received = []\n"
            f"    while len(received) < {len(typed)}:\n"
            "        data = os.read(0, 1)\n"
            "        received.append([data.decode('ascii'), time.monotonic()])\n"
            "    stopped.set()\n"
            "    writer.join(timeout=3)\n"
            f"    pathlib.Path({str(busy_result)!r}).write_text(json.dumps(received))\n"
            "finally:\n"
            "    stopped.set()\n"
            "    os.write(1, b'\\x1b[0m\\x1b[?25h\\x1b[?1049l')\n"
            "    termios.tcsetattr(0, termios.TCSANOW, old)\n"
        )
        type_text(f"python3 {busy_script}")
        wait_for(busy_ready.exists, "Continuously refreshing TUI did not start")
        time.sleep(0.2)
        sent = []
        for char in typed:
            sent.append(time.monotonic())
            key(ord(char)); key(ord(char), False)
            time.sleep(0.008)
        wait_for(busy_result.exists, "Input stalled during continuous TUI repainting")
        received = json.loads(busy_result.read_text())
        assert "".join(char for char, _ in received) == typed, "Busy TUI input was lost or reordered"
        latencies = sorted(at - started for (_, at), started in zip(received, sent))
        p95 = latencies[int(len(latencies) * 0.95)]
        assert p95 < 0.150, f"Busy TUI input p95 latency is {p95 * 1000:.1f} ms"
        print(f"Busy TUI input: p95={p95 * 1000:.1f} ms, max={max(latencies) * 1000:.1f} ms", flush=True)
        time.sleep(0.2)
        # Wide Chinese glyphs must retain ink in both terminal columns. Use
        # escapes for input so this check does not need an installed IME.
        chinese = "".join(r"\u" + f"{ord(char):04x}" for char in "腾讯应用宝")
        type_text(r"printf '\033[2J\033[H\033[33m" + chinese + r"\033[0m\n\033[1;33m" + chinese + r"\033[0m\n'")
        time.sleep(0.25)
        cjk_image = ImageGrab.grab(xdisplay=os.environ["DISPLAY"]).convert("RGB")
        cjk_image.save(ROOT / "artifacts/chinese-rendering.png")
        for row in range(2):
            for col in range(5):
                glyph = cjk_image.crop((12 + col * 18, 50 + row * 22, 30 + col * 18, 72 + row * 22))
                ink_columns = [x for x in range(18) if any(
                    glyph.getpixel((x, y))[0] > 130 and glyph.getpixel((x, y))[1] > 100
                    for y in range(22))]
                assert ink_columns and max(ink_columns) - min(ink_columns) >= 10, (row, col, ink_columns)
        # Main-screen PageUp/PageDown alter the visible history; Shift+End
        # restores the exact same live text (exclude the blinking cursor).
        paging_ready = temp / "paging-ready"
        type_text(r"for i in {1..80}; do printf 'history %03d\n' $i; done; touch " + str(paging_ready))
        wait_for(paging_ready.exists, "History output did not complete")
        time.sleep(0.25)
        def history_pixels():
            return ImageGrab.grab(xdisplay=os.environ["DISPLAY"]).crop((12, 50, 700, 490)).tobytes()
        live_pixels = history_pixels()
        press(0xFF55)
        assert history_pixels() != live_pixels, "PgUp did not move through history"
        press(0xFF56)
        assert history_pixels() == live_pixels, "PgDn did not restore the live screen"
        key(0xFFE1)
        press(0xFF50)  # Shift+Home
        key(0xFFE1, False)
        assert history_pixels() != live_pixels, "Shift+Home did not reach history"
        key(0xFFE1)
        press(0xFF57)  # Shift+End
        key(0xFFE1, False)
        assert history_pixels() == live_pixels, "Shift+End did not return to live output"
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
        # Repeated zoom must release old glyph atlases without breaking text
        # rendering or leaving the PTY at an obsolete font size.
        def open_preferences(x=350, y=100):
            click(x, y, 3)
            for _ in range(8):
                press(0xFF54)
            press(0xFF0D)
        open_preferences()
        for _ in range(3):
            click(879, 211)  # Preferences font size +.
            click(829, 211)  # Preferences font size -.
        click(879, 211)
        press(0xFF1B)
        zoomed = terminal_size("zoomed-size", 1200, 780)
        open_preferences()
        click(829, 211)
        press(0xFF1B)
        reset = terminal_size("font-reset-size", 1200, 780)
        assert zoomed[0] < reset[0] and zoomed[1] < reset[1], (zoomed, reset)
        assert reset == grown, (reset, grown)
        # Reordering must preserve shell identity and update shortcut/close targets.
        type_text("export XITERM_TAB=first")
        for identity in ("second", "third"):
            chord("t")
            type_text(f"export XITERM_TAB={identity}")
        wait_for(lambda: len(children(process.pid)) == 3, "Drag setup did not open three tabs")
        drag_tab(580, 100)  # third, first, second
        drag_marker = temp / "drag-active"
        type_text(f'printf "$XITERM_TAB" > {drag_marker}')
        wait_for(drag_marker.exists, "Dragged tab lost keyboard focus")
        assert drag_marker.read_text() == "third", "Drag switched to a different shell"
        for index, identity in enumerate(("third", "first", "second"), 1):
            chord(str(index), modifiers=(0xFFE9,))  # Alt+1/2/3
            result = temp / f"drag-slot-{index}"
            type_text(f'printf "$XITERM_TAB" > {result}')
            wait_for(result.exists, "Number shortcut failed after drag")
            assert result.read_text() == identity, f"Wrong shell in reordered slot {index}"
        drag_tab(100, 580)  # first, second, third
        click(750, 20)  # Close third at its new position.
        wait_for(lambda: len(children(process.pid)) == 2, "Reordered close button did not reap its shell")
        result = temp / "drag-close"
        type_text(f'printf "$XITERM_TAB" > {result}')
        wait_for(result.exists, "Closing reordered tab lost the remaining shell")
        assert result.read_text() == "second", "Close button removed the wrong shell"
        type_text("exit")
        wait_for(lambda: len(children(process.pid)) == 1, "Drag cleanup did not restore the first tab")
        time.sleep(0.3)  # Let session cleanup and the input target change finish.
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
        # Even-sized windows put split origins on fractional pixels. Adjacent
        # background runs must stay opaque there, including after font zoom.
        def check_grid_backgrounds(font_size):
            grid_ready = temp / f"grid-{font_size}-ready"
            grid = "\\033[2J\\033[H\\033[?25l"
            for row in range(4):
                grid += f"\\033[{row + 1};1H"
                for col in range(20):
                    color = "152;195;121" if (row + col) % 2 == 0 else "86;182;194"
                    grid += f"\\033[48;2;{color}m "
            grid += "\\033[0m\\033[6;1H"
            type_text(f"printf '{grid}'; touch {grid_ready}")
            wait_for(grid_ready.exists, "Grid background output did not complete")
            time.sleep(0.2)
            grid_image = ImageGrab.grab(xdisplay=os.environ["DISPLAY"]).convert("RGB")
            grid_image.save(ROOT / f"artifacts/grid-backgrounds-{font_size}.png")
            # The lower-right pane starts at (600.5, 411.5), then has 12 px
            # horizontal padding, a 24 px header and 8 px vertical padding.
            # Stay inside the outer edges while inspecting every inner seam.
            region = grid_image.crop((614, 445, 790, 443 + 4 * (font_size + 7)))
            colors = set(region.getdata())
            allowed = {(152, 195, 121), (86, 182, 194)}
            assert colors == allowed, f"Grid background seams at font {font_size}: {colors - allowed}"
        check_grid_backgrounds(15)
        open_preferences(800, 500)
        click(879, 211)  # Font size 16: fractional cell advance.
        press(0xFF1B)
        check_grid_backgrounds(16)
        open_preferences(800, 500)
        click(829, 211)
        press(0xFF1B)
        type_text(r"printf '\033[2J\033[H\033[?25h'")
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
        print(f"PASS: cd input, Up/Down history/drafts, Home/End editing, full-screen navigation, Chinese glyphs, history paging/endpoints, real PTY grow/shrink {base}/{grown}/{shrunk}, repeated font zoom/reset, SVG context menu, nested splits, seamless grid backgrounds at 15/16 px, tab drag/reorder/shortcuts/close, search, save, exit/last-tab cleanup; screenshot {output}")
    finally:
        if process.poll() is None:
            ImageGrab.grab(xdisplay=os.environ["DISPLAY"]).save(ROOT / "artifacts/native-failure.png")
            process.terminate()
            process.wait(timeout=5)
        x11.XCloseDisplay(display)
