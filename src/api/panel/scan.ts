import request from '@/utils/request'

/**
 * 扫描导入 API。
 *
 * 自动扫描本机 Docker 容器，按镜像名推断服务分类，生成候选卡片。
 * 对着一堆容器手工建卡片很繁琐，这个功能能一次生成全部入口。
 *
 * 分类规则（下载 / 媒体 / 数据库 / 网络运维 / AI …）在服务端维护，
 * 前端只负责展示与勾选。
 */
const api = {
  candidates: '/docker/scan/candidates',
}

/** 扫描出的候选卡片 */
export interface ScanCandidate {
  containerId: string
  name: string
  image: string
  state: string
  port: number
  lanUrl: string
  wanUrl: string
  /** 服务分类，由后端按镜像名推断 */
  group: string
  iconUrl: string
  /** 该容器是否已被导入过（避免重复添加） */
  exists: boolean
}

export interface ScanResult {
  list: ScanCandidate[]
  /** 是否已挂载 docker.sock；false 时前端提示"未检测到 Docker" */
  dockerReady: boolean
}

/**
 * 扫描容器生成候选列表。
 *
 * silent: true —— 这是探测性请求，Docker 未挂载或守护进程不可用时
 * 应安静返回，不该弹错误提示（用户点一下看到"未检测到 Docker"即可）。
 */
export const getScanCandidates = () => {
  return request<ScanResult>({
    url: api.candidates,
    method: 'post',
    silent: true,
  })
}
