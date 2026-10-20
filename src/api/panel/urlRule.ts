import request from '@/utils/request'

/**
 * 地址匹配规则 API。
 *
 * 用途：替代"内网/外网二选一"的粗糙切换。
 * 例如"某站点内网走 A、外网走 B、另一域名走 C"。
 *
 * 规则由后端统一存储与匹配（models/UrlRule.match.go），
 * Vue 端与 SSR 端共用同一份数据与同一套匹配算法，行为不会分叉。
 */
const api = {
  list: '/panel/urlRule/list',
  edit: '/panel/urlRule/edit',
  deletes: '/panel/urlRule/deletes',
  test: '/panel/urlRule/test',
}

/** 规则结构（与后端 models.UrlRule 的 JSON 字段对齐） */
export interface UrlRule {
  id: number
  pattern: string
  target: string
  description: string
  /** 1 启用 / 0 停用 */
  enabled: number
  sort: number
}

export const getList = (data: { enabled?: number } = {}) => {
  return request<{ list: UrlRule[]; count: number }>({
    url: api.list,
    method: 'post',
    data,
  })
}

export const edit = (data: Partial<UrlRule>) => {
  return request<{ id: number }>({
    url: api.edit,
    method: 'post',
    data,
  })
}

export const deletes = (data: { ids: number[] }) => {
  return request<{ deleted: number }>({
    url: api.deletes,
    method: 'post',
    data,
  })
}

/** 用真实地址试跑规则，返回命中的规则与最终地址 */
export const test = (data: { url: string }) => {
  return request<{ hit: boolean; target: string; rule?: UrlRule }>({
    url: api.test,
    method: 'post',
    data,
  })
}