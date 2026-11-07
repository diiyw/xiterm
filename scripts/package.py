#!/usr/bin/env python3
"""Build native XiTerm release artifacts. Only the host platform is packaged."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import plistlib
import re
import shutil
import subprocess
import tarfile
import tempfile
import xml.etree.ElementTree as ET

ROOT = Path(__file__).resolve().parents[1]
DIST = ROOT / "dist"


def release_version(tag):
    match = re.fullmatch(r"v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)", tag)
    if not match:
        raise ValueError("Release tag must be vMAJOR.MINOR.PATCH (for example v0.1.0)")
    parts = tuple(map(int, match.groups()))
    if parts[0] > 255 or parts[1] > 255 or parts[2] > 65535:
        raise ValueError("Release version exceeds MSI limits (255.255.65535)")
    return ".".join(map(str, parts))


def run(*args, **kwargs):
    print("+", *map(str, args), flush=True)
    return subprocess.run(list(map(str, args)), check=True, cwd=ROOT, **kwargs)


def copy_notices(target):
    target.mkdir(parents=True, exist_ok=True)
    shutil.copy2(ROOT / "README.md", target / "README.md")
    for source in (ROOT / "assets/fonts").glob("*.txt"):
        shutil.copy2(source, target / source.name)
    shutil.copy2(ROOT / "assets/icons/ui/LICENSE.txt", target / "TABLER-ICONS-LICENSE.txt")
    # Include every dependency's own LICENSE/NOTICE files in the distribution.
    modules = subprocess.check_output(["go", "list", "-m", "-json", "all"], cwd=ROOT, text=True)
    decoder, offset = json.JSONDecoder(), 0
    while offset < len(modules):
        while offset < len(modules) and modules[offset].isspace():
            offset += 1
        if offset == len(modules):
            break
        module, offset = decoder.raw_decode(modules, offset)
        if module.get("Main") or "Dir" not in module:
            continue
        for source in Path(module["Dir"]).iterdir():
            if source.is_file() and source.name.upper().startswith(("LICENSE", "NOTICE", "COPYING", "THIRD_PARTY_NOTICES")):
                name = module["Path"].replace("/", "_") + "-" + source.name
                shutil.copy2(source, target / name)


def build_binary(version, target, windows=False):
    target.parent.mkdir(parents=True, exist_ok=True)
    flags = f"-s -w -X main.version={version}"
    if windows:
        flags += " -H=windowsgui -linkmode=external -extldflags=-static"
        run("go-winres", "make", "--in", ROOT / "packaging/windows/winres.json",
            "--out", ROOT / "cmd/xiterm/rsrc", "--arch", "amd64",
            "--file-version", version + ".0", "--product-version", version + ".0")
    run("go", "build", "-trimpath", "-ldflags", flags, "-o", target, "./cmd/xiterm")


def linux_package(version, arch):
    name = f"xiterm-{version}-linux-{arch}"
    with tempfile.TemporaryDirectory(prefix="xiterm-package-") as temp:
        stage = Path(temp) / name
        build_binary(version, stage / "bin/xiterm")
        copy_notices(stage / "share/doc/xiterm")
        icon_dir = stage / "share/icons/hicolor/512x512/apps"
        icon_dir.mkdir(parents=True)
        shutil.copy2(ROOT / "assets/icons/xiterm.png", icon_dir / "xiterm.png")
        desktop_dir = stage / "share/applications"
        desktop_dir.mkdir(parents=True)
        shutil.copy2(ROOT / "packaging/linux/xiterm.desktop", desktop_dir)
        archive = DIST / f"{name}.tar.gz"
        with tarfile.open(archive, "w:gz") as tar:
            tar.add(stage, arcname=name)
    return archive


def macos_package(version, arch):
    with tempfile.TemporaryDirectory(prefix="xiterm-package-") as temp:
        stage = Path(temp) / "volume"
        app = stage / "XiTerm.app"
        contents = app / "Contents"
        resources = contents / "Resources"
        resources.mkdir(parents=True)
        build_binary(version, contents / "MacOS/xiterm")
        shutil.copy2(ROOT / "assets/icons/xiterm.icns", resources / "xiterm.icns")
        copy_notices(resources / "Notices")
        with (ROOT / "packaging/macos/Info.plist").open("rb") as f:
            info = plistlib.load(f)
        info.update(CFBundleShortVersionString=version, CFBundleVersion=version)
        with (contents / "Info.plist").open("wb") as f:
            plistlib.dump(info, f)
        # Ad-hoc signing makes the arm64 bundle structurally valid. Developer ID
        # signing/notarization needs the maintainer's credentials and is separate.
        run("codesign", "--force", "--deep", "--sign", "-", app)
        run("codesign", "--verify", "--deep", "--strict", app)
        (stage / "Applications").symlink_to("/Applications", target_is_directory=True)
        shutil.copy2(ROOT / "assets/icons/xiterm.icns", stage / ".VolumeIcon.icns")
        setfile = shutil.which("SetFile")
        if setfile:
            run(setfile, "-a", "C", stage)
        image = DIST / f"xiterm-{version}-macos-{arch}.dmg"
        run("hdiutil", "create", "-volname", "XiTerm", "-srcfolder", stage,
            "-ov", "-format", "UDZO", "-fs", "HFS+", image)
        run("hdiutil", "verify", image)
    return image


def windows_package(version, arch):
    if arch != "amd64":
        raise ValueError("The Windows installer currently supports amd64")
    with tempfile.TemporaryDirectory(prefix="xiterm-package-") as temp:
        stage = Path(temp)
        build_binary(version, stage / "xiterm.exe", windows=True)
        copy_notices(stage / "Notices")
        # Generate stable component identifiers for the license payload.
        namespace = "http://wixtoolset.org/schemas/v4/wxs"
        ET.register_namespace("", namespace)
        document = ET.Element(f"{{{namespace}}}Wix")
        fragment = ET.SubElement(document, f"{{{namespace}}}Fragment")
        group = ET.SubElement(fragment, f"{{{namespace}}}ComponentGroup", Id="Notices", Directory="NoticesDirectory")
        for index, file in enumerate(sorted((stage / "Notices").iterdir())):
            component = ET.SubElement(group, f"{{{namespace}}}Component", Id=f"Notice{index}", Guid="*")
            ET.SubElement(component, f"{{{namespace}}}File", Id=f"NoticeFile{index}", Source=str(file), KeyPath="yes")
        notices = stage / "Notices.wxs"
        ET.ElementTree(document).write(notices, encoding="utf-8", xml_declaration=True)
        installer = DIST / f"xiterm-{version}-windows-{arch}.msi"
        run("wix", "build", "-arch", "x64", "-d", f"Version={version}",
            "-d", f"SourceDir={stage}", "-d", f"IconPath={ROOT / 'assets/icons/xiterm.ico'}",
            "-intermediateFolder", stage / "wix-obj", "-o", installer,
            ROOT / "packaging/windows/Package.wxs", notices)
    return installer


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--tag", required=True)
    parser.add_argument("--metadata", action="store_true", help="validate tag and print release metadata without building")
    parser.add_argument("--github-output", type=Path)
    args = parser.parse_args()
    version = release_version(args.tag)
    if args.metadata:
        output = f"version={version}\ntag=v{version}\n"
        print(output, end="")
        if args.github_output:
            with args.github_output.open("a", encoding="utf-8") as f:
                f.write(output)
        return
    arch = {"x86_64": "amd64", "AMD64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(platform.machine())
    if arch is None:
        raise ValueError(f"Unsupported architecture: {platform.machine()}")
    DIST.mkdir(exist_ok=True)
    run("go", "mod", "download")
    host = platform.system()
    if host == "Linux":
        result = linux_package(version, arch)
    elif host == "Darwin":
        result = macos_package(version, arch)
    elif host == "Windows":
        result = windows_package(version, arch)
    else:
        raise ValueError(f"Unsupported platform: {host}")
    digest = hashlib.sha256(result.read_bytes()).hexdigest()
    result.with_name(result.name + ".sha256").write_text(f"{digest}  {result.name}\n", encoding="utf-8", newline="\n")
    print(f"Created {result}")


if __name__ == "__main__":
    main()
