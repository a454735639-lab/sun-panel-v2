/**
 * 缓存策略辅助：stale-while-revalidate（先展示旧数据，后台静默刷新）。
 *
 * ## 为什么需要它
 *
 * 原来的实现是「缓存命中就直接 return」：
 *
 *     if (cachedData) {
 *       items.value = cachedData
 *       return            // ← 永不回源
 *     }
 *
 * 好处是打开页面极快（不发请求），但代价严重：
 * **数据永远不会更新**。在另一台设备上加/改了卡片，这台设备
 * 除非手动点「刷新数据」否则一直看不到。多设备用户会认为"同步坏了"。
 *
 * ## 采用的策略
 *
 * 先渲染缓存（保住"秒开"这个核心体验），同时后台发一次请求：
 *   - 请求成功且数据有变化 → 更新界面
 *   - 请求成功但数据相同 → 什么都不做（避免无意义的重渲染导致闪烁）
 *   - 请求失败 → 静默保留缓存（网络抖动不该让用户看到空面板）
 *
 * ## 为什么不用 Dexie / IndexedDB
 *
 * 卡片数据量很小（几十个组的卡片），localStorage 完全够用，
 * 且现有代码已基于它构建。引入 IndexedDB 只是换个存储介质，
 * 解决不了"永不回源"这个真正的问题 —— 那是逻辑问题，不是存储问题。
 * 增加一个依赖却得不到对等的收益。
 *
 * ## 并发控制
 *
 * 首页会为每个分组并发调用刷新。若某分组已在刷新中，
 * 重复调用会共享同一个 Promise，避免同一份数据被请求多次。
 */

/** 判断两份数据是否等价（用于决定是否需要更新 UI） */
export function isSameData(a: unknown, b: unknown): boolean {
  if (a === b)
    return true
  if (a === null || b === null || a === undefined || b === undefined)
    return false

  try {
    // 卡片数据是纯 JSON 结构（无函数/循环引用），序列化比较足够。
    // 键的顺序由后端稳定返回，不会因顺序差异误判为"变化"。
    return JSON.stringify(a) === JSON.stringify(b)
  }
  catch {
    // 极端情况（循环引用）下宁可认为"有变化"，让 UI 更新一次，
    // 也好过漏掉真实更新。
    return false
  }
}

/** 正在进行的刷新任务，key 为缓存键，用于并发合并 */
const inflight = new Map<string, Promise<unknown>>()

/**
 * 合并并发请求：同一个 key 的刷新只发一次。
 *
 * 首页为每个分组各调一次刷新，若用户快速切换页面（onActivated 会重跑），
 * 同一分组可能被连续请求两次。合并后只有第一个真正发出。
 */
export function dedupe<T>(key: string, fn: () => Promise<T>): Promise<T> {
  const existing = inflight.get(key)
  if (existing)
    return existing as Promise<T>

  const p = fn().finally(() => {
    // 完成后清理，允许下次重新请求
    inflight.delete(key)
  })
  inflight.set(key, p)
  return p
}

export interface SwrOptions<T> {
  /** 缓存键（同时用于并发合并） */
  key: string
  /** 读取缓存（同步，返回 null 表示无缓存） */
  readCache: () => T | null
  /** 写缓存 */
  writeCache: (data: T) => void
  /** 从服务端拉取最新数据；返回 null 表示失败（保留缓存） */
  fetchRemote: () => Promise<T | null>
  /** 数据有变化时回调（用于更新 UI） */
  onUpdate: (data: T) => void
  /** 数据未变化时回调（可选，多用于"全部加载完成"这类收尾逻辑） */
  onSettled?: () => void
}

/**
 * 执行 stale-while-revalidate。
 *
 * 若命中缓存：立即 onUpdate(缓存) 让界面先有内容，再后台刷新；
 * 若无缓存：等待远端结果后再 onUpdate。
 *
 * 注意：本函数不阻塞 —— 命中缓存时同步返回，刷新在后台进行。
 */
export function staleWhileRevalidate<T>(opts: SwrOptions<T>): void {
  const { key, readCache, writeCache, fetchRemote, onUpdate, onSettled } = opts

  const cached = readCache()
  if (cached !== null) {
    // 先让界面有内容 —— 这是"秒开"的来源
    onUpdate(cached)
  }

  // 后台刷新。用 dedupe 合并同一 key 的并发请求。
  dedupe(key, fetchRemote)
    .then((remote) => {
      if (remote === null) {
        // 拉取失败：保留缓存，静默收尾
        onSettled?.()
        return
      }

      if (cached !== null && isSameData(cached, remote)) {
        // 数据没变：不触发 onUpdate，避免无意义的重渲染
        onSettled?.()
        return
      }

      writeCache(remote)
      onUpdate(remote)
      onSettled?.()
    })
    .catch(() => {
      // fetchRemote 内部已处理失败，这里兜底防止未捕获的 rejection
      onSettled?.()
    })
}
