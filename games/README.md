# 游戏部署集

每个子目录是一套"拉取/构建脚本 + 启动脚本 + 清单模板"，配合仓库根 README 与
`docs/device-quirks.md`（本地应用契约）使用。二进制游戏文件不入库，
由脚本从各自的公开发布源获取。

## C1 小屏修正版

先在对应上游固定提交应用 `patches/README.md` 指定的两个补丁，然后运行：

```text
python games/build_fixed.py --go <go可执行文件> --terminal-source <C1Terminal目录> --paperclips-source <Universal-Paperclips目录>
```

此命令执行主机测试、静态检查、MIPS 交叉检查与构建，并校验真实 ELF32 小端 o32 标记；输出在 `games/build/`。禁止修改 ELF 头伪装 ISA。

终端采用原生 16px 字体与 2px 行距，37列×8行。回形针低行数布局固定保留两行HUD、操作提示及滚动状态，其余正文分页，避免文字挤叠。

回形针小屏版：`a` 弯针，等待售出；`w` 买丝，`b` 买自动机，`u/d` 调价，`z` 换页，`j/k` 滚动，项目页上下选择后 `r` 研究。`S` 开关存档菜单，`q` 或 Back/Esc 保存退出。终端键盘单按 SHIFT 切换小写/大写/符号层；大写 S 需先切到 ABC 层，OK 是 Ctrl 修饰键，不是 Enter。HOME 为宿主强制退出，不保证保存，应优先在游戏内保存退出。

侠客行通过 `C1TERM_ENCODING=gbk` 转码；只转换 native telnet 已处理协议后的文本，不新增账号登录或中文输入法。Zork/Jigsaw 启动脚本固定为 37×8 并进入各自 `apps/<id>/data` 存档目录；历史存档迁移必须先核对，不能覆盖。Frotz 默认使用 `.qzl` 存档后缀。

下列旧 `deploy.sh` 仍是部署示例，不是完整安装器：其中清单哈希占位符和现有 `<id>-run` 前置条件必须先补齐，不能直接执行。更新已有设备时先确认应用退出、备份程序/脚本/存档、核对旧哈希，暂存校验后原子替换。整个 local-apps 退出重进会重新读取清单。

通用注册流程（设备已启用 root ADB、已装 C1ancher 与 local-apps 启动器）：

1. 按子目录 README 准备文件到 `/storage/c1/local-apps/apps/<id>/`
2. 清单放到 `/storage/c1/local-apps/manifests/<n>-<id>.json`（字段契约见 quirks 文档第 2 条）
3. 设备本地应用列表里 **R 刷新** → OK 启动

`zork/` 里的 `build-dfrotz.sh` 还会产出 `dfrotz`，`jigsaw/` 直接复用它。

| 目录 | 游戏 | 运行方式 |
| --- | --- | --- |
| `zork/` | Zork I（互动小说鼻祖，Infocom 免费发行） | dfrotz @ c1term |
| `jigsaw/` | Jigsaw（Graham Nelson 时间旅行长篇） | dfrotz @ c1term |
| `xkx/` | 北大侠客行 MUD（mud.pkuxkx.net:8080） | telnet @ c1term（GBK 双向转码） |
| `lavax/` | 魔法纪元 / 三国争霸（文曲星 LavaX） | c1lavax 直绘 |
| `paperclips/` | Universal Paperclips（Go 移植版） | Bubble Tea @ c1term |

许可说明：Zork/Jigsaw 为免费软件（IF Archive/Infocom 官方发布渠道）；
LavaX 游戏文件来自 gitee.com/aliencoder/lavaxos，随其上游许可，不随本仓库再分发；
Universal Paperclips 移植版为 github.com/xieguaiwu/universal-paperclips，随其上游许可。
