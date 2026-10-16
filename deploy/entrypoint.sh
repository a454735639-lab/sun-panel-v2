#!/usr/bin/env sh
set -eu

APP_DIR="${SUNPANEL_APP_DIR:-/app}"
DATA_DIR="${SUNPANEL_DATA_DIR:-/data}"

# Create persistent dirs
mkdir -p "${DATA_DIR}/conf" "${DATA_DIR}/database" "${DATA_DIR}/uploads" "${DATA_DIR}/runtime/runlog"

# Seed the config template on a fresh volume.
# The app itself generates conf/conf.ini from its embedded bindata on first start.
if [ -z "$(ls -A "${DATA_DIR}/conf" 2>/dev/null || true)" ]; then
  if [ -f "${APP_DIR}/defaults/conf/conf.example.ini" ]; then
    cp -a "${APP_DIR}/defaults/conf/conf.example.ini" "${DATA_DIR}/conf/" || true
  fi
fi

# Link the relative paths the app expects (./conf ./database ./uploads ./runtime)
# to the persistent volume. Never delete a real directory: it may be a legacy
# user bind mount that must keep working.
link_dir() {
  src="$1"; dst="$2"
  if [ -L "${src}" ]; then
    rm -f "${src}" 2>/dev/null || true
    ln -s "${dst}" "${src}"
    return
  fi
  if [ -e "${src}" ]; then
    return
  fi
  ln -s "${dst}" "${src}"
}

link_dir "${APP_DIR}/conf"     "${DATA_DIR}/conf"
link_dir "${APP_DIR}/database" "${DATA_DIR}/database"
link_dir "${APP_DIR}/uploads"  "${DATA_DIR}/uploads"
link_dir "${APP_DIR}/runtime"  "${DATA_DIR}/runtime"

exec "$@"
