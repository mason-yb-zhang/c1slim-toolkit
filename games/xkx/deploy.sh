#!/bin/sh
# 北大侠客行 MUD 部署：设备自带 busybox telnet，经 c1term 直连。
# 已知问题：服务器为 GBK 编码，需配合 GBK→UTF-8 转换器（本仓库 TODO）才能正常显示中文。
set -eu
APPS=/storage/c1/local-apps/apps
MANIFESTS=/storage/c1/local-apps/manifests

cat > play.sh <<'EOF'
#!/bin/sh
exec /usr/bin/telnet mud.pkuxkx.net 8080
EOF

adb shell "mkdir -p $APPS/xkx/bin"
adb push play.sh "$APPS/xkx/bin/play.sh"

cat > 07-xkx.json <<'EOF'
{
  "id": "xkx",
  "name": "侠客行 MUD",
  "executable": "/storage/c1/local-apps/bin/xkx-run",
  "sha256": "<c1apprun 二进制 sha256>"
}
EOF
adb push 07-xkx.json "$MANIFESTS/07-xkx.json"
echo "完成。英文指令可用；中文显示等 GBK 转换器。"
