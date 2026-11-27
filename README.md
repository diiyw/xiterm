<img src="assets/icons/xiterm.png" alt="XiTerm Logo" width="104" height="104">

# XiTerm

XiTerm 是一款适用于 **macOS、Linux 和 Windows** 的原生桌面终端模拟器。用标签页和可调整大小的分屏并排运行本地 Shell、SSH 会话和终端应用，随时搜索历史输出。

XiTerm is a native, GPU-accelerated terminal emulator for macOS, Linux, and Windows, with tabs, split panes, scrollback search, shell profiles, and ZMODEM file transfers on Linux and macOS. It is built in Go with Onix.

**[下载最新版](https://github.com/diiyw/xiterm/releases/latest)** · [快捷键](#快捷键) · [配置](#配置) · [从源码构建](docs/development.md#从源码构建)

## 下载与安装

从 [GitHub Releases](https://github.com/diiyw/xiterm/releases/latest) 下载适合当前系统的文件。每个安装包均附带 SHA-256 校验文件。

| 平台 | 下载和启动 |
| --- | --- |
| macOS 13+ | Apple Silicon 下载 `macos-arm64.dmg`，Intel 下载 `macos-amd64.dmg`。打开 DMG，将 **XiTerm.app** 拖入「应用程序」。安装包使用临时签名（ad-hoc），尚未经过 Apple 公证。 |
| Linux x86-64 | 下载 `linux-amd64.tar.gz`，解压后运行 `bin/xiterm`。也可将解压后的 `bin/` 和 `share/` 安装到 `~/.local/`。运行需要图形桌面和 OpenGL 系统库。 |
| Windows 10 1809+ / Windows 11 x64 | 下载 `windows-amd64.msi`，使用管理员权限安装，然后从「开始」菜单启动 **XiTerm**。MSI 和 EXE 当前未签名。 |

首次启动直接打开本地 Shell。macOS 和 Linux 使用 `$SHELL`（未设置时使用 `/bin/sh`），Windows 使用 `COMSPEC`（通常是 `cmd.exe`）。

## 主要功能

- **标签页与分屏**。在同一窗口管理多个独立会话，横向或纵向分屏，拖动分隔线调整大小。
- **历史搜索与链接**。搜索滚动历史并逐个跳转，点击 HTTP(S) 地址或 OSC 8 超链接可在默认浏览器中打开。
- **剪贴板与文件路径**。复制、粘贴文本，拖入文件可插入带引号的路径。OSC 52 支持写入系统文本剪贴板。可粘贴截图供支持 OSC 5522 粘贴通知的应用读取，普通会话也可将截图作为临时 PNG 文件路径粘贴。[OSC 5522 协议说明](https://sw.kovidgoyal.net/kitty/clipboard/#allowing-terminal-applications-to-respond-to-paste-events)
- **Shell Profiles**。为每个配置指定 Shell、启动参数、起始目录和环境变量，也可直接启动指定主机的 SSH 会话。
- **原生渲染与字体**。使用 GPU 渲染，支持真彩色、UTF-8 和双列中文，内置 JetBrains Mono NL Nerd Font Mono，缺失字形由系统字体补齐。
- **ZMODEM 文件传输**。在 Linux 和 macOS 的 SSH 会话中使用 `rz` 和 `sz` 上传或下载文件，支持多文件和取消。

## 快捷键

| 操作 | Linux / Windows | macOS |
| --- | --- | --- |
| 新建标签页 | Ctrl+Shift+T | Cmd+T |
| 关闭面板 | Ctrl+Shift+W | Cmd+W |
| 左右分屏 | Ctrl+Shift+D | Cmd+D |
| 上下分屏 | Ctrl+Shift+E | Cmd+E / Cmd+Shift+D |
| 搜索历史 | Ctrl+Shift+F | Cmd+F |
| 复制 / 粘贴 | Ctrl+Shift+C / V | Cmd+C / V |
| 切换标签页 | Ctrl+Tab / Ctrl+Shift+Tab | Ctrl+Tab / Ctrl+Shift+Tab |
| 按序号选择标签页 | Alt+1…9 | Option+1…9 |
| 切换面板 | Alt+← / → | Option+← / → |
| 选择 Profile | Ctrl+Shift+P | Cmd+P |
| 打开设置 | Ctrl+Shift+, | Cmd+, |
| 保存设置 | Ctrl+Shift+S | Cmd+S |
| 放大 / 缩小 / 重置字号 | Ctrl+Shift+= / - / 0 | Cmd+= / - / 0 |
| 清理历史并重绘 | Ctrl+Shift+K | Cmd+K |
| 全屏 / 快捷键帮助 | F11 / F1 | F11 / F1 |

右键菜单也可新建标签页、分屏、复制粘贴和打开设置。搜索时用 `Enter` / `Shift+Enter` 跳转到下一个 / 上一个匹配，按 `Esc` 回到终端。普通终端屏幕中用 `PageUp` / `PageDown` 翻页，`Shift+Home` / `Shift+End` 跳到最早历史或当前输出。使用备用屏幕的全屏程序会接收这些按键。

当终端应用捕获鼠标时，按住 `Shift` 可拖选文本，按住 `Ctrl`（macOS 也可用 `Cmd`）单击可打开链接。

## rz / sz 文件传输

Linux 和 macOS 的本机与远端都需要安装 `lrzsz`。启动 XiTerm 时，本机的 `rz`、`sz` 必须在 `PATH` 中。Linux 的文件选择框还需要 `xdg-desktop-portal` 和对应的桌面后端。

```sh
# Debian / Ubuntu
sudo apt install lrzsz

# macOS
brew install lrzsz
```

在 XiTerm 中通过 `ssh` 登录远端后，执行 `rz` 会打开本机文件选择框，选中的文件上传到远端当前目录。执行 `sz file1 file2` 会让你选择本机保存目录，遇到同名文件时自动重命名下载项。可在文件选择框中取消，也可在传输面板按 `Ctrl+C` 或 `Esc` 中止。

## 配置

打开 **Settings** 并保存即可生成配置文件。可以直接编辑 JSON，重启后生效。也可运行 `xiterm -print-default-config` 查看默认设置，或用 `xiterm -config /path/to/config.json` 指定文件。

| 平台 | 默认配置路径 |
| --- | --- |
| macOS | `~/Library/Application Support/xiterm/config.json` |
| Linux | `~/.config/xiterm/config.json`，优先遵循 `XDG_CONFIG_HOME` |
| Windows | `%AppData%\xiterm\config.json` |

例如，以下配置增加一个本地 Zsh Profile。请将 Shell 和起始目录换成当前系统上的路径。

```json
{
  "default_profile": "Local",
  "font_size": 15,
  "scrollback": 10000,
  "profiles": [
    {
      "name": "Local",
      "shell": "/bin/zsh",
      "args": ["-l"],
      "directory": "/home/yourname",
      "env": ["EDITOR=vim"]
    }
  ]
}
```

用 `font_path` 设置自定义 TTF/OTF 字体，或将 `scrollback` 设为 `0` 来关闭历史。新标签页和分屏继承当前 Profile 与 Shell 通过 OSC 7 报告的本地目录。也可用 `xiterm -directory /path/to/project` 从指定目录打开。[在 Zsh 中启用目录继承](docs/development.md#shell-目录继承)

## 开发

源码构建、测试、打包和图标生成方法见 [开发指南](docs/development.md)。终端历史、渲染和内存使用的实现细节见 [内存优化记录](docs/memory-optimization.md)。
