<script setup lang="ts">
/**
 * Docker 容器管理页。
 *
 * 对照作者 v2 的「系统应用 → Docker」：一张表格列出全部容器
 * （容器名 + 镜像名、状态、资源概览、操作），而不只是每张卡片上一个小圆点。
 *
 * 为什么值得单独做一页：
 * 卡片上的状态点只能回答"我绑的那几个容器在不在跑"，
 * 回答不了"我这台 NAS 上现在有哪些容器、哪个挂了、哪个吃满内存"。
 * 后者恰恰是用户打开面板时最常想确认的事。
 */
import { computed, h, onMounted, onUnmounted, ref } from 'vue'
import { NAlert, NButton, NDataTable, NInput, NSpace, NTag, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { containerAction, listContainers, type DockerContainer } from '@/api/panel/dockerManage'
import { useAuthStore } from '@/store'
import { t } from '@/locales'

const ms = useMessage()
const authStore = useAuthStore()

const list = ref<DockerContainer[]>([])
const dockerReady = ref<boolean | undefined>(undefined)
const loading = ref(false)
const keyword = ref('')
/** 正在执行操作的容器 id（用于按钮 loading，防重复点击） */
const actingId = ref('')

const isAdmin = computed(() => authStore.userInfo?.role === 1)

async function refresh() {
  loading.value = true
  try {
    const res = await listContainers(true)
    if (res?.code === 0) {
      list.value = res.data?.list || []
      dockerReady.value = res.data?.dockerReady
    }
  }
  catch {
    // silent 已在 API 层设过，这里不弹提示
  }
  finally {
    loading.value = false
  }
}

const filtered = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  if (!k)
    return list.value
  return list.value.filter(c =>
    (c.name || '').toLowerCase().includes(k)
    || (c.image || '').toLowerCase().includes(k))
})

/** 状态 → 展示文案与颜色（与卡片上的状态点语义保持一致） */
function stateInfo(c: DockerContainer): { text: string; type: 'success' | 'warning' | 'error' | 'default' } {
  const s = (c.state || '').toLowerCase()
  if (c.running)
    return { text: t('apps.dockerManage.stateRunning'), type: 'success' }
  if (s === 'dead')
    return { text: t('apps.dockerManage.stateDead'), type: 'error' }
  if (s === 'restarting' || s === 'paused')
    return { text: t('apps.dockerManage.statePending'), type: 'warning' }
  if (s === 'exited')
    return { text: t('apps.dockerManage.stateExited'), type: 'default' }
  return { text: s || t('apps.dockerManage.stateUnknown'), type: 'default' }
}

function fmtBytes(v?: number) {
  if (!v || v <= 0)
    return '—'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let n = v
  while (n >= 1024 && i < u.length - 1) { n /= 1024; i++ }
  return `${(i === 0 ? n.toFixed(0) : n.toFixed(n >= 100 ? 0 : 1))}${u[i]}`
}

/** 资源概览：运行中才显示 CPU/内存，其余留空（避免 0% 造成"在跑但没占用"的误读） */
function resourceText(c: DockerContainer) {
  if (!c.running)
    return '—'
  const cpu = (c.cpuPercent ?? 0).toFixed(1)
  return `CPU ${cpu}% · ${fmtBytes(c.memUsage)}`
}

async function doAction(c: DockerContainer, action: 'start' | 'stop' | 'restart') {
  if (actingId.value)
    return
  actingId.value = c.id
  try {
    const res = await containerAction(c.id, action)
    if (res?.code === 0 && res.data?.ok) {
      ms.success(t(`apps.dockerManage.${action}Success`))
      // 稍等一拍再刷新：容器状态切换需要一点时间，
      // 立刻查会读到切换前的旧状态。
      setTimeout(refresh, 700)
    }
    else {
      ms.error(res?.msg || t('common.failed'))
    }
  }
  catch {
    // 非 silent，请求层已弹过网络错误提示
  }
  finally {
    actingId.value = ''
  }
}

