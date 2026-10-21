import { post } from '@/utils/request'

export function getList<T>() {
  return post<T>({
    url: '/file/getList',
  })
}

export function deletes<T>(ids: number[]) {
  return post<T>({
    url: '/file/deletes',
    data: { ids },
  })
}

/**
 * 公共图库：所有账号共享的图片。
 *
 * 仍要求登录 —— 它是「站内所有账号都能用」，不是「对互联网公开」。
 */
export function getPublicList<T>() {
  return post<T>({
    url: '/file/publicList',
  })
}

/**
 * 把自己上传的文件设为共享 / 取消共享。
 *
 * 后端会带上 user_id 条件，只能操作自己的文件 ——
 * 否则任何用户都能把别人的图"共享"出去，等于变相公开他人的私人图片。
 */
export function setPublic<T>(ids: number[], isPublic: 0 | 1) {
  return post<T>({
    url: '/file/setPublic',
    data: { ids, isPublic },
  })
}

/**
 * 设置文件的用途分类：0 未分类 / 1 壁纸 / 2 图标。
 *
 * 支持批量 —— 用户常会一次圈选多张一起归类。
 * 后端带 user_id 条件，只能操作自己的文件。
 */
export function setUsage<T>(ids: number[], usage: 0 | 1 | 2) {
  return post<T>({
    url: '/file/setUsage',
    data: { ids, usage },
  })
}
