# XiTerm 开发指南

## 从源码构建

需要 Go **1.27+**、CGO 和 C/C++ 工具链。以下命令在仓库根目录执行。

在 Debian / Ubuntu 上安装原生依赖后即可运行。

```sh
sudo apt install build-essential pkg-config libx11-dev libxext-dev libxrender-dev libxi-dev libwayland-dev libxkbcommon-dev libgl-dev
make run
```

macOS 需要 Xcode Command Line Tools。

```sh
xcode-select --install
make run
```

使用以下命令构建并运行独立二进制。

```sh
make build
./bin/xiterm
./bin/xiterm -directory "$PWD"
```

## 系统字体

三个桌面平台的 UI 字体遵循同一规则：读取各自当前的系统界面字体，统一使用系统字形回退、文字测量和缩放逻辑。使用自定义 `font_path` 时，私有区图标仍回退到内置 Nerd Font。

| 平台 | UI 字体来源 |
| --- | --- |
| Windows | 系统当前 `NONCLIENTMETRICS` 消息字体 |
| macOS | CoreText 当前系统 UI 字体及对应字体集合索引 |
| Linux | 当前桌面字体设置，经 Fontconfig 解析；未提供桌面设置时使用系统默认字体 |

三端均使用已安装的系统中文字体，不随应用打包中文字体。系统字体加载、中文常规 / 粗体回退和 UI 宽度计算由三平台 CI 检查。

## 终端历史存储

`scrollback` 是每个面板保留的最大历史行数，设为 `0` 可关闭历史。主屏幕历史采用紧凑的环形存储：普通 ASCII 行只存文本，Unicode 行保留压缩列信息，样式按连续区间共享，环形索引按需增长。历史行直接压缩，不产生完整单元格行的临时副本。仅绘制当前视口，按需展开可见历史行；滚出视口、返回实时输出或隐藏标签页时释放展开缓存，搜索和复制直接读取紧凑数据。每个面板的紧凑历史数据有 **32 MiB** 上限，所有标签和分屏共享 **64 MiB** 总上限；行数或内存达到上限时，优先淘汰最旧的历史。备用屏幕不保留历史。

## Shell 目录继承

新标签页和分屏使用 Shell 通过 OSC 7 报告的本地目录。Shell 尚未报告目录时，使用 Profile 的起始目录。用 Zsh 时可将以下代码加入 `.zshrc`。

```sh
if [[ "$TERM_PROGRAM" == XiTerm ]]; then
  autoload -Uz add-zsh-hook
  function _xiterm_directory() {
    printf '\033]7;file://%s%s\033\\' "$HOST" "$PWD"
  }
  add-zsh-hook precmd _xiterm_directory
fi
```

## 验证

```sh
make test
make check
go test -race ./assets ./internal/config ./internal/terminal ./internal/workspace ./internal/app
```

测试覆盖真实 shell 输入和退出、中文字符、PTY 尺寸、ANSI 调色板、终端查询响应、括号粘贴、备用屏幕、搜索与选择、分屏布局、拖动和配置校验。

安装 `lrzsz` 后，测试还会验证真实 ZMODEM 上传 / 下载、二进制内容、中文文件名、多文件、同名文件保留、取消及传输后的 shell 输入；未安装时跳过这些集成用例。

分配回归覆盖空闲轮询、菜单命中、面板查询、Unicode 搜索，以及终端、搜索、菜单和设置等界面的绘制指令生成。稳定布局、查询容量和字形缓存预热后，这些路径可达到 `0 B/op`、`0 allocs/op`；绘制分配测试使用 CPU Canvas，真实窗口另由原生测试验证。界面搜索按行保存累计匹配数，只为可见区域生成高亮结果，保留完整结果计数和上下导航；`Session.FindInto` / `Find` 仍提供完整结果列表。

PTY 输出队列最多 16 块，输出池最多缓存 18 个 32 KiB 块；输入和传输协议队列保持原容量。空闲 2 秒后输出池保留两个预热块，并释放较大的已完成解析缓冲、字体 CPU 缓存、系统字体回退缓存和 Metal 临时 GPU 缓冲。VT 控制字符串缓冲从 64 KiB 按需增长到 4 MiB，不丢弃尚未完成的分段序列。macOS 使用两个 Metal 在途帧及 drawable，保持原始 Retina 分辨率。VT 解析、新历史行、首次字形加载和容量增长仍可能分配。实现细节、测量结果及平台验证范围见 [内存优化记录](memory-optimization.md)。

```sh
go test ./internal/app -run '^$' -bench 'Benchmark(IdlePoll|MenuHitTest|WarmDraw)' -benchmem
go test ./internal/terminal -run '^$' -bench 'Benchmark(FindHistoryInto|IndexedSearchDense|IOBlockReuse|PackHistoryLine|ContinuousOutput)' -benchmem
```

原生窗口检查（Linux，需要 `xvfb`；完整检查还需要 `libxtst6` 和 Python Pillow）：

```sh
make smoke
make native-test
```

完整检查向真实 X11 窗口发送键盘和鼠标事件，验证 `cd` 文本输入、上下键历史选择及草稿恢复、Home/End 命令行编辑、全屏程序导航键序列、历史翻页和首尾跳转、中文常规和粗体的双列绘制、窗口放大/缩小时真实 PTY 行列变化、右键菜单、独立标签页、嵌套分屏、设置保存、`exit` 关闭及最后标签退出，截图输出至 `artifacts/native-smoke.png`、`artifacts/context-menu.png` 和 `artifacts/chinese-rendering.png`。

## 项目结构与路线图

