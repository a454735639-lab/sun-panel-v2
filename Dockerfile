# ==========================================================================
# Sun-Panel-V2 —— 纯 Go 单二进制镜像（无 node / 无前端构建链）
#
# 设计说明：
#   1. 无 node 阶段
#      Vue 产物（dist）在服务端已无任何引用（详见 README「收尾方案」）；
#      页面模板与静态资源由 service/webrender 里的
#      //go:embed templates / //go:embed static 打进二进制，
#      因此运行期不需要任何外部静态资源目录。
#
#   2. CGO_ENABLED=0
#      service/initialize/database/connect.go 实际使用
#      github.com/glebarez/sqlite（纯 Go 驱动），
#      go.sum 里的 mattn/go-sqlite3 仅为传递依赖。
#      因此可关闭 cgo —— 这不只省掉 gcc/musl-dev，
#      也解除了「必须锁 alpine3.18、否则 sqlite3 编译失败」的限制。
#
#   3. 原生交叉编译（替代 QEMU 模拟）
#      --platform=$BUILDPLATFORM 让构建阶段跑在 runner 的原生架构上，
#      由 Go 自己交叉编译出 TARGETARCH 产物。
#      linux/arm64 一路不再需要 QEMU 模拟编译（NAS 场景的关键加速项）。
#
#   4. 多阶段 + 最小运行镜像
#      运行阶段仅 alpine + 静态二进制，无需 bash/su-exec。
# ==========================================================================

# ---------- 构建阶段：纯 Go ----------
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