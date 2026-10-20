#!/bin/sh
# Sun-Panel-V2 容器入口脚本
#
# 职责：
#   1. 把 /app 下的运行期目录软链到持久卷 /data，使升级不丢数据
#   2. 首次启动时生成默认配置
#   3. 启动主程序
#
# 注意：本脚本用 POSIX sh（精简镜像里可能只有 sh），因此**不能**使用 bash 专属语法。
#       原先的 `mkdir -p "$DATA_DIR"/{conf,database,...}` 依赖 brace expansion，
#       而这是 bash 特性，sh 不支持——会被当作字面量目录名，
#       在 /data 下生成名为 `{conf,database,uploads,runtime` 的畸形目录。
#       这里改为逐个显式创建，彻底避开该问题。
set -eu

APP_DIR="${SUNPANEL_APP_DIR:-/app}"
DATA_DIR="${SUNPANEL_DATA_DIR:-/data}"

# ---- 1. 准备持久卷目录（显式逐个创建，不用 brace expansion）----
for d in conf database uploads runtime runtime/runlog; do
  mkdir -p "$DATA_DIR/$d"
done

# ---- 2. 把 /app 下的运行期目录软链到持久卷 ----
for d in conf database uploads runtime; do
  src="$APP_DIR/$d"
  dst="$DATA_DIR/$d"
  # 若目标位置存在真实目录（非软链），先移除，否则无法建立软链
  if [ -d "$src" ] && [ ! -L "$src" ]; then
    rm -rf "$src"
  fi
  if [ ! -e "$src" ]; then
    ln -s "$dst" "$src"
  fi
done

# ---- 3. 首次启动生成配置 ----
if [ ! -f "$DATA_DIR/conf/conf.ini" ]; then
  if [ -f "$APP_DIR/conf.example.ini" ]; then
    cp "$APP_DIR/conf.example.ini" "$DATA_DIR/conf/conf.ini"
  elif [ -f "$APP_DIR/defaults/conf/conf.example.ini" ]; then
    cp "$APP_DIR/defaults/conf/conf.example.ini" "$DATA_DIR/conf/conf.ini"
  fi
  if [ -f "$DATA_DIR/conf/conf.ini" ]; then
    sed -i 's/^http_port=.*/http_port=3002/' "$DATA_DIR/conf/conf.ini"
  fi
fi

# ---- 4. 管理员账号提示（仅当设置了环境变量时打印）----
if [ -n "${SUNPANEL_ADMIN_USERNAME:-}" ] || [ -n "${SUNPANEL_ADMIN_PASSWORD:-}" ]; then
  echo "[entrypoint] 检测到管理员账号环境变量：用户名=${SUNPANEL_ADMIN_USERNAME:-admin}"
  echo "[entrypoint] 仅在数据库中没有用户时生效；已有用户则忽略。"
fi

# ---- 5. 启动 ----
cd "$APP_DIR"
exec ./sun-panel