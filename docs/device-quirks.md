# 设备怪癖记录（实测）

## 1. zig cc + musl 静态程序的 pipe() 返回值
- 现象：`pipe(fds)` 填充 fds 正确，但**返回读端 fd 号（如 3）而非 0**。
- 原因：MIPS o32 内核 pipe 系统调用把读 fd 放在 $v0 返回，musl 未按"0=成功"抹平。
- 对策：判断 `fds[0] >= 0`，不要判断返回值。fork/wait 正常。
- 影响面：所有用 zig cc `-target mipsel-linux-musleabihf` 编译的 C 程序。

## 2. 本地应用（local-apps）启动器的隐性契约
从 local-launcher 二进制字符串 + 实测还原：
- manifest JSON 字段：`id` / `name` / `executable` / `sha256` / 可选 `environment`。
- `executable` 必须是**真 MIPS ELF 二进制**，检查小端、o32 和实际启动器支持的 ISA。不能仅凭 CPU 是 r2 就要求所有应用伪装成 r2。**shell 脚本会被拒**（"需要MIPS小端o32 ELF程序"）。
- 可执行文件必须在可信存储内（`/storage/c1/` 前缀），拒绝符号链接，
  要求可执行位、不可被组/其他用户写、sha256 与清单一致（"程序SHA256不匹配，已拒绝启动"）。
- 启动器**自己持有 C1ancher 外部应用租约**（flock `/dev/shm/c1ancher-external-app.runlock`
  + mode 发布），桌面随之让出屏幕与输入；应用**不要自己抢租约**（互斥会直接失败）。
- 启动器把租约 fd 以 ExtraFiles 传给子进程；子进程退出（或被杀）即归还。
- UI 按键：↑↓选择、OK启动、R刷新、返回退出；日志落 `/storage/c1/local-apps/logs/<id>.log`。

## 3. Go 交叉编译的 ELF 头是 MIPS32r1
- `GOARCH=mipsle` 产物 e_flags 的 ISA 域是 MIPS32r1（0x5），而设备 CPU 是 XBurst r2。
- 必须保留编译器生成的 ELF 标记，修改 `e_flags` 不会改变实际机器指令或 ABI。
- 使用 `GOOS=linux GOARCH=mipsle GOMIPS=hardfloat CGO_ENABLED=0` 构建，再核对真实 ELF、启动器校验和设备运行结果。若启动器拒绝兼容产物，应核对其校验规则，不修改二进制头来绕过。

## 4. 墨水屏帧格式
- 296×152，1bit，按 8 像素横条打包：`byte[(y/8)*296 + x]` 的第 `0x80>>(y&7)` 位，黑=1。
- 整帧 5624 字节，写 `/dev/epaper_lcd`；全刷需另写
  `/sys/devices/platform/e0266a128/epaper/refresh`。
- `tools/frame2png.py` 可在电脑上渲染帧文件。

## 5. 音频
- 放音：`aplay -D hw:0,0 -f S16_LE -r <rate> -c <ch> -t raw`（Pinao 同款参数加
  `-B 20000 -R 0 -T 500000`）。
- 音量：本设备 `amixer sget DAC` 实测范围为 0..190，每级约 0.5 dB；把应用百分比直接线性映射到该范围会使中低档非常小声。应用应保存进入时的 DAC 值，退出后恢复。
- 2026-10-02 实测 `/etc/ssl/certs` 为空，curl 请求中国之声 HTTPS 流报错误 60。使用 curl 官方 Mozilla CA 包并指定 `--cacert` 后返回 HTTP 200 和音频数据；不要使用 `-k` 关闭验证。
- 设备原生 FFmpeg 的 ALSA `default` 输出已通过静音探测，PCM 为 RUNNING，播放期间 Speaker Enable 自动打开。空闲时 Speaker Enable 为 off 本身不是故障证据。

## 6. 电子纸/全刷参数
- `/sys/devices/platform/e0266a128/epaper/fast_refresh_only`、`refresh_max` 可临时调节，
  应用退出前恢复（见 radio/LavaX 的 wrapper 写法）。
