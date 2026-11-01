# WebSocket 与关键源码索引

[返回总览](README.md) | [数据契约](data-contracts.md)

## 12. WebSocket 请求全量对照

统一请求结构为 `{type, data}`。下面列的是传输字段名，不是新 UI 的展示文案。

| 请求类型 | 归属 | data 字段 | 后端位置 |
|---|---|---|---|
| `start_task` | G01/G03 | ipType, threads, port, delay, scanMode, autoSpeed, officialTargetDC, officialSpeedPort, officialSpeedURL, officialSpeedMin, officialSpeedLimit | `combined_refactor/server.go:127` |
| `start_test` | G01 | dc, port, delay, scanMode | `combined_refactor/server.go:178` |
| `start_speed_test` | G01/G03 | ip, port, url | `combined_refactor/server.go:198` |
| `start_official_speed_batch` | G03 | port, url, speedMin, speedLimit, results, skipTested | `combined_refactor/server.go:211` |
| `start_nsb_task` | G02 | fileName, fileContent, sourceURL, outFile, maxThreads, fallbackPort, speedTest, speedURL, enableTLS, delay, resultLimit, dc, speedMin, speedLimit, compact, scanMode, relinkHost | `combined_refactor/server.go:233` |
| `start_nsb_reallink` | G02 | results, relinkHost, outFile, speedTest, speedURL, enableTLS, speedMin, speedLimit, compact, scanMode | `combined_refactor/server.go:299` |
| `start_nsb_speed_batch` | G02/G03 | results, speedTest, speedURL, enableTLS, speedMin, speedLimit, skipTested, compact | `combined_refactor/server.go:337` |
| `stop_task` | G04 | 无业务参数 | `combined_refactor/server.go:363` |
| `run_in_background` | G04 | 无业务参数 | `combined_refactor/server.go:374` |
| `follow_background_task` | G04 | 无业务参数 | `combined_refactor/server.go:382` |
| `get_background_task_status` | G04 | 无业务参数 | `combined_refactor/server.go:420` |
| `compact_ipv4` | G09 | 无业务参数 | `combined_refactor/server.go:428` |
| `reset_all_config` | G09 | 无业务参数 | `combined_refactor/server.go:433` |
| `get_speed_url_status` | G03 | 无业务参数 | `combined_refactor/server.go:436` |
| `get_management_proxy` | G09 | 无业务参数 | `combined_refactor/server.go:439` |
| `set_management_proxy` | G09 | value；空字符串表示清除程序代理 | `combined_refactor/server.go:442` |
| `get_config` | G08 | 无业务参数 | `combined_refactor/server.go:472` |
| `check_proxy_country` | G09 | 无业务参数 | `combined_refactor/server.go:486` |
| `github_upload` | G07 | token, owner, repo, branch, path, message, content, silent | `combined_refactor/server.go:496` |
| `edgetunnel_upload` | G07 | host, password, content, mode, silent | `combined_refactor/server.go:520` |

## 13. 前端事件全量对照

事件名和旧接收入口逐一登记；新接收层要覆盖这些事件，不代表继续保留旧 DOM 实现。

