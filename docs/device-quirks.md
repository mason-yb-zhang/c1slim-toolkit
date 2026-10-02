# 设备怪癖记录（实测）

## 1. zig cc + musl 静态程序的 pipe() 返回值
- 现象：`pipe(fds)` 填充 fds 正确，但**返回读端 fd 号（如 3）而非 0**。
- 原因：MIPS o32 内核 pipe 系统调用把读 fd 放在 $v0 返回，musl 未按"0=成功"抹平。
- 对策：判断 `fds[0] >= 0`，不要判断返回值。fork/wait 正常。
- 影响面：所有用 zig cc `-target mipsel-linux-musleabihf` 编译的 C 程序。

## 2. 本地应用（local-apps）启动器的隐性契约
从 local-launcher 二进制字符串 + 实测还原：
- manifest JSON 字段：`id` / `name` / `executable` / `sha256` / 可选 `environment`。
- `executable` 必须是**真 MIPS ELF 二进制**（校验 ELF 头、MIPS 小端、o32、
  且 ISA 需 MIPS32r2——Go 默认产 r1 头，见下）；**shell 脚本会被拒**（"需要MIPS小端o32 ELF程序"）。
- 可执行文件必须在可信存储内（`/storage/c1/` 前缀），拒绝符号链接，
  要求可执行位、不可被组/其他用户写、sha256 与清单一致（"程序SHA256不匹配，已拒绝启动"）。
- 启动器**自己持有 C1ancher 外部应用租约**（flock `/dev/shm/c1ancher-external-app.runlock`
  + mode 发布），桌面随之让出屏幕与输入；应用**不要自己抢租约**（互斥会直接失败）。
- 启动器把租约 fd 以 ExtraFiles 传给子进程；子进程退出（或被杀）即归还。
- UI 按键：↑↓选择、OK启动、R刷新、返回退出；日志落 `/storage/c1/local-apps/logs/<id>.log`。

## 3. Go 交叉编译的 ELF 头是 MIPS32r1
- `GOARCH=mipsle` 产物 e_flags 的 ISA 域是 MIPS32r1（0x5），而设备 CPU 是 XBurst r2。
- 代码能跑（r1 指令集是 r2 子集），但过启动器校验需 r2：把 e_flags 的 ISA 域改为 0x7。
- 本仓库 `tools/` 中各构建脚本均含此补丁步骤（读 ELF 头偏移 36 的 u32，改高 nibble）。

## 4. 墨水屏帧格式
- 296×152，1bit，按 8 像素横条打包：`byte[(y/8)*296 + x]` 的第 `0x80>>(y&7)` 位，黑=1。
- 整帧 5624 字节，写 `/dev/epaper_lcd`；全刷需另写
  `/sys/devices/platform/e0266a128/epaper/refresh`。
- `tools/frame2png.py` 可在电脑上渲染帧文件。

## 5. 音频
- 放音：`aplay -D hw:0,0 -f S16_LE -r <rate> -c <ch> -t raw`（Pinao 同款参数加
  `-B 20000 -R 0 -T 500000`）。
- 音量：`amixer sset DAC 0..190`（默认 191≈100%，调试建议调低）。
- 设备自带 curl 8.4（OpenSSL）可直连 HTTPS 流。

## 6. 电子纸/全刷参数
- `/sys/devices/platform/e0266a128/epaper/fast_refresh_only`、`refresh_max` 可临时调节，
  应用退出前恢复（见 radio/LavaX 的 wrapper 写法）。
