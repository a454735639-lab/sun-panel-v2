/**
 * 分组级卡片样式。
 *
 * 每个分组可以单独设置卡片风格与文字色 —— 例如「NAS 应用」用详情横卡
 * （能显示 CPU/内存），而「站点收藏」用小图标方卡（一屏放更多）。
 * 这是 SSR 版已有、Vue 版一直缺失的能力。
 *
 * 存储：item_icon_group 表的 card_style 字段（JSON 字符串），
 * 格式与后端 pages/panelconfig.go 的 GroupCardStyle 保持一致，
 * 这样两套前端读写同一份数据，不会互相看不懂。
 */

/** 卡片风格。数值与后端 GroupCardStyle.Style 对齐，不可随意改。 */
export enum GroupCardStyleEnum {
  /** 详情横卡：图标在左，标题+描述在右 */
  info = 0,
  /** 图标方卡：图标在上，标题在下（默认） */
  icon = 1,
  /** 大卡（后端已定义，当前 Vue 未实现渲染，先保留以便对齐） */
  card = 2,
}

export interface GroupCardStyle {
  style: GroupCardStyleEnum
  /** 该分组卡片文字颜色 */
  textColor: string
  /** 详情横卡下是否隐藏描述 */
  textInfoHideDescription: boolean
  /** 图标方卡下是否隐藏标题 */
  textIconHideTitle: boolean
}

/**
 * 默认样式。
 *
 * 与后端 parseGroupCardStyle 的回落值一致（style=1、白色）。
 *
 * 特别注意 textColor 用空串而非 #ffffff —— 这里留空表示
 * "跟随全局设置"。因为全局的 iconTextColor 用户可能改过，
 * 如果分组默认就写死 #ffffff，新建分组会突然变成白色，
 * 与用户已调好的主题不一致。只有用户显式给分组设了色才覆盖。
 */
export function defaultGroupCardStyle(): GroupCardStyle {
  return {
    style: GroupCardStyleEnum.icon,
    textColor: '',
    textInfoHideDescription: false,
    textIconHideTitle: false,
  }
}

/**
 * 解析分组的 card_style 字段。
 *
 * 该字段是历史遗留的可空字符串，可能是：
 *   · 空串 / "null"  → 用默认值
 *   · 合法 JSON      → 合并到默认值（缺字段用默认）
 *   · 损坏的 JSON    → 用默认值（不能让一个坏字段把整个分组渲染搞崩）
 */
export function parseGroupCardStyle(raw?: string | null): GroupCardStyle {
  const def = defaultGroupCardStyle()
  if (!raw)
    return def
  const s = String(raw).trim()
  if (!s || s === 'null')
    return def

  try {
    const obj = JSON.parse(s)
    if (!obj || typeof obj !== 'object')
      return def
    return {
      // style 可能是 0，不能用 || 回退（0 是有效值）
      style: typeof obj.style === 'number' ? obj.style : def.style,
      textColor: typeof obj.textColor === 'string' ? obj.textColor : def.textColor,
      textInfoHideDescription: !!obj.textInfoHideDescription,
      textIconHideTitle: !!obj.textIconHideTitle,
    }
  }
  catch {
    // 脏数据兜底：宁可显示默认样式，也不要整组不渲染
    return def
  }
}

/** 序列化为后端可读的 JSON */
export function stringifyGroupCardStyle(v: GroupCardStyle): string {
  return JSON.stringify({
    style: v.style,
    textColor: v.textColor,
    textInfoHideDescription: v.textInfoHideDescription,
    textIconHideTitle: v.textIconHideTitle,
  })
}

/** 供选择器使用的选项 */
export function groupCardStyleOptions() {
  return [
    { label: '分组默认', value: -1 },
    { label: '详情图标', value: GroupCardStyleEnum.info },
    { label: '小图标', value: GroupCardStyleEnum.icon },
  ]
}
