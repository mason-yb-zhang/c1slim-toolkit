# c1slim-toolkit

快易典 C1-Slim / MP-D261（296×152 单色墨水屏、MIPS32r2、64MB RAM、C1ancher 桌面）
的自研工具链与应用集。全部经过真机验证（2026-09 ~ 2026-10）。

## 内容

| 目录 | 说明 |
| --- | --- |
| `radio/` | **网络收音机**——原生 C1ancher 应用（Go + c1device）。8 个中文电台，方向键选台、OK 播放/停止、左右键/音量键调音量、BACK 退出。中文 UI 用点阵字库。 |
| `c1apprun/` | 本地应用通用二进制启动器——解决启动器"executable 必须是 MIPS ELF 脚本不行"的契约问题；按 `argv[0]` 找同名 `.cmd` 文件执行命令。 |
| `c1dec/` | MP3 流解码桥（minimp3）：stdin 进 MP3 字节，解码后自动拉起 `aplay` 播放。配合 `curl` 一条命令播网络电台。 |
| `tools/` | 三个 Python 工具：`gen_lvm.py`（合成文曲星 LavaX 的 LVM.bin 字体）、`gen_cjk_go.py`（生成 C1Terminal 的 CJK 字表）、`frame2png.py`（屏幕帧转 PNG，电脑上"截图"）。 |
| `patches/` | 对 C1-Slim-Ports 的补丁：C1Terminal 中文显示/帧转储/C1TERM_EXEC、vt10x 宽字符、C1LavaX HOME 键退出（含调试追踪开关说明）。 |
| `docs/` | `device-quirks.md` 设备怪癖实录（musl pipe 返回值、启动器契约、ELF r1/r2、帧格式、音频、全刷参数）。 |

## 快速开始（网络收音机）

```bash
cd radio
# 交叉编译（Go 1.26+）
GOOS=linux GOARCH=mipsle GOMIPS=hardfloat CGO_ENABLED=0 \
  go build -trimpath -ldflags "-s -w -buildid=" -o radio .
# e_flags 的 ISA 域改 r2（启动器校验要求；脚本见 docs/device-quirks.md 第 3 条）
# 推送到设备
adb push radio /storage/c1/local-apps/apps/radio/bin/radio
adb shell chmod 755 /storage/c1/local-apps/apps/radio/bin/radio
# 注册清单（manifest 模板见 docs/device-quirks.md 第 2 条）后设备上 R 刷新即可见
```

解码器单独使用：

```bash
zig cc -target mipsel-linux-musleabihf -O2 -static -s -o c1dec c1dec/decoder.c -lm
adb push c1dec /storage/c1lavax/c1dec
adb shell 'curl -s "http://lhttp.qingting.fm/live/270/64k.mp3" | /storage/c1lavax/c1dec'
```

## 上游与许可

- 设备框架：[fwz233-RE/C1auncher](https://github.com/fwz233-RE/C1auncher)（GPL-3.0，
  本工具链的动态链接/引用部分随其许可；`radio/` 应用为独立 Go 程序）。
- 终端与 LavaX 移植：[Kasiin/C1-Slim-Ports](https://github.com/Kasiin/C1-Slim-Ports)，补丁见 `patches/`。
- `radio/assets/pkg-font.bin`：GNU Unifont 子集，SIL OFL 1.1（许可文件随附）。
- `c1dec/minimp3.h`：[lieff/minimp3](https://github.com/lieff/minimp3)，CC0。
- 本仓库新增源码（radio、c1apprun、c1dec/decoder.c、tools）：MIT。

## 免责声明

仅适用于自有或已获授权的设备。镜像/固件/个人数据不属于本仓库范围。
