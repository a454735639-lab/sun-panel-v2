import type { AxiosProgressEvent, AxiosResponse, GenericAbortSignal } from 'axios'
import request from './axios'
import { apiRespErrMsg, message } from './apiMessage'
import { t } from '@/locales'
import { useAppStore, useAuthStore } from '@/store'
import { router } from '@/router'

let loginMessageShow = false
export interface HttpOption {
  url: string
  data?: any
  method?: string
  headers?: any
  onDownloadProgress?: (progressEvent: AxiosProgressEvent) => void
  signal?: GenericAbortSignal
  beforeRequest?: () => void
  afterRequest?: () => void
  /**
   * 静默模式：请求失败时不弹全局错误提示，只 reject。
   *
   * 为什么需要它：failHandler 会给**任何**失败弹一个
   * duration=50000 的错误提示。这对用户主动触发的操作是合理的，
   * 但对**后台轮询**是灾难 —— 轮询可能每 2-5 秒一次，
   * 一旦接口不可用（容器未挂载、服务重启中、网络抖动），
   * 提示会不断堆积，把面板糊满，用户感受就是
   * "总是提示网络错误，请稍后重试"，而且关掉一个又冒一个。
   *
   * 更隐蔽的是：调用方即便写了 try/catch 也拦不住 ——
   * 提示在 failHandler 里就已弹出，try/catch 只能接住随后的 reject。
   * 因此必须由请求层提供开关。
   *
   * 约定：后台定时轮询、状态探测一律传 silent: true，
   * 由调用方自己决定降级行为（通常保留上次数据静默重试）。
   */
  silent?: boolean
}

export interface Response<T = any> {
  data: T
  // message: string | null
  // status: string
  msg: string
  code: number
}

function http<T = any>(
  { url, data, method, headers, onDownloadProgress, signal, beforeRequest, afterRequest, silent }: HttpOption,
) {
  const authStore = useAuthStore()
  const appStore = useAppStore()
  const successHandler = (res: AxiosResponse<Response<T>>) => {
    if (res.data.code === 0)
      return res.data

    if (res.data.code === 1001) {
      // 避免重复弹窗
      if (loginMessageShow === false) {
        loginMessageShow = true
        message.warning(t('api.loginExpires'), {
          // message.warning('登录过期', {
          onLeave() {
            loginMessageShow = false
          },
        })
      }

      router.push({ path: '/login' })
      authStore.removeToken()
      return res.data
    }

    if (res.data.code === 1000) {
      router.push({ path: '/login' })
      authStore.removeToken()
      return res.data
    }

    if (res.data.code === 1005) {
      message.warning(res.data.msg)
      return res.data
    }

    if (res.data.code === -1) {
      // message.warning(res.data.msg)
      // router.push({ path: '/login' })
      // authStore.removeToken()
      return res.data
    }

    if (!apiRespErrMsg(res.data))
      return Promise.reject(res.data)
    else
      return res.data
  }

  const failHandler = (error: Response<Error>) => {
    afterRequest?.()
    // 静默模式（后台轮询）不弹提示，见 HttpOption.silent 的说明。
    // 仍然 reject，让调用方能感知失败并自行降级。
    if (!silent) {
      message.error(t('common.networkError'), {
        duration: 50000,
        closable: true,
      })
    }
    throw new Error(error?.msg || 'Error')
  }

  beforeRequest?.()

  method = method || 'GET'

  const params = Object.assign(typeof data === 'function' ? data() : data ?? {}, {})
  if (!headers)
    headers = {}

  headers.token = authStore.token
  headers.lang = appStore.language
  return method === 'GET'
    ? request.get(url, { params, headers, signal, onDownloadProgress }).then(successHandler, failHandler)
    : request.post(url, params, { headers, signal, onDownloadProgress }).then(successHandler, failHandler)
}

export function get<T = any>(
  { url, data, method = 'GET', onDownloadProgress, signal, beforeRequest, afterRequest, silent }: HttpOption,
): Promise<Response<T>> {
  return http<T>({
    url,
    method,
    data,
    onDownloadProgress,
    signal,
    beforeRequest,
    afterRequest,
    silent,
  })
}

export function post<T = any>(
  { url, data, method = 'POST', headers, onDownloadProgress, signal, beforeRequest, afterRequest, silent }: HttpOption,
): Promise<Response<T>> {
  return http<T>({
    url,
    method,
    data,
    headers,
    onDownloadProgress,
    signal,
    beforeRequest,
    afterRequest,
    silent,
  })
}

export default post
