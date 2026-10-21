import { post } from '@/utils/request'

/**
 * Docker 容器管理 API。
 *
 * 与 src/api/panel/docker.ts 的分工：
 *   - docker.ts        —— 只做「卡片状态轮询」（按 containerId 批量查状态）
 *   - 本文件           —— 管理能力：列出全部容器、启停/重启
 *
 * 为什么要拆：状态轮询是高频只读、面向已绑定的容器；
 * 管理是低频写操作、面向全部容器。两者的调用时机与鉴权要求都不同
 * （轮询任何登录用户都跑；启停只有管理员能用）。
 */
const api = {
  list: '/panel/docker/list',
  action: '/panel/docker/action',
}

/** 容器信息（对应后端 dockerApi.ContainerInfo） */
export interface DockerContainer {
  id: string
  name: string
  image: string
  state: string
  status: string
  created: number
  running: boolean
  primaryPort?: number
  /** CPU 占用百分比（需 withStats 才回填） */
  cpuPercent?: number
  /** 内存占用字节（需 withStats 才回填） */
  memUsage?: number
}

export interface DockerListResult {
  list: DockerContainer[]
  dockerReady: boolean
  count?: number
}

/**
 * 列出全部容器。
 *
 * withStats=false 时不读 cgroup —— "选择容器"只需要名称与状态，
 * 拉指标会让下拉打开变慢，没有必要。
 */
export const listContainers = (withStats = false) => {
  return post<DockerListResult>({
    url: api.list,
    data: { withStats },
    // 打开编辑器时失败不该弹提示：用户仍可编辑其他字段，
    // 只是容器下拉为空（与小组件类型清单的处理一致）
    silent: true,
  })
}

/**
 * 启停/重启容器。仅管理员可用（后端也会校验）。
 *
 * 这里**不设 silent**：这是用户主动点的操作，
 * 失败必须让他知道（否则会以为容器已经停了）。
 */
export const containerAction = (id: string, action: 'start' | 'stop' | 'restart') => {
  return post<{ ok: boolean; dockerReady?: boolean; msg?: string }>({
    url: api.action,
    data: { id, action },
  })
}
