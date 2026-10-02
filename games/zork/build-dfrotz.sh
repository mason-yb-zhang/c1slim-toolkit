#!/bin/sh
# 交叉编译 dumb-frotz（纯 stdio Z-machine 解释器，适合终端/墨水屏）。
# 产物 dfrotz：MIPS32r2 静态，仅依赖 libc。
set -eu

ZIG=${ZIG:-zig}
TARGET=mipsel-linux-musleabihf
VERSION=2.56pre
GIT_HASH=042d7bcadc1e2a8090cf737c841d8b7fc14b6eff

[ -d frotz-src ] || {
    echo "下载 frotz 源码..."
    curl -fSL -o frotz.tar.gz "https://gitlab.com/DavidGriffith/frotz/-/archive/master/frotz-master.tar.gz"
    tar xzf frotz.tar.gz
    mv frotz-master frotz-src
}

cd frotz-src

# Makefile 自动生成这两个头，离线手动生成（内容即官方规则）
[ -f src/common/defs.h ] || printf '#ifndef COMMON_DEFINES_H\n#define COMMON_DEFINES_H\n#define VERSION "%s"\n#define RELEASE_NOTES "Development release."\n#endif\n' "$VERSION" > src/common/defs.h
[ -f src/common/hash.h ] || printf '#ifndef VERSION\n#define VERSION "%s"\n#endif\n#ifndef RELEASE_NOTES\n#define RELEASE_NOTES "Development release."\n#endif\n#define GIT_HASH "%s"\n#define GIT_HASH_SHORT "%s"\n#define GIT_DATE "2026-09-21 03:20:35 +0000"\n' "$VERSION" "$GIT_HASH" "${GIT_HASH:0:7}" > src/common/hash.h

"$ZIG" cc -target $TARGET -O2 -static \
    -Isrc/include -Isrc/common -Isrc/blorb -Isrc/dumb \
    -o ../dfrotz src/common/*.c src/blorb/*.c src/dumb/*.c -lm

cd ..
ls -la dfrotz
echo "完成。部署: adb push dfrotz /storage/c1/local-apps/apps/zork/bin/dfrotz"
