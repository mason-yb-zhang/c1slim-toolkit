# 文曲星 LavaX 游戏（魔法纪元 / 三国争霸）

## 运行时
c1lavax 的构建与补丁见仓库 `patches/`（HOME 键退出补丁必需——LavaX 游戏大多无自带退出）。
构建（需要 zig）：

```bash
git clone --depth 1 https://github.com/Kasiin/C1-Slim-Ports.git
cd C1-Slim-Ports
git apply ../../patches/c1-slim-ports-cjk-home-exit.patch   # 只需 C1LavaX 部分
zig c++ -target mipsel-linux-musleabi -mcpu=mips32r2 -std=c++17 -Oz -static -s \
    -I C1LavaX/port/third_party/lavax_vm \
    -o c1lavax C1LavaX/port/src/main.cpp \
    C1LavaX/port/third_party/lavax_vm/{lava,lava_disp,lava_proc,lava_ram}.cpp
adb push c1lavax /storage/c1lavax/c1lavax
```

## 字体（必需，否则无文字）
`tools/gen_lvm.py` 从 C1ancher book-reader 的 pkg-font.bin 合成 `LVM.bin`，推到
`/storage/c1lavax/LVM.bin`。

## 游戏文件（不随本仓库分发）
来自 <https://gitee.com/aliencoder/lavaxos>：

```bash
git clone --depth 1 https://gitee.com/aliencoder/lavaxos.git
# 魔法纪元：lavaxos/LavaXOS/PROGRAM/魔法纪元.app/{魔法纪元.lav, LavaData/*}
# 三国：    lavaxos/LavaXOS/PROGRAM/Lee三国.app/{Lee三国.lav, LavaData/*}
```

## 部署结构（以魔法纪元为例）
```
/storage/c1/local-apps/apps/mofa/
  bin/play.sh        # 显示参数调节 + exec c1lavax（见下方模板）
  os/mofa.lav        # 程序文件（ASCII 文件名，避免脚本引用问题）
  os/LavaData/*.dat  # 数据文件
/storage/c1/local-apps/manifests/08-mofa.json
```

play.sh 模板：

```sh
#!/bin/sh
install_dir=/storage/c1/local-apps/apps/mofa
refresh_max=/sys/devices/platform/e0266a128/epaper/refresh_max
fast_only=/sys/devices/platform/e0266a128/epaper/fast_refresh_only
saved_rm=; saved_fo=
cleanup() { trap - 0 1 2 15
  [ -n "$saved_rm" ] && printf '%s' "$saved_rm" > "$refresh_max" 2>/dev/null
  [ -n "$saved_fo" ] && printf '%s' "$saved_fo" > "$fast_only" 2>/dev/null
  exit 0; }
trap cleanup 0 1 2 15
[ -r "$fast_only" ] && { IFS= read -r saved_fo < "$fast_only"; printf 1 > "$fast_only" 2>/dev/null; }
[ -r "$refresh_max" ] && { IFS= read -r saved_rm < "$refresh_max"; printf 100 > "$refresh_max" 2>/dev/null; }
C1LAVAX_ROOT="$install_dir/os" C1LAVAX_STABLE_FRAMES=1 \
    /storage/c1lavax/c1lavax /mofa.lav >> "$install_dir/run.log" 2>&1
```

清单模板：`{"id":"mofa","name":"魔法纪元","executable":"/storage/c1/local-apps/bin/mofa-run",
"sha256":"<c1apprun 二进制 sha256>"}`，配套 `mofa.cmd`：
`sh /storage/c1/local-apps/apps/mofa/bin/play.sh`。

## 已知问题
- 魔法纪元在 C1LavaX 0.8.1 上**按键无响应**（指令追踪显示游戏只查询过一次 checkKey(128)
  后不再取输入；根因待查，可能该程序期望不同的输入模型）。三国未单独验证。
- HOME 键退出依赖补丁中的宿主级拦截。
