import { ref, computed, onUnmounted } from 'vue'
import { getWidgetBatch, type WidgetItem } from '@/api/panel/widget'

/**
 * 小组件数据轮询。
 *
 * 与 useDockerStates 采用同一套设计（那套已在真实环境验证过），
 * 理由相同：
 *
 * 1. 全局单例轮询，而非每张卡片各自请求
 *    面板上可能有多张小组件卡片，各自轮询会让请求数随卡片数线性增长。
 *    这里合并成一次批量请求。
 * 2. 页面隐藏时暂停
 *    切到后台继续轮询毫无意义，还会持续占用 NAS 的 CPU。
 *    回到前台立即刷新一次（数据可能已过期）。
 * 3. 失败静默重试
 *    小组件连的是外部服务（qB / Emby 等），它们离线是常态，
 *    不该因此弹错误提示。失败时保留上一次数据。
 * 4. 空列表不请求
 *    面板上没有小组件卡片时完全不发请求。
 *
 * 与 Docker 版的区别：
 *   - 间隔更长（15 秒）。小组件抓取要访问外部服务，比读本地 docker.sock 慢，
 *     且这类数据（下载进度、媒体库统计）本身变化不快。
 *   - 记录错误信息到 map 中而非直接丢弃，便于卡片上显示"连接失败"。
 */

const results = ref<Record<number, WidgetItem>>({})
const polling = ref(false)
let timer: ReturnType<typeof setInterval> | null = null
let currentIds: number[] = []
let visibleHandler: (() => void) | null = null

const POLL_MS = 15000

async function poll() {
  if (!currentIds.length)
    return
  try {
    const res = await getWidgetBatch(currentIds)
    const list = res?.data?.list
    if (Array.isArray(list)) {
      const map: Record<number, WidgetItem> = {}
      for (const s of list)
        map[s.cardId] = s
      results.value = map
    }
  }
  catch {
    // 静默：保留上一次数据，下轮重试
  }
}

function start() {
  if (timer)
    return
  polling.value = true
  poll()
  timer = setInterval(() => {
    if (typeof document !== 'undefined' && document.hidden)
      return
    poll()
  }, POLL_MS)

  if (!visibleHandler && typeof document !== 'undefined') {
    visibleHandler = () => {
      if (!document.hidden)
        poll()
    }
    document.addEventListener('visibilitychange', visibleHandler)
  }
}

function stop() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
  polling.value = false
  if (visibleHandler && typeof document !== 'undefined') {
    document.removeEventListener('visibilitychange', visibleHandler)
    visibleHandler = null
  }
}

/**
 * 设置需要监控的卡片 ID。
 *
 * 卡片增删时调用；列表不变则不做任何事（避免无谓的重启轮询）。
 */
export function setWatchedWidgetCards(ids: number[]) {
  const uniq = Array.from(new Set(ids.filter(Boolean)))
  const changed = uniq.length !== currentIds.length
    || uniq.some((id, i) => id !== currentIds[i])
  if (!changed)
    return

  stop()
  currentIds = uniq
  if (uniq.length) {
    // 清掉已不再监控的卡片，避免显示过期数据
    const kept: Record<number, WidgetItem> = {}
    for (const id of uniq) {
      if (results.value[id])
        kept[id] = results.value[id]
    }
    results.value = kept
    start()
  }
  else {
    results.value = {}
  }
}

/** 供组件使用 */
export function useWidgets() {
  onUnmounted(() => {
    // 延迟判断：同一轮渲染中可能还有其他组件在用
    setTimeout(() => {
      if (!document.querySelector('[data-widget-card-id]'))
        setWatchedWidgetCards([])
    }, 0)
  })

  return {
    results: computed(() => results.value),
    polling: computed(() => polling.value),
    resultOf: (cardId: number) => results.value[cardId],
  }
}
