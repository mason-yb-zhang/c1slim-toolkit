import argparse
import hashlib
import json
import os
from pathlib import Path
import struct
import subprocess

p = argparse.ArgumentParser()
p.add_argument("--go", default="go")
p.add_argument("--terminal-source", required=True, type=Path)
p.add_argument("--paperclips-source", required=True, type=Path)
a = p.parse_args()
root = Path(__file__).resolve().parent
out = root / "build"
out.mkdir(exist_ok=True)
env = os.environ.copy()
if Path(a.go).is_absolute():
    env["GOROOT"] = str(Path(a.go).parent.parent)
    env["PATH"] = str(Path(a.go).parent) + os.pathsep + env.get("PATH", "")
env["GOTOOLCHAIN"] = "local"
info = json.loads(subprocess.check_output([a.go, "env", "-json", "GOHOSTOS", "GOHOSTARCH"], env=env, text=True))
env.update(GOOS=info["GOHOSTOS"], GOARCH=info["GOHOSTARCH"], CGO_ENABLED="0")
target = dict(env, GOOS="linux", GOARCH="mipsle", GOMIPS="hardfloat")


def go(cwd, *args, cross=False):
    subprocess.run([a.go, *args], cwd=cwd, env=target if cross else env, check=True)


reports = []
for name, source, package, options in (
    ("c1term", a.terminal_source, "./cmd/c1term", []),
    ("paperclips", a.paperclips_source, ".", ["-mod=vendor"]),
):
    go(source, "test", *options, "-count=1", "./...")
    go(source, "vet", *options, "./...")
    go(source, "vet", *options, "./...", cross=True)
    if name == "c1term":
        vendor = source / "third_party" / "vt10x-5011da428d02"
        go(vendor, "test", "./...")
        go(vendor, "vet", "./...")
    output = out / name
    go(source, "build", *options, "-trimpath", "-ldflags=-s -w -buildid=", "-o", str(output), package, cross=True)
    data = output.read_bytes()
    flags = struct.unpack_from("<I", data, 36)[0]
    if data[:6] != b"\x7fELF\x01\x01" or struct.unpack_from("<H", data, 18)[0] != 8 or flags & 0xF000 != 0x1000:
        raise SystemExit(f"Invalid MIPS o32 ELF: {name}")
    reports.append({"name": name, "bytes": len(data), "elf_flags": hex(flags), "sha256": hashlib.sha256(data).hexdigest()})
(out / "build-report.json").write_text(json.dumps(reports, indent=2), encoding="utf-8")
print(json.dumps(reports, indent=2))
