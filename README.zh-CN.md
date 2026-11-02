# OpenOrion2

[English](README.md) · [Русский](README.ru.md) · **中文** · [Deutsch](README.de.md) · [Français](README.fr.md)

![OpenOrion2](https://github.com/openorion2/openorion2/releases/download/v0.9.0/openorion2-play.gif)

OpenOrion2 是 **Master of Orion II: Battle at Antares**（《银河霸主 2》，1996
年）的开源重新实现：游戏以源代码的形式存在，任何人都可以阅读、修复、修改并让它
延续下去。它无需 DOSBox 即可在现代的 macOS、Windows 和 Linux 上运行。它的玩法
与原版完全一致：游戏逻辑是从原版可执行文件中逐个例程重建的，并与原版逐一核对，
连随机数的抽取和 AI 的决策都相同。原版的存档可以载入并继续游玩。

在原版之上，它还增加了：

- macOS、Windows 和 Linux 的原生版本，支持 x86-64 和 ARM（包括 Apple
  Silicon）：单个文件，无需 DOSBox，无需模拟器，无需安装任何东西；
- 基于 TCP 的联网对战（取代 IPX），可在局域网或互联网上进行；
- **[Galactic Council](https://openorion2.org)** 在线大厅：聊天、私信、游戏
  列表，游戏数据由服务器中转，因此无需端口转发；
- 全屏（最大化窗口或按 Alt+Enter）以及任意大小的窗口；
- 浏览器版本（WebAssembly）。

哪些已移植、哪些尚未移植：[docs/STATUS.md](docs/STATUS.md)（英文）。

> OpenOrion2 是一个爱好者项目，与 Master of Orion 的所有者无关，也未获其认可。
> Master of Orion 是其各自所有者的商标。OpenOrion2 不包含原版游戏的任何文件：
> 你需要自己拥有一份 Master of Orion II。

## 为什么用 Go？

Go 是项目作者的主要编程语言，而承担大量工作的 AI 代理用哪种语言写代码都一样
好。重点不在语言，而在于游戏是开放的。

## 安装

OpenOrion2 是程序本身；游戏数据来自你自己的 Master of Orion II。它针对
**1.40b23 版本，即 [GOG](https://www.gog.com/en/game/master_of_orion_1_2)
发行版** 制作和测试。其他版本（1996 年的 CD 版、1.31、其他商店）未经测试。

1. **获取游戏。** 在 GOG 上购买 Master of Orion II（以
   [Master of Orion 1+2](https://www.gog.com/en/game/master_of_orion_1_2)
   的形式出售）并安装。
2. **获取 OpenOrion2。** 从
   [Releases](https://github.com/openorion2/openorion2/releases) 下载适合你
   系统的二进制文件（`openorion2-<系统>-<处理器>`：`darwin` 即 macOS；`arm64`
   是 Apple Silicon 和 ARM 电脑，`amd64` 是其余的），或者自行构建（见下文）。
3. **把它放进游戏目录**，也就是存放 `.LBX` 文件的目录（在 Windows 上即 GOG
   的安装目录），然后在那里启动：它会在自己旁边找到游戏文件。在 macOS 或
   Linux 上，可以从 Windows 安装中复制该目录，或用
   [innoextract](https://constexpr.org/innoextract/) 解包 GOG 的 Windows
   安装程序。
4. **首次启动。** 这些二进制文件没有经过 Apple 或 Microsoft 签名。在 macOS
   上，执行 `chmod +x openorion2-darwin-*` 后，用 `xattr -d
   com.apple.quarantine openorion2-darwin-*` 移除下载隔离（或者在 macOS
   拒绝一次之后，前往“系统设置”、“隐私与安全性”，点击“仍要打开”）。在
   Windows 上，在 SmartScreen 中点击“更多信息”、“仍要运行”。

OpenOrion2 也会在当前目录或它旁边名为 `game` 的目录中寻找游戏文件；如果在其他
位置，请指明路径：

```
openorion2 -game "/path/to/Master of Orion 2"
```

OpenOrion2 会像原版一样，把存档（`SAVE1.GAM` … `SAVE10.GAM`）、设置和名人堂
写入游戏目录，并读取那里的原版存档。如果想让原版存档保持独立，请使用目录的副本。

发行版在启动时会检查是否有新版本并提示安装（`-noupdate` 可关闭此功能）。

## 构建

需要 Go 1.27 或更高版本。SDL3 已嵌入二进制文件：无需安装其他任何东西。

```
go build ./cmd/openorion2
./openorion2 -game /path/to/game
```

`go run ./cmd/openorion2 -h` 列出所有选项。

自行构建的版本从不检查更新。如何发布：[docs/release.md](docs/release.md)。

## 与他人一起玩

- **热座（同一台电脑）**：Multiplayer、Hotseat。
- **网络**：Multiplayer、Network。Start New Game 监听 TCP 端口 52525
  （`-host :端口` 可指定其他端口）；Join Game 会在局域网中寻找游戏，或用
  `-join 主机[:端口]` 直接加入。参见 [docs/net/ui.md](docs/net/ui.md)。
- **Galactic Council Online Lobby**：在
  [openorion2.org](https://openorion2.org) 注册，然后选择 Multiplayer、
  Galactic Council Online Lobby。在那里创建或加入游戏；大厅会中转数据，所以
  任何人都不需要开放端口。私信：点击名字，或输入 `/w 名字 内容`。

如果某位玩家断线，其他人会回到本回合开始时等待：该玩家重新启动 OpenOrion2
并再次加入游戏（从大厅的列表，或用 `-join`）即可继续。在等待界面上按 START NET
GAME 会不等他继续游戏，由电脑接管他的帝国。

## 浏览器版本

OpenOrion2 也能以 WebAssembly 的形式在浏览器中运行：同样的游戏代码，存档保存在
浏览器中，并可与原生版本的玩家联网对战。
[tools/web/build.sh](tools/web/build.sh) 负责构建，
[cmd/webserve](cmd/webserve) 会连同你的游戏文件一起提供服务：
[docs/web.md](docs/web.md)。

目前没有可供游玩的公共网站。浏览器版本需要加载它的服务器上有原版文件，而把这些
文件提供给任何打开页面的人就等于分发游戏本身，本项目无权这样做。请用你自己的游戏
副本，在自己的机器上或通过 `webserve -auth` 为朋友自行部署。

## 计划

- 开放在线大厅服务器的源代码，让任何人都能自己运行一个。
- 尽可能多地引入社区非官方 1.50 补丁的内容。
- 为 macOS 版本签名并进行公证，让 macOS 启动时不再弹出警告。

## 文档

[docs/](docs) 中存放着重建游戏所依据的规格说明：文件格式、引擎、每个界面、回合、
战斗、联网对战和大厅，并附有原版例程的名称和地址（英文）。

- [ARCHITECTURE.md](ARCHITECTURE.md)：代码地图，以及每个部分从哪里入手（英文）。
- [CONTRIBUTING.md](CONTRIBUTING.md)：构建、测试、规则，以及如何安全地修改移植的例程（英文）。
- [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)：行为准则，Contributor Covenant（英文）。

## 许可证

OpenOrion2 是自由软件，采用 GNU 通用公共许可证第 3 版：见 [LICENSE](LICENSE)。

移植版自有的美术资源（[assets/](assets)）不含任何原版游戏的内容。其中包括
Fusion Pixel Font（SIL Open Font License 1.1，
[assets/lobby/font/](assets/lobby/font)），并通过
[go-sdl3](https://github.com/Zyko0/go-sdl3) 嵌入了 SDL3（zlib 许可证）。
