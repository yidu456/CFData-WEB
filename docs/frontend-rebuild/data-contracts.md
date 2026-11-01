# 结果与状态数据契约

[返回总览](README.md) | [WebSocket 与源码索引](protocol.md)

## 4. 完整结果字段基线

### 4.1 官方扫描结果

`IP`、`Port`、`DataCenter`、`DCCountry`、`Region`、`City`、`LatencyStr`、`TCPDuration`。前端另显示扫描方式。数据中心汇总有 `IPCount`、`MinLatency`。

### 4.2 官方详细结果

`IP`、`Port`、`DataCenter`、`DCCountry`、`Region`、`City`、`MinLatency`、`MaxLatency`、`AvgLatency`、`LossRate`、`Speed`。

- Go `time.Duration` 在此 JSON 中为纳秒数，展示毫秒必须除以 1,000,000。
- `LossRate` 是 0-1 比例，展示百分比必须乘以 100。
- 官方结果中的位置元数据可能来自对应扫描结果，不能只根据当前表格可见行查找。
- `Speed` 既可能是 `12.34MB/s`，也可能是未测速/错误文本；解析与显示不能只按 float 处理。

### 4.3 非标结果

精简 CSV 的 16 项：IP地址、端口号、TLS、丢包率、扫描方式、网络延迟、下载速度、出站IP、IP类型、原始输入、数据中心、源IP位置、地区、城市、ASN号码、ASN组织。

完整 CSV 再加 9 项：访问协议、TLS版本、SNI、HTTP版本、WARP、Gateway、RBI、密钥交换、时间戳，总计 25 项。

实时消息还包含 `speedQualified`。原始输入用于追溯域名/目标来源；出站 IP 不等于输入 IP；源 IP 位置与数据中心落地位置不是同一概念。

`nsbScanMessage` 的端口、延迟、丢包、速度多为字符串，不能沿用官方结果的数值转换。行身份应至少考虑 IP+端口，而不只是 IP。

### 4.4 后台任务快照

标签、模式、阶段、说明、current/total/percent、结果数、测速数、扫描总数/成功/失败、测速总数/成功/失败/达标、详细测试数、数据中心数/说明、是否运行、开始/更新时间、参数摘要。它是摘要，不是完整结果仓库。
