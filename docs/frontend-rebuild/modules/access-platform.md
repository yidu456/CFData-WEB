# 访问认证与 Android 兼容

[返回总览](../README.md) | [数据契约](../data-contracts.md) | [接口索引](../protocol.md)

以下编号记录现有功能/兼容约束；未来建议另行标注。源码位置均相对 CFData-WEB 仓库根目录。

### G10 访问与认证

| 编号 | 功能 | 状态与边界 |
|---|---|---|
| A01 | 条件启用 Web 登录 | Web/后端已有；同时设置启动用户名和密码才启用，否则无需登录 |
| A02 | 用户名/密码登录与错误提示 | Web 已有；表单 POST；后端还支持 JSON 登录请求 |
| A03 | Cookie 会话与过期 | 后端已有；默认 720 分钟；新页面请求/WS 建连仍需要认证 |
| A04 | 未认证页面跳转与 WS 401 | 后端已有；需要区分未登录和普通网络断开，不把前者无限重连当唯一方案 |
| A05 | 退出登录 | 后端已有 `/auth/logout`；当前主页没有退出按钮，不能算已有可见 Web 操作 |

账号、密码、监听地址、会话有效期由启动参数控制；没有现成 Web 管理员增删改/改密码接口。

主要依据：`handleLoginPage`、`handleLoginPost`、`handleLogout`、`requireAuth`、`authEnabled`。

### G11 Android 与响应式兼容

| 编号 | 功能 | 状态与边界 |
|---|---|---|
| H01 | 桌面/移动端布局与横向数据浏览 | Web 已有；新 UI 可替换所有旧结构，但保留所有操作的移动可达性 |
| H02 | Android 内置本地后端启动/加载/错误提示 | 包装层已有；WebView 访问 loopback:13335；不是远程服务控制面 |
| H03 | Android 文件选择 | 包装层已有；系统文件选择器回传 TXT/CSV 等内容 |
| H04 | Android 文本/CSV 保存 | 兼容约束；原生 ACTION_CREATE_DOCUMENT，支持成功/失败提示和 UTF-8 BOM |
| H05 | Android 主题与状态栏同步 | 兼容约束；`CFDataAndroid.isDarkMode()`、`setDarkModeStatusBar(dark)` |
| H06 | 外部链接交给系统打开 | 包装层已有；项目、发行页、IP 详情不应导航到不可信页面后仍暴露原生桥 |
| H07 | 返回键和进程生命周期 | 包装层已有；WebView 可回退；Activity 销毁会终止后端进程，因此不能承诺退出 APK 仍继续后台任务 |

关键约束：Android 当前通过注入脚本替换 **全局 `window.downloadFile(content, nameBase, ext)`** 来实现导出。模块化重构如果删除这个全局函数，必须同时更新原生包装层或保留等效适配入口；只在桌面验证 Blob 下载不够。
