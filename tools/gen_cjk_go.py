#!/usr/bin/env python3
"""Generate cjkfont.go for C1Terminal: 12x8 CJK bitmaps (double-width cells).

Source: book-reader pkg-font.bin (C1BF: [LE u32 cp][w u8][16 BE row-words]).
Downsample 16x16 -> 12x8: width nearest (x*4//3), height OR-merge of row pairs
(stroke-preserving). Packed 2 bytes/row MSB-first, 8 rows = 16 bytes/char.
Coverage: all GB2312 chars (same set as the LVM synthesis).
"""
import struct

SRC = 'D:/dev/c1-slim/repos/C1auncher/App/book-reader/assets/pkg-font.bin'
OUT = 'D:/dev/c1-slim/repos/C1-Slim-Ports/C1Terminal/internal/terminalui/cjkfont.go'

data = open(SRC, 'rb').read()
glyphs = {}
for i in range(4, len(data) - 36, 37):
    cp = struct.unpack('<I', data[i:i + 4])[0]
    w = data[i + 4]
    rows = [struct.unpack('>H', data[i + 5 + r * 2:i + 7 + r * 2])[0] for r in range(16)]
    glyphs[cp] = (w, rows)

FALLBACK = glyphs.get(0xFFFD)


def grid(cp):
    w, rows = glyphs.get(cp, FALLBACK)
    g = [[bool(rows[y] & (0x8000 >> x)) for x in range(w)] for y in range(16)]
    if w < 16:
        g = [r + [False] * (16 - w) for r in g]
    return g


def downsample(g):
    # 16x16 -> 12x8
    out = [[False] * 12 for _ in range(8)]
    for y in range(8):
        for x in range(12):
            out[y][x] = g[y * 2][x * 4 // 3] or g[y * 2 + 1][x * 4 // 3]
    return out


def pack(g):
    b = bytearray()
    for y in range(8):
        word = 0
        for x in range(12):
            if g[y][x]:
                word |= 1 << (15 - x)
        b += struct.pack('>H', word)
    return bytes(b)


entries = []
for c1 in range(0xA1, 0xF8):
    for c2 in range(0xA1, 0xFF):
        try:
            cp = ord(bytes([c1, c2]).decode('gb2312'))
        except UnicodeDecodeError:
            continue
        if cp not in glyphs:
            continue
        entries.append((cp, pack(downsample(grid(cp)))))
entries.sort()
# dedupe (overlap zones may repeat cps)
seen = set()
uniq = []
for cp, bmp in entries:
    if cp not in seen:
        seen.add(cp)
        uniq.append((cp, bmp))

with open(OUT, 'w', encoding='utf-8') as f:
    f.write('package terminalui\n\n')
    f.write('// Generated 12x8 CJK bitmaps (double-width) from GNU Unifont.\n')
    f.write('// Each glyph: 8 rows x 2 bytes, MSB-first, 12 pixels wide.\n')
    f.write('type cjkEntry struct {\n\tcp  rune\n\tbmp [16]byte\n}\n\n')
    f.write('var cjkTable = []cjkEntry{\n')
    for cp, bmp in uniq:
        f.write('\t{0x%x, [16]byte{%s}},\n' % (cp, ', '.join('0x%02x' % b for b in bmp)))
    f.write('}\n\n')
    f.write('func cjkBitmap(r rune) ([16]byte, bool) {\n')
    f.write('\tlo, hi := 0, len(cjkTable)-1\n')
    f.write('\tfor lo <= hi {\n')
    f.write('\t\tmid := (lo + hi) / 2\n')
    f.write('\t\tif cjkTable[mid].cp < r {\n\t\t\tlo = mid + 1\n')
    f.write('\t\t} else if cjkTable[mid].cp > r {\n\t\t\thi = mid - 1\n')
    f.write('\t\t} else {\n\t\t\treturn cjkTable[mid].bmp, true\n\t\t}\n\t}\n')
    f.write('\treturn [16]byte{}, false\n}\n')
print('chars:', len(uniq), '->', OUT)
