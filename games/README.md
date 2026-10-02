# 游戏部署集

每个子目录是一套"拉取/构建脚本 + 启动脚本 + 清单模板"，配合仓库根 README 与
`docs/device-quirks.md`（本地应用契约）使用。二进制游戏文件不入库，
由脚本从各自的公开发布源获取。

通用注册流程（设备已启用 root ADB、已装 C1ancher 与 local-apps 启动器）：

1. 按子目录 README 准备文件到 `/storage/c1/local-apps/apps/<id>/`
2. 清单放到 `/storage/c1/local-apps/manifests/<n>-<id>.json`（字段契约见 quirks 文档第 2 条）
3. 设备本地应用列表里 **R 刷新** → OK 启动

`zork/` 里的 `build-dfrotz.sh` 还会产出 `dfrotz`，`jigsaw/` 直接复用它。

| 目录 | 游戏 | 运行方式 |
| --- | --- | --- |
| `zork/` | Zork I（互动小说鼻祖，Infocom 免费发行） | dfrotz @ c1term |
| `jigsaw/` | Jigsaw（Graham Nelson 时间旅行长篇） | dfrotz @ c1term |
| `xkx/` | 北大侠客行 MUD（mud.pkuxkx.net:8080） | telnet @ c1term（GBK 转换待补） |
| `lavax/` | 魔法纪元 / 三国争霸（文曲星 LavaX） | c1lavax 直绘 |
| `paperclips/` | Universal Paperclips（Go 移植版） | Bubble Tea @ c1term |

许可说明：Zork/Jigsaw 为免费软件（IF Archive/Infocom 官方发布渠道）；
LavaX 游戏文件来自 gitee.com/aliencoder/lavaxos，随其上游许可，不随本仓库再分发；
Universal Paperclips 移植版为 github.com/xieguaiwu/universal-paperclips，随其上游许可。
