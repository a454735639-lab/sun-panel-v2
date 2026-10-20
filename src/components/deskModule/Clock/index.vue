<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { t } from '@/locales'

const props = defineProps<{
  hideSecond?: boolean
  /**
   * 时钟文字颜色。留空则沿用父级的文字颜色（默认白色）。
   *
   * 为什么用可选 prop 而不是直接读 store：
   * Clock 是通用展示组件，不该依赖面板配置的具体结构；
   * 由调用方决定颜色，组件可被复用（如放到别处、或在预览里用固定色）。
   */
  color?: string
}>()

interface CurrentDate {
  time: string
  date: string
  week: string
}

const currentDate = ref<CurrentDate>({
  time: '--:--',
  date: '------',
  week: '--',
})

function updateCurrentDate() {
  const now = new Date()
  const hours = String(now.getHours()).padStart(2, '0')
  const minutes = String(now.getMinutes()).padStart(2, '0')

  if (!props.hideSecond) {
    const seconds = String(now.getSeconds()).padStart(2, '0')
    currentDate.value.time = `${hours}:${minutes}:${seconds}`
  }
  else {
    currentDate.value.time = `${hours}:${minutes}`
  }

  // 获取当前的日期
  const day = now.getDate()
  const month = now.getMonth() + 1 // 月份从0开始，所以要加1
  // const year = now.getFullYear()

  const daysOfWeek = [
    t('deskModule.clock.sun'),
    t('deskModule.clock.mon'),
    t('deskModule.clock.tue'),
    t('deskModule.clock.wed'),
    t('deskModule.clock.thu'),
    t('deskModule.clock.fri'),
    t('deskModule.clock.sat'),
  ]
  // const daysOfWeek = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
  currentDate.value.week = daysOfWeek[now.getDay()]
  currentDate.value.date = `${month}-${day}`
}

const updateClock = () => {
  updateCurrentDate()
}

const intervalId = setInterval(updateClock, 1000)

onMounted(() => {
  updateClock()
  updateCurrentDate()
})

onBeforeUnmount(() => {
  clearInterval(intervalId)
})
</script>

<template>
  <!-- color 未设置时不写 style，让父级的 text-white 生效（保持原有外观） -->
  <div
    class="clock w-full text-center"
    :style="color ? { color } : undefined"
  >
    <span class="clock-time text-2xl sm:text-2xl md:text-3xl font-[600]">
      {{ currentDate.time }}
    </span>
    <div class="hidden sm:hidden md:block">
      <span class="clock-date mr-1">
        {{ currentDate.date }}
      </span>
      <span class="clock-week">
        {{ currentDate.week }}
      </span>
    </div>
  </div>
</template>
