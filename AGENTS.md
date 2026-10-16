# AGENTS.md

## Project Shape

sun-panel-v2 (75412701 fork) is a single-repository Go + Vue application, forked from
`hslr-s/sun-panel` v1.3.0 (MIT, `Copyright (c) 2023 红烧猎人`).

- Backend: Go module `sun-panel`, Gin + Gorm + SQLite/MySQL, sources under **`service/`**
  (`main.go`, `api/`, `router/`, `initialize/`, `models/`, `lib/`, `global/`, `structs/`).
- Frontend: Vue 3 + Vite + TypeScript + Pinia + Naive UI under `src/`.
- Built frontend output is served by the backend from **`./web`** (relative to the
  backend working directory). `dist/` is **not** committed.
- Runtime mutable data lives in `service/conf/`, `service/database/`, `service/uploads/`,
  `service/runtime/`.

### Fork-specific additions over upstream v1.3.0

Bookmark manager (`bookmark`), notepad (`notepad`), custom search engines
(`searchEngine`), public-IP detection + smart LAN/WAN switching (`publicIP`, `ping`),
Weibo hot search (`weiboHotRank`), system settings page, registration page.

This fork has **no PRO features** — there is no Docker container management, no public
gallery, and no backup/migrate module. Do not assume upstream PRO behaviour exists here.

## Hard Constraints

- **Keep the Go module path as `sun-panel`.** Imports and `-ldflags` keys depend on it.
  Do not rename unless doing a full repository-wide migration.
- **The static file directory is hardcoded to `./web`** in
  `service/router/A_ENTER.go` (~line 30). The frontend build output must be copied to
  `service/web/` or the app will serve 404 for `/`.
- **`conf/` must exist before running `./sun-panel -config`**, otherwise the config file
  is not generated.
- Expect historical `Sun-Panel` naming in code, scripts and UI metadata. `package.json`
  name is still `sun-panel` (version `0.0.0`) — the project has **not** been rebranded.
- Gorm uses **singular** table names (`user`, `item_icon`, `bookmark`, ...). New
  persistent models must be appended to `AutoMigrate` in
  `service/initialize/database/connect.go`.
- Do not overwrite user data paths (`conf/conf.ini`, `database/`, `uploads/`,
  `runtime/`) during development or migration work.
- `service/conf/conf.example.ini` is the template for generated config; keep it in sync
  when adding config keys.

### Known repository defect

The repo **commits runtime data** that should never be tracked:

- `service/conf/conf.ini`
- `service/database/database.db`
- `service/runtime/runlog/running.log` (~34 MB)

Back these up and delete them before building or running. Do not commit new copies.

## Build And Verification

### Backend

```bash
cd service
export CGO_CFLAGS="-D_LARGEFILE64_SOURCE -D_GNU_SOURCE"   # musl only, see below
go build -o ../sun-panel-bin \
  -ldflags="-X sun-panel/global.RUNCODE=release -X sun-panel/global.ISDOCKER=docker" .
```

- The release `-ldflags` are **required** for a production build. Without them the binary
  runs in dev mode.
- SQLite goes through `mattn/go-sqlite3`, which needs **cgo**. Install `gcc` + `musl-dev`.
- On musl (Alpine, iSH) the build fails with `'pread64' undeclared` / `unknown type name
  'off64_t'` unless `CGO_CFLAGS="-D_LARGEFILE64_SOURCE -D_GNU_SOURCE"` is set.
- Low-memory hosts (e.g. iSH): use `GOMAXPROCS=1 GOFLAGS="-p=1 -mod=mod" GOGC=30`.
  `sqlite3-binding.c` alone dominates the build time (~20 min on a single slow core;
  a cached rebuild relinking only takes under a minute).

### Frontend

```bash
npm install --no-audit --no-fund     # or pnpm install
npx vite build                       # emits dist/
cp -r dist service/web               # REQUIRED: backend serves ./web
```

- The frontend **does** build cleanly from `src/`. Do not block or disable the Vite
  build; it is the supported workflow in this fork.
- If Vite fails with `Cannot find module @rollup/rollup-linux-arm64-musl`, npm skipped an
  optional native dependency. Fix:
  `npm install --no-save @rollup/rollup-linux-arm64-musl`
- `npm run build` also runs `type-check` (`vue-tsc --noEmit`) and `add-version`; use
  `npx vite build` when you only need the bundle.

### Runtime smoke test

```bash
cd service && mkdir -p conf database runtime/runlog
./sun-panel-bin &
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:3002/          # expect 200
curl -s http://127.0.0.1:3002/api/openness/getDisclaimer                 # expect code 0
```

A fresh database auto-creates 11 tables: `bookmark`, `file`, `item_icon`,
`item_icon_group`, `module_config`, `notepad`, `search_engine`, `sqlite_sequence`,
`system_setting`, `user`, `user_config`.

## Runtime Behaviour

- Startup is `initialize.InitApp()` then `router.InitRouters(":" + http_port)`.
- Default port comes from `[base] http_port` in `conf/conf.ini`, currently `3002`.
- Backend serves static frontend files from `./web` and uploads from
  `[base] source_path` (default `./uploads`).
- SQLite is the default driver; MySQL is also supported.
- Icons are migrated to base64 at startup (`initialize/updateIconBase64.go`).

## API And Backend Conventions

- Routes are mounted under `/api`.
- Add an endpoint by updating **both** the router package (`service/router/system`,
  `service/router/panel`, or `service/router/openness`) and the handler package
  (`service/api/api_v1/...`).
- Use the shared helpers in `service/api/api_v1/common/apiReturn`; the frontend expects
  `{ code, msg, data? }`. Success is `code: 0`.
- Error codes in use (`apiReturn/ErrorCode.go`): `1000` not logged in, `1003` wrong
  credentials, `1004` disabled, `1005` no permission, `1006` no such account, `1007` old
  password wrong, `1200` database error, `1201` keep at least one, `1202` no record,
  `1300` upload failed, `1301` unsupported format, `1400` parameter format error.
- Reuse the middleware in `service/api/api_v1/middleware` for login / admin / public-mode
  checks.
- Keep JSON field names compatible with the typings in `src/typings`.

## Frontend Conventions

- Vue 3 Composition API; use `@/` imports for `src/`.
- Add API wrappers under `src/api/**` using `get`/`post` from `@/utils/request`.
- Request headers automatically carry `token` and `lang`; do not duplicate per call.
- Do not use browser `alert` / `confirm` / `prompt`. Use the Naive UI message,
  notification, dialog or modal patterns.
- Pinia stores live in `src/store/modules/**`.
- Routing uses hash history (`createWebHashHistory`); do not switch history mode without
  matching backend static-serving changes.
- Maintain user-visible strings in both `src/locales/zh-CN.json` and
  `src/locales/en-US.json`. Backend language files are `service/lang/zh-cn.ini` and
  `service/lang/en-us.ini`.

## Assets

`service/assets/bindata.go` is generated from `service/assets/` and committed so builds
are deterministic across architectures. Regenerate with `tools/update-bindata.sh` after
changing anything under `service/assets/`.

## Deployment

- `Dockerfile` builds the frontend and backend in separate stages and copies `dist/` to
  `/app/web`.
- `deploy/entrypoint.sh` initialises `/data/{conf,database,uploads,runtime}` and
  symlinks the app's relative paths to those persistent locations.
- `deploy/systemd/sunpanel-v2.service` is the bare-metal systemd unit.
- Docker deployment persists `/data/*` so upgrades keep user data.

## Change Discipline

- Prefer small, targeted changes that match existing naming and directory boundaries.
- When a request names one exact property, change only that property.
- Preserve backward compatibility for existing config files, database rows, uploaded
  file paths and API response shapes.
- Consider both fresh installs and existing installations before changing schema or seed
  behaviour.
- Avoid broad formatting-only rewrites; the codebase contains mixed historical style.
- Do not commit secrets, local database contents, generated logs or local runtime uploads.
