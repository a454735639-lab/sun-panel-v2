#!/usr/bin/env sh
set -eu
APP_DIR="${SUNPANEL_APP_DIR:-/app}"
DATA_DIR="${SUNPANEL_DATA_DIR:-/data}"
mkdir -p "$DATA_DIR"/{conf,database,uploads,runtime/runlog}
for d in conf database uploads runtime; do
  src="$APP_DIR/$d"; dst="$DATA_DIR/$d"
  if [ -d "$src" ] && [ ! -L "$src" ]; then rm -rf "$src"; fi
  if [ ! -e "$src" ]; then ln -s "$dst" "$src"; fi
done
if [ ! -f "$DATA_DIR/conf/conf.ini" ]; then
  cp "$APP_DIR/conf.example.ini" "$DATA_DIR/conf/conf.ini"
  sed -i 's/^http_port=.*/http_port=3002/' "$DATA_DIR/conf/conf.ini"
fi
cd "$APP_DIR"
exec ./sun-panel