| 事件 | 主要归属 | 旧接收位置 |
|---|---|---|
| `init_config` | G03/G04/G08/G09 | `combined_refactor/index.html:6283` |
| `speed_url_status` | G03 | `combined_refactor/index.html:6301` |
| `proxy_status` | G09 | `combined_refactor/index.html:6304` |
| `proxy_updated` | G09 | `combined_refactor/index.html:6307` |
| `version_info` | G09 | `combined_refactor/index.html:6314` |
| `proxy_country_result` | G09 | `combined_refactor/index.html:6317` |
| `config_data` | G08 | `combined_refactor/index.html:6320` |
| `scan_progress` | G01/G04/G05 | `combined_refactor/index.html:6324` |
| `scan_result` | G01/G04/G05 | `combined_refactor/index.html:6328` |
| `scan_complete_wait_dc` | G01/G04/G05 | `combined_refactor/index.html:6332` |
| `test_progress` | G01/G04/G05 | `combined_refactor/index.html:6343` |
| `test_result` | G01/G04/G05 | `combined_refactor/index.html:6349` |
| `test_complete` | G01/G04/G05 | `combined_refactor/index.html:6353` |
| `speed_test_result` | G03 | `combined_refactor/index.html:6375` |
| `official_speed_progress` | G03 | `combined_refactor/index.html:6379` |
| `official_speed_complete` | G03 | `combined_refactor/index.html:6386` |
| `github_upload_result` | G07 | `combined_refactor/index.html:6413` |
| `github_upload_status` | G07 | `combined_refactor/index.html:6422` |
| `github_upload_error` | G07 | `combined_refactor/index.html:6426` |
| `edgetunnel_upload_result` | G07 | `combined_refactor/index.html:6436` |
| `edgetunnel_upload_status` | G07 | `combined_refactor/index.html:6442` |
| `edgetunnel_upload_error` | G07 | `combined_refactor/index.html:6446` |
| `nsb_progress` | G02/G04/G05 | `combined_refactor/index.html:6456` |
| `nsb_scan_result` | G02/G04/G05 | `combined_refactor/index.html:6469` |
| `nsb_scan_sorted` | G02/G04/G05 | `combined_refactor/index.html:6476` |
| `nsb_speed_complete` | G02/G04/G05 | `combined_refactor/index.html:6483` |
| `nsb_csv_ready` | G02/G04/G05 | `combined_refactor/index.html:6499` |
| `nsb_csv_complete` | G02/G04/G05 | `combined_refactor/index.html:6509` |
| `background_task_enabled` | G04 | `combined_refactor/index.html:6541` |
| `background_task_found` | G04 | `combined_refactor/index.html:6542` |
| `background_task_update` | G04 | `combined_refactor/index.html:6543` |
| `background_task_following` | G04 | `combined_refactor/index.html:6548` |
| `background_task_missing` | G04 | `combined_refactor/index.html:6555` |
| `task_stopped` | G04 | `combined_refactor/index.html:6562` |
| `error` | G04 | `combined_refactor/index.html:6571` |
| `reset_config_result` | G09 | `combined_refactor/index.html:6625` |
| `compact_ipv4_progress` | G09 | `combined_refactor/index.html:6628` |
| `compact_ipv4_hit` | G09 | `combined_refactor/index.html:6637` |
| `compact_ipv4_done` | G09 | `combined_refactor/index.html:6646` |
| `log` | G04 | `combined_refactor/index.html:6662` |
| `task_complete` | G04 | `combined_refactor/index.html:6665` |

## 14. 关键源码入口索引

源码位置相对 CFData-WEB 仓库根目录。重构验收应同时看前端入口和后端执行逻辑。

| 归属 | 函数 | 源码位置 |
|---|---|---|
| G01 | `startOfficialTask` | `combined_refactor/index.html:4874` |
| G01 | `maybeStartOfficialAutoTest` | `combined_refactor/index.html:4937` |
| G01 | `showDCSelection` | `combined_refactor/index.html:4476` |
| G01 | `selectDC` | `combined_refactor/index.html:4493` |
| G01 | `runOfficialTask` | `combined_refactor/tasks_official.go:313` |
| G01 | `runDetailedTest` | `combined_refactor/tasks_official.go:453` |
| G02 | `startNSBTask` | `combined_refactor/index.html:4952` |
| G02 | `startNSBReallink` | `combined_refactor/index.html:3914` |
| G02 | `runNSBTask` | `combined_refactor/tasks_nsb.go:646` |
| G02 | `runNSBReallinkTask` | `combined_refactor/tasks_nsb.go:1024` |
| G02 | `parseNSBInputs` | `combined_refactor/utils.go:89` |
| G02 | `resolveHostToIPs` | `combined_refactor/tasks_nsb.go:32` |
| G03 | `resolveStartupSpeedTestURL` | `combined_refactor/speed_url.go:70` |
| G03 | `speedURLStatusPayload` | `combined_refactor/speed_url.go:347` |
| G03 | `runOfficialSpeedBatch` | `combined_refactor/tasks_speed.go:223` |
| G03 | `runNSBSpeedBatch` | `combined_refactor/tasks_nsb.go:1115` |
| G03 | `getSpeedStatus` | `combined_refactor/index.html:4034` |
| G04 | `startTaskNamed` | `combined_refactor/session.go:97` |
| G04 | `beginTask` | `combined_refactor/session.go:184` |
| G04 | `shouldCancelOnDisconnect` | `combined_refactor/session.go:154` |
| G04 | `backgroundSummary` | `combined_refactor/session.go:254` |
| G04 | `handleMessage` | `combined_refactor/index.html:6281` |
| G05 | `openSortModal` | `combined_refactor/index.html:5119` |
| G05 | `applySortModal` | `combined_refactor/index.html:5232` |
| G05 | `getFilteredOfficialTestData` | `combined_refactor/index.html:4552` |
| G05 | `renderScanRowsCSV` | `combined_refactor/index.html:4119` |
| G05 | `renderIPCell` | `combined_refactor/index.html:4062` |
| G06 | `getSelectedCopyRows` | `combined_refactor/index.html:5791` |
| G06 | `getCurrentExportRows` | `combined_refactor/index.html:5924` |
| G06 | `buildResultContent` | `combined_refactor/index.html:6032` |
| G06 | `formatCSVCell` | `combined_refactor/index.html:6058` |
| G06 | `downloadFile` | `combined_refactor/index.html:6271` |
| G07 | `uploadSelectedResultsToGitHub` | `combined_refactor/index.html:6190` |
| G07 | `autoUploadResults` | `combined_refactor/index.html:6226` |
| G07 | `uploadToEdgetunnel` | `combined_refactor/server.go:807` |
| G07 | `saveCurrentGitHubAccount` | `combined_refactor/index.html:5497` |
| G07 | `saveCurrentEdgetunnelAccount` | `combined_refactor/index.html:5630` |
| G08 | `readFormValues` | `combined_refactor/index.html:2803` |
| G08 | `applyConfigValues` | `combined_refactor/index.html:2851` |
| G08 | `mapCLIConfigToForm` | `combined_refactor/index.html:2923` |
| G08 | `autoLoadConfig` | `combined_refactor/index.html:2992` |
| G08 | `applyTheme` | `combined_refactor/index.html:2540` |
| G09 | `setManagementProxy` | `combined_refactor/proxy.go:52` |
| G09 | `detectCloudflareTraceCountry` | `combined_refactor/geo_warning.go:17` |
| G09 | `runCompactIPv4Task` | `combined_refactor/tasks_compact.go:30` |
| G09 | `resetAllConfigFiles` | `combined_refactor/storage.go:81` |
| G09 | `renderVersionInfo` | `combined_refactor/index.html:6697` |
| G10 | `handleLoginPage` | `combined_refactor/auth.go:116` |
| G10 | `handleLoginPost` | `combined_refactor/auth.go:134` |
| G10 | `handleLogout` | `combined_refactor/auth.go:202` |
| G10 | `requireAuth` | `combined_refactor/auth.go:211` |
| G10 | `authEnabled` | `combined_refactor/auth.go:28` |
| G11 | `injectExportBridge` | `app/src/main/java/com/cfdata/web/MainActivity.java:312` |
| G11 | `configureWebView` | `app/src/main/java/com/cfdata/web/MainActivity.java:165` |
| G11 | `saveTextFile` | `app/src/main/java/com/cfdata/web/MainActivity.java:360` |
| G11 | `onDestroy` | `app/src/main/java/com/cfdata/web/MainActivity.java:424` |
| CLI | `runCLIMenu` | `combined_refactor/menu.go:195` |
| CLI | `runScheduleMenu` | `combined_refactor/schedule.go:85` |
| CLI | `configureSchedule` | `combined_refactor/schedule.go:127` |
| CLI | `disableSchedule` | `combined_refactor/schedule.go:268` |
| CLI | `defaultCLIConfigPath` | `combined_refactor/cli.go:950` |
