<script setup lang="ts">
import { computed, onMounted, ref, withDefaults } from 'vue'
import { Icon } from '@iconify/vue'

const props = withDefaults(defineProps<{
  icon: string
  className?: string
}>(), { className: '' })

function compatibleName(inputString: string): string {
  // 使用正则表达式替换所有的冒号
  return inputString.replace(/:/g, '-')
}

/**
 * 本地 sprite 中实际存在的图标名集合。
 *
 * 为什么要探测：vite-plugin-svg-icons 在启动时把
 * `src/assets/svg-icons/*.svg` 打成 sprite 注入 DOM，组件用
 * `<use href="#名字">` 引用。**引用不存在的 id 不会有任何报错** ——
 * 编译通过、控制台干净，只是那个位置永远空白。
 *
 * 于是"忘了放 svg 文件"或"图标名写错"都是静默失败，
 * 只能靠肉眼逐页看才暴露（实际就漏过：容器管理、公共图库两项菜单空白）。
 * 这里主动核对一次，取不到就回退在线图标。
 */
let spriteIds: Set<string> | null = null

function getSpriteIds(): Set<string> {
  if (spriteIds)
    return spriteIds
  const ids = new Set<string>()
  try {
    document.querySelectorAll('symbol').forEach((el) => {
      if (el.id)
        ids.add(el.id)
    })
  }
  catch {
    // 非浏览器环境忽略
  }
  // 只有确实扫到内容才缓存 —— 避免首次渲染过早把空集合缓存住
  if (ids.size)
    spriteIds = ids
  return ids
}

// sprite 由 main.ts 在挂载前注入，正常此时已就绪；
// 用 ready 触发挂载后再判定一次，避免早于注入时误判。
const ready = ref(false)

// 缺失只提示一次，避免同一图标在长列表里反复刷屏
const warned = new Set<string>()

onMounted(() => {
  ready.value = true
  const key = compatibleName(props.icon)
  if (!getSpriteIds().has(key) && !warned.has(key)) {
    warned.add(key)
    console.warn(
      `[SvgIcon] 本地图标缺失：${props.icon}（期望 src/assets/svg-icons/${key}.svg），已回退在线图标库`,
    )
  }
})

const symbolId = computed(() => `#${compatibleName(props.icon)}`)

const svgClass = computed(() => {
  if (props.className)
    return `svg-icon ${props.className}`
  return 'svg-icon'
})

/** 本地 sprite 是否有这个图标（ready 之前先按本地渲染，与原有行为一致） */
const hasLocal = computed(() => {
  if (!ready.value)
    return true
  return getSpriteIds().has(compatibleName(props.icon))
})
</script>

<template>
  <!-- 本地命中：用 sprite 引用（离线可用、无额外请求） -->
  <svg v-if="hasLocal" :class="svgClass" aria-hidden="true">
    <use class="svg-use" :href="symbolId" />
  </svg>
  <!-- 本地缺失：回退在线图标库。
       把"静默空白"变成"要么显示、要么在控制台留一条可排查的线索"。 -->
  <Icon v-else :icon="icon" :class="className" />
</template>

<style scoped>
  .svg-icon {
    width: 1em;
    height: 1em;
    fill: currentColor;
    overflow: hidden;
  }
</style>