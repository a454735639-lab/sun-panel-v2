<script setup lang="ts">
/**
 * 小组件数据展示。
 *
 * 渲染后端 lib/wgt 返回的 Result：文本 + 指标 + 进度条。
 *
 * 为什么不做成"每个类型一个组件"：
 * 后端已把各种服务的差异归一成统一结构（text / stats / progress），
 * 前端只需按结构渲染。后端加新类型（比如加个 Sonarr）时前端零改动 ——
 * 这正是把抓取放在后端的好处之一。
 */
import { computed } from 'vue'
import type { WidgetItem } from '@/api/panel/widget'

const props = defineProps<{
  data?: WidgetItem
  /** 文字颜色，跟随主题 */
  textColor?: string
}>()

/** 指标色调 → 具体颜色。未知 tone 回落到中性色，不报错。 */
function toneColor(tone?: string): string {
  switch (tone) {
    case 'good': return '#22c55e'
    case 'warn': return '#eab308'
    case 'bad': return '#ef4444'
    case 'muted': return '#9ca3af'
    default: return 'inherit'
  }
}

const text = computed(() => props.data?.text || '')
const stats = computed(() => props.data?.stats || [])
const progress = computed(() => props.data?.progress)

/** 出错时显示简短原因，而不是让卡片空白 */
const errorText = computed(() => {
  const d = props.data
  if (!d || d.ok)
    return ''
  return d.error || '连接失败'
})
</script>

<template>
  <div class="widget-box">
    <!-- 出错：显示原因，让用户知道是配置问题还是服务离线 -->
    <div v-if="errorText" class="widget-error" :style="{ color: textColor }">
      {{ errorText }}
    </div>

    <template v-else>
      <!-- 主文本（如"3 个任务下载中"） -->
      <div v-if="text" class="widget-text" :style="{ color: textColor }">
        {{ text }}
      </div>

      <!-- 进度条 -->
      <div v-if="progress" class="widget-progress">
        <div class="widget-progress-track">
          <div
            class="widget-progress-fill"
            :style="{ width: Math.min(100, Math.max(0, progress.value)) + '%' }"
          />
        </div>
        <div v-if="progress.label" class="widget-progress-label" :style="{ color: textColor }">
          {{ progress.label }}
        </div>
      </div>

      <!-- 指标行 -->
      <div v-if="stats.length" class="widget-stats">
        <span
          v-for="(s, i) in stats"
          :key="i"
          class="widget-stat"
        >
          <span class="widget-stat-label" :style="{ color: textColor }">{{ s.label }}</span>
          <span class="widget-stat-value" :style="{ color: toneColor(s.tone) }">{{ s.value }}</span>
        </span>
      </div>

      <!-- 未取到数据时的占位（首次加载或轮询间隙） -->
      <div v-if="!text && !stats.length && !progress" class="widget-empty" :style="{ color: textColor }">
        …
      </div>
    </template>
  </div>
</template>

<style scoped>
.widget-box {
  width: 100%;
  padding: 0 10px;
  font-size: 0.7rem;
  line-height: 1.4;
  overflow: hidden;
}

.widget-error {
  opacity: 0.85;
  font-size: 0.68rem;
  /* 错误信息可能较长，限两行避免撑坏卡片 */
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.widget-text {
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.widget-progress {
  margin-top: 3px;
}

.widget-progress-track {
  height: 4px;
  border-radius: 2px;
  background: rgba(255, 255, 255, 0.25);
  overflow: hidden;
}

.widget-progress-fill {
  height: 100%;
  border-radius: 2px;
  background: #4e8cff;
  transition: width 0.3s ease;
}

.widget-progress-label {
  margin-top: 2px;
  opacity: 0.75;
  font-size: 0.62rem;
}

.widget-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 2px;
}

.widget-stat {
  display: inline-flex;
  align-items: baseline;
  gap: 2px;
  white-space: nowrap;
}

.widget-stat-label {
  opacity: 0.65;
  font-size: 0.62rem;
}

.widget-stat-value {
  font-weight: 600;
}

.widget-empty {
  opacity: 0.5;
}
</style>
