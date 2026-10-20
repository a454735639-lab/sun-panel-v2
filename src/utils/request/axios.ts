import axios, { type AxiosResponse } from 'axios'
import { useAuthStore } from '@/store'
import { logHttpFailure } from '@/utils/diagnostics'

// 扩展 axios 的配置类型，让自定义的 silent 字段合法。
// 它会被原样带到拦截器里 —— 诊断日志据此标出
// "这条失败来自静默请求（本不应弹提示）"。
declare module 'axios' {
  export interface AxiosRequestConfig {
    silent?: boolean
  }
}

const service = axios.create({
  baseURL: import.meta.env.VITE_GLOB_API_URL,
  withCredentials: true,
})

service.interceptors.request.use(
  (config) => {
    const token = useAuthStore().token
    if (token)
      config.headers.Authorization = `Bearer ${token}`
    return config
  },
  (error) => {
    return Promise.reject(error.response)
  },
)

service.interceptors.response.use(
  (response: AxiosResponse): AxiosResponse => {
    if (response.status === 200)
      return response

    // 非 200：说明后端返回了 2xx 之外的状态（如 pages 层的裸 401）。
    // 记录后再抛，让上层错误处理照常执行 —— 埋点不应改变原有行为。
    logHttpFailure({
      method: response.config?.method || '?',
      url: response.config?.url || '?',
      status: response.status,
      silent: (response.config as any)?.silent,
      respText: typeof response.data === 'string'
        ? response.data
        : JSON.stringify(response.data),
    })

    throw new Error(response.status.toString())
  },
  (error) => {
    // 网络层错误与非 2xx 响应都会到这里 —— 这是信息最全的一处埋点：
    // 能同时拿到状态码、响应体与请求配置（含 silent 标记）。
    const cfg = error?.config || {}
    const respData = error?.response?.data

    logHttpFailure({
      method: cfg.method || '?',
      url: cfg.url || '?',
      status: error?.response?.status,
      // 业务码取自响应体：HTTP 200 但业务失败的情况也在这里能看到
      code: respData?.code,
      silent: cfg.silent,
      respText: respData
        ? (typeof respData === 'string' ? respData : JSON.stringify(respData))
        : '',
      errorText: error?.message || '',
    })

    return Promise.reject(error)
  },
)

export default service