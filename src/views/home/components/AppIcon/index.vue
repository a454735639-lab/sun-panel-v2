<script setup lang="ts">
import { computed, inject } from 'vue'
import { ItemIcon } from '@/components/common'
import { PanelPanelConfigStyleEnum } from '@/enums'
import { parseContainerId } from '@/hooks/useDockerStates'
import WidgetDisplay from './WidgetDisplay.vue'
import type { WidgetItem } from '@/api/panel/widget'

interface Prop {
  itemInfo?: Panel.ItemInfo
  size?: number // 默认70
  forceBackground?: string // 强制背景色
  iconTextColor?: string
  iconTextInfoHideDescription: boolean
  iconTextIconHideTitle: boolean
  style: PanelPanelConfigStyleEnum
}

const props = withDefaults(defineProps<Prop>(), {
  size: 70,
})

// 由首页提供的容器状态查询函数（见 home/index.vue）。
// 用 inject 而非在卡片内自行轮询：面板上可能有多张 Docker 卡片，
// 集中轮询一次即可，避免请求数随卡片数增长。
const stateOf = inject<(id: string) => DockerStateLike | undefined>('dockerStateOf', () => undefined)

// 小组件数据查询函数（同样由首页集中轮询）
const widgetOf = inject<(cardId: number) => WidgetItem | undefined>('widgetOf', () => undefined)

/**
 * 该卡片配置的小组件数据。
 *
 * 与 Docker 卡片互斥：Docker 用的是 cardType + expandParam，
 * 小组件用的是 widgetType + widgetConfig，两者不会同时配置
 * （编辑界面里是两条独立路径）。
 */
const widgetData = computed(() => {
  const type = props.itemInfo?.widgetType
  const id = props.itemInfo?.id
  if (!type || !id)
    return undefined
  return widgetOf(Number(id))
})

const hasWidget = computed(() => !!props.itemInfo?.widgetType)

const defaultBackground = '#2a2a2a6b'