const columns = computed<DataTableColumns<DockerContainer>>(() => [
  {
    title: t('apps.dockerManage.colName'),
    key: 'name',
    // 容器名 + 镜像名两行：作者也是这么显示的。
    // 仅名字有时不足以分辨同名不同镜像的容器。
    render(c) {
      return h('div', { class: 'leading-tight' }, [
        h('div', { class: 'font-medium text-sm' }, c.name || '-'),
        h('div', { class: 'text-xs text-gray-500 break-all' }, c.image || ''),
      ])
    },
  },
  {
    title: t('apps.dockerManage.colState'),
    key: 'state',
    width: 110,
    render(c) {
      const s = stateInfo(c)
      return h(NTag, { size: 'small', type: s.type, round: true }, { default: () => s.text })
    },
  },
  {
    title: t('apps.dockerManage.colResource'),
    key: 'resource',
    width: 170,
    render: c => h('span', { class: 'text-xs text-gray-500' }, resourceText(c)),
  },
  {
    title: t('apps.dockerManage.colAction'),
    key: 'action',
    width: 150,
    render(c) {
      if (!isAdmin) {
        // 非管理员只读：不渲染按钮比渲染后禁用更清楚
        return h('span', { class: 'text-xs text-gray-400' }, t('apps.dockerManage.adminOnly'))
      }
      const busy = actingId.value === c.id
      return h(NSpace, { size: 6 }, {
        default: () => [
          c.running
            ? h(NButton, {
                size: 'tiny',
                loading: busy,
                onClick: () => doAction(c, 'stop'),
              }, { default: () => t('apps.dockerManage.stop') })
            : h(NButton, {
                size: 'tiny',
                type: 'primary',
                loading: busy,
                onClick: () => doAction(c, 'start'),
              }, { default: () => t('apps.dockerManage.start') }),
          h(NButton, {
            size: 'tiny',
            loading: busy,
            onClick: () => doAction(c, 'restart'),
          }, { default: () => t('apps.dockerManage.restart') }),
        ],
      })
    },
  },
])

// 供 columns 的 render 使用（naive-ui 的 render 需要 h）
// ——h 已在顶部从 vue 导入

let timer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  refresh()
  // 每 10 秒刷新一次：这一页是"看状态"用的，数据必须新鲜。
  // 与卡片的 5 秒轮询错开，避免同时打满 docker daemon。
  timer = setInterval(() => {
    if (typeof document !== 'undefined' && document.hidden)
      return
    refresh()
  }, 10000)
})

onUnmounted(() => {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
})
</script>

<template>
  <div class="bg-slate-200 dark:bg-zinc-900 rounded-[10px] p-[8px] overflow-auto h-full">
    <NAlert v-if="dockerReady === false" type="warning" :bordered="false">
      {{ $t('apps.dockerManage.unavailable') }}
    </NAlert>

    <NAlert v-else type="info" :bordered="false">
      {{ $t('apps.dockerManage.alertText') }}
    </NAlert>

    <div class="flex items-center justify-between mt-[10px] mb-[8px] flex-wrap gap-2">
      <NInput
        v-model:value="keyword"
        size="small"
        clearable
        :placeholder="$t('apps.dockerManage.searchPlaceholder')"
        style="max-width: 260px;"
      />
      <NSpace :size="8">
        <span class="text-xs text-gray-500 self-center">
          {{ $t('apps.dockerManage.total', { count: list.length }) }}
        </span>
        <NButton size="small" :loading="loading" @click="refresh">
          {{ $t('apps.dockerManage.refresh') }}
        </NButton>
      </NSpace>
    </div>

    <NDataTable
      :columns="columns"
      :data="filtered"
      :loading="loading"
      :bordered="false"
      size="small"
      :row-key="(r: DockerContainer) => r.id"
      :max-height="520"
    />
  </div>
</template>