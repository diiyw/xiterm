# XiTerm

基于 [Onix](https://github.com/onix-engine/onix) 的原生桌面终端，使用 Go 实现。以 iTerm2 的工作流为目标，当前是第一版可运行实现，尚未达到全部功能对等。

界面通过 Onix 的原生窗口、GPU Canvas2D 和字体系统渲染；每个面板连接独立的 Unix PTY 或 Windows ConPTY，实际运行本机 shell。没有浏览器或 WebView。

## 运行

需要 Go **1.27+**、CGO 和 C/C++ 工具链。当前已在 Linux/X11 下验证，并通过 Windows amd64 交叉编译。Windows 使用 ConPTY（要求 Windows 10 1809+），默认运行 `COMSPEC` 指定的 shell，通常为 `cmd.exe /Q`。macOS / Windows 的原生构建和安装包验证由 GitHub Actions 对应 runner 执行。

Debian / Ubuntu 安装原生依赖：

```sh
sudo apt install build-essential pkg-config libx11-dev libxext-dev libxrender-dev libxi-dev libwayland-dev libxkbcommon-dev
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

Linux / macOS 默认使用 `$SHELL` 和登录 shell 参数 `-l`；未设置 `$SHELL` 时使用 `/bin/sh`。默认内嵌 **JetBrains Mono NL Nerd Font Mono v3.5.1** 的常规和粗体版本，支持 Powerline、文件类型、GitHub、终端、Material Design 等图标字形，无须在系统安装字体。使用自定义 `font_path` 时，私有区图标仍回退到内置 Nerd Font，其他缺失字符由 Onix 尝试系统字体回退。

## 已实现

- 真实 PTY shell、UTF-8 字符、ANSI / VT 解析、16 / 256 色和真彩色。
- 独立标签页、递归横向 / 纵向分屏、拖动分隔线调整比例、面板焦点切换。
- 窗口和字体大小变化同步至 PTY，支持 SIGWINCH。
- 主屏幕历史滚动、历史搜索、匹配高亮、拖动文本选择、系统剪贴板复制粘贴。
- 应用光标模式、备用屏幕、终端查询响应、括号粘贴、基础 TUI 鼠标事件。
- JSON 配置、多 shell profiles、起始目录、自定义环境变量、自定义字体、设置保存。
- OSC 0/2 标题、OSC 7 工作目录、文件拖入后生成带引号的路径。
- 会话退出状态显示，关闭活动会话时清理 shell 进程组。
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
| 滚动历史 | Shift+PageUp / PageDown、滚轮 | 同左 |
| 全屏 | F11 | F11 |
| 帮助 | F1 | F1 |

搜索时输入查询，`Enter` / `Shift+Enter` 选择下一个 / 上一个匹配，`Esc` 返回终端。TUI 开启鼠标捕获时，按住 `Shift` 可拖动选择文本。工具栏也提供新标签页、分屏、搜索和配置入口。

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
  "width": 1200,
  "height": 780,
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

原生窗口检查（Linux，需要 `xvfb`；完整检查还需要 `libxtst6` 和 Python Pillow）：

```sh
make smoke
make native-test
```

完整检查向真实 X11 窗口发送键盘事件，验证命令执行、独立标签页、嵌套分屏、设置保存和 shell 清理，截图输出至 `artifacts/native-smoke.png`。没有启动 Onix 项目编辑器。

## 结构与后续工作

- `cmd/xiterm`：启动参数和程序入口。
- `internal/app`：Onix 渲染、输入、搜索和桌面界面。
- `internal/terminal`：PTY 生命周期、VT 模拟、历史、搜索与选择。
- `internal/workspace`：标签页和递归分屏布局。
- `internal/config`：配置校验与原子保存。
- `assets`：内嵌 Nerd Font、应用图标及许可证。

后续重点：会话布局恢复、完整 shell integration、快捷键自定义、广播输入、触发器、tmux 集成、热键窗口、超链接操作、图片协议、Developer ID 签名 / 公证，以及 Windows ARM64 支持。

当前限制：历史按终端行存储，缩放不重排旧行；选择和搜索按物理行工作。尚未实现 IME 预编辑文字的绘制，以及复杂 emoji / ZWJ 字形塑形、完整终端协议一致性测试。关闭活动面板会结束其会话，没有未结束任务确认流程。

Onix 版本固定在 `e5ec5303825c`，可独立通过 Go modules 构建，无需本机 Onix checkout。PTY 使用 `creack/pty` / `charmbracelet/x/conpty`，VT 解析使用 `charmbracelet/x/vt`；依赖版本记录在 `go.mod` / `go.sum`。Nerd Font 许可见 `assets/fonts/LICENSE.txt` 和 `assets/fonts/NERD-FONTS-LICENSE.txt`。


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

图标源为 [`assets/icons/xiterm.svg`](assets/icons/xiterm.svg)，保留提供的矢量内容。PNG 用于应用窗口、工具栏和 Linux 桌面；ICO 用于 Windows EXE 和 MSI；ICNS 用于 macOS 应用和 DMG。

生成文件已保存在仓库，正常构建无需图像工具。修改 SVG 后重新生成：

```sh
npm install --prefix /tmp/xiterm-icon-tools @resvg/resvg-js@2.6.2
NODE_PATH=/tmp/xiterm-icon-tools/node_modules node scripts/render-icons.cjs
# 需要 Python Pillow
python3 scripts/pack-icons.py
```
