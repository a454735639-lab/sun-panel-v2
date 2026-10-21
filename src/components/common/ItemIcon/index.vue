<script setup lang="ts">
import { NAvatar, NImage } from 'naive-ui'
import { computed, ref, withDefaults } from 'vue'
import { SvgIconOnline } from '@/components/common'

interface Prop {
  itemIcon?: Panel.ItemIcon | null
  size?: number // 默认70
  forceBackground?: string // 强制背景色
  /** 用于图标缺失时生成兜底展示（取首字）。通常传卡片标题。 */
  fallbackText?: string
}

const props = withDefaults(defineProps<Prop>(), { size: 70 })
const defaultBackground = '#2a2a2a6b'
const defaultStyle = ref({
  width: `${props.size}px`,
  height: `${props.size}px`,
})

// 计算内部元素的大小比例，使图标内容随外部尺寸等比缩放
const innerSize = computed(() => {
  // 内部元素占外部容器的60%，保持适当的边距
  return Math.round(props.size * 0.6)
})

const iconExt = computed(() => {
  return props.itemIcon?.src?.split('.').pop()
})

/**
 * 兜底显示的文字。
 *
 * 场景：卡片没配图标、或配置了 itemType=2（图片）但 src 为空。
 * 原实现下 NAvatar 无内容 → 页面出现纯空白方块，用户以为坏了。
 * 取标题首字（中文取 1 个、拉丁字母取 2 个）配上背景色，
 * 至少能看出"这是哪张卡"。
 */
const fallbackChars = computed(() => {
  const t = (props.fallbackText || '').trim()
  if (!t)
    return '·'
  // 中日韩字符取 1 个，其余取 2 个（如 "NAS" → "NA"）
  const isCJK = /[\u4E00-\u9FFF\u3040-\u30FF\uAC00-\uD7AF]/.test(t[0])
  return isCJK ? t.slice(0, 1) : t.slice(0, 2).toUpperCase()
})

/** 图标是否"实际可见"（有内容可渲染） */
const hasVisibleIcon = computed(() => {
  const ic = props.itemIcon
  if (!ic)
    return false
  if (ic.itemType === 2)
    return !!ic.src // 图片类型但没地址 → 视为无图标
  if (ic.itemType === 3)
    return !!ic.text // svg 类型但没名字 → 视为无图标
  if (ic.itemType === 1)
    return !!ic.text
  return false
})
</script>

<template>
  <div class="item-icon" :style="defaultStyle">
    <slot>
      <template v-if="itemIcon && hasVisibleIcon">
        <template v-if="itemIcon?.itemType === 1">
          <NAvatar :size="props.size" :style="{ backgroundColor: (forceBackground ?? itemIcon?.backgroundColor) || defaultBackground, borderRadius: '12px' }">
            {{ itemIcon.text }}
          </NAvatar>
        </template>

        <template v-else-if="itemIcon?.itemType === 2">
          <div v-if="iconExt === 'svg'" :style="{ backgroundColor: (forceBackground ?? itemIcon?.backgroundColor) || defaultBackground, ...defaultStyle, borderRadius: '12px' }" class="flex justify-center items-center">
            <img :src="itemIcon?.src" :style="{ width: `${innerSize}px`, height: `${innerSize}px` }">
          </div>
          <NImage v-else :style="{ backgroundColor: (forceBackground ?? itemIcon?.backgroundColor) || defaultBackground, ...defaultStyle, borderRadius: '12px' }" :src="itemIcon?.src" preview-disabled />
        </template>

        <template v-else-if="itemIcon?.itemType === 3">
          <NAvatar :size="props.size" :style="{ backgroundColor: (forceBackground ?? itemIcon?.backgroundColor) || defaultBackground, borderRadius: '12px' }">
            <SvgIconOnline :style="{ fontSize: `${innerSize}px` }" :icon="itemIcon.text" />
          </NAvatar>
        </template>
      </template>

      <!-- 兜底：图标缺失时用标题首字占位，而不是留空白方块 -->
      <template v-else>
        <NAvatar
          :size="props.size"
          :style="{ backgroundColor: defaultBackground, borderRadius: '12px' }"
        >
          {{ fallbackChars }}
        </NAvatar>
      </template>
    </slot>
  </div>
</template>
