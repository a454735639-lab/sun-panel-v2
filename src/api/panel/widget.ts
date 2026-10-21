import { get, post } from '@/utils/request'

/**
 * 小组件（Widget）API。
 *
 * 后端 lib/wgt 已实现 4 种小组件的数据抓取与缓存
 * （qBittorrent / Transmission / Emby / Jellyfin），
 * 本模块与配套的 useWidgets hook 负责把它接到界面上。
 *
 * 设计取舍：抓取全部在后端完成，前端只渲染结果。
 * 原因：这些服务都要带凭据访问（API Key / 账号密码），
 * 若放在前端，凭据会暴露在浏览器里，且会遇到跨域限制。
 * 后端抓取还能顺带做缓存（20 秒 TTL），多个客户端共享。
 */
const api = {
  types: '/widget/types',
  batch: '/widget/batch',
}

/** 小组件类型元信息 */
export interface WidgetTypeInfo {
  value: string
  label: string
  /** 该类型需要的配置字段名（如 url / apikey / username / password） */
  fields: string[] | null
  hint: string
}

/** 单项指标 */
export interface WidgetStat {
  label: string
  value: string
  /** good / warn / bad / muted —— 决定数值的配色 */
  tone?: string
}

/** 进度条 */
export interface WidgetProgress {
  value: number
  label: string
}

/** 小组件输出 */
export interface WidgetResult {
  ok: boolean
  error?: string
  text?: string
  stats?: WidgetStat[]
  detail?: string[][]
  progress?: WidgetProgress
}

/** 批量查询的返回项（额外带上 cardId 用于关联卡片） */
export interface WidgetItem extends WidgetResult {
  cardId: number
}

/**
 * 获取可用的小组件类型。
 * 字段与提示都由后端提供，前端不写死 —— 后端加新类型时前端无需改动。
 */
export const getWidgetTypes = () => {
  // 必须用 get() 而不是默认导出（= post）：
  // 后端该路由是 api.GET("/widget/types")，用 POST 会 404。
  //
  // 早先这里写的是 `request({ method: 'get' })` —— 看起来对，但
  // `src/utils/request/index.ts` 里判断的是 `method === 'GET'`（大写），
  // 小写的 'get' 匹配不上，于是走了 post 分支 → 404。
  // 直接用专用的 get() 出口，不再依赖字符串大小写是否写对。
  return get<WidgetTypeInfo[]>({
    url: api.types,
    // 类型清单是静态的，失败时不该打扰用户（编辑器里会提示重试）
    silent: true,
  })
}

/**
 * 批量获取卡片的小组件数据。
 *
 * 固定 silent：这是后台轮询，服务连不上时不该弹提示
 * （与 Docker 状态同样的理由 —— 否则提示会持续堆叠）。
 */
export const getWidgetBatch = (cardIds: number[]) => {
  return post<{ list: WidgetItem[] }>({
    url: api.batch,
    data: { cardIds },
    silent: true,
  })
}