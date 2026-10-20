# ==========================================================================
# Sun-Panel-V2 Dockerfile —— 纯 Go 单二进制（无 node / 无构建链）
#
# 关键设计：
#   1. CGO_ENABLED=0  —— 用 glebarez/sqlite 纯 Go 驱动，免 cgo，
#                        因此基础镜像可升级、不再需要锁 alpine3.18
#   2. 无 node 阶段    —— Vue 产物在服务端已无引用，
#                        templates/ 与 static/ 由 //go:embed 打进二进制
#   3. 支持多架构       —— 配 --platform=$BUILDPLATFORM 时，
#                        arm64 由 Go 自己交叉编译，无需 QEMU 模拟
#
# 本文件假设 sun-panel 二进制已预编译好放在同目录（NAS 场景推荐，
# 避免在 NAS 这种 ARM 小机器上跑编译）。
# ==========================================================================

FROM alpine:3.21

WORKDIR /app

# bash 供 entrypoint 的可选脚本使用；curl 供 HEALTHCHECK 使用
RUN apk add --no-cache bash ca-certificates tzdata curl

# 预编译的二进制（go build CGO_ENABLED=0 GOOS=linux GOARCH=arm64）
COPY sun-panel /app/sun-panel
COPY entrypoint.sh /app/entrypoint.sh
COPY conf.example.ini /app/conf.example.ini

RUN chmod +x /app/sun-panel /app/entrypoint.sh

# 注意：这里**不**预先创建 /app/{conf,database,uploads,runtime}。
# entrypoint.sh 需要它们不存在，才能建立指向 /data 的软链；
# 若预先创建，软链会失败，数据将无法持久化。

EXPOSE 3002

# 健康检查：命中 /healthz（该端点无需鉴权）
HEALTHCHECK --interval=30s --timeout=6s --start-period=15s --retries=3 \
  CMD curl -fsS http://127.0.0.1:3002/healthz || exit 1

CMD ["/app/entrypoint.sh"]