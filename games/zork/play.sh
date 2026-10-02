#!/bin/sh
set -eu
root=/storage/c1/local-apps/apps/zork
umask 077
mkdir -p "$root/data"
cd "$root/data"
exec "$root/bin/dfrotz" -w 37 -h 8 "$root/share/zork1.z3"
