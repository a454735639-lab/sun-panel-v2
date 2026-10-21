<script setup lang="ts">
import { NAlert, NButton, NButtonGroup, NCard, NEllipsis, NGrid, NGridItem, NImage, NImageGroup, NSpin, NTabPane, NTabs, useDialog, useMessage } from 'naive-ui'
import { computed, onMounted, ref } from 'vue'
import { deletes, getList, setPublic, setUsage } from '@/api/system/file'
import { set as savePanelConfig } from '@/api/panel/userConfig'
import { RoundCardModal, SvgIcon } from '@/components/common'
import { copyToClipboard, timeFormat } from '@/utils/cmn'
import { t } from '@/locales'
import { usePanelState } from '@/store'
import { ss } from '@/utils/storage'

// 用户配置缓存键
const USER_CONFIG_CACHE_KEY = 'USER_CONFIG_CACHE'

interface InfoModalState {
  title: string
  show: boolean
  fileInfo: File.Info | null
}
// 全量文件列表（未按 tab 过滤）。
// imageList 是「当前 tab 下显示的部分」，由 computed 派生 —— 两者命名上区分开。
const allFiles = ref<File.Info[]>([])
const ms = useMessage()
const dialog = useDialog()
const panelStore = usePanelState()
const loading = ref(false)
const infoModalState = ref<InfoModalState>({
  show: false,
  title: '',
  fileInfo: null,
})

async function getFileList() {
  loading.value = true
  const { data } = await getList<Common.ListResponse<File.Info[]>>()
  allFiles.value = data.list
  loading.value = false
}

/* ---------- 用途分类（全部 / 壁纸 / 图标） ---------- */
const activeTab = ref<'all' | 'wallpaper' | 'icon'>('all')

/**
 * 当前 tab 下要显示的文件。
 *
 * 用 computed 过滤而不是重新请求 —— 文件列表本来就在内存里，
 * 切 tab 再发一次请求既慢又没必要。
 * 但**过滤后要重新按 tab 渲染**，所以 imageList 改为 computed。
 */
const imageList = computed(() => {
  if (activeTab.value === 'wallpaper')
    return allFiles.value.filter(f => f.usage === 1)
  if (activeTab.value === 'icon')
    return allFiles.value.filter(f => f.usage === 2)
  return allFiles.value
})

/** 标记用途：0 未分类 / 1 壁纸 / 2 图标 */
async function handleSetUsage(fileInfo: File.Info, usage: 0 | 1 | 2) {
  // 点已选中的分类 = 取消标记，省一个"清除"按钮
  const next: 0 | 1 | 2 = fileInfo.usage === usage ? 0 : usage
  try {
    const { code, msg } = await setUsage([fileInfo.id as number], next)
    if (code === 0)
      getFileList()
    else
      ms.error(`${t('common.failed')}:${msg}`)
  }
  catch {
    ms.error(t('common.failed'))
  }
}

async function copyImageUrl(text: string) {
  const res = await copyToClipboard(text)
  if (res)
    ms.success(t('apps.uploadsFileManager.copySuccess'))

  else
    ms.error(t('apps.uploadsFileManager.copyFailed'))
}

function handleDelete(id: number) {
  dialog.warning({
    title: t('common.warning'),
    content: t('apps.uploadsFileManager.deleteWarningText'),
    positiveText: t('common.confirm'),
    negativeText: t('common.cancel'),
    onPositiveClick: () => {
      deletesImges(id)
    },
  })
}

async function deletesImges(id: number) {
  try {
    const { code, msg } = await deletes([id])
    if (code === 0) {
      getFileList()
      ms.success(t('common.success'))
    }
    else {
      ms.error(`${t('common.failed')}:${msg}`)
    }
  }
  catch (error) {
    ms.error(t('common.failed'))
  }
}

/**
 * 切换「共享到公共图库」。
 *
 * 共享后文件仍归上传者所有（可随时取消），只是额外出现在公共图库里
 * 供其他账号选用。因此这里用切换而非单向的"发布"。
 */
async function handleToggleShare(fileInfo: File.Info) {
  const next: 0 | 1 = fileInfo.isPublic === 1 ? 0 : 1
  try {
    const { code, msg } = await setPublic([fileInfo.id as number], next)
    if (code === 0) {
      ms.success(next === 1
        ? t('apps.uploadsFileManager.shareSuccess')
        : t('apps.uploadsFileManager.unshareSuccess'))
      getFileList()
    }
    else {
      ms.error(`${t('common.failed')}:${msg}`)
    }
  }
  catch {
    ms.error(t('common.failed'))
  }
}

function handleInfoClick(fileInfo: File.Info) {
  infoModalState.value.fileInfo = fileInfo
  infoModalState.value.show = true
}

function handleSetWallpaper(imgSrc: string) {
  panelStore.panelConfig.backgroundImageSrc = imgSrc
  savePanelConfig({ panel: panelStore.panelConfig }).then((res) => {
    if (res.code === 0) {
      ms.success(t('apps.baseSettings.configSaved'))
      ss.remove(USER_CONFIG_CACHE_KEY)
    }
  })
}

onMounted(() => {
  getFileList()
})
</script>

