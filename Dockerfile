# ==========================================================================
# Sun-Panel-V2 —— 双前端镜像
#
# 设计说明：
#   1. 两套前端并存
#      Vue SPA（产物挂 /v2/）与 Go SSR（挂在根路径）同时提供服务，
#      便于对比与回退。Vue 产物打进镜像的 /app/web。
#
#   2. 前端构建阶段不加 GOOS/GOARCH
#      --platform=$BUILDPLATFORM 让前端与 Go 都跑在 runner 的原生架构上，
#      linux/arm64 一路不需要 QEMU 模拟（NAS 场景的关键加速项）。
#      前端产物与目标架构无关，用原生架构构建即可。
#
#   3. CGO_ENABLED=0
#      service/initialize/database/connect.go 使用
#      github.com/glebarez/sqlite（纯 Go 驱动），故无需 cgo，
#      也就不必锁 alpine3.18（那是 cgo 版 sqlite3 的编译限制）。
#
#   4. 多阶段 + 最小运行镜像
#      运行阶段仅 alpine + 静态二进制 + 前端产物，无需 node。
# ==========================================================================

# ---------- 阶段 1：构建 Vue 前端 ----------
FROM --platform=$BUILDPLATFORM node:20-alpine AS web-build

# 国内镜像加速（CI 在境外时可删）
RUN npm config set registry https://registry.npmmirror.com

WORKDIR /web

# 先只拷依赖清单，让依赖安装这层可被缓存复用
COPY package.json package-lock.json ./
# 用 npm ci 而非 install：严格按 lock 文件安装，构建可复现。
# 注意本仓库的 lock 是 package-lock.json（npm），不是 pnpm-lock.yaml。
RUN npm ci --no-audit --no-fund

# 再拷前端源码与配置
# 注意 postcss.config.js 与 tailwind.config.js 必须一并带上：
# postcss 配置里启用了 tailwindcss + autoprefixer，缺配置会导致 vite build 失败。
COPY index.html vite.config.ts tsconfig.json ./
COPY postcss.config.js tailwind.config.js ./
COPY src/ ./src/
COPY public/ ./public/

# base=/v2/ 与后端 router/vueweb.go 的 VueBasePath 保持一致：
# 资源引用会生成为 /v2/assets/...，否则与 SSR 的 /static/* 冲突。
ENV VITE_PUBLIC_BASE=/v2/
RUN npx vite build

# ---------- 阶段 2：构建 Go 后端 ----------
FROM --platform=$BUILDPLATFORM golang:1.21-alpine AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

# 国内模块加速（CI 在境外时可删）
RUN go env -w GO111MODULE=on \
    && go env -w GOPROXY=https://goproxy.cn,direct

# 先只拷依赖清单，让 go mod download 这层可被缓存复用
COPY service/go.mod service/go.sum ./
RUN go mod download

# 再拷服务端全部源码
# 说明：templates/ 与 static/ 由 webrender 的 //go:embed 打包，
#       assets/bindata.go 已随仓库提交，故不再需要构建期执行 go-bindata
#       （去掉一次构建期网络依赖）。若改动了 assets/lang/ 或
#       conf.example.ini，请在本地重新生成 bindata.go 后再提交。
COPY service/ ./

# 生成一份默认配置（用于首次启动）。
# 关键：这一步**不加** GOOS/GOARCH，编译出的是构建阶段的原生架构二进制；
#       若用目标架构编译再执行，arm64 一路会在 amd64 runner 上
#       触发 "exec format error"。因为构建阶段本就跑在 $BUILDPLATFORM，
#       所以这一步完全不需要 QEMU。
RUN CGO_ENABLED=0 go build -o /tmp/sun-panel-gen . \
    && /tmp/sun-panel-gen -config \
    && rm -f /tmp/sun-panel-gen

# 正式产物：静态编译 + 去符号表 + 注入运行模式
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
      go build -trimpath \
        -ldflags="-s -w -X sun-panel/global.RUNCODE=release -X sun-panel/global.ISDOCKER=docker" \
        -o /out/sun-panel .

# ---------- 运行阶段 ----------
FROM alpine:3.21

WORKDIR /app

# curl 供 HEALTHCHECK 使用；tzdata 供时区；ca-certificates 供出网 HTTPS
RUN apk add --no-cache ca-certificates tzdata curl

COPY --from=build /out/sun-panel /app/sun-panel
# 默认配置样例：entrypoint 首次启动时用它生成 conf/conf.ini
COPY --from=build /src/conf/conf.example.ini /app/conf.example.ini
# Vue 前端产物（vite build 输出），由 router/vueweb.go 托管在 /v2/
COPY --from=web-build /web/dist /app/web
# 入口脚本：把 /app/{conf,database,uploads,runtime} 软链到持久卷 /data，
# 使升级镜像不丢数据（与本地 deploy/ 方案保持一致）
COPY deploy/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/sun-panel /app/entrypoint.sh

# 注意：这里**不**预创建 /app/{conf,database,uploads,runtime}。
# entrypoint 需要它们不存在，才能成功建立指向 /data 的软链；
# 若预先创建，软链会失败，数据将无法持久化。

# 持久卷挂载点：compose 里把 ./data 挂到这里
VOLUME ["/data"]

EXPOSE 3002

# 健康检查：/healthz 无需鉴权
HEALTHCHECK --interval=30s --timeout=6s --start-period=15s --retries=3 \
  CMD curl -fsS http://127.0.0.1:3002/healthz || exit 1

ENTRYPOINT ["/app/entrypoint.sh"]