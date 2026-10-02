#!/usr/bin/env python3
"""Generate native 16px terminal glyphs from the bundled C1BF Unifont data."""
import argparse
from pathlib import Path
import struct

parser = argparse.ArgumentParser()
parser.add_argument("--source", type=Path, default=Path(__file__).resolve().parents[1] / "radio/assets/pkg-font.bin")
parser.add_argument("--output", type=Path, required=True)
args = parser.parse_args()
data = args.source.read_bytes()
if data[:4] != b"C1BF" or (len(data) - 4) % 37:
    raise SystemExit("Invalid C1BF font")
wanted = set(range(0x20, 0x7f)) | {0xfffd}
for lead in range(0xa1, 0xf8):
    for tail in range(0xa1, 0xff):
        try:
            wanted.add(ord(bytes([lead, tail]).decode("gb2312")))
        except UnicodeDecodeError:
            pass
entries = {}
for offset in range(4, len(data), 37):
    cp = struct.unpack_from("<I", data, offset)[0]
    width = data[offset + 4]
    if not 1 <= width <= 16:
        raise SystemExit("Invalid glyph width")
    if cp in wanted:
        entries[cp] = data[offset + 5:offset + 37]
if ord('?') not in entries:
    raise SystemExit("Missing fallback glyph")
with args.output.open("w", encoding="utf-8", newline="\n") as out:
    out.write("package terminalui\n\n")
    out.write("// Generated native 16px GNU Unifont glyphs; do not resample the strokes.\n")
    out.write("type cjkEntry struct {\n\tcp rune\n\tbmp [32]byte\n}\n\nvar cjkTable = []cjkEntry{\n")
    for cp, bitmap in sorted(entries.items()):
        out.write("\t{0x%x, [32]byte{%s}},\n" % (cp, ",".join("0x%02x" % value for value in bitmap)))
    out.write("}\n\nfunc cjkBitmap(r rune) ([32]byte, bool) {\n")
    out.write("\tlo, hi := 0, len(cjkTable)-1\n\tfor lo <= hi {\n\t\tmid := (lo+hi)/2\n")
    out.write("\t\tif cjkTable[mid].cp < r { lo = mid+1 } else if cjkTable[mid].cp > r { hi = mid-1 } else { return cjkTable[mid].bmp, true }\n\t}\n\treturn [32]byte{}, false\n}\n")
print(f"Native 16px glyphs: {len(entries)} -> {args.output}")
