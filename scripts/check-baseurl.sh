#!/bin/sh
# 校验前端产物中的 axios baseURL 不为空。
#
# 用法: sh scripts/check-baseurl.sh [web根目录]     默认 /app/web
#
# 为什么需要这个检查：
#
#   曾发生一次真实事故 —— .dockerignore 用 !.env 放行了 .env，
#   但 Dockerfile 的 web-build 阶段漏了 COPY .env，导致构建时
#   VITE_GLOB_API_URL 缺失，axios 的 baseURL 变成空字符串。
#   前端请求于是打到 /panel/... 而不是 /api/panel/...，全部 404，
#   界面上只显示「网络错误」—— 排查时极易误判为"网络问题"或
#   后端故障，实际是构建期环境变量没进去。
#
#   仅检查"文件存在"抓不到这类问题（文件都在，值却是错的），
#   必须读取产物里的实际值。
#
# 放在独立脚本而非内联在 workflow 里，原因有二：
#   1. 内联时 shell 与 YAML 的引号转义会互相干扰（实测踩过坑）；
#   2. 独立文件可在本地直接跑，便于改动后立即验证。
#
# 路径映射注意：入口页引用的是 URL 路径 /v2/assets/xxx.js，
# 而容器内文件在 /app/web/assets/xxx.js —— /v2 是 URL 前缀，不是目录。
# 因此拼接前必须去掉 /v2，否则会去找 /app/web/v2/assets/… 而误判。

set -e

WEBROOT="${1:-/app/web}"

if [ ! -f "$WEBROOT/index.html" ]; then
  echo "找不到入口页: $WEBROOT/index.html"
  exit 1
fi

entry=$(grep -oE '/v2/assets/index-[A-Za-z0-9_-]+\.js' "$WEBROOT/index.html" | head -1)
if [ -z "$entry" ]; then
  echo "入口页中找不到 /v2/assets/index-*.js 引用"
  exit 1
fi

rel=${entry#/v2}                 # /assets/index-xxx.js
f="$WEBROOT$rel"

if [ ! -f "$f" ]; then
  echo "找不到入口 JS: $f"
  exit 1
fi

# 空值检测：用 -F 固定串匹配，避免正则转义带来的歧义。
# terser 会保留字符串字面量的引号形态，所以空值就是 baseURL:""。
if grep -qF 'baseURL:""' "$f"; then
  echo "baseURL 为空 —— .env 未参与构建（VITE_GLOB_API_URL 缺失）"
  echo "  受影响文件: $f"
  exit 1
fi
# 单引号形态（当前构建不会产生，但换打包器/压缩器后可能出现）
if grep -qF "baseURL:''" "$f"; then
  echo "baseURL 为空（单引号形态）"
  echo "  受影响文件: $f"
  exit 1
fi

# 取值回显，便于在 CI 日志里直接确认
val=$(sed -n 's/.*\(baseURL:"[^"]*"\).*/\1/p' "$f" | head -1)
if [ -z "$val" ]; then
  echo "产物中未找到 baseURL 字段 —— 构建可能异常"
  exit 1
fi

echo "baseURL 校验通过：$val"
