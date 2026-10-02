#!/bin/sh
# Zork I 部署：游戏文件（Infocom 免费软件）+ 启动脚本 + 清单。
# 前置：先跑 build-dfrotz.sh 得到 dfrotz。
set -eu
APPS=/storage/c1/local-apps/apps
MANIFESTS=/storage/c1/local-apps/manifests

echo "拉取 Zork I (release 88 / serial 840726, eblong Infocom 档案)..."
mkdir -p zork-pkg/share
curl -fSL -o zork-pkg/share/zork1.z3 "https://eblong.com/infocom/gamefiles/zork1-r88-s840726.z3"
cp dfrotz zork-pkg/bin-dfrotz 2>/dev/null || { echo "先运行 build-dfrotz.sh"; exit 1; }

adb shell "mkdir -p $APPS/zork/bin $APPS/zork/share"
adb push zork-pkg/bin-dfrotz "$APPS/zork/bin/dfrotz"
adb push zork-pkg/share/zork1.z3 "$APPS/zork/share/zork1.z3"

cat > play.sh <<'EOF'
#!/bin/sh
exec /storage/c1/local-apps/apps/zork/bin/dfrotz /storage/c1/local-apps/apps/zork/share/zork1.z3
EOF
adb push play.sh "$APPS/zork/bin/play.sh"

# 清单：executable 为打过 r2 补丁的 c1term；按 docs/device-quirks.md 第 3 条补 ELF 头
cat > 06-zork.json <<'EOF'
{
  "id": "zork",
  "name": "Zork 地下城",
  "executable": "/storage/c1/local-apps/bin/zork-run",
  "sha256": "<c1apprun 二进制 sha256>"
}
EOF
adb push 06-zork.json "$MANIFESTS/06-zork.json"
echo "完成。设备上 R 刷新后启动「Zork 地下城」。HOME 退出。"
