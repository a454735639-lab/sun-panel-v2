import CryptoJS from 'crypto-js'
import moment from 'moment'

const VERSION = 1 // 当前配置文件版本
const ALLOW_LOW_VERSION = 1 // 最小支持的配置文件版本号
const APPNAME = 'Sun-Panel-Config'

export class FormatError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'FormatError'
  }
}

export class ConfigVersionLowError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'ConfigVersionLowError'
  }
}

export interface JsonStructure {
  version: number
  appName: 'Sun-Panel-Config'
  exportTime: string
  appVersion: string
  icons?: any
  // styleConfig: Panel.panelConfig
  md5: string
}

// 图标
export interface Icon {
  title: string
  sort: number
  icon: Panel.ItemIcon | null
  url: string
  lanUrl: string
  description: string
  openMethod: number
  lanOnly?: number
}

// 图标组
export interface IconGroup {
  title: string
  sort: number
  children: Icon[]
}

/**
 * SSR 版（服务端 /api/config/export）的备份格式。
 *
 * 两套前端的备份格式**互不兼容**：
 *
 *   SSR（后端）                      Vue（前端）
 *   app: "sun-panel-v2"             appName: "Sun-Panel-Config"
 *   exportedAt: RFC3339             exportTime: "YYYY-MM-DD HH:mm:ss"
 *   groups: [...], cards: [...]     icons: [{ ..., children: [...] }]
 *   无 md5                          有 md5
 *   有 migrate 迁移                  无迁移
 *
 * 数据组织方式不同是根本差异：SSR 把「组」和「卡片」平铺成两个数组，
 * 用 item_icon_group_id 关联；Vue 则是组内嵌套 children。
 *
 * 若不做兼容，从 SSR 版迁移过来的用户会发现「导出的备份在新版里
 * 导不进去」，报"文件格式错误" —— 而文件其实是好的。
 */
export interface SsrBackupGroup {
  id?: number
  title?: string
  sort?: number
  [key: string]: any
}

export interface SsrBackupCard {
  id?: number
  itemIconGroupId?: number
  title?: string
  url?: string
  lanUrl?: string
  description?: string
  openMethod?: number
  lanOnly?: number
  sort?: number
  icon?: Panel.ItemIcon | null
  [key: string]: any
}

interface ExportJsonResult {
  addIconsData(datas: IconGroup[]): ExportJsonResult
  exportFile(): void
  string(): string
}

// 导出数据
export function exportJson(appVersion?: string): ExportJsonResult {
  const jsonData: JsonStructure = {
    version: VERSION,
    appName: APPNAME,
    exportTime: moment().format('YYYY-MM-DD HH:mm:ss'),
    appVersion: appVersion || '',
    md5: '',
  }

  // MD5 生成函数
  function generateMD5AndUpdate() {
    jsonData.md5 = generateMD5(JSON.stringify(jsonData))
  }

  return {
    // 添加图标信息
    addIconsData(datas: IconGroup[]) {
      jsonData.icons = datas
      return this
    },

    // 导出json文件
    exportFile() {
      generateMD5AndUpdate()
      const jsonString = JSON.stringify(jsonData)
      if (jsonString) {
        const blob = new Blob([jsonString], { type: 'application/json' })
        const link = document.createElement('a')
        link.href = URL.createObjectURL(blob)
        link.download = `SunPanel-Data${moment().format('YYYYMMDDHHmm')}.sun-panel.json`
        link.click()
      }
    },

    // 返回字符串
    string() {
      generateMD5AndUpdate()
      return JSON.stringify(jsonData)
    },
  }
}

export interface ImportJsonResult {
  isPassCheckMd5: () => boolean
  isPassCheckConfigVersionOld: () => boolean // 配置（数据）的版本是否过旧
  isPassCheckConfigVersionNew: () => boolean // 配置（数据）的版本是否过新
  isPassCheckConfigVersionBest: () => boolean // 验证程序的导入版本驱动是否为最佳 当配置文件和驱动版本相等的时候为最佳,否则不匹配过新或者过旧
  jsonStruct: JsonStructure // 根据实际情况提供更具体的类型定义
  /** 是否由 SSR 版备份转换而来（UI 据此提示用户） */
  isConvertedFromSsr: boolean
  hasProperty: (key: string) => boolean
  geticons: () => IconGroup[] // 根据实际情况提供更具体的类型定义
}

// 导入json数据
export function importJsonString(jsonString: string): ImportJsonResult | null {
  let data: any
  try {
    data = JSON.parse(jsonString)
  }
  catch (error) {
    throw new FormatError('file format error')
  }

  // 先识别来源格式。SSR 版（服务端导出）与 Vue 版结构不同，
  // 直接按 Vue 结构解析会报"文件格式错误"，而文件其实是好的 ——
  // 从 SSR 版迁移过来的用户会因此无法恢复自己的备份。
  const format = detectBackupFormat(data)

  if (format === 'ssr') {
    const icons = convertSsrBackup(data)
    // 转成 Vue 结构后走同一条导入流程。
    // md5 用一个稳定值填充：SSR 备份本就没有校验和，
    // 不该因此拒绝导入；这里填自身值以通过等值比较。
    const normalized: JsonStructure = {
      version: typeof data.version === 'number' ? data.version : 1,
      appName: APPNAME,
      exportTime: data.exportedAt || '',
      appVersion: data.app || '',
      icons,
      md5: '',
    }
    normalized.md5 = generateMD5(JSON.stringify(normalized))

    return {
      isPassCheckMd5: () => true, // SSR 备份无校验和，视为通过（并在 UI 上另行提示）
      isPassCheckConfigVersionOld: () => true,
      isPassCheckConfigVersionNew: () => true,
      isPassCheckConfigVersionBest: () => true,
      jsonStruct: normalized,
      isConvertedFromSsr: true,
      hasProperty: (key: string): boolean => {
        return key in normalized
      },
      geticons: (): IconGroup[] => {
        return normalized.icons || []
      },
    }
  }

  const jsonStruct = transformJson(data)
  const md5 = generateMD5(jsonString)

  if (!jsonStruct) {
    throw new FormatError('file format error')
  }

  if (data.version < ALLOW_LOW_VERSION)
    throw new ConfigVersionLowError('')

  return {
    isPassCheckMd5: () => md5 === jsonStruct.md5,
    isPassCheckConfigVersionOld: () => !(jsonStruct.version < VERSION),
    isPassCheckConfigVersionNew: () => !(jsonStruct.version > VERSION),
    isPassCheckConfigVersionBest: () => jsonStruct.version === VERSION,
    jsonStruct,
    isConvertedFromSsr: false,
    hasProperty: (key: string): boolean => {
      return key in jsonStruct
    },
    geticons: (): IconGroup[] => {
      return jsonStruct.icons || []
    },
  }
}

function transformJson(jsonData: any): JsonStructure | null {
  // 检查必须存在的键
  const requiredKeys: Array<keyof JsonStructure> = ['version', 'appName', 'exportTime', 'appVersion', 'md5']
  for (const key of requiredKeys) {
    if (!(key in jsonData))
      return null
  }

  // 使用类型断言将 JSON 数据转换为指定类型
  const transformedData: JsonStructure = jsonData as JsonStructure

  // 返回转换后的数据
  return transformedData
}

/** 识别备份文件的来源格式 */
export type BackupFormat = 'vue' | 'ssr' | 'unknown'

/**
 * 判断这是哪一套前端导出的备份。
 *
 * 靠标志性字段区分，而不是靠 version 数字 —— 两边都用 version:1，
 * 单看版本号无法分辨。
 */
export function detectBackupFormat(data: any): BackupFormat {
  if (!data || typeof data !== 'object')
    return 'unknown'

  // SSR 版标志：app + groups/cards 平铺
  const looksSsr = data.app === 'sun-panel-v2' || ('groups' in data && 'cards' in data)
  // Vue 版标志：appName + icons 嵌套
  const looksVue = data.appName === APPNAME || 'icons' in data

  if (looksVue && !looksSsr)
    return 'vue'
  if (looksSsr && !looksVue)
    return 'ssr'
  // 两者特征都有：优先按 Vue（本程序的原生格式）
  if (looksVue)
    return 'vue'
  if (looksSsr)
    return 'ssr'
  return 'unknown'
}

/**
 * 把 SSR 版备份转换成 Vue 版结构。
 *
 * SSR 里「组」与「卡片」是两个平铺数组，靠 card.itemIconGroupId 关联。
 * Vue 要求组内嵌套 children，因此需要按 groupId 分组。
 *
 * 边界处理：
 *   · 找不到所属组的卡片 → 放进一个「未分组」组，而不是丢弃。
 *     用户更希望看到"有张卡片没归好类"，而不是"导入后少了几张卡"。
 *   · 组按 sort 升序，卡片同理，保持原有顺序。
 */
export function convertSsrBackup(data: any): IconGroup[] {
  const rawGroups: SsrBackupGroup[] = Array.isArray(data?.groups) ? data.groups : []
  const rawCards: SsrBackupCard[] = Array.isArray(data?.cards) ? data.cards : []

  const groups: IconGroup[] = rawGroups.map(g => ({
    title: g.title || '',
    sort: typeof g.sort === 'number' ? g.sort : 0,
    children: [],
  }))

  // 用组在原数组中的下标作为关联依据。
  // SSR 备份里卡片存的是 itemIconGroupId（数据库主键），
  // 而导入时组会被重新创建、拿到新 id，所以只能按导出文件里的
  // 旧 id 做一次映射。
  const idToIndex = new Map<number, number>()
  rawGroups.forEach((g, i) => {
    if (typeof g.id === 'number')
      idToIndex.set(g.id, i)
  })

  const orphans: Icon[] = []

  for (const c of rawCards) {
    const icon: Icon = {
      title: c.title || '',
      sort: typeof c.sort === 'number' ? c.sort : 99999,
      icon: c.icon ?? null,
      url: c.url || '',
      lanUrl: c.lanUrl || '',
      description: c.description || '',
      openMethod: typeof c.openMethod === 'number' ? c.openMethod : 1,
      lanOnly: typeof c.lanOnly === 'number' ? c.lanOnly : 0,
    }

    const idx = typeof c.itemIconGroupId === 'number' ? idToIndex.get(c.itemIconGroupId) : undefined
    if (idx !== undefined)
      groups[idx].children.push(icon)
    else
      orphans.push(icon)
  }

  // 排序，保持原文件的展示顺序
  groups.sort((a, b) => (a.sort || 0) - (b.sort || 0))
  groups.forEach(g => g.children.sort((a, b) => (a.sort || 0) - (b.sort || 0)))

  if (orphans.length) {
    orphans.sort((a, b) => (a.sort || 0) - (b.sort || 0))
    groups.push({
      title: '未分组（来自 SSR 备份）',
      sort: 99999,
      children: orphans,
    })
  }

  return groups
}

function generateMD5(jsonString: string): string {
  try {
    // 解析 JSON 字符串
    const data: any = JSON.parse(jsonString)
    // 移除 md5 字段及其对应的值
    removeMD5Field(data)
    // 将修改后的 JSON 对象转换回字符串
    const modifiedJsonString = JSON.stringify(data)
    // 使用 crypto-js 计算 MD5 值
    const md5 = CryptoJS.MD5(modifiedJsonString).toString()
    return md5
  }
  catch (error) {
    return ''
  }
}

function removeMD5Field(obj: any): void {
  for (const key in obj) {
    if (key === 'md5') {
      // 移除 md5 字段
      delete obj[key]
      return
    }
  }
}
