# 本地依赖补丁

根目录 `go.mod` 使用 `replace` 固定以下两份源码，构建不依赖开发者机器上的模块缓存修改。

| 目录 | 来源 | 许可证 |
| --- | --- | --- |
| `vt` | `github.com/charmbracelet/x/vt v0.1.0` | [MIT](vt/LICENSE) |
| `onix` | `github.com/onix-engine/onix v0.1.5`（`5967841f067b`） | [MIT](onix/LICENSE) |

`vt` 保留上游模块源码和测试。`onix` 以该版本 `go mod vendor` 选出的运行时源码为基础，保留 XiTerm 所需包的各平台文件、原生头文件和着色器，以及原始 `go.mod`、`go.sum`、许可证；不包含上游编辑器、示例和无关资源。Metal 的四个 `render/shaders/metal_*.inc` 文件由上游单独补入，`go mod vendor` 不会自动复制这些 C 包含文件。另保留字体解析、系统字体发现测试，并增加内存回归测试和 Metal 原生测试。

## 补丁范围

`vt`：

- 禁止备用屏幕保留不可访问的历史。
- 同步投递借用的历史行，供 XiTerm 直接压缩，避免完整 `uv.Cell` 行的临时复制。
- 控制字符串缓冲从 64 KiB 按需增长到原来的 4 MiB 上限；完成后可在空闲期缩回，未完成的分段序列继续保留。

`onix`：

- CFF INDEX 和 cmap format 12 引用字体原始数据，避免按字形重复分配索引；FDSelect 使用单字节字典编号。
- 字体 CPU 位图缓存限制为每个 face 1 MiB / 256 项，弱轮廓索引限制为 4096 项，提供空闲释放接口。
- 系统回退源弱引用已加载字体，空闲时释放解析结果缓存；仍在使用的字体保持有效。
- 字形图集池预算从 8 MiB 缩至 2 MiB。
- Metal 在途帧和 CAMetalLayer drawable 数量统一为 2；XiTerm 空闲时调用现有的 GPU 临时缓冲回收接口。

## 维护与验证

在仓库根目录运行 `make test` 和 `make check`，会同时验证应用与修改过的依赖。macOS 上 `TestMetalMemoryLifecycle` 会调用 Xcode Command Line Tools 编译原生测试，使用真实 Metal 设备检查两帧循环、资源延迟释放和空闲回收；无 Metal 设备时明确跳过。

当前 Onix 运行时已同步至 v0.1.5，保留下述内存补丁；此次升级仅完成编译，按要求未运行测试。

升级依赖时，先对比对应上游版本，重新应用仍需要的补丁并执行各平台检查。不要直接用新一次 `go mod vendor` 的结果覆盖本目录，否则会丢失本地补丁和回归测试。测量方法与当前结果见 [内存优化记录](../docs/memory-optimization.md)。
