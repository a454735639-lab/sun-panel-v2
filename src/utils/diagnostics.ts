/**
 * 前端诊断日志收集器。
 *
 * 存在的意义：面板在用户环境里出错时，开发者看不到现场 ——
 * 只能靠"我这边不复现"来回猜。这个模块把**失败请求**与 **JS 异常**
 * 记录下来，用户可在「设置 → 诊断日志」里一键复制并反馈。
 *
 * 设计取舍：
 *
 * 1. 只记失败，不记成功
 *    成功请求量级大（轮询每 2-5 秒一次），全记会淹没真正有用的信息，
 *    也会把内存吃满。失败才是诊断对象。
 *
 * 2. 环形缓冲，上限固定
 *    超过上限丢弃最旧的。避免长时间运行导致内存无限增长 ——
 *    这东西要一直挂在页面上，不能成为新的性能问题。
 *
 * 3. 记录"是否静默请求"与"是否来自轮询"
 *    这是区分问题的关键：用户主动操作失败 → 该弹提示；
 *    后台轮询失败 → 不该弹。有了这个标记，一眼就能看出
 *    某条错误到底该不该出现在界面上。
 *
 * 4. 复制为纯文本而非 JSON
 *    用户是在手机上操作，JSON 难读也难核对。纯文本行格式
 *    可以直接贴进聊天窗口。
 *
 * 5. 主动脱敏
 *    token 等凭据绝不能进日志 —— 用户可能把日志公开贴出来。
 */

export type LogKind = 'http' | 'js' | 'unhandled'

export interface ErrorLogItem {
  /** 记录时间（本地时间字符串，便于用户对照操作时刻） */
  time: string
  /** 类别：HTTP 失败 / JS 异常 / 未处理的 Promise 拒绝 */
  kind: LogKind
  /** 一句话摘要 */
  summary: string
  /** 详细信息（多行） */
  detail: string
  /** 请求方法（仅 http） */
  method?: string
  /** 请求 URL（仅 http） */
  url?: string
  /** HTTP 状态码（仅 http） */
  status?: number
  /** 业务错误码（仅 http） */
  code?: number
  /** 是否为静默请求（仅 http）—— 静默请求失败本不应弹提示 */
  silent?: boolean
  /** 错误发生时所在的页面路由 */
  route: string
}

const MAX_ITEMS = 200

const items: ErrorLogItem[] = []
let listeners: Array<() => void> = []

/** 记录一条日志。超过上限时丢弃最旧的。 */
export function pushLog(item: ErrorLogItem) {
  items.push(item)
  if (items.length > MAX_ITEMS)
    items.splice(0, items.length - MAX_ITEMS)
  // 通知订阅者（诊断页面用）
  for (const fn of listeners) {
    try {
      fn()
    }
    catch { /* 监听器自身出错不应影响记录 */ }
  }
}

export function getLogs(): ErrorLogItem[] {
  return items
}

export function clearLogs() {
  items.length = 0
  for (const fn of listeners) {
    try {
      fn()
    }
    catch { /* 同上 */ }
  }
}

/** 订阅变化，返回取消订阅函数 */
export function subscribeLogs(fn: () => void) {
  listeners.push(fn)
  return () => {
    listeners = listeners.filter(x => x !== fn)
  }
}

/** 取当前页面路由（用于判断错误发生在哪个页面） */
function currentRoute(): string {
  try {
    return window.location.hash || window.location.pathname || '-'
  }
  catch {
    return '-'
  }
}

