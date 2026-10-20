import { post } from '@/utils/request'

// 说明：下面这几个接口会被 SystemMonitor 组件**定时轮询**（约 2 秒一次），
// 因此统一传 silent: true —— 否则接口一旦不可用，错误提示会每 2 秒
// 弹一个且持续 50 秒，迅速堆满屏幕。这正对应"总是提示网络错误"的现象。
// 注意 failHandler 在 reject 之前就弹提示，所以调用方的 try/catch
// 拦不住，必须在请求层关闭。

export function getAll<T>() {
  return post<T>({
    url: '/system/monitor/getAll',
    silent: true,
  })
}

export function getCpuState<T>() {
  return post<T>({
    url: '/system/monitor/getCpuState',
    silent: true,
  })
}

export function getDiskStateByPath<T>(path: string) {
  return post<T>({
    url: '/system/monitor/getDiskStateByPath',
    data: { path },
    silent: true,
  })
}

export function getMemonyState<T>() {
  return post<T>({
    url: '/system/monitor/getMemonyState',
    silent: true,
  })
}

export function getDiskMountpoints<T>() {
  return post<T>({
    url: '/system/monitor/getDiskMountpoints',
  })
}