const calculateLuminance = (color: string) => {
  const hex = color.replace(/^#/, '')
  const r = parseInt(hex.substring(0, 2), 16)
  const g = parseInt(hex.substring(2, 4), 16)
  const b = parseInt(hex.substring(4, 6), 16)
  return (0.299 * r + 0.587 * g + 0.114 * b) / 255
}

const textColor = computed(() => {
  const luminance = calculateLuminance(props.itemInfo?.icon?.backgroundColor || defaultBackground)
  return luminance > 0.5 ? 'black' : 'white'
})

// 根据面板样式计算ItemIcon的尺寸，使图标内容随容器等比缩放
const itemIconSize = computed(() => {
  if (props.style === PanelPanelConfigStyleEnum.info) {
    // Info模式下，图标容器尺寸为60px/50px/40px（响应式），内部图标占80%
    return Math.round(props.size * 0.8)
  } else {
    // Icon模式下，内部图标占容器的80%
    return Math.round(props.size * 0.8)
  }
})

// ---- Docker 容器卡片状态 ----
// cardType=2 表示这是 Docker 卡片，expandParam 里存着 containerId
const containerId = computed(() => {
  if (props.itemInfo?.cardType !== 2) return ''
  return parseContainerId(props.itemInfo?.expandParam)
})

const containerState = computed(() => {
  if (!containerId.value) return undefined
  return stateOf(containerId.value)
})

// 状态点颜色：运行中=绿、已停止/退出=红、其它（创建中/重启中）=黄
const stateClass = computed(() => {
  const st = containerState.value
  if (!st) return 'unknown'
  if (st.running) return 'running'
  const s = (st.state || '').toLowerCase()
  if (s === 'exited' || s === 'dead') return 'stopped'
  return 'pending'
})

function fmtBytes(v?: number) {
  if (!v || v <= 0) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let n = v
  while (n >= 1024 && i < u.length - 1) { n /= 1024; i++ }
  return (i === 0 ? n.toFixed(0) : n.toFixed(n >= 100 ? 0 : 1)) + u[i]
}

// 悬停提示：容器名 + 状态 + 指标
const stateTitle = computed(() => {
  const st = containerState.value
  if (!st) return ''
  let t = st.name + ' · ' + st.status
  if (st.running) {
    t += ' · CPU:' + (st.cpuPercent ?? 0).toFixed(1) + '% RAM:' + fmtBytes(st.memUsage)
  }
  return t
})

// 卡片右下角显示的指标文本（仅运行中显示，避免占位）
const metricsText = computed(() => {
  const st = containerState.value
  if (!st || !st.running) return ''
  return 'CPU ' + (st.cpuPercent ?? 0).toFixed(0) + '% · ' + fmtBytes(st.memUsage)
})

// 类型占位：避免从 api 直接 import 造成循环依赖
interface DockerStateLike {
  id: string
  name: string
  state: string
  status: string
  cpuPercent: number
  memUsage: number
  running: boolean
}
</script>

<template>
  <div class="app-icon w-full">
    <!-- 详情图标 -->
    <div
      v-if="style === PanelPanelConfigStyleEnum.info"
      class="app-icon-info w-full rounded-2xl  transition-all duration-200 hover:shadow-[0_0_20px_10px_rgba(0,0,0,0.2)] flex relative"
      :style="{ background: itemInfo?.icon?.backgroundColor || defaultBackground }"
      :data-container-id="containerId || undefined"
      :title="stateTitle || undefined"
    >
      <!-- 图标 -->
      <div class="app-icon-info-icon">
        <div class="w-full h-full flex items-center justify-center ">
          <ItemIcon :item-icon="itemInfo?.icon" force-background="transparent" :size="itemIconSize" class="overflow-hidden rounded-xl" />
        </div>
      </div>

      <!-- 文字 -->
      <!-- 如果为纯白色，将自动根据背景的明暗计算字体的黑白色 -->
      <div class="text-white flex items-center" :style="{ color: (iconTextColor === '#ffffff') ? textColor : iconTextColor, maxWidth: 'calc(100% - 20px)', padding: '0 10px' }">
        <div class="app-icon-info-text-box w-full">
          <div class="app-icon-info-text-box-title font-semibold w-full">
            {{ itemInfo?.title }}
          </div>
          <div
            v-if="!iconTextInfoHideDescription"
            class="app-icon-info-text-box-description"
            :title="itemInfo?.description"
          >
            {{ itemInfo?.description }}
          </div>
          <!-- 容器指标：仅 Docker 卡片且容器运行中时显示。
               放在描述下方，不占据额外空间，无数据时整行不渲染。 -->
          <div v-if="metricsText" class="app-icon-metrics app-icon-info-text-box-metrics">
            {{ metricsText }}
          </div>
        </div>
      </div>

      <!-- Docker 状态点：卡片右上角 -->
      <span
        v-if="containerId"
        class="docker-dot"
        :class="'docker-dot--' + stateClass"
        :title="stateTitle"
      />
    </div>

    <!-- 极简(小)图标（APP） -->
    <div v-if="style === PanelPanelConfigStyleEnum.icon" class="app-icon-small">
      <div
        class="app-icon-small-icon overflow-hidden rounded-2xl sunpanel mx-auto rounded-2xl transition-all duration-200 hover:shadow-[0_0_20px_10px_rgba(0,0,0,0.2)] relative"
        :title="stateTitle || itemInfo?.description"
        :data-container-id="containerId || undefined"
      >
        <ItemIcon :item-icon="itemInfo?.icon" :size="itemIconSize" :fallback-text="itemInfo?.title" />
        <!-- Docker 状态点 -->
        <span
          v-if="containerId"
          class="docker-dot"
          :class="'docker-dot--' + stateClass"
          :title="stateTitle"
        />
      </div>
      <div
        v-if="!iconTextIconHideTitle"
        class="app-icon-small-title text-center app-icon-text-shadow cursor-pointer mt-[2px]"
        :style="{ color: iconTextColor }"
      >
        <span>{{ itemInfo?.title }}</span>
      </div>
      <div
        v-if="!iconTextInfoHideDescription && itemInfo?.description"
        class="app-icon-small-description text-center cursor-pointer mt-[1px]"
        :style="{ color: iconTextColor }"
        :title="itemInfo?.description"
      >
        {{ itemInfo?.description }}
      </div>
      <!-- 容器指标：小卡同样要显示，否则用户看不出容器是否吃满资源。
           用比描述更小的字号，并在无数据时不渲染整行。 -->
      <div
        v-if="metricsText"
        class="app-icon-metrics app-icon-small-metrics text-center opacity-70 mt-[1px]"
        :style="{ color: iconTextColor }"
      >
        {{ metricsText }}
      </div>
    </div>

    <!-- 小组件数据：下载进度 / 媒体库统计等。
         放在卡片**下方**而非卡内：info 模式是横向 flex 布局，
         塞进去会挤成第三列；放外面则对两种卡片样式都安全，
         也符合"卡片显示入口、下方显示实时状态"的信息层次。 -->
    <div
      v-if="hasWidget"
      class="app-icon-widget"
      :data-widget-card-id="itemInfo?.id"
    >
      <WidgetDisplay :data="widgetData" :text-color="iconTextColor" />
    </div>
  </div>
</template>

<style scoped>
/* 小组件区：卡片下方的独立区域。
   加一点上边距与文字阴影，让它在壁纸上也能看清。 */
.app-icon-widget {
  margin-top: 4px;
  text-shadow: 0 0 6px rgba(0, 0, 0, 0.45);
}

/* 小卡的描述文字。
   关键约束：**必须单行截断**。
   小卡网格是等宽列（minmax(100px, 1fr)），若描述换行，
   长描述会把所在那一行撑高，破坏整片宫格的对齐 —— 也就失去了
   小卡"齐整、好扫读"的核心优势。完整内容通过 title 悬停查看。 */
.app-icon-small-description {
  font-size: 0.68rem;
  line-height: 1.35;
  opacity: 0.8;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* info 模式的标题与描述。
 *
 * 两条约束都是为了"卡片高度齐整"：
 *   - 标题单行截断：长标题（如 "moviepilot-v3"）换行会把卡片撑高
 *   - 描述最多两行：描述长度差异很大，不限行会导致同一行卡片高矮不一，
 *     视觉上参差。完整内容通过 title 悬停查看。
 */
.app-icon-info-text-box-title {
  font-size: 0.95rem;
  line-height: 1.3;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.app-icon-info-text-box-description {
  font-size: 0.7rem;
  line-height: 1.35;
  opacity: 0.8;
  /* 限制两行，超出省略 */
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  word-break: break-word;
}

/* ---------- Docker 容器状态点 ----------
   绿=运行中，红=已停止/退出，黄=过渡态（创建中/重启中），灰=状态未知。
   用 absolute 定位在卡片右上角，不参与布局，因此不会挤压标题与描述。
   加白色描边是为了在深浅色卡片背景上都能看清。 */
.docker-dot {
  position: absolute;
  top: 6px;
  right: 6px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  box-shadow: 0 0 0 1.5px rgba(255, 255, 255, 0.55);
  pointer-events: auto;
  z-index: 2;
}
.docker-dot--running {
  background: #22c55e;
  /* 运行中加一圈柔和光晕，让"在线"更醒目 */
  box-shadow: 0 0 0 1.5px rgba(255, 255, 255, 0.55), 0 0 6px rgba(34, 197, 94, 0.8);
}
.docker-dot--stopped { background: #ef4444; }
.docker-dot--pending { background: #eab308; }
.docker-dot--unknown { background: #9ca3af; }

/* 容器指标：小字、半透明，避免与标题争夺注意力。
   .app-icon-metrics 是两种卡片样式共用的标记类，
   便于统一查询（测试与样式）而不必分别匹配。 */
.app-icon-info-text-box-metrics,
.app-icon-small-metrics {
  font-size: 0.7rem;
  opacity: 0.75;
  line-height: 1.3;
  margin-top: 1px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* 响应式图标块设计 */
.app-icon-info-icon {
  width: min(70px, 100%);
  height: min(70px, 100%);
  min-width: min(70px, 100%);
  aspect-ratio: 1 / 1;
}

.app-icon-small-icon {
  width: min(70px, 100%);
  height: min(70px, 100%);
  aspect-ratio: 1 / 1;
}

/* 确保item-icon完全填充容器 */
:deep(.item-icon) {
  width: 100% !important;
  height: 100% !important;
  display: flex;
  align-items: center;
  justify-content: center;
}

/* 针对不同屏幕尺寸的等比缩小 */
@media (max-width: 1024px) {
  .app-icon-info-icon {
    width: min(55px, 100%);
    height: min(55px, 100%);
    min-width: min(55px, 100%);
  }
  
  .app-icon-small-icon {
    width: min(55px, 100%);
    height: min(55px, 100%);
  }
}

@media (max-width: 768px) {
  .app-icon-info-icon {
    width: min(45px, 100%);
    height: min(45px, 100%);
    min-width: min(45px, 100%);
  }
  
  .app-icon-small-icon {
    width: min(45px, 100%);
    height: min(45px, 100%);
  }
  
  .app-icon-info-text-box-title {
    font-size: 0.85rem !important;
    word-wrap: break-word;
    overflow-wrap: break-word;
    white-space: normal;
  }
  
  .app-icon-info-text-box-description {
    font-size: 0.7rem !important;
    word-wrap: break-word;
    overflow-wrap: break-word;
    white-space: normal;
  }
  
  .app-icon-small-title {
    font-size: 0.75rem !important;
  }
}

@media (max-width: 480px) {
  .app-icon-info-icon {
    width: min(40px, 100%);
    height: min(40px, 100%);
    min-width: min(40px, 100%);
  }
  
  .app-icon-small-icon {
    width: min(40px, 100%);
    height: min(40px, 100%);
  }
  
  .app-icon-info-text-box-title {
    font-size: 0.8rem !important;
  }
  
  .app-icon-info-text-box-description {
    font-size: 0.65rem !important;
  }
  
  .app-icon-small-title {
    font-size: 0.7rem !important;
  }
}

@media (max-width: 360px) {
  .app-icon-info-icon {
    width: min(35px, 100%);
    height: min(35px, 100%);
    min-width: min(35px, 100%);
  }
  
  .app-icon-small-icon {
    width: min(35px, 100%);
    height: min(35px, 100%);
  }
  
  .app-icon-info-text-box-title {
    font-size: 0.75rem !important;
  }
  
  .app-icon-info-text-box-description {
    font-size: 0.6rem !important;
  }
  
  .app-icon-small-title {
    font-size: 0.65rem !important;
  }
}
</style>
