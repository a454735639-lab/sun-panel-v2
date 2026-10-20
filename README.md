<div align=center>

<img src="./doc/images/logo.png" width="100" height="100" />

# Sun-Panel-V2

基于 [Sun-Panel](https://github.com/hslr-s/sun-panel) 修改的 NAS 导航面板，**服务端已重写为 SSR 架构**。

一个服务器 / NAS 导航面板、Homepage、浏览器首页、书签。

个人自用版本，持续更新完善。如果对你有用，欢迎点亮右上角星星 ⭐

</div>

---

## ✨ 功能特性

### 导航面板
- **智能内外网切换**：自动获取公网 IP（多源容灾，每 10 分钟更新），
  按客户端网段与 NAT 回环自动选择内外网地址，无需配置 pingUrl
- **地址规则匹配**：支持通配符与端口的多规则逐条匹配，
  可为不同站点指定不同打开地址（如「这个域名内网走 A、外网走 B」）
- **卡片分组与拖拽排序**：前端排序后批量事务保存，杜绝并发写入冲突与序号重复
- **两种卡片样式**：详情横卡 / 图标方卡；文字颜色随卡片背景亮度自动取黑白
- **图标支持**：URL 外链、纯透明、在线图标库、上传图片、图库管理
- **搜索栏**：可搜索站点与搜索引擎
- **右键菜单**：快速打开内网 / 外网地址、复制链接、编辑

### 书签与便签
- **书签管理**：参考 Chrome 的书签管理交互，支持**导入浏览器书签**，与首页导航分开
- **便签**：方便多端临时同步文本与文件，手机→电脑传个链接不用再折腾微信

### 系统能力
- **多账号 + 数据隔离**：每个账号的分组、卡片、书签、便签相互独立
- **Docker 容器状态**：卡片上显示容器状态灯、CPU / RAM 占用、运行计数
- **导入导出**：JSON 备份，含**版本校验**（备份版本高于当前会拒绝，避免静默丢数据）
- **退出登录 / 会话管理**
- **响应式**：PC 与移动端自适应

### 工程特性
- **单二进制**：25 MB 静态编译，镜像内仅 3 个文件，**无需 Node 构建**
- **SSR 直出**：服务端渲染 HTML，**JS 加载失败也能正常浏览**
- **多架构**：`linux/amd64` + `linux/arm64`，NAS 上自动匹配
- **测试覆盖**：核心逻辑（规则匹配、版本校验）有单元测试

---

## 🚀 部署

### Docker Compose（推荐）

```yml
services:
  sunpanel-v2:
    image: ghcr.io/a454735639-lab/sun-panel-v2:dev
    container_name: sunpanel-v2
    restart: unless-stopped
    ports:
      - "3002:3002"
    environment:
      TZ: Asia/Shanghai
      # 管理员账号（仅首次初始化时生效）
      SUNPANEL_ADMIN_USERNAME: admin
      SUNPANEL_ADMIN_PASSWORD: 改成你自己的密码
    volumes:
      # 数据统一持久化到 /data（配置、数据库、上传文件、运行期）
      - ./data:/data
      # 需要面板显示 Docker 容器状态时挂载（可选）
      - /var/run/docker.sock:/var/run/docker.sock:ro
```

```bash
docker compose up -d
```

打开 `http://你的IP:3002` 登录。

### 环境变量

| 变量 | 说明 | 默认 |
|---|---|---|
| `SUNPANEL_ADMIN_USERNAME` | 初始管理员账号 | `admin` |
| `SUNPANEL_ADMIN_PASSWORD` | 初始管理员密码 | `123456` |
| `TZ` | 时区 | `UTC` |

> ⚠️ 管理员账号**仅在数据库里一个用户都没有时**创建。
> 已有用户时这两个变量会被忽略 —— 因此**改它不会重置现有密码**。
> 要重置密码需删除 `data/database` 后重建容器。

### Docker Run

```bash
docker run -d \
  --name sunpanel-v2 \
  --restart unless-stopped \
  -p 3002:3002 \
  -e TZ=Asia/Shanghai \
  -e SUNPANEL_ADMIN_PASSWORD=你的密码 \
  -v $(pwd)/data:/data \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  ghcr.io/a454735639-lab/sun-panel-v2:dev
```

### 镜像标签

| 标签 | 更新时机 |
|---|---|
| `dev` | main 分支每次推送自动构建（**当前只有这个**） |
| `latest` | 推送 `v*` 标签时构建 |

> `latest` 需仓库打 `v*` tag 才会生成。目前建议直接用 `dev`。

### 数据目录

所有持久化数据都在挂载的 `/data` 下：

```
data/
├── conf/       配置文件（conf.ini）
├── database/   数据库（database.db，含账号与卡片）
├── uploads/    上传的图标与图片
└── runtime/    运行期文件
```

**升级只需替换镜像，数据不会丢。**

---

## 🛠 本地开发

需要 Go 1.21+。

```bash
# 后端（SSR 服务）
cd service
go build ./...
go test ./...          # 跑单元测试
go run main.go         # 默认监听 :3002
```

首次启动会自动生成 `conf/conf.ini` 并创建默认管理员。

---

## 📦 与原版的差异

本项目 fork 自 [75412701/sun-panel-v2](https://github.com/75412701/sun-panel-v2)（上游），
**服务端已从 Vue SPA 重写为 Go SSR**：

| | 上游原版 | 本项目 |
|---|---|---|
| 前端形态 | Vue 3 + Vite，浏览器端渲染 | **Go 模板 + htmx，服务端渲染** |
| 部署产物 | 二进制 + `/app/web` 前端产物 | **单个二进制，无前端产物** |
| 构建依赖 | 需 Node + pnpm | **仅 Go** |
| 未启用 JS | 白屏 | **可正常浏览** |

**因此两者的数据目录布局不同，不能混用镜像与 compose 配置：**

| | 上游原版 | 本项目 |
|---|---|---|
| 工作目录 | `/app` | `/app` |
| 持久化 | `/app/conf`、`/app/uploads`、`/app/database` | **统一 `/data`** |

> ⚠️ **从原版迁移**：原版的数据在 `/app/conf` 等分散目录，
> 本项目统一收到 `/data`。迁移时把原版的
> `conf/`、`database/`、`uploads/` 拷进新挂载的 `data/` 对应子目录即可。

---

## ❤️ Thanks

- [红烧猎人](https://blog.enianteam.com/u/sun/content/11) —— 上游 Sun-Panel 作者
- [75412701](https://github.com/75412701/sun-panel-v2) —— 本项目的直接上游

---

[![Star History Chart](https://api.star-history.com/svg?repos=a454735639-lab/sun-panel-v2&type=Date)](https://star-history.com/#a454735639-lab/sun-panel-v2&Date)# 触发 CI Tue Oct 20 10:39:57 PM CST 2026
