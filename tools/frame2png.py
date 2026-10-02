#!/usr/bin/env python3
"""Render a 5624-byte C1-Slim epaper frame (296x152, strip-packed) to PNG."""
import struct
import sys
import zlib

W, H, STRIP = 296, 152, 8


def read_frame(path):
    d = open(path, 'rb').read()
    assert len(d) == W * (H // STRIP), f'bad frame size {len(d)}'
    return d


def px(frame, x, y):
    return frame[(y // STRIP) * W + x] & (0x80 >> (y % STRIP))


def write_png(frame, out):
    rows = []
    for y in range(H):
        row = bytearray([0])  # filter type 0
        for x in range(W):
            row.append(0 if px(frame, x, y) else 255)
        rows.append(bytes(row))
    raw = b''.join(rows)

    def chunk(kind, data):
        c = struct.pack('>I', len(data)) + kind + data
        return c + struct.pack('>I', zlib.crc32(kind + data) & 0xffffffff)

    png = b'\x89PNG\r\n\x1a\n'
    png += chunk(b'IHDR', struct.pack('>IIBBBBB', W, H, 8, 0, 0, 0, 0))
    png += chunk(b'IDAT', zlib.compress(raw, 9))
    png += chunk(b'IEND', b'')
    open(out, 'wb').write(png)
    print(out, len(png), 'bytes')


if __name__ == '__main__':
    write_png(read_frame(sys.argv[1]), sys.argv[2])
