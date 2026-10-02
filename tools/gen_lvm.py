#!/usr/bin/env python3
"""Synthesize LavaX LVM.bin from book-reader's GNU Unifont bitmap font (C1BF format).

LVM.bin layout (see lavax_vm/lava_disp.cpp):
  [0]      ASCII 6x12   128 slots x 12B  (index = raw byte)
  [1536]   ASCII 8x16   128 slots x 16B
  [3584]   GB2312 12x12 81*94 slots x 24B (VM slot formula, symbols then hanzi)
  [186320] GB2312 16x16 81*94 slots x 32B
  [+0x3e]  screen keyboard bitmap 6400B (zeros; unused by games)
  [+6400]  pinyin table (zeros)
"""
import struct
import sys

SRC = 'D:/dev/c1-slim/repos/C1auncher/App/book-reader/assets/pkg-font.bin'
OUT = 'D:/dev/c1-slim/text-rpg/LVM.bin'

data = open(SRC, 'rb').read()
assert data[:4] == b'C1BF', 'bad magic'
glyphs = {}
for i in range(4, len(data) - 36, 37):
    cp = struct.unpack('<I', data[i:i + 4])[0]
    w = data[i + 4]
    rows = [struct.unpack('>H', data[i + 5 + r * 2:i + 7 + r * 2])[0] for r in range(16)]
    glyphs[cp] = (w, rows)

FALLBACK = glyphs.get(0xFFFD)
assert FALLBACK, 'no fallback glyph'


def grid_of(cp):
    w, rows = glyphs.get(cp, FALLBACK)
    g = [[bool(rows[y] & (0x8000 >> x)) for x in range(w)] for y in range(16)]
    if w < 16:
        g = [row + [False] * (16 - w) for row in g]
    return g


def pack(grid, w, h):
    out = bytearray()
    for y in range(h):
        acc = 0
        n = 0
        for x in range(w):
            if grid[y][x]:
                acc |= 1 << (7 - n)
            n += 1
            if n == 8:
                out.append(acc)
                acc = 0
                n = 0
        if n:
            out.append(acc)
    return bytes(out)


def downsample(grid, w_in, h_in, w_out, h_out):
    return [[grid[min(h_in - 1, y * h_in // h_out)][min(w_in - 1, x * w_in // w_out)]
             for x in range(w_out)] for y in range(h_out)]


ascii12 = bytearray(128 * 12)
ascii16 = bytearray(128 * 16)
for b in range(128):
    g16 = grid_of(b)
    ascii16[b * 16:(b + 1) * 16] = pack(g16, 8, 16)
    ascii12[b * 12:(b + 1) * 12] = pack(downsample(g16, 8, 16, 6, 12), 6, 12)

gb12 = bytearray(81 * 94 * 24)
gb16 = bytearray(81 * 94 * 32)
covered = 0
for c1 in range(0xA1, 0xF8):
    for c2 in range(0xA1, 0xFF):
        try:
            cp = ord(bytes([c1, c2]).decode('gb2312'))
        except UnicodeDecodeError:
            continue
        if cp not in glyphs:
            continue
        slot = (c1 - 0xA1) * 94 + (c2 - 0xA1) if c1 < 0xB0 else (c1 - 0xA7) * 94 + (c2 - 0xA1)
        g = grid_of(cp)
        gb16[slot * 32:(slot + 1) * 32] = pack(g, 16, 16)
        gb12[slot * 24:(slot + 1) * 24] = pack(downsample(g, 16, 16, 12, 12), 12, 12)
        covered += 1

lvm = bytes(ascii12) + bytes(ascii16) + bytes(gb12) + bytes(gb16) + bytes(0x3e) + bytes(6400) + bytes(20000)
open(OUT, 'wb').write(lvm)
print(f'glyphs in source: {len(glyphs)}, GB2312 covered: {covered}, LVM.bin size: {len(lvm)}')

if '-v' in sys.argv:
    sys.stdout.reconfigure(encoding='utf-8', errors='replace')

    def show(label, blob, w, h, bytes_per):
        print(f'--- {label} ---')
        for y in range(h):
            row = ''
            for x in range(w):
                byte = blob[y * bytes_per + x // 8]
                row += '#' if byte & (0x80 >> (x % 8)) else '.'
            print(row)
    show('A 8x16', ascii16[ord('A') * 16:ord('A') * 16 + 16], 8, 16, 1)
    show('A 6x12', ascii12[ord('A') * 12:ord('A') * 12 + 12], 6, 12, 1)
    # 中 = GB (0xD6,0xD0)
    slot = (0xD6 - 0xA7) * 94 + (0xD0 - 0xA1)
    show('中 16x16', gb16[slot * 32:slot * 32 + 32], 16, 16, 2)
    show('中 12x12', gb12[slot * 24:slot * 24 + 24], 12, 12, 2)
