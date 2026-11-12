# 内存优化记录

## 已处理的占用

| 路径 | 调整 |
| --- | --- |
| 备用屏幕 | 不再保存无法回看的 10,000 行历史；主屏幕内容继续保留 |
| 主屏幕历史 | 借用滚出的行直接压缩，消除完整单元格行的临时复制；ASCII 只存文本；Unicode 元数据由 12 降至 8 字节/单元格；环形索引按需增长 |
| 多标签 / 分屏 | 每面板历史上限 32 MiB，所有会话合计 64 MiB，跨会话优先淘汰最旧行；关闭面板后退出预算 |
| 搜索 | 保留逐行累计计数，导航时解码目标行，高亮只展开可见结果；结果数量不截断 |
| VT 解析 | 固定 4 MiB 改为初始 64 KiB，控制字符串按需增长至 4 MiB；空闲只缩减已完成序列的缓冲 |
| PTY 输出 | 输出队列 64 → 16，池容量 66 → 18 块，每块 32 KiB，空闲保留两块；输入及 ZMODEM 队列容量不变 |
| 字体元数据 | CFF INDEX 和 cmap format 12 借用原始表，FDSelect 字典编号使用 `uint8` |
| 字体缓存 | CPU 位图上限每 face 1 MiB / 256 项，弱轮廓索引最多 4096 项，字形图集池预算 2 MiB；空闲清理回退解析和 CPU 缓存，回退源不再永久强引用字体 |
| macOS GPU | 在途帧及 drawable 统一为 2；空闲 2 秒、帧外调用 Metal 回收，先等待 GPU 完成再释放临时缓冲及退休资源；保持 Retina 分辨率 |

64 MiB 是紧凑历史的预算，不是整个进程的内存上限；视口单元格、字体、图集、窗口及 GPU 驱动也占内存。Go 和系统分配器释放对象后，RSS 不一定立即下降。

## 本次实测

环境：Linux/X11、Xvfb、1440 × 900、字号 15、历史上限 10,000 行、Go 1.27.3，使用同一份额外输出 Go MemStats / heap profile 的临时构建入口比较修改前后二进制。每项独立运行 12 秒，单会话，输出后保持进程空闲。8 秒时显式 GC 用于比较存活堆；正式程序没有新增强制 GC。

| 场景 | 修改前 RSS（约） | 修改后 RSS（约） | GC 后 Go 存活堆（前 → 后） |
| --- | ---: | ---: | ---: |
| 备用屏幕输出 12,000 × 140 个 ASCII 字符 | 336 MiB | 142 MiB | 177 → 9 MiB |
| 主屏幕输出同样日志 | 186 MiB | 146 MiB | 34 → 11 MiB |
| 中文常规与粗体回退 | 141 MiB | 134 MiB | 19 → 7 MiB |
| 初始空闲 | 124 MiB | 129 MiB | 11 → 6 MiB |

RSS 包括原生驱动、共享库等，采样和分配时机有波动；初始空闲 RSS 没有改善。以上不能直接换算成 macOS 活动监视器的读数，也不代表所有负载都有同等降幅。

负载可在新会话中用 Python 复现，分别启动独立窗口比较；输出后等待空闲回收，再记录进程及 GPU 内存。主屏幕长日志：

```sh
python3 -c 'import time; print(("x"*140+"\n")*12000, flush=True); time.sleep(30)'
```

备用屏幕：

```sh
python3 -c 'import time; print("\033[?1049h"+("x"*140+"\n")*12000, flush=True); time.sleep(30); print("\033[?1049l", flush=True)'
```

密集搜索测试在 10,000 行、每行 140 个 `x` 的历史中搜索 `x`，约 140 万个结果仅保留约 80 KiB 行计数索引；测试同时检查所有结果计数、跨行导航、可见高亮和预热后的零分配。空闲轮询、菜单命中、预热绘制及 I/O 复用基准仍为 `0 B/op`、`0 allocs/op`。

## 平台验证

Linux 完成应用、VT、字体测试和竞态检查，完成真实 X11 窗口的中文、搜索、连续输出下输入响应、字号反复缩放、窗口缩放、标签和分屏、PTY 以及 ZMODEM 回归。Windows amd64 使用 MinGW 成功交叉编译。

后续在 macOS 26.2 / arm64、Go 1.27.1 上完成原生构建、应用 / VT / 字体测试及 `go vet`，并通过真实 Metal 设备上的 `TestMetalMemoryLifecycle`。原生测试覆盖两个槽位反复轮换、资源延迟释放、空闲回收和回收后重建，已接入 macOS 发布 CI；无 Metal 设备会明确跳过。应用通过签名校验，并以独立配置完成备用屏幕 12,000 × 140 字符输出、中文常规 / 粗体和空闲回收的真实窗口启动检查。

macOS 检查同时发现并补齐了 `go mod vendor` 未复制的四个 Metal `.inc` 着色器文件。字体和渲染依赖的原生 FFI 使用 `uintptr` 传递句柄，对这两个包单独关闭 `vet` 的 `unsafeptr` 分析；应用和 VT 仍执行常规 `go vet`。目前尚未完成 macOS 修改前后的同负载内存对照及流畅度测量，不能直接把上面的 Linux 结果当作 macOS 降幅。

```sh
make test
make check
go test -race ./assets ./internal/config ./internal/terminal ./internal/workspace ./internal/app github.com/charmbracelet/x/vt github.com/onix-engine/onix/font
# macOS，需 Xcode Command Line Tools 和 Metal 设备
go test github.com/onix-engine/onix/render -run '^TestMetalMemoryLifecycle$' -count=1 -v
```

macOS 后续应比较相同窗口尺寸下：空闲、长日志、退出 TUI、中文常规/粗体、反复缩放及多个标签/分屏。分别观察活动监视器的进程内存、Go heap 和 Metal 资源，避免把 GPU / WindowServer 占用误判为 Go 堆泄漏。
