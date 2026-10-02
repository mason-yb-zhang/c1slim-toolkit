# C1-Slim 生态第三方补丁

对 [Kasiin/C1-Slim-Ports](https://github.com/Kasiin/C1-Slim-Ports) 工作副本的修改，
文字游戏使用 `c1terminal-games.patch`：在干净的 C1-Slim-Ports 提交 `23bfd4d9f5c8badf0485ca5a4624a53e134b940a` 上应用，已包含原终端 CJK/帧转储/C1TERM_EXEC 改动，以及原生 16px 字库、37×8 网格与 2px 行距、有界刷新、UTF-8 分片、宽字边界、GBK 转码和 footer 修复。不要再叠加旧补丁的 C1Terminal 部分。

`paperclips-small-screen.patch` 应用于 Universal Paperclips 提交 `676909843a51ea1c487093e11813ec959313dea4`，包含 37×8 大字低行数布局（兼容其他终端尺寸）、分页提示、保存菜单及存档安全修复。

旧 `c1-slim-ports-cjk-home-exit.patch` 保留作历史参考，其中 C1LavaX 改动不属于本次文字游戏修复，也不用于重新安装已移除的游戏。

## 内容

### C1Terminal（图形终端）
- `cmd/c1term/main_linux.go`：支持 `C1TERM_EXEC` 环境变量——指定要运行的程序替代默认 bash，
  可把终端当作任意 TUI 应用的"壳"使用。
- `internal/terminalui/frame.go`：原生 16px 字体渲染，ASCII 单格 8px、中文双格 16px，行高 18px；实际 37 列×8 行，底部另有 8px 操作条。不再把中文压缩为 8px 高。
  数据在 `cjkfont.go`（7317 字，由 `tools/gen_cjk_go.py` 从 `radio/assets/pkg-font.bin` 原样提取，勿手改；字体许可见同目录 `font-LICENSE.txt`）。
- `internal/terminalui/display_linux.go`：`C1TERM_DUMP=/path` 帧转储——每帧写 5624 字节
  屏幕原始数据到文件，配合 `tools/frame2png.py` 可在电脑上"截图"。
- `third_party/vt10x-5011da428d02/parse.go`：vt10x 宽字符支持补丁（上游每 rune 占一格，
  CJK 会互相覆盖；现宽字符写 0 占位格并前进两列）。

### C1LavaX（文曲星 LavaX 虚拟机）
- `port/src/main.cpp`：`KEY_HOME(102)` 宿主级退出——大量 LavaX 程序自身无退出功能，
  按 HOME 直接退出 VM（原来 BACK 被映射成游戏内 Esc，游戏不理会就会卡死）。
- `port/third_party/lavax_vm/lava_proc.cpp`：**调试用按键/指令追踪**（`[key]`/`[op]`/`[vm]`
  输出到 stderr）。发布构建可删除这些行（搜 `[key]`、`[op]`、`[vm]` 即得）。

## 注意
- 上游 LICENSE 不变，衍生部分随上游许可。
- 应用补丁后需自行交叉编译（见主 README 的工具链一节）。
