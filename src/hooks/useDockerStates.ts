import { ref, computed, onUnmounted } from 'vue'
import { getContainerStates, type ContainerState } from '@/api/panel/docker'

/**
 * Docker 容器状态轮询。
 *
 * 设计要点：
 *
 * 1. 全局单例轮询，而非每张卡片各自请求
 *    面板上可能有多个 Docker 卡片，若每张卡片独立轮询，
 *    请求数会随卡片数线性增长。这里改为模块级共享一份状态，
 *    由「有卡片时启动、无卡片时停止」的单一计时器驱动。
 *
 * 2. 页面隐藏时暂停
 *    浏览器切到后台后继续轮询毫无意义，还会持续占用 NAS 的 CPU。
 *    监听 visibilitychange，回到前台立即刷新一次（数据可能已过期）。
 *
 * 3. 失败静默重试
 *    容器状态是「锦上添花」的信息，不应因 Docker 未挂载或守护进程
 *    暂时不可用而弹出错误干扰用户。失败时保留上一次数据，下轮重试。
 *
 * 4. 空列表不请求
 *    面板上若没有 Docker 卡片，或卡片未绑定容器，则完全不发请求。
 */

// ---- 模块级共享状态（所有调用方看到同一份） ----
const states = ref<Record<string, ContainerState>>({})
const polling = ref(false)
let timer: ReturnType<typeof setInterval> | null = null
let currentIds: string[] = []
let visibleHandler: (() => void) | null = null

const POLL_MS = 5000

async function poll() {
  if (!currentIds.length) return
  try {
    const res = await getContainerStates(currentIds, true)
    const list = res?.data?.list
    if (Array.isArray(list)) {
      const map: Record<string, ContainerState> = {}
      for (const s of list) map[s.id] = s
      states.value = map
    }
  } catch {
    // 静默：保留上一次数据，下轮重试
  }
}

function start() {
  if (timer) return
  polling.value = true
  poll()
  timer = setInterval(() => {
    // 页面在后台时跳过这一轮
    if (typeof document !== 'undefined' && document.hidden) return
    poll()
  }, POLL_MS)

  // 回到前台立刻刷新：后台期间数据可能已过期很久
  if (!visibleHandler && typeof document !== 'undefined') {
    visibleHandler = () => {
      if (!document.hidden) poll()
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
 * 设置需要监控的容器 ID 集合。
 *
 * 调用方传入当前面板上所有 Docker 卡片绑定的容器 ID（可重复，内部去重）。
 * 列表变化时自动重启轮询；传空数组则停止 —— 避免面板上没有 Docker 卡片
 * 时空转。
 */
export function setWatchedContainers(ids: string[]) {
  const uniq = Array.from(new Set(ids.filter(Boolean)))
  const changed = uniq.length !== currentIds.length ||
    uniq.some((id, i) => id !== currentIds[i])

  if (!changed) return

  stop()
  currentIds = uniq
  // 清掉已不再监控的容器，避免显示过期状态
  if (uniq.length) {
    const kept: Record<string, ContainerState> = {}
    for (const id of uniq) {
      if (states.value[id]) kept[id] = states.value[id]
    }
    states.value = kept
    start()
  } else {
    states.value = {}
  }
}

/** 从 expandParam 中解析容器 ID；非 Docker 卡片返回空串 */
export function parseContainerId(expandParam?: string): string {
  if (!expandParam) return ''
  try {
    const p = JSON.parse(expandParam)
    return typeof p?.containerId === 'string' ? p.containerId : ''
  } catch {
    return ''
  }
}

/** 供组件使用的响应式接口 */
export function useDockerStates() {
  // 组件卸载时若无人再使用，停止轮询。
  // 注意：这里只在卸载时尝试停止，实际是否停止由 setWatchedContainers
  // 的后续调用决定 —— 面板卸载即意味着不再需要轮询。
  onUnmounted(() => {
    // 延迟判断：同一轮渲染中可能还有其他组件在用
    setTimeout(() => {
      if (!document.querySelector('[data-container-id]')) {
        setWatchedContainers([])
      }
    }, 0)
  })

  return {
    states: computed(() => states.value),
    polling: computed(() => polling.value),
    /** 取某个容器的状态，未监控或未取到则为 undefined */
    stateOf: (id: string) => states.value[id],
  }
}