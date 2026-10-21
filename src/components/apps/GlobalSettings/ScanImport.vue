<script setup lang="ts">
/**
 * 扫描导入：自动扫描 Docker 容器，按服务分类生成候选卡片。
 *
 * 对着一堆容器手工建卡片很繁琐，这里一次扫描就能把全部入口建好。
 *
 * 几个刻意的设计：
 *
 * 1. 按分类分组展示，而不是一个平铺的长列表
 *    扫描出来可能有几十个容器，平铺难以挑选。后端已按镜像名推断
 *    分类（下载/媒体/数据库/…），按类分组后一眼能看出该选哪些。
 *
 * 2. 已导入的默认不勾选，且明确标注
 *    避免重复添加造成面板里出现一模一样的卡片。
 *
 * 3. 逐条调用既有的卡片创建接口，而不是另做批量导入接口
 *    这样创建逻辑只有一条路径（与手工新建完全一致），
 *    不会出现"扫描导入的卡片和手工建的卡片行为不同"这类问题。
 *    代价是多次请求，但导入是一次性操作，可以接受。
 *
 * 4. 顺序导入并显示进度
 *    并发创建会让"导入到第几个"难以反馈，也可能给 NAS 造成瞬时压力。
 */
import { computed, ref, onMounted } from 'vue'
import {
  NAlert, NButton, NCard, NCheckbox, NCheckboxGroup, NEmpty, NSpace,
  NSpin, NTag, useMessage,
} from 'naive-ui'
import { getScanCandidates, type ScanCandidate } from '@/api/panel/scan'
import { edit as editCard } from '@/api/panel/itemIcon'
import { edit as addGroup } from '@/api/panel/itemIconGroup'
import { ss } from '@/utils/storage/local'
import { t } from '@/locales'

const message = useMessage()

const loading = ref(false)
const importing = ref(false)
const dockerReady = ref(true)
const candidates = ref<ScanCandidate[]>([])
const checked = ref<string[]>([])

/** 导入进度：已完成 / 总数 */
const progress = ref({ done: 0, total: 0 })

/**
 * 按分类分组。
 *
 * 顺序按"常见程度"排：下载/媒体/影音管理在最前，
 * 这些是 NAS 上最常建入口的服务。
 */
const GROUP_ORDER = ['下载', '媒体', '影音管理', '资讯', '网络运维', 'AI', '自动化', '密码管理', '数据库', '导航面板', '未分组']

const groups = computed(() => {
  const map: Record<string, ScanCandidate[]> = {}
  for (const c of candidates.value) {
    // 后端标记为排除的工具容器（adminer / sun-panel 自身等）不展示
    if ((c as any).excluded)
      continue
    const g = c.group || '未分组'
    if (!map[g])
      map[g] = []
    map[g].push(c)
  }
  const keys = Object.keys(map).sort((a, b) => {
    const ia = GROUP_ORDER.indexOf(a)
    const ib = GROUP_ORDER.indexOf(b)
    return (ia === -1 ? 999 : ia) - (ib === -1 ? 999 : ib)
  })
  return keys.map(k => ({ name: k, items: map[k] }))
})

/** 可导入的候选（排除工具容器） */
const selectable = computed(() =>
  candidates.value.filter(c => !(c as any).excluded),
)

async function scan() {
  loading.value = true
  try {
    const res = await getScanCandidates()
    if (res?.code === 0 && res.data) {
      dockerReady.value = res.data.dockerReady
      candidates.value = res.data.list || []
      // 默认勾选"未导入过"的，已导入的留给用户主动勾选
      checked.value = (res.data.list || [])
        .filter(c => !c.exists && !(c as any).excluded)
        .map(c => c.containerId)
    }
    else {
      candidates.value = []
    }
  }
  catch {
    // silent 请求，失败时给一个温和提示
    message.warning(t('apps.scanImport.scanFailed'))
    candidates.value = []
  }
  finally {
    loading.value = false
  }
}

function toggleAll(select: boolean) {
  checked.value = select
    ? selectable.value.filter(c => !c.exists).map(c => c.containerId)
    : []
}

/**
 * 执行导入。
 *
 * 按分类建分组（不存在则创建），再把容器逐条转成卡片。
 * 组名直接用扫描推断的分类名，用户之后可以再改。
 */
async function doImport() {
  const picked = selectable.value.filter(c => checked.value.includes(c.containerId))
  if (!picked.length) {
    message.warning(t('apps.scanImport.nothingSelected'))
    return
  }

  importing.value = true
  progress.value = { done: 0, total: picked.length }

  try {
    // 1) 取现有分组，避免重复创建同名组
    const groupIdByName: Record<string, number> = {}
    const existing = await import('@/api/panel/itemIconGroup').then(m => m.getList<any>())
    if (existing?.code === 0 && existing.data?.list) {
      for (const g of existing.data.list)
        groupIdByName[g.title] = g.id
    }

    // 2) 逐条导入
    let ok = 0
    for (const c of picked) {
      const groupName = c.group || '未分组'
      let gid = groupIdByName[groupName]

      if (!gid) {
        const created = await addGroup<any>({ title: groupName, sort: 999 })
        if (created?.code === 0 && created.data?.id) {
          gid = created.data.id
          groupIdByName[groupName] = gid
        }
      }
      if (!gid)
        continue

      const res = await editCard<any>({
        itemIconGroupId: gid,
        title: c.name,
        url: c.wanUrl || c.lanUrl,
        lanUrl: c.lanUrl,
        description: '',
        openMethod: 2,
        lanOnly: 0,
        icon: {
          itemType: 1,
          text: (c.name || '?').slice(0, 2),
          src: c.iconUrl || '',
          backgroundColor: '#2a2a2a6b',
        },
        // Docker 卡片：状态灯会显示这个容器
        cardType: 2,
        expandParam: JSON.stringify({ containerId: c.containerId, containerName: c.name }),
      } as any)

      if (res?.code === 0)
        ok++
      progress.value = { done: progress.value.done + 1, total: picked.length }
    }

    // 3) 清缓存，让首页下次加载拿到新数据
    ss.remove('groupListCache')
    Object.keys(localStorage).forEach((k) => {
      if (k.startsWith('itemIconList_'))
        ss.remove(k)
    })

    message.success(t('apps.scanImport.imported', { count: ok }))
    await scan()
  }
  catch {
    message.error(t('apps.scanImport.importFailed'))
  }
  finally {
    importing.value = false
  }
}

onMounted(scan)
</script>

<template>
  <NCard style="border-radius:10px" class="mt-[10px]" size="small">
    <div class="flex items-center justify-between mb-[8px] flex-wrap gap-2">
      <div class="text-slate-500 font-bold text-base">
        {{ t('apps.scanImport.title') }}
      </div>
      <NSpace :size="8">
        <NButton size="small" @click="toggleAll(false)" :disabled="!checked.length">
          {{ t('apps.scanImport.selectNone') }}
        </NButton>
        <NButton size="small" @click="toggleAll(true)" :disabled="!selectable.length">
          {{ t('apps.scanImport.selectNew') }}
        </NButton>
        <NButton size="small" @click="scan" :loading="loading">
          {{ t('apps.scanImport.rescan') }}
        </NButton>
        <NButton
          size="small"
          type="primary"
          :loading="importing"
          :disabled="!checked.length"
          @click="doImport"
        >
          {{ t('apps.scanImport.import') }}
          <template v-if="checked.length">（{{ checked.length }}）</template>
        </NButton>
      </NSpace>
    </div>

    <div class="text-sm text-gray-600 dark:text-gray-400 leading-relaxed mb-[10px]">
      {{ t('apps.scanImport.desc') }}
    </div>

    <!-- 导入进度 -->
    <NAlert v-if="importing" type="info" :bordered="false" class="mb-[10px]">
      {{ t('apps.scanImport.importing', { done: progress.done, total: progress.total }) }}
    </NAlert>

    <div v-if="loading" class="py-[20px] text-center">
      <NSpin size="small" />
      <div class="text-sm text-gray-500 mt-[8px]">{{ t('apps.scanImport.scanning') }}</div>
    </div>

    <NAlert v-else-if="!dockerReady" type="warning" :bordered="false">
      {{ t('apps.scanImport.noDocker') }}
    </NAlert>

    <NEmpty
      v-else-if="!groups.length"
      class="my-[16px]"
      :description="t('apps.scanImport.empty')"
    />

    <!-- 按分类分组 -->
    <div v-else class="max-h-[460px] overflow-y-auto">
      <div v-for="g in groups" :key="g.name" class="mb-[12px]">
        <div class="flex items-center gap-2 mb-[4px]">
          <span class="font-medium text-sm text-gray-700 dark:text-gray-200">{{ g.name }}</span>
          <NTag size="tiny" :bordered="false">{{ g.items.length }}</NTag>
        </div>

        <NCheckboxGroup v-model:value="checked">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-x-[10px] gap-y-[2px]">
            <div
              v-for="c in g.items"
              :key="c.containerId"
              class="flex items-center gap-2 text-xs"
            >
              <NCheckbox :value="c.containerId" :disabled="c.exists">
                <span class="font-mono">{{ c.name }}</span>
              </NCheckbox>
              <span class="text-gray-400 font-mono">:{{ c.port }}</span>
              <NTag v-if="c.exists" size="tiny" type="success" :bordered="false">
                {{ t('apps.scanImport.already') }}
              </NTag>
              <NTag v-else-if="c.state !== 'running'" size="tiny" :bordered="false">
                {{ c.state }}
              </NTag>
            </div>
          </div>
        </NCheckboxGroup>
      </div>
    </div>
  </NCard>
</template>
