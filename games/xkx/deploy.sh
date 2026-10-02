#!/bin/sh
# 北大侠客行 MUD 部署：设备自带 busybox telnet，经 c1term 直连。
# 需使用修正版 c1term，并通过 xkx.cmd 启用 C1TERM_ENCODING=gbk。
set -eu
APPS=/storage/c1/local-apps/apps
MANIFESTS=/storage/c1/local-apps/manifests

cat > play.sh <<'EOF'
#!/bin/sh
exec /usr/bin/telnet mud.pkuxkx.net 8080
EOF

adb shell "mkdir -p $APPS/xkx/bin"
adb push play.sh "$APPS/xkx/bin/play.sh"
adb push xkx.cmd /storage/c1/local-apps/bin/xkx.cmd

cat > 07-xkx.json <<'EOF'
{
  "id": "xkx",
  "name": "侠客行 MUD",
  "executable": "/storage/c1/local-apps/bin/xkx-run",
  "sha256": "<c1apprun 二进制 sha256>"
}
EOF
adb push 07-xkx.json "$MANIFESTS/07-xkx.json"
echo "完成。需安装支持GBK转码的c1term；账号登录由用户自行操作。"
