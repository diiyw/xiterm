#!/usr/bin/env python3
"""Extract and exercise the distributed Linux executable, not the build-tree binary."""
import argparse
from pathlib import Path
import subprocess
import tarfile
import tempfile

from package import DIST

parser = argparse.ArgumentParser()
parser.add_argument("--version", required=True)
args = parser.parse_args()
archive = DIST / f"xiterm-{args.version}-linux-amd64.tar.gz"
with tempfile.TemporaryDirectory(prefix="xiterm-install-test-") as temp:
    stage = Path(temp)
    with tarfile.open(archive) as tar:
        tar.extractall(stage, filter="data")
    package = stage / f"xiterm-{args.version}-linux-amd64"
    assert (package / "share/icons/hicolor/512x512/apps/xiterm.png").is_file()
    assert (package / "share/applications/xiterm.desktop").is_file()
    binary = package / "bin/xiterm"
    output = subprocess.check_output([binary, "-version"], text=True).strip()
    assert output == f"XiTerm {args.version}", output
    subprocess.run(["xvfb-run", "-a", str(binary), "-smoke-duration=3s"], check=True)
    print("PASS: extracted Linux archive, version, icon, desktop entry and native window")