/** 时间戳：HH:mm:ss，便于用户对照"我几点点的" */
function nowStr(): string {
  const d = new Date()
  const p = (n: number) => String(n).padStart(2, '0')
  return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

/**
 * 脱敏：抹掉可能出现的凭据。
 *
 * 必须做这一步 —— 用户会把这段文本贴到聊天/issue 里，
 * 若带出 token 等于泄露账号。
 */
function scrub(s: string): string {
  if (!s)
    return ''
  return s
    // JWT / 长 token
    .replace(/eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{6,}/g, '<jwt>')
    // Bearer xxx
    .replace(/(Bearer\s+)[A-Za-z0-9._-]{8,}/gi, '$1<token>')
    // token=xxx / "token":"xxx"
    .replace(/(["']?token["']?\s*[:=]\s*["']?)[A-Za-z0-9._-]{8,}/gi, '$1<token>')
    // 密码字段
    .replace(/(["']?password["']?\s*[:=]\s*["']?)[^"'&,\s}]{3,}/gi, '$1<password>')
}

/** 截断过长的响应体，避免日志被一大段 HTML 撑爆 */
function truncate(s: string, n = 300): string {
  if (!s)
    return ''
  const t = s.replace(/\s+/g, ' ').trim()
  return t.length > n ? `${t.slice(0, n)}…（共 ${t.length} 字符）` : t
}

/** 记录一次 HTTP 失败 */
export function logHttpFailure(opts: {
  method: string
  url: string
  status?: number
  code?: number
  silent?: boolean
  respText?: string
  errorText?: string
}) {
  const { method, url, status, code, silent, respText, errorText } = opts

  // 摘要要能一眼看出"谁失败了、怎么失败的"
  let summary = `${method.toUpperCase()} ${url}`
  if (status)
    summary += ` → HTTP ${status}`
  if (code !== undefined)
    summary += `${status ? '' : ' →'} code=${code}`
  if (silent)
    summary += '（静默请求）'

  const detailLines = [
    `URL: ${url}`,
    `方法: ${method.toUpperCase()}`,
    status ? `HTTP 状态: ${status}` : '',
    code !== undefined ? `业务码: ${code}` : '',
    `静默请求: ${silent ? '是（本不应弹提示）' : '否（会弹提示）'}`,
    errorText ? `异常: ${truncate(scrub(errorText), 200)}` : '',
    respText ? `响应: ${truncate(scrub(respText))}` : '',
  ].filter(Boolean)

  pushLog({
    time: nowStr(),
    kind: 'http',
    summary,
    detail: detailLines.join('\n'),
    method,
    url,
    status,
    code,
    silent,
    route: currentRoute(),
  })
}

/** 记录 JS 运行时异常 */
export function logJsError(message: string, stack?: string) {
  pushLog({
    time: nowStr(),
    kind: 'js',
    summary: truncate(scrub(message), 160),
    detail: [scrub(message), stack ? `堆栈:\n${scrub(stack)}` : ''].filter(Boolean).join('\n'),
    route: currentRoute(),
  })
}

/** 记录未处理的 Promise 拒绝（常被忽略，但往往是"静默失败"的真因） */
export function logUnhandledRejection(reason: any) {
  const text = typeof reason === 'string'
    ? reason
    : (reason && (reason.message || reason.msg)) || String(reason)
  // 带上堆栈 —— 只有 message 定位不到是哪一行，
  // 而堆栈是唯一能指出"哪段代码抛的"的线索（实测踩过：
  // 一条 "Cannot read properties of undefined (reading 'id')"
  // 光看 message 完全无从下手）。
  const stack = reason && typeof reason === 'object' ? (reason.stack || '') : ''
  pushLog({
    time: nowStr(),
    kind: 'unhandled',
    summary: truncate(scrub(text), 160),
    detail: [scrub(text), stack ? `堆栈:\n${scrub(stack)}` : ''].filter(Boolean).join('\n'),
    route: currentRoute(),
  })
}

/**
 * 安装全局错误捕获。
 *
 * 除了 axios 失败，JS 运行时异常与未处理的 Promise 拒绝同样重要 ——
 * 后者经常是"界面没反应但控制台有报错"的真正原因，而用户
 * 在手机上根本看不到控制台。
 */
export function installGlobalErrorCapture() {
  if (typeof window === 'undefined')
    return
  // 防止热更新或重复调用导致重复安装
  if ((window as any).__spDiagInstalled)
    return
  ;(window as any).__spDiagInstalled = true

  window.addEventListener('error', (e) => {
    // 资源加载失败（img/script）也会触发 error 事件，但无 message。
    // 这类信息价值较低，且面板上图片多，容易刷屏 —— 只记有 message 的。
    if (!e.message && !e.error)
      return
    logJsError(e.message || String(e.error), e.error?.stack)
  })

  window.addEventListener('unhandledrejection', (e) => {
    logUnhandledRejection(e.reason)
  })
}

/** 导出为可直接粘贴的纯文本 */
export function exportAsText(): string {
  const list = getLogs()
  const head = [
    '=== Sun-Panel 前端诊断日志 ===',
    `导出时间: ${new Date().toLocaleString()}`,
    `页面: ${typeof window !== 'undefined' ? window.location.href : '-'}`,
    `日志条数: ${list.length}（上限 ${MAX_ITEMS}，仅记录失败与异常）`,
    `浏览器: ${typeof navigator !== 'undefined' ? navigator.userAgent : '-'}`,
    '',
  ]

  if (!list.length) {
    return `${head.join('\n')}\n（本次会话未记录到任何失败或异常）`
  }

  const body = list.map((it, i) => {
    const tag = it.kind === 'http' ? 'HTTP' : (it.kind === 'js' ? 'JS异常' : 'Promise拒绝')
    return [
      `#${i + 1} [${it.time}] [${tag}] ${it.route}`,
      it.detail,
      '',
    ].join('\n')
  })

  return head.join('\n') + body.join('\n')
}