"""Compare independent software-Vulkan processes; no editor, GC or forced trim."""

import argparse
import csv
import os
from pathlib import Path
import statistics
import subprocess
import sys
import tempfile
import time


def snapshot(pid):
    fields = {}
    for line in Path(f"/proc/{pid}/smaps_rollup").read_text().splitlines():
        key, _, value = line.partition(":")
        if key in ("Rss", "Pss", "Private_Dirty", "Anonymous"):
            fields[key] = int(value.split()[0])
    return fields


def measure(binary, draws, environment, render):
    process = subprocess.Popen(
        [str(binary), str(draws)], cwd=render, env=environment,
        stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True,
    )
    rows = []
    try:
        for line in process.stdout:
            kind, stage, buffers, used, free, mapped, trims, trim_ns = line.split()
            if kind != "sample":
                raise RuntimeError(f"unexpected probe message: {line!r}")
            samples = []
            for _ in range(5):
                samples.append(snapshot(process.pid))
                time.sleep(0.01)
            row = {key: int(statistics.median(s[key] for s in samples)) for key in samples[0]}
            row.update(stage=stage, buffers=int(buffers), libc_used=int(used), libc_free=int(free),
                       libc_mapped=int(mapped), trims=int(trims), trim_ns=int(trim_ns))
            rows.append(row)
            process.stdin.write("\n")
            process.stdin.flush()
        if process.wait(timeout=30):
            raise RuntimeError(f"probe exited with {process.returncode}")
        if [row["stage"] for row in rows] != ["startup", "warm", "peak", "low", "steady"]:
            raise RuntimeError("incomplete probe samples")
        return rows
    finally:
        process.stdin.close()
        process.stdout.close()
        if process.poll() is None:
            process.kill()
            process.wait()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--baseline-render", type=Path, help="snapshot of the render directory before optimization")
    parser.add_argument("--repeats", type=int, default=5)
    parser.add_argument("--draws", type=int, default=32768)
    parser.add_argument("--icd", default="/usr/share/vulkan/icd.d/lvp_icd.json")
    arguments = parser.parse_args()
    if arguments.repeats < 1:
        parser.error("--repeats must be positive")
    render = Path(__file__).resolve().parents[1]
    sources = {"after": render}
    if arguments.baseline_render:
        sources = {"before": arguments.baseline_render.resolve(), **sources}
    columns = ["variant", "run", "stage", "Rss", "Pss", "Private_Dirty", "Anonymous",
               "buffers", "libc_used", "libc_free", "libc_mapped", "trims", "trim_ns"]
    writer = csv.DictWriter(sys.stdout, columns)
    writer.writeheader()
    rows = []
    with tempfile.TemporaryDirectory(prefix="onix-vulkan-rss-") as temporary:
        binaries = {}
        for name, source in sources.items():
            binary = Path(temporary) / name
            subprocess.run(["cc", "-std=c11", "-O2", "-I" + str(source),
                            str(render / "tests/vulkan_rss_probe.c"), "-l:libvulkan.so.1", "-lX11", "-lm", "-o", str(binary)], check=True)
            binaries[name] = binary
        server = subprocess.Popen(["Xvfb", "-displayfd", "1", "-screen", "0", "1280x720x24", "-nolisten", "tcp"],
                                  stdout=subprocess.PIPE, text=True)
        try:
            display = server.stdout.readline().strip()
            if not display.isdecimal():
                raise RuntimeError("Xvfb did not provide a display")
            environment = dict(os.environ, DISPLAY=":" + display, LP_NUM_THREADS="2",
                               VK_DRIVER_FILES=arguments.icd, MESA_SHADER_CACHE_DISABLE="true")
            for run in range(1, arguments.repeats + 1):
                order = list(binaries) if run % 2 else list(reversed(binaries))
                for name in order:
                    for row in measure(binaries[name], arguments.draws, environment, render):
                        row.update(variant=name, run=run)
                        writer.writerow(row)
                        rows.append(row)
                    sys.stdout.flush()
        finally:
            server.terminate()
            server.wait(timeout=10)
            server.stdout.close()
    for name in binaries:
        for stage in ("warm", "peak", "low", "steady"):
            selected = [row for row in rows if row["variant"] == name and row["stage"] == stage]
            rss = statistics.median(row["Rss"] for row in selected) / 1024
            private = statistics.median(row["Private_Dirty"] for row in selected) / 1024
            print(f"{name} {stage}: median RSS={rss:.3f} MiB, private dirty={private:.3f} MiB", file=sys.stderr)


if __name__ == "__main__":
    main()
