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

/**
 * Docker 是否可用（后端明确告知，而非前端猜测）。
 *
 * 为什么需要它：未挂载 docker.sock 时接口会稳定返回空列表，
 * 前端若只看"有没有数据"，无法区分
 *   「Docker 正常但确实没容器」 vs 「根本读不到 Docker」。
 * 前者显示 0/N 是对的，后者会永远停在"状态获取中"——
 * 用户以为再等等就有，实际永远不会。
 *
 * 后端在 data.dockerReady 里直接给出结论（见 pages/docker_states.go）。
 * undefined 表示"还没收到过响应"。
 */
const dockerReady = ref<boolean | undefined>(undefined)

const POLL_MS = 5000

async function poll() {
  if (!currentIds.length) return
  try {
    const res = await getContainerStates(currentIds, true)
    // 后端明确告知 Docker 是否可用，优先采用它
    const ready = res?.data?.dockerReady
    if (typeof ready === 'boolean')
      dockerReady.value = ready

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
  // 换了监控目标就重置可用性判断，等新一轮响应重新确认
  dockerReady.value = undefined
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
    /**
     * 汇总：运行中数量 / 监控总数 / 已拿到状态的容器列表。
     *
     * 供顶部横幅显示「N / M 个容器运行中」——
     * 原作者 SSR 版有这条横幅（见 webrender/templates/panel.html 的
     * docker-banner），Vue 版原先只有每张卡片的小圆点，
     * 用户要一个个数才知道整体情况。
     *
     * 注意 running 只统计"确实拿到状态且 running=true"的容器：
     * 状态还没回来时不该把未监控的算成已停止。
     */
    summary: computed(() => {
      const list = Object.values(states.value)
      return {
        total: currentIds.length,
        sampled: list.length,
        running: list.filter(s => s.running).length,
        /**
         * Docker 不可用（未挂载 docker.sock）。
         *
         * 依据后端的明确信号，而不是"轮询了几次还没数据"这类猜测 ——
         * 猜测会把"启动慢"误判成"不可用"，也会把"确实没容器"误判成"读取中"。
         */
        unavailable: dockerReady.value === false,
        /** 正在获取中：有目标但还没收到任何响应 */
        loading: dockerReady.value === undefined && list.length === 0,
      }
    }),
  }
}