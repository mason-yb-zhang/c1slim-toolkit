#!/bin/sh
# Universal Paperclips（Go 移植版）部署示例；先完成备份与清单哈希核验。
# 上游：https://github.com/xieguaiwu/universal-paperclips （Bubble Tea TUI，三阶段完整）
set -eu
APPS=/storage/c1/local-apps/apps
MANIFESTS=/storage/c1/local-apps/manifests

[ -d universal-paperclips ] || git clone --depth 1 https://github.com/xieguaiwu/universal-paperclips.git
cd universal-paperclips
GOOS=linux GOARCH=mipsle GOMIPS=hardfloat CGO_ENABLED=0 \
    go build -mod=vendor -trimpath -ldflags "-s -w" -o paperclips .
cd ..

adb shell "mkdir -p $APPS/paperclips/bin /storage/c1games/upc/data"
adb push universal-paperclips/paperclips "/storage/c1games/upc/paperclips"

cat > play.sh <<'EOF'
#!/bin/sh
export HOME=/storage/c1games/upc
export XDG_DATA_HOME=/storage/c1games/upc/data
export C1TERM_EXEC=/storage/c1games/upc/paperclips
exec /storage/c1lavax/c1term
EOF
adb push play.sh "$APPS/paperclips/bin/play.sh"

cat > 10-paperclips.json <<'EOF'
{
  "id": "paperclips",
  "name": "回形针宇宙",
  "executable": "/storage/c1/local-apps/bin/paperclips-run",
  "sha256": "<c1apprun 二进制 sha256>"
}
EOF
adb push 10-paperclips.json "$MANIFESTS/10-paperclips.json"
echo "完成。a=弯针 w=买丝 u/d=调价 q=保存退出。"
