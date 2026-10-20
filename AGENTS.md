# AGENTS.md — 交接文档

> 给接手的人（或 AI agent）看的项目说明。**先读这一页再动手。**

## 这是什么

Sun-Panel 导航面板的**服务端渲染（SSR）重写版**，fork 自
`75412701/sun-panel-v2`（该 fork 又 fork 自 `hslr-s/sun-panel` v1.3.0，MIT 许可）。

**核心变化：前端从 Vue SPA 换成了 Go 模板 + htmx。**

| 项 | 原版（Vue SPA） | 现在（SSR） |
|---|---|---|
| 前端 | 49 个 `.vue` + 82 个 `.ts` | 24 个 `.html` 模板 |
| 构建 | `npm install` + `vite build`（18 分钟） | **无构建步骤** |
| JS 依赖 | 1105 个 npm 包 | htmx + SortableJS（内嵌，共 92 KB） |
| 二进制 | 25 MB | 36.5 MB（纯 Go sqlite） |
| 镜像 | 50 MB | **49.9 MB** |
| 首页传输 | 1.8 KB HTML + 2.5 MB JS | **20.7 KB**（gzip 后） |

**旧的 Vue 前端（`src/`）仍在仓库里但已不参与构建**，可以删除。

## 目录结构

```
service/                    ← 后端（Go module 名仍为 sun-panel）
├── main.go
├── webrender/              ← 【新增】模板渲染层
│   ├── render.go           模板引擎（embed + 磁盘热加载）、FuncMap、静态资源
│   ├── gzip.go             gzip 中间件 + 静态缓存
│   ├── templates/          24 个 .html 模板（内嵌进二进制）
│   └── static/             app.css / app.js / htmx / sortable / 壁纸
├── pages/                  ← 【新增】页面处理器（13 个文件）
│   ├── base.go             CardVM/GroupVM、loadGroups、pickUrl、isLanIP、认证
│   ├── handlers.go         面板、登录、退出
│   ├── hx.go               卡片/分组 CRUD、排序
│   ├── settings.go         系统应用页（8 个 tab 的数据加载）
│   ├── settings_actions.go 设置保存、导出导入、图库、账号、Docker 操作
│   ├── scan.go             扫描 Docker 容器生成卡片
│   ├── widget.go           服务状态小组件接口
│   ├── bookmark.go         书签 + 便签
│   ├── search.go           搜索（卡片/书签/便签）
│   ├── misc.go             局部刷新、内外网切换、图标上传/选择器
│   ├── panelconfig.go      面板外观配置解析
│   ├── ratelimit.go        登录暴力破解防护
│   └── util.go             小工具
├── lib/
│   ├── wgt/                ← 【新增】服务状态小组件
│   │   ├── base.go         Result/Stat 结构、格式化
│   │   ├── qbittorrent.go  qB（支持 API Key 免密）
│   │   ├── transmission.go Transmission（RPC + 409 会话协商）
│   │   ├── emby.go         Emby / Jellyfin
│   │   └── registry.go     类型注册表
│   └── dockerApi/
│       └── client.go       ← 【新增】Docker Engine API（unix socket，无官方 SDK）
├── initialize/
│   └── docker.go           ← 【新增】Docker 客户端初始化
├── models/                 数据模型（已扩展官方 v1.4+ 字段）
├── router/
│   ├── A_ENTER.go          主路由（已改：SPA 静态文件 → InitSSR）
│   └── ssr.go              ← 【新增】所有页面 + htmx 端点注册
└── conf/conf.example.ini

deploy/
├── Dockerfile              多阶段构建（Alpine 基础）
├── entrypoint.sh           /data 持久化软链
└── systemd/
```

## 构建

**关键：必须用纯 Go sqlite，不要用 cgo。**

```bash
cd service
export CGO_ENABLED=0
export GOFLAGS="-p=1 -mod=mod"
go build -o ../sun-panel .
```

依赖 `github.com/glebarez/sqlite`（纯 Go，基于 modernc.org/sqlite）。
**不要改回 `gorm.io/driver/sqlite`**（那个要 cgo，在部分环境编不过）。

首次构建约 20-30 分钟（modernc sqlite 代码量巨大），之后增量约 1 分钟。

**可选：热加载模板**（改模板不用重编）
```bash
SUNPANEL_TEMPLATE_DIR=./webrender/templates \
SUNPANEL_STATIC_DIR=./webrender/static \
./sun-panel
```

## 部署

```bash
# 打包
tar czf deploy.tar.gz sun-panel entrypoint.sh conf.example.ini Dockerfile

# 传到 NAS
scp deploy.tar.gz user@nas:/volume1/docker/sunpanel-ssr/

# 构建镜像
cd /volume1/docker/sunpanel-ssr
DOCKER_BUILDKIT=0 docker build -t sunpanel-ssr:final .

# 启动（3300 端口，挂 docker.sock 才能用 Docker 功能）
docker rm -f sunpanel-ssr
docker run -d --name sunpanel-ssr --restart unless-stopped \
  -p 3300:3002 -e TZ=Asia/Shanghai \
  -v /volume1/docker/sunpanel-v2/data:/data \
  -v /var/run/docker.sock:/var/run/docker.sock \
  sunpanel-ssr:final
```

**`DOCKER_BUILDKIT=0` 必须加**——BuildKit 会强制联网校验基础镜像 manifest，网络不好时构建失败。

## 已实现的功能

**页面**
- `/` 面板（三档卡片尺寸：小 96px / 中 200px / 大 260px，按分组独立配置）
- `/search` 搜索（卡片 + 书签 + 便签，带类型徽章）
- `/bookmarks` 书签（树形，支持浏览器 HTML 导入）
- `/notepad` 便签
- `/settings/{export-import,style,docker,scan,gallery,users,user,about}` 系统应用 8 个 tab

**交互**
- 卡片/分组增删改、拖拽排序
- 右键菜单（打开 / 内网地址 / 外网地址 / 复制链接 / 编辑 / 删除）
- **删除二次确认**：普通操作弹窗确认，**删分组需输入分组名，删账号需输入用户名**
- 图标：URL / 上传 / iconify 搜索（24 万图标）/ 图库选图
- 亮色/暗色主题切换（cookie 记忆，默认跟随系统）

**网络**
- 自动内外网切换：按客户端 IP 判定 + 客户端连通性探测兜底
- 手动切换按钮（自动 / 内网 / 外网）
- 公网 IP 每 2 分钟刷新（NAT 回环判定依赖它）

**监控**
- Docker 容器状态点（绿/灰）、CPU / 内存指标
- 服务状态小组件：qBittorrent / Transmission / Emby / Jellyfin（下载速度、播放会话）

**安全**
- 登录限流：5 次失败锁 15 分钟，键为 `IP|用户名`（避免 NAT 后互相影响）
- Cookie `HttpOnly`，模板自动转义

## 数据模型扩展

在原 fork 基础上加了官方 v1.4+ 的字段（AutoMigrate 会自动加列）：

| 表 | 新增字段 |
|---|---|
| `item_icon` | `card_type`（0 普通 / 2 Docker）、`expand_param`、`widget_type`、`widget_config`、`only_name`、`background_color` |
| `item_icon_group` | `card_style`（JSON：`{"style":0\|1\|2}`） |

**注意**：`BaseModel` 没有 `DeletedAt`（官方移除了），查询**不要**加 `deleted_at IS NULL`。

## 已知问题 / 待办

1. **扫描导入的归类逻辑简化了** —— `pages/scan.go` 的 `serviceClass()` 按镜像名关键词匹配，
   不如原版精细。需要的话可以加更多关键词或支持 `nav.*` Docker label。
2. **没有 i18n** —— 全站中文硬编码。
3. **没做公开访客模式** —— 必须登录才能看面板。
4. **设置页的 scan tab 未验证 Docker 不可用时的降级**。
5. **`src/` 目录（旧 Vue 前端）可以删**，不参与构建。
6. **无自动化测试** —— 只有手工 curl 验证。

## 调试技巧

**看服务端错误**：模板错误会直接显示在页面（`template exec error: ...`）。

**Docker 功能不工作**：看启动日志有没有 `DOCKER MANAGER CONNECTED`，没有说明
socket 没挂或没权限。

**编译报 `cgo: signal: segmentation fault`**：环境问题，不是代码问题。确认
`CGO_ENABLED=0` 且用的是 `glebarez/sqlite`。

**端口占用测到旧版本**：`pkill -f sun-panel` 可能匹配不到，用
`ps aux | grep '[s]un-panel'` 找 PID 再 kill -9。

## 许可

MIT，保留原作者版权 `Copyright (c) 2023 红烧猎人`。