- `cmd/xiterm`：启动参数和程序入口。
- `internal/app`：Onix 渲染、输入、搜索和桌面界面。
- `internal/terminal`：PTY 生命周期、VT 模拟、历史、搜索与选择。
- `internal/workspace`：标签页和递归分屏布局。
- `internal/config`：配置校验与原子保存。
- `assets`：内嵌 Nerd Font、应用图标及许可证。

后续重点：会话布局恢复、完整 shell integration、快捷键自定义、广播输入、触发器、tmux 集成、热键窗口、图片协议、Developer ID 签名 / 公证，以及 Windows ARM64 支持。

当前限制：历史按终端行存储，缩放不重排旧行；选择和搜索按物理行工作。尚未实现复杂 emoji / ZWJ 字形塑形和完整终端协议一致性测试。关闭活动面板会结束其会话，没有未结束任务确认流程。

Onix 使用官方发布的 [`v0.1.11`](https://github.com/onix-engine/onix/releases/tag/v0.1.11)，由 Go 下载。`charmbracelet/x/vt v0.1.0` 的补丁随仓库存放在 [`third_party`](../third_party/README.md)，由 `go.mod` 的本地替换固定。PTY 使用 `creack/pty` / `charmbracelet/x/conpty`。依赖版本记录在 `go.mod` / `go.sum`。Nerd Font 许可见 `assets/fonts/LICENSE.txt` 和 `assets/fonts/NERD-FONTS-LICENSE.txt`。

## Tag 自动打包

推送 `vMAJOR.MINOR.PATCH` tag 会触发下表中的安装包构建。全部构建和测试通过后，工作流将产物上传到对应的 GitHub Release。[发布工作流](../.github/workflows/release.yml)

| 平台 | 架构 | 产物 |
| --- | --- | --- |
| Linux | amd64 | `xiterm-<version>-linux-amd64.tar.gz` |
| macOS | Apple Silicon arm64 | `xiterm-<version>-macos-arm64.dmg` |
| macOS | Intel amd64 | `xiterm-<version>-macos-amd64.dmg` |
| Windows | amd64 | `xiterm-<version>-windows-amd64.msi` |

每个文件附带 `.sha256` 校验文件。Release 只会在所有平台成功后发布；失败的平台可单独重跑。工作流也支持手动指定已存在的 tag 进行预览构建，手动构建仅上传 Actions artifacts。

将 `XITERM_VERSION` 设为待发布的版本 tag。

```sh
git tag -a "$XITERM_VERSION" -m "XiTerm $XITERM_VERSION"
git push origin "$XITERM_VERSION"
```

工作流仅使用内置 `GITHUB_TOKEN` 发布 Release，不需要额外发布密钥。tag 使用三段数字版本，受 Windows MSI 版本字段限制，最大为 `255.255.65535`。预发布后缀暂不接受，避免 MSI 升级版本顺序不明确。

- **Linux**：tar 包包含程序、桌面入口、PNG 图标和依赖许可。解压后可直接运行 `bin/xiterm`；如需系统集成，将 `bin/` 和 `share/` 内容安装到 `~/.local/` 或 `/usr/local/`。运行环境需要 X11 / Wayland 和 OpenGL 系统库。
- **macOS**：DMG 内含 `XiTerm.app` 和指向 Applications 的快捷方式。`.app` 包含 ICNS 图标和版本信息，最低 macOS 13；可拖入 Applications 安装。当前使用 ad-hoc 签名，尚未使用 Apple Developer ID 签名或 Apple 公证。
- **Windows**：MSI 按计算机安装至 Program Files，添加开始菜单入口和卸载入口，固定 UpgradeCode 支持新版本升级。安装需要管理员权限；配置保存在用户 AppData，升级与卸载不会删除用户配置。EXE 和安装 / 卸载入口使用提供的图标，GCC 运行库静态链接。MSI / EXE 当前没有 Authenticode 签名。

本地打包需要在目标操作系统执行（macOS 使用 `codesign` / `hdiutil`，Windows 使用 WiX **4.0.6** 和 `go-winres` **0.3.3**）：

```sh
python3 scripts/package.py --tag v0.1.0
# 或 make package TAG=v0.1.0
```

Windows 打包前执行：

```powershell
go install github.com/tc-hib/go-winres@v0.3.3
dotnet tool install --global wix --version 4.0.6
$env:DOTNET_ROLL_FORWARD = 'Major'
python scripts/package.py --tag v0.1.0
```

CI 包含 Go 测试、字体字形与等宽检查、工作流版本校验、真实 ConPTY 测试、Linux tar 解压及原生窗口检查、macOS bundle 签名 / DMG 校验，以及 Windows MSI 静默安装 / 版本 / 图标 / 卸载检查。Windows CI 不依赖图形硬件启动完整 GUI，完整界面仍需在真实桌面环境验证。

## 应用图标

图标将终端提示符 `>` 和下划线光标融入首字母 X，矢量源文件为 [`assets/icons/xiterm.svg`](../assets/icons/xiterm.svg)。PNG 用于应用窗口、顶部标签栏和 Linux 桌面；ICO 用于 Windows EXE 和 MSI；ICNS 用于 macOS 应用和 DMG。

生成文件已保存在仓库，正常构建无需图像工具。修改 SVG 后重新生成：

```sh
npm install --prefix /tmp/xiterm-icon-tools @resvg/resvg-js@2.6.2
NODE_PATH=/tmp/xiterm-icon-tools/node_modules node scripts/render-icons.cjs
# 需要 Python Pillow
python3 scripts/pack-icons.py
```

界面操作图标采用内嵌的 Tabler SVG，通过 Onix SVG 解码器按三倍分辨率栅格化并绘制；不使用文本字形模拟按钮图标。SVG 源文件及 MIT 许可位于 `assets/icons/ui/`，打包时附带许可。