<template>
  <div class="bg-slate-200 dark:bg-zinc-900 p-2 h-full">
    <NSpin v-show="loading" size="small" />
    <NAlert type="info" :bordered="false">
      {{ $t('apps.uploadsFileManager.alertText') }}
    </NAlert>

    <!-- 用途分类。壁纸与图标是两种不同的使用场景，混在一起找图很费劲。
         分类由用户手动标记 —— 按文件名/尺寸自动推断不准，猜错更难找。 -->
    <NTabs v-model:value="activeTab" type="segment" size="small" class="mt-2">
      <NTabPane name="all" :tab="$t('apps.uploadsFileManager.tabAll')" />
      <NTabPane name="wallpaper" :tab="$t('apps.uploadsFileManager.tabWallpaper')" />
      <NTabPane name="icon" :tab="$t('apps.uploadsFileManager.tabIcon')" />
    </NTabs>
    <div class="flex justify-center mt-2">
      <div v-if="imageList.length === 0 && !loading" class="flex">
        {{ $t('apps.uploadsFileManager.nothingText') }}
      </div>
      <NImageGroup v-else>
        <NGrid cols="2 300:2 600:4 900:6 1100:9" :x-gap="5" :y-gap="5">
          <NGridItem v-for=" item, index in imageList" :key="index">
            <NCard size="small" style="border-radius: 5px;" :bordered="true">
              <template #cover>
                <div class="card transparent-grid">
                  <NImage :lazy="true" style="object-fit: contain;height: 100%;" :src="item.src" />
                </div>
              </template>
              <template #footer>
                <span class="text-xs">
                  <NEllipsis>
                    {{ item.fileName }}
                  </NEllipsis>
                </span>
                <div class="flex justify-center mt-[10px]">
                  <NButtonGroup>
                    <NButton size="tiny" tertiary style="cursor: pointer;" :title="$t('apps.uploadsFileManager.copyLink')" @click="copyImageUrl(item.src)">
                      <template #icon>
                        <SvgIcon icon="ion-copy" />
                      </template>
                    </NButton>
                    <NButton size="tiny" tertiary style="cursor: pointer;" :title="timeFormat(item.createTime)" @click="handleInfoClick(item)">
                      <template #icon>
                        <SvgIcon icon="mdi-information-box-outline" />
                      </template>
                    </NButton>
                    <NButton size="tiny" tertiary style="cursor: pointer;" :title="$t('apps.uploadsFileManager.setWallpaper')" @click="handleSetWallpaper(item.src)">
                      <template #icon>
                        <SvgIcon icon="lucide:wallpaper" />
                      </template>
                    </NButton>
                    <!-- 共享到公共图库。已共享的用实心高亮表示，
                         让用户一眼能看出哪些图是公开的。 -->
                    <NButton
                      size="tiny"
                      tertiary
                      :type="item.isPublic === 1 ? 'primary' : 'default'"
                      :title="item.isPublic === 1 ? $t('apps.uploadsFileManager.unshare') : $t('apps.uploadsFileManager.share')"
                      style="cursor: pointer;"
                      @click="handleToggleShare(item)"
                    >
                      <template #icon>
                        <SvgIcon :icon="item.isPublic === 1 ? 'material-symbols:public' : 'material-symbols:public-off'" />
                      </template>
                    </NButton>
                    <!-- 标记用途。点已选中的分类即取消标记（省一个"清除"按钮）。
                         用图标而非文字，避免把按钮组撑得过宽。 -->
                    <NButton
                      size="tiny"
                      tertiary
                      :type="item.usage === 1 ? 'primary' : 'default'"
                      :title="$t('apps.uploadsFileManager.markWallpaper')"
                      style="cursor: pointer;"
                      @click="handleSetUsage(item, 1)"
                    >
                      <template #icon>
                        <SvgIcon icon="lucide:image" />
                      </template>
                    </NButton>
                    <NButton
                      size="tiny"
                      tertiary
                      :type="item.usage === 2 ? 'primary' : 'default'"
                      :title="$t('apps.uploadsFileManager.markIcon')"
                      style="cursor: pointer;"
                      @click="handleSetUsage(item, 2)"
                    >
                      <template #icon>
                        <SvgIcon icon="material-symbols:category-outline" />
                      </template>
                    </NButton>
                    <NButton size="tiny" tertiary type="error" style="cursor: pointer;" :title="$t('common.delete')" @click="handleDelete(item.id as number)">
                      <template #icon>
                        <SvgIcon icon="material-symbols-delete" />
                      </template>
                    </NButton>
                  </NButtonGroup>
                </div>
              </template>
            </NCard>
          </NGridItem>
        </NGrid>
      </NImageGroup>
    </div>

    <RoundCardModal v-model:show="infoModalState.show" style="max-width: 300px;" size="small" :title="$t('apps.uploadsFileManager.infoTitle')">
      <div>
        <div>
          <div class="mb-2">
            <span class="text-slate-500">
              {{ $t('apps.uploadsFileManager.fileName') }}
            </span>
            <div class="text-xs">
              {{ infoModalState.fileInfo?.fileName }}
            </div>
          </div>
          <div class="mb-2">
            <span class="text-slate-500">
              {{ $t('apps.uploadsFileManager.path') }}
            </span>
            <div class="text-xs">
              {{ infoModalState.fileInfo?.src }}
            </div>
          </div>
          <div class="mb-2">
            <span class="text-slate-500">
              {{ $t('apps.uploadsFileManager.uploadTime') }}
            </span>
            <div class="text-xs">
              {{ timeFormat(infoModalState.fileInfo?.createTime) }}
            </div>
          </div>
        </div>
      </div>
    </RoundCardModal>
  </div>
</template>

<style scoped>
.card {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 80px;
}

.transparent-grid {
  background-image: linear-gradient(45deg, #f0f0f0 25%, transparent 25%, transparent 75%, #f0f0f0 75%),
    linear-gradient(45deg, #f0f0f0 25%, transparent 25%, transparent 75%, #f0f0f0 75%);
  background-size: 16px 16px;
  background-position: 0 0, 8px 8px;
}
</style>
