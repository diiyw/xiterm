# XiTerm

基于 [Onix](https://github.com/onix-engine/onix) 的原生桌面终端，使用 Go 实现。以 iTerm2 的工作流为目标，当前是第一版可运行实现，尚未达到全部功能对等。

界面通过 Onix 的原生窗口、GPU Canvas2D 和字体系统渲染；每个面板连接独立的 Unix PTY 或 Windows ConPTY，实际运行本机 shell。没有浏览器或 WebView。

## 运行

需要 Go **1.27+**、CGO 和 C/C++ 工具链。当前已在 Linux/X11 下验证，并通过 Windows amd64 交叉编译。Windows 使用 ConPTY（要求 Windows 10 1809+），默认运行 `COMSPEC` 指定的 shell，通常为 `cmd.exe /Q`。macOS / Windows 的原生构建和安装包验证由 GitHub Actions 对应 runner 执行。

Debian / Ubuntu 安装原生依赖：

```sh
sudo apt install build-essential pkg-config libx11-dev libxext-dev libxrender-dev libxi-dev libwayland-dev libxkbcommon-dev libgl-dev
make run
```

macOS 需要 Xcode Command Line Tools：

```sh
xcode-select --install
make run
```

构建独立二进制：

```sh
make build
./bin/xiterm
./bin/xiterm -directory "$PWD"
```

Linux / macOS 默认使用 `$SHELL` 和登录 shell 参数 `-l`；未设置 `$SHELL` 时使用 `/bin/sh`。默认内嵌 **JetBrains Mono NL Nerd Font Mono v3.5.1** 的常规和粗体版本，支持 Powerline、文件类型、GitHub、终端、Material Design 等图标字形，无须在系统安装字体。缺失的中文及其他字形使用系统已安装字体回退，中文按双列宽度显示，不额外捆绑中文字体。三个桌面平台的 UI 字体遵循同一规则：读取各自当前的系统界面字体，统一使用系统字形回退、文字测量和缩放逻辑。使用自定义 `font_path` 时，私有区图标仍回退到内置 Nerd Font。

| 平台 | UI 字体来源 |
| --- | --- |
| Windows | 系统当前 `NONCLIENTMETRICS` 消息字体 |
| macOS | CoreText 当前系统 UI 字体及对应字体集合索引 |
| Linux | 当前桌面字体设置，经 Fontconfig 解析；未提供桌面设置时使用系统默认字体 |

三端均使用已安装的系统中文字体，不随应用打包中文字体。系统字体加载、中文常规 / 粗体回退和 UI 宽度计算由三平台 CI 检查。

## 已实现

- 真实 PTY shell、UTF-8 字符、ANSI / VT 解析、16 / 256 色和真彩色。
- 顶部单行标签页、右键上下文菜单、SVG 操作图标；递归横向 / 纵向分屏、拖动分隔线调整比例、面板焦点切换。
- 窗口默认 1440 × 900；放大、缩小及字体大小变化时重算终端行列并同步至 PTY，支持 SIGWINCH。
- 主屏幕历史滚动、历史搜索、匹配高亮、拖动文本选择、系统剪贴板复制粘贴。
- 应用光标模式、备用屏幕、终端查询响应、括号粘贴、基础 TUI 鼠标事件。
- JSON 配置、多 shell profiles、起始目录、自定义环境变量、自定义字体、设置保存。
- OSC 0/2 标题、OSC 7 工作目录、文件拖入后生成带引号的路径。
- Linux / macOS 自动识别 `rz` / `sz` 的 ZMODEM 握手，弹出文件 / 保存目录选择框，支持多文件传输和取消。
- `exit` 或 shell 结束后自动关闭对应面板，空标签自动移除；最后一个标签关闭后退出程序，清理 shell 进程组。
- 空闲时等待窗口事件 / PTY 输出，光标闪烁时刷新。

## 快捷键

Linux 使用 `Ctrl+Shift`，macOS 使用 `Cmd` 作为应用快捷键前缀。`Ctrl+C` 等控制字符继续传给 shell。

| 操作 | Linux | macOS |
| --- | --- | --- |
| 新标签页 | Ctrl+Shift+T | Cmd+T |
| 关闭面板 | Ctrl+Shift+W | Cmd+W |
| 左右分屏 | Ctrl+Shift+D | Cmd+D |
| 上下分屏 | Ctrl+Shift+E | Cmd+E / Cmd+Shift+D |
| 搜索历史 | Ctrl+Shift+F | Cmd+F |
| 复制 / 粘贴 | Ctrl+Shift+C / V | Cmd+C / V |
| Profiles | Ctrl+Shift+P | Cmd+P |
| 设置 | Ctrl+Shift+, | Cmd+, |
| 保存设置 | Ctrl+Shift+S | Cmd+S |
| 放大 / 缩小字体 | Ctrl+Shift+= / - | Cmd+= / - |
| 重置字体 | Ctrl+Shift+0 | Cmd+0 |
| 清理历史并重绘 | Ctrl+Shift+K | Cmd+K |
| 切换标签页 | Ctrl+Tab / Ctrl+Shift+Tab | 同左 |
| 按序号选择标签页 | Alt+1…9 | 同左 |
| 切换面板 | Alt+Left / Right | 同左 |
| 滚动历史 | PageUp / PageDown（也支持 Shift）、滚轮 | 同左 |
| 历史首尾 | Shift+Home / End | 同左 |
| 命令行首尾 | Home / End | 同左 |
| 全屏 | F11 | F11 |
| 帮助 | F1 | F1 |

终端中的 `↑` / `↓` 传给 shell，用于预览、选择命令历史；`Enter` 才执行选中的命令，`↓` 可返回尚未提交的输入（具体行为由 shell 决定）。搜索时输入查询，`↓` / `↑` 或 `Enter` / `Shift+Enter` 选择下一个 / 上一个匹配，`Esc` 返回终端。右键菜单提供新标签页、分屏、复制粘贴、搜索、Profiles、设置及帮助，支持上下键选择、`Enter` 确认、`Esc` 关闭。TUI 开启鼠标捕获时，按住 `Shift` 可拖动选择文本。

普通屏幕中，`PageUp` / `PageDown`（或加 `Shift`）翻阅历史，`Shift+Home` / `Shift+End` 跳到最早历史 / 当前输出；`Home` / `End` 交给 shell 编辑命令行。使用备用屏幕的全屏程序会收到这些导航键及其修饰键，按程序自己的规则移动或翻页。

## rz / sz 文件传输

Linux / macOS 本机和远端都需要安装 `lrzsz`，本机的 `rz`、`sz` 必须在启动 XiTerm 时的 `PATH` 中：

```sh
# Debian / Ubuntu
sudo apt install lrzsz
# macOS
brew install lrzsz
```

在 XiTerm 中通过 `ssh` 登录远端后：

- 执行 `rz`：自动打开本机文件选择框，可一次选择多个文件，上传至远端当前目录。
- 执行 `sz file1 file2`：自动选择本机保存目录，下载远端文件；本机同名文件会自动改名，保留已有文件。
- 文件选择框中取消，或在传输面板按 `Ctrl+C` / `Esc`，会终止传输并恢复终端输入。面板底部显示传输状态，结束或失败时显示通知；连续 30 秒没有协议数据时自动中止。

协议数据直接交给本机 `lrzsz` 进程，不进入终端渲染和历史；接收端启用受限模式，禁止远端指定目录或执行命令。Linux 文件选择框使用桌面的 `xdg-desktop-portal`，需安装相应桌面后端。缺少本机工具或无法打开选择框时会提示并中止远端传输。Windows ConPTY 不能透明传递 ZMODEM 二进制流，当前会提示不支持。

## 配置

Linux 默认为 `~/.config/xiterm/config.json`（遵循 `XDG_CONFIG_HOME`）；macOS 默认为 `~/Library/Application Support/xiterm/config.json`；Windows 默认为 `%AppData%\xiterm\config.json`。缺少配置文件时直接使用默认值；打开 Settings 并保存可生成配置文件。无效配置会报错，不会覆盖原文件。

```sh
./bin/xiterm -print-default-config
./bin/xiterm -config /path/to/config.json
```

示例：

```json
{
  "default_profile": "Default",
  "font_size": 15,
  "scrollback": 10000,
  "width": 1440,
  "height": 900,
  "profiles": [
    {
      "name": "Default",
      "shell": "/bin/zsh",
      "args": ["-l"],
      "directory": "/home/yourname",
      "env": ["EDITOR=vim"]
    },
    {
      "name": "SSH",
      "shell": "/usr/bin/ssh",
      "args": ["your-host"],
      "directory": "/home/yourname"
    }
  ]
}
```

将示例的 shell 和目录替换为本机路径；目录和环境变量不会进行 shell 展开。可选的 `font_path` 指向本机 TTF/OTF 字体。修改配置文件后重启生效；设置面板中的字号立即生效，点击保存后持久化。

`scrollback` 是每个面板保留的最大历史行数，设为 `0` 可关闭历史。主屏幕历史采用紧凑的环形存储，共享同一行内的样式。仅绘制当前视口，按需展开可见历史行；滚出视口、返回实时输出或隐藏标签页时释放展开缓存，搜索和复制直接读取紧凑数据。每个面板的紧凑历史数据另有 **32 MiB** 上限；行数或内存达到上限时，优先淘汰最旧的历史，持续输出不会无限累积历史数据。

新标签页 / 分屏继承当前 profile 和已报告的本地目录。shell 未发送 OSC 7 时，目录保持为 profile 的起始目录。zsh 可在 `.zshrc` 中加入：

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

分配回归覆盖空闲轮询、菜单命中、面板查询、Unicode 搜索，以及终端、搜索、菜单和设置等界面的绘制指令生成。稳定布局、查询容量和字形缓存预热后，这些路径可达到 `0 B/op`、`0 allocs/op`；绘制分配测试使用 CPU Canvas，真实窗口另由原生测试验证。`Session.FindInto` 复用调用方的匹配切片，`Find` 仍返回独立持有的结果。PTY 收发复用有界缓冲块，关闭会话时释放；每会话最多保留 66 个 32 KiB 输出块和 66 个 1 KiB 输入块。VT 解析、新历史行、首次字形加载和容量增长仍可能分配。

```sh
go test ./internal/app -run '^$' -bench 'Benchmark(IdlePoll|MenuHitTest|WarmDraw)' -benchmem
go test ./internal/terminal -run '^$' -bench 'Benchmark(FindHistoryInto|IOBlockReuse|PackHistoryLine|ContinuousOutput)' -benchmem
```

原生窗口检查（Linux，需要 `xvfb`；完整检查还需要 `libxtst6` 和 Python Pillow）：

```sh
make smoke
make native-test
```

完整检查向真实 X11 窗口发送键盘和鼠标事件，验证 `cd` 文本输入、上下键历史选择及草稿恢复、Home/End 命令行编辑、全屏程序导航键序列、历史翻页和首尾跳转、中文常规和粗体的双列绘制、窗口放大/缩小时真实 PTY 行列变化、右键菜单、独立标签页、嵌套分屏、设置保存、`exit` 关闭及最后标签退出，截图输出至 `artifacts/native-smoke.png`、`artifacts/context-menu.png` 和 `artifacts/chinese-rendering.png`。没有启动 Onix 项目编辑器。

## 结构与后续工作

- `cmd/xiterm`：启动参数和程序入口。
- `internal/app`：Onix 渲染、输入、搜索和桌面界面。
- `internal/terminal`：PTY 生命周期、VT 模拟、历史、搜索与选择。
- `internal/workspace`：标签页和递归分屏布局。
- `internal/config`：配置校验与原子保存。
- `assets`：内嵌 Nerd Font、应用图标及许可证。

后续重点：会话布局恢复、完整 shell integration、快捷键自定义、广播输入、触发器、tmux 集成、热键窗口、超链接操作、图片协议、Developer ID 签名 / 公证，以及 Windows ARM64 支持。

当前限制：历史按终端行存储，缩放不重排旧行；选择和搜索按物理行工作。尚未实现 IME 预编辑文字的绘制，以及复杂 emoji / ZWJ 字形塑形、完整终端协议一致性测试。关闭活动面板会结束其会话，没有未结束任务确认流程。

Onix 版本固定为 [`v0.1.2`](https://github.com/onix-engine/onix/releases/tag/v0.1.2)（`9e88d1c2091e`），可独立通过 Go modules 构建，无需本机 Onix checkout。PTY 使用 `creack/pty` / `charmbracelet/x/conpty`，VT 解析使用 `charmbracelet/x/vt`；依赖版本记录在 `go.mod` / `go.sum`。Nerd Font 许可见 `assets/fonts/LICENSE.txt` 和 `assets/fonts/NERD-FONTS-LICENSE.txt`。


## Tag 自动打包

[`.github/workflows/release.yml`](.github/workflows/release.yml) 在推送 `vMAJOR.MINOR.PATCH` tag 后构建安装包，全部构建和测试通过后上传到对应 GitHub Release：

| 平台 | 架构 | 产物 |
| --- | --- | --- |
| Linux | amd64 | `xiterm-<version>-linux-amd64.tar.gz` |
| macOS | Apple Silicon arm64 | `xiterm-<version>-macos-arm64.dmg` |
| macOS | Intel amd64 | `xiterm-<version>-macos-amd64.dmg` |
| Windows | amd64 | `xiterm-<version>-windows-amd64.msi` |

每个文件附带 `.sha256` 校验文件。Release 只会在所有平台成功后发布；失败的平台可单独重跑。工作流也支持手动指定已存在的 tag 进行预览构建，手动构建仅上传 Actions artifacts。

```sh
git tag v0.1.0
git push origin v0.1.0
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

图标源为 [`assets/icons/xiterm.svg`](assets/icons/xiterm.svg)，保留提供的矢量内容。PNG 用于应用窗口、顶部标签栏和 Linux 桌面；ICO 用于 Windows EXE 和 MSI；ICNS 用于 macOS 应用和 DMG。

生成文件已保存在仓库，正常构建无需图像工具。修改 SVG 后重新生成：

```sh
npm install --prefix /tmp/xiterm-icon-tools @resvg/resvg-js@2.6.2
NODE_PATH=/tmp/xiterm-icon-tools/node_modules node scripts/render-icons.cjs
# 需要 Python Pillow
python3 scripts/pack-icons.py
```

界面操作图标采用内嵌的 Tabler SVG，通过 Onix SVG 解码器按三倍分辨率栅格化并绘制；不使用文本字形模拟按钮图标。SVG 源文件及 MIT 许可位于 `assets/icons/ui/`，打包时附带许可。
