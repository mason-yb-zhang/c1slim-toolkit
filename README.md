# c1slim-toolkit

快易典 C1-Slim / MP-D261（296×152 单色墨水屏、MIPS32r2、64MB RAM、C1ancher 桌面）
的自研工具链与应用集。构建成功、画面预览和真机放音是不同验证项；各组件的验证结果需分别记录。

## 内容

| 目录 | 说明 |
| --- | --- |
| `radio/` | **网络收音机**——原生 C1ancher 应用（Go + c1device）。26 个电台（云听官方 19 套中央频率 + 7 个其他电台），方向键选台、OK 播放/停止、左右键/音量键调音量、BACK 退出。中文 UI 用点阵字库。 |
| `c1apprun/` | 本地应用通用二进制启动器——解决启动器"executable 必须是 MIPS ELF 脚本不行"的契约问题；按 `argv[0]` 找同名 `.cmd` 文件执行命令。 |
| `c1dec/` | MP3 流解码桥（minimp3）：stdin 进 MP3 字节，解码后自动拉起 `aplay` 播放。配合 `curl` 一条命令播网络电台。 |
| `tools/` | 三个 Python 工具：`gen_lvm.py`（合成文曲星 LavaX 的 LVM.bin 字体）、`gen_cjk_go.py`（生成 C1Terminal 的 CJK 字表）、`frame2png.py`（屏幕帧转 PNG，电脑上"截图"）。 |
| `patches/` | 对 C1-Slim-Ports 的补丁：C1Terminal 中文显示/帧转储/C1TERM_EXEC、vt10x 宽字符、C1LavaX HOME 键退出（含调试追踪开关说明）。 |
| `games/` | 游戏部署集：Zork / Jigsaw / 侠客行 MUD / 魔法纪元 / 三国 / 回形针宇宙——拉取与构建脚本、启动脚本、清单模板（游戏二进制由脚本从公开源获取，不入库）。 |
| `docs/` | `device-quirks.md` 设备怪癖实录（musl pipe 返回值、启动器契约、ELF r1/r2、帧格式、音频、全刷参数）。 |

## 快速开始（网络收音机）

```bash
cd radio
# 交叉编译（Go 1.26+）
GOOS=linux GOARCH=mipsle GOMIPS=hardfloat CGO_ENABLED=0 \
  go build -trimpath -ldflags "-s -w -buildid=" -o radio .
# 保留编译器生成的真实 ELF 标记，不修改 e_flags。
# 先退出正在运行的收音机，核对目标设备序列号并备份旧程序/清单。
# 将程序以临时文件推到 radio/bin，校验 SHA-256 后原子改名；不要覆盖运行中的程序。
# 同步 assets/cacert.pem 到 /storage/c1/local-apps/apps/radio/assets/cacert.pem。
# 更新 manifests/12-radio.json 中的程序 SHA-256，然后按 R 刷新本地列表。
```

收音机依赖设备原生 `/usr/bin/curl`、`/usr/bin/ffmpeg` 和 `/usr/bin/amixer`，通过独立进程管道放音，不再使用共享的 `c1dec`。中央台使用云听官方 `https://ytmsout.radio.cn/web/appBroadcast/list` 目录中的频道 ID，每次播放实时获取有效地址，不硬编码带时效参数的链接。`tools/fetch_yunting_stations.py` 可查询当前官方目录；19 套中央频率于 2026-10-02 核对。HLS 由独立 Go helper 下载并验证 HTTPS 证书链、主机名，再向 FFmpeg 传递音频字节；限制远端域名、播放清单/分片大小与等待时间，拒绝加密或不支持的 HLS 变体。官方目录当前提供部分 HTTP 媒体地址，这些音频传输本身不加密；目录请求仍使用 HTTPS。CA 包来自 curl 官方维护的 Mozilla 根证书集合，仅供本应用使用；HTTPS 保持证书验证且不允许降级重定向到 HTTP。只有 FFmpeg 报告正的音频输出时间后，界面才显示正在播放。连接/解码失败会显示错误码，详细原因写入本地启动器的 `logs/radio.log`。退出前回收两个子进程，并恢复进入应用前的 DAC 音量及电子纸刷新模式。

依赖 `c1device` 已固定到公开源码修订，`go.sum` 记录模块校验值，不再依赖另一台电脑上的目录。测试使用 `go test ./...`，静态检查使用 `go vet ./...`；目标平台再运行 `GOOS=linux GOARCH=mipsle GOMIPS=hardfloat CGO_ENABLED=0 go vet ./...`。

旧解码器单独使用（不属于当前收音机播放链）：

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
