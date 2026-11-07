#!/usr/bin/env python3
"""Convert rendered source PNG into multi-resolution Windows and macOS icons."""
from pathlib import Path
from PIL import Image

icons = Path(__file__).resolve().parents[1] / 'assets/icons'
with Image.open(icons / 'xiterm-1024.png') as source:
    image = source.convert('RGBA')
    image.save(icons / 'xiterm.ico', sizes=[(n, n) for n in [16, 24, 32, 48, 64, 128, 256]])
    image.save(icons / 'xiterm.icns')
