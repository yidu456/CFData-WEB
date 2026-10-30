# CFData-Web

CFData-Web 是一个基于 Go 的 Cloudflare IP 测试与筛选工具，提供本地 Web 与 CLI 两种使用方式，支持官方 IP 段扫描、非标目标测试、测速、结果筛选、导出和 GitHub 上传。

[在线演示站](https://cfdata-demo.cce.de5.net/) 仅使用浏览器内虚拟数据，用于预览界面与交互；真实使用请下载正式版本。

![image](img/demo.png)

### 更新说明

项目功能已趋于完善，基本达到了作者预期的效果。后续更新将以维护和新功能为主，版本迭代频率会有所降低。如果你在使用过程中有新的需求或建议，欢迎提交 Issue，作者会在评估后酌情纳入后续版本。

近期主要更新：

- 非标链路复验：扫描完成后对候选做真实 TCP+TLS+HTTP 探针验证（Web 支持手动复验按钮），不可用候选自动剔除
- 测速地址可用性预检：启动时并行探测全部内置与自定义测速地址，自动选择时排除不可用项，全部不可用回退 CF 官方地址；Web 下拉框实时展示可用性
- 结果区「筛选排序」合一：原独立筛选下拉与一键排序合并进单个弹窗，新增「延迟低于 xx ms」「丢包低于 xx%」筛选，并与复制/导出内容联动
- 管理面代理：新增 -proxy 参数与配置文件 proxy 键（支持 socks5://、socks5h://、http://、https://），覆盖数据下载、版本检查、GitHub/订阅上传；Web 设置菜单可直接配置，未配置时自动使用系统环境变量代理；扫描与测速保持直连
- CLI 交互菜单：`-cli` 进入菜单（按配置启动 / 自定义参数启动 / 修改配置参数 / 定时任务 / 帮助），`-cli qs` 按已保存配置快速启动，`-cli -参数…` 直接执行；启动时输出 Banner 与「调试等级」
- 参数设置向导：分模块逐项设置，提示格式与可选值，可保存为配置文件
- 定时任务：仅 Linux systemd，按天/周/月执行，开机自启，失败自动回滚清理，unit 与日志在程序目录
- 官方/非标的扫描并发、延迟阈值、测速网址在命令行与配置文件中完全独立，互不影响
- 配置文件自动迁移：键名无变化时静默按新模板迁移；存在旧键名时交互提示（仅迁移同名值），原文件备份为 `.bak`，非交互环境自动跳过
- 启动输出统一为简洁的「当前配置」单块，版本行下方显示调试等级

## 功能

- 官方优选：扫描 Cloudflare IPv4/IPv6，按数据中心继续详细延迟测试。
- 非标优选：上传本地 txt/csv 或填写网络 URL，测试自定义 IP/域名与端口。
- 测速：支持单点测速、批量测速、非标并发测速和测速阈值筛选。
- 链路复验：扫描完成后对候选做真实 TCP+TLS+HTTP 探针验证，自动剔除不可用项；Web 支持手动复验按钮。
- 导出：支持 CSV/TXT、自定义字段、IP 类型筛选、合格结果筛选。
- 上传：支持将导出结果上传到 GitHub。
- CLI：交互菜单、参数设置向导、定时任务、配置自动迁移。
- APK：支持 Android WebView 壳运行内置后端。

## 快速开始

从 [Releases](https://github.com/PoemMisty/CFData-WEB/releases/latest) 下载对应平台程序后运行。

默认启动 Web 模式：

```text
服务启动于 http://localhost:13335
当前测速网址: auto
```

浏览器打开终端中的地址即可使用。

CLI 模式（进入交互菜单）：

```bash
./cfdata-linux-amd64 -cli
```

菜单提供：按配置启动（菜单 1，等价 `-cli qs`）、自定义参数启动（向导）、修改配置参数、定时任务、帮助。

首次使用 CLI 配置文件时会生成模板并退出，编辑配置后重新运行即可；也可通过菜单「修改配置参数」逐项设置。升级后启动时若配置键名有变化会提示迁移：同名值保留，原文件备份为 `.bak`，非交互环境自动跳过。

简单示例：

```bash
# 交互菜单（不带其他参数时默认进入菜单）
./cfdata-linux-amd64 -cli

# 按已保存配置快速启动（qs 大小写不敏感）
./cfdata-linux-amd64 -cli qs

# 官方模式直接执行：扫描 IPv4，测试 443 端口，测速地址自动选择
./cfdata-linux-amd64 -cli -mode official -offiptype 4 -offport 443 -offurl auto

# 非标模式直接执行：读取本地文件，开启 TLS 和 5 个测速线程
./cfdata-linux-amd64 -cli -mode nsb -nsbfile ip.txt -nsbtls=true -nsbspeedtest 5 -nsburl auto
```

## Web 使用

界面顶部「扫描方式」选择器支持 TCPing（默认）和 HTTPing。不同扫描模式的延迟数据不可互相比较，仅同模式内的对比才有意义。

### 扫描方式说明

- **TCPing**：测量 TCP 握手延迟，基准值。
- **HTTPing**：测量 HTTP TTFB（Time To First Byte），延迟比 TCPing 高属正常现象。延迟阈值和渲染颜色已按倍率自动缩放，倍率仅为延迟等级参考值，非精确换算：
  - 无 TLS（HTTP 端口）：×1.3
  - 有 TLS（HTTPS 端口）：×4.0

### 官方优选

1. 选择 IPv4 或 IPv6。
2. 设置测试端口、扫描并发、延迟阈值。
3. 点击“开始扫描与测试”。
4. 扫描完成后选择数据中心继续详细测试。
5. 在详细测试结果中可单点测速或批量测速。

### Web 多中心排名与华为云 DNS（开发预览）

1. 先扫描发现数据中心，再在顶部下拉框按代码、城市或国家搜索（不区分大小写），任意勾选多个中心。搜索只筛选选项，不改变已选中心；清空搜索恢复全部发现项。下拉框与表格同步，支持键盘选择；表格使用 24px 原生复选框和至少 44px 点击区域，单中心「选择测试」按钮仍独立。在扫描结果区点击「测试已选数据中心」；任务运行期间禁止修改选择，重新扫描会清空搜索与选择。扫描前没有固定 FRA/SJC 名单。
2. 设置详细延迟上限（ms）、最低下载速度（MB/s）和「最终总数」（默认 10）。所有选中中心内延迟合格的 IP 都测完速度后，跨中心去重，按速度降序、延迟升序排名，取全局前 N；没有每中心配额，少于 N 个合格结果也有效。未发现的中心会明确提示，不会换用其他中心；未测速、失败或不达标的 IP 不入选。
3. 原有单中心「自动测速」与目标中心仍是独立模式，其合格数量达到上限即可停止；不要把该停止上限与多中心全量测完后的最终总数混淆。

点击扫描或测试区的「结果操作」，先选择「导出文件」「上传 GitHub」「上传 edgetunnel」或「更新华为云 DNS」，再填写该操作的配置。各操作只显示自己的账户和按钮，输出操作复用格式、筛选与预览；「返回」或 Escape 返回操作选择，选择页再按 Escape 关闭并返回入口。切换操作保留输入和快速上传文件，不会自动上传或更新 DNS；关闭整个结果操作后清空尚未应用的 Huawei AK/SK。

选择「更新华为云 DNS」打开现有面板；没有扫描/测试结果时仍可配置和只读发现，但不能发布。右上角设置菜单的「华为云 DNS」入口仍可直接使用：

- 输入 AK/SK，选择支持的固定 DNS 区域，并填写该区域的 **Project ID（32 位十六进制，非账号 ID）**。程序使用官方华为云 SDK 与区域项目鉴权，不接受自定义签名端点；凭据应仅有读取公网域名/记录和更新所选记录所需的权限。
- 默认仅保留于后端内存；显式勾选私有保存后写入操作系统用户配置目录的 `cfdata-web/huawei-dns.private`（Windows 使用当前用户 DPAPI，其他平台使用私有目录/0600 文件）。AK/SK 留空保留已有值，界面只显示 AK 掩码和 SK 是否存在，不会回填密钥。取消保存勾选不会删除先前保存的文件；如需移除，停止程序后删除该私有文件。
- 「测试连接（只读）」/发现公网域名 → 选择域名 → 选择已有 A/AAAA 记录及线路；核对 Zone ID、Record ID、名称、类型、线路和 TTL。应用配置后须重新发现和选择目标；连接测试本身不写 DNS。
- 完成同一会话的多中心测试后，点击「预览完成批次」，核对旧/新地址，再点击「确认更新」。更新仅替换所选现有记录的值，保留 TTL、线路等属性，不创建、删除或重建记录。A 仅用 IPv4，AAAA 仅用 IPv6，DNS 值不带端口；地址集合未变或没有匹配地址族时跳过。
- 如需自动更新，明确勾选「下次多中心测试完成后自动更新此记录集」；GitHub 则复用结果操作面板的账户/仓库配置，可手动上传完成批次或独立勾选下次完成后上传。两项开关与状态相互独立，一项失败不代表另一项失败。多中心发布由后端完成快照决定，不采用浏览器传入的 IP/导出文本。

安全与边界：仅在 `127.0.0.1`/`localhost` 本地访问；如要远程访问，必须同时启用 `-user` 和 `-password`，并保护传输链路，新的配置/发布操作还要求同源。不要将密钥、GitHub Token 或私有配置放入 Git、日志或浏览器配置/结果导出。空结果、失败、取消或失效的批次不会覆盖 DNS；重测、断线、目标/云端状态变化后须重新获取有效完成批次和预览。请求已发出后若超时/断线，云端可能已经应用变更，先只读核对记录或仓库再重试，不能把本地失败视为云端回滚。

这是 Web 优先的开发预览，独立测试构建不覆盖现有生产 EXE；本次未增加对应 CLI 功能或 Windows 定时任务。详细范围见 [Web 多中心与 DNS 说明](docs/superpowers/specs/2026-10-30-web-multi-dc-dns.md)。

### 开发验证（不连接真实云）

在 `combined_refactor/` 使用 `go.mod` 要求的 Go 1.26 工具链（当前验证为 1.26.2）：

```bash
go test ./...
go vet ./...
node web_batch_ui_test.js
node web_publish_ui_test.js
node huawei_ui_test.js
node result_actions_ui_test.js
mkdir -p ../release_assets
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o ../release_assets/cfdata-web-preview.exe .
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o ../release_assets/cfdata-test .
```

有可用 C 编译器时启用 CGO 另跑 `go test -race ./...`；用 `gofmt -l` 检查本次改动的 Go 文件，不要格式化根目录参考源码或无关旧文件。浏览器烟测需要本机已安装的 Playwright 和 Chrome/Chromium，不新增依赖、不下载浏览器。PowerShell 示例：

```powershell
$env:NODE_PATH = '<已安装依赖的 node_modules 目录>' # 本地 require("playwright") 已可用时可省略
$env:CHROME_PATH = '<本机 Chrome/Chromium 可执行文件>' # 已有 Playwright 浏览器时可省略
$env:QA_OUTPUT_DIR = Join-Path $env:TEMP 'cfdata-web-preview-qa' # 可选，截图默认写入系统临时目录
node --check tests/web_preview_smoke.cjs
node tests/web_preview_smoke.cjs
```

烟测在浏览器内加载真实 HTML，以模拟 WebSocket 验证操作选择/独立配置/返回与焦点、切换时保留输入/快速文件、无结果的 DNS 入口、多选/排名显示、空结果/取消/重连、DNS 配置/预览/确认与独立发布状态、后台重测来源校验、旧结果上传和桌面/手机布局；阻断其他 HTTP(S) 请求，不启动服务、不扫描、不上传真实结果、不修改真实 DNS。Go 测试使用模拟网络验证后端全量测量/排名与发布权限，烟测不能替代真实华为云权限、线路兼容性和网络测速验收。手工预览应使用独立测试 EXE，并显式加 `-host 127.0.0.1`；真实云更新仅在用户自行配置、核对并确认后进行。

### 非标优选

1. 切换到“非标优选”。
2. 上传 txt/csv，或填写网络 URL（二选一）。
3. 设置备用端口、并发、TLS、结果上限、测速线程、测速阈值等参数。
4. 点击“开始扫描与测试”。
5. 在结果表格查看、筛选、导出或上传。

非标输入推荐格式：

```text
1.2.3.4 443
5.6.7.8 8443
2606:4700::1111 443
1.1.1.1
```

未提供端口时会使用备用端口；备用端口默认随 TLS 模式自动选择，关闭 TLS 为 80，开启 TLS 为 443。

## 测速地址

默认测速地址为 `auto`，表示由后端自动选择内置测速源。

启动时会对全部内置测速地址和已输入的自定义测速地址做可用性测试（小体积探测），自动选择时跳过不可用地址；若全部不可用则回退 CF 官方地址。Web 端会把测试结果同步到测速地址下拉框，不可用项会标注「不可用」。

Web 下拉项：

- 自动选择
- Cloudflare
- CM提供
- 移动专属
- 手动输入

CLI 可通过 `-offurl`/`-nsburl` 指定：

```bash
./cfdata-linux-amd64 -cli -offurl auto
./cfdata-linux-amd64 -cli -offurl speed.cloudflare.com/__down?bytes=99999999
./cfdata-linux-amd64 -cli -offurl https://example.com/file.bin
```

说明：测速只读取响应字节流计算速度，不会把测速文件保存到本地。

## 常用参数

```text
-cli              启用 CLI 模式
-mode             official 或 nsb
-scanmode         扫描方式：tcping（默认，TCP 握手延迟）或 httping（HTTP TTFB，延迟比 tcping 高属正常，不同模式数据不可对比）
-offthreads       官方扫描并发数
-nsbthreads       非标扫描并发数
-offport          官方测试/测速端口
-offdelay         官方延迟阈值，单位毫秒
-nsbdelay         非标延迟阈值，单位毫秒；0=不筛延迟
-offurl           官方测速下载地址，默认 auto
-nsburl           非标测速下载地址，默认 auto
-dns              自定义 DNS 服务器
-debug            调试日志等级：false、error、all
-skipgeo          跳过地区/代理环境验证（启动时不再提示代理警告）
-out              输出文件名（官方/非标通用），默认 cfdata-results
```

非标常用参数：

```text
-nsbfile          本地输入文件
-nsbsourceurl     网络输入 URL
-nsbfallbackport  非标输入缺省端口；不传时随 TLS 自动使用 443/80
-nsbtls           非标是否启用 TLS
-nsbspeedtest     非标测速线程数，0 表示不测速。多 IP 并发影响实际速度，需要准确应设为 1
-nsbresultlimit   非标延迟测试结果上限
-nsbspeedmin      非标测速合格阈值，单位 MB/s
-nsbspeedlimit    非标测速合格结果上限
-relink           链路复验开关（默认开启）；扫描完成时对结果做真实链路验证，不可用的候选将被剔除
-relinkhost       链路复验探针域名；默认使用内置公共探针，配合 -relink 开关使用
-proxy            管理面代理地址（支持 socks5://、socks5h://、http://、https://）；下载 ASN 库/locations、版本检查、GitHub/订阅上传走此代理，扫描测速不受影响；留空走系统环境变量代理
```

完整参数可运行：

```bash
./cfdata-linux-amd64 -h
```

## 本地缓存

Web 右上角设置菜单提供“恢复全部默认配置”，会清理本地缓存文件，例如 `ips-v4.txt`、`ips-v6.txt`、`locations.json`、ASN 数据库等。任务运行中不会直接清理，避免影响测试。

## 免责声明

本程序仅限用于学习与研究目的。请在下载后24小时内自行删除。使用本程序时，应自行遵守所在地区的法律法规。作者不对使用本程序所产生的任何后果承担责任。下载或使用本程序即视为已阅读、理解并同意上述声明。

## 致谢

- TG 频道：[CF中转IP](https://t.me/CF_NAT)
- GitHub：[Kwisma/iptest](https://github.com/Kwisma/iptest)

## License

Copyright (C) 2026 PoemMisty

This project is licensed under the GNU General Public License v3.0 or later.
See the LICENSE file for details.
