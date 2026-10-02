# C1-Slim 生态第三方补丁

对 [Kasiin/C1-Slim-Ports](https://github.com/Kasiin/C1-Slim-Ports) 工作副本的修改，
单个补丁文件：`c1-slim-ports-cjk-home-exit.patch`（`git apply` 于该仓库根目录）。

## 内容

### C1Terminal（图形终端）
- `cmd/c1term/main_linux.go`：支持 `C1TERM_EXEC` 环境变量——指定要运行的程序替代默认 bash，
  可把终端当作任意 TUI 应用的"壳"使用。
- `internal/terminalui/frame.go`：CJK 双宽字符渲染（12×8 点阵跨两格），
  数据在新增的 `cjkfont.go`（7221 字，由 `tools/gen_cjk_go.py` 生成，勿手改）。
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
