#!/bin/sh
# Jigsaw 部署（复用 zork/ 的 dfrotz 构建）。游戏文件来自 IF Archive，免费软件。
set -eu
APPS=/storage/c1/local-apps/apps
MANIFESTS=/storage/c1/local-apps/manifests

mkdir -p jigsaw-pkg/share
curl -fSL -o jigsaw-pkg/share/jigsaw.z8 "https://ifarchive.org/if-archive/games/zcode/Jigsaw.z8"
[ -f dfrotz ] || { echo "先运行 zork/build-dfrotz.sh"; exit 1; }
cp dfrotz jigsaw-pkg/bin-dfrotz

adb shell "mkdir -p $APPS/jigsaw/bin $APPS/jigsaw/share"
adb push jigsaw-pkg/bin-dfrotz "$APPS/jigsaw/bin/dfrotz"
adb push jigsaw-pkg/share/jigsaw.z8 "$APPS/jigsaw/share/jigsaw.z8"

cat > play.sh <<'EOF'
#!/bin/sh
exec /storage/c1/local-apps/apps/jigsaw/bin/dfrotz /storage/c1/local-apps/apps/jigsaw/share/jigsaw.z8
EOF
adb push play.sh "$APPS/jigsaw/bin/play.sh"

cat > 11-jigsaw.json <<'EOF'
{
  "id": "jigsaw",
  "name": "Jigsaw 时间之谜",
  "executable": "/storage/c1/local-apps/bin/jigsaw-run",
  "sha256": "<c1apprun 二进制 sha256>"
}
EOF
adb push 11-jigsaw.json "$MANIFESTS/11-jigsaw.json"
echo "完成。"
