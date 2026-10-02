import argparse
import hashlib
import json
import os
from pathlib import Path
import struct
import subprocess

parser = argparse.ArgumentParser()
parser.add_argument("--go", default="go")
args = parser.parse_args()
root = Path(__file__).resolve().parent
env = os.environ.copy()
go = args.go
if Path(go).is_absolute():
    env["GOROOT"] = str(Path(go).parent.parent)
    env["PATH"] = str(Path(go).parent) + os.pathsep + env.get("PATH", "")


def run(*command, capture=False, target_env=None):
    return subprocess.run(
        command, cwd=root, env=target_env or env, check=True,
        text=True, capture_output=capture,
    )


host = json.loads(run(go, "env", "-json", "GOHOSTOS", "GOHOSTARCH", capture=True).stdout)
env.update(GOOS=host["GOHOSTOS"], GOARCH=host["GOHOSTARCH"], CGO_ENABLED="0", GOTOOLCHAIN="local")
run(go, "test", "-count=1", "./...")
run(go, "vet", "./...")
target = dict(env, GOOS="linux", GOARCH="mipsle", GOMIPS="hardfloat", CGO_ENABLED="0")
run(go, "vet", "./...", target_env=target)
run(go, "build", "-trimpath", "-ldflags=-s -w -buildid=", "-o", "radio", ".", target_env=target)
program = (root / "radio").read_bytes()
if program[:6] != b"\x7fELF\x01\x01" or struct.unpack_from("<H", program, 18)[0] != 8:
    raise SystemExit("Expected ELF32 little-endian MIPS output")
flags = struct.unpack_from("<I", program, 36)[0]
if flags & 0xF000 != 0x1000:
    raise SystemExit(f"Expected o32 ABI, got e_flags={flags:#x}")
report = {
    "go": run(go, "version", capture=True).stdout.strip(),
    "bytes": len(program),
    "sha256": hashlib.sha256(program).hexdigest(),
    "elf_flags": hex(flags),
    "ca_sha256": hashlib.sha256((root / "assets" / "cacert.pem").read_bytes()).hexdigest(),
    "checks": ["host test", "host vet", "MIPS vet", "MIPS build", "ELF32 little-endian o32"],
}
print(json.dumps(report, indent=2))
