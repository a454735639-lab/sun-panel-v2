import request from '@/utils/request'

/**
 * Docker 容器状态查询。
 *
 * 用途：Docker 卡片（cardType=2）在面板上显示实时状态灯与 CPU/内存数字。
 *
 * 注意：该端点在后端**同时支持两种鉴权**（见 middleware/token.go 的
 * extractToken）—— 因为 SSR 版用 fetch + Cookie 直接调用，
 * 而这里走 axios 带 Bearer 头。两者都能通，无需特殊适配。
 */
const api = {
  getContainerStates: '/docker/container/getContainerStates',
}

export interface ContainerState {
  id: string
  name: string
  state: string
  status: string
  cpuPercent: number
  memUsage: number
  memLimit: number
  netRx: number
  netTx: number
  running: boolean
}

/**
 * 批量查询容器状态。
 *
 * withStats 为 true 时会附带 CPU/内存数据（需读 cgroup，稍慢），
 * 因此仅在卡片上确实要显示指标时才开启。
 */
export const getContainerStates = (containerIds: string[], withStats = true) => {
  return request<{ list: ContainerState[] }>({
    url: api.getContainerStates,
    method: 'post',
    data: { containerIds, withStats, ts: Date.now() },
  })
}

/** 本地上次查询结果，供多处复用（避免同一轮询周期重复请求） */
let lastStates: ContainerState[] = []
let lastAt = 0

export function setCachedStates(list: ContainerState[]) {
  lastStates = list
  lastAt = Date.now()
}

export function getCachedStates(): { list: ContainerState[]; at: number } {
  return { list: lastStates, at: lastAt }
}