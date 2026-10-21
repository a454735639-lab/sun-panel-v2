<script setup lang="ts">
/**
 * 公共图库：浏览所有账号共享的图片。
 *
 * 与「上传文件管理」的区别：
 *   · 只读为主 —— 别人分享的图不该被我删掉
 *   · 不显示上传/删除按钮，只提供「复制链接」「设为壁纸」这类使用动作
 *   · 标注上传者，便于辨认来源
 *
 * 权限边界：这里的图确实对所有登录账号可见（这正是"公共"的含义），
 * 但删除仍只能在「上传文件管理」里由上传者本人执行 ——
 * 后端 Deletes 也带 user_id 条件，双端一致。
 */
import { onMounted, ref, computed } from 'vue'
import { NButton, NCard, NEmpty, NInput, NSpin, NTag, useMessage } from 'naive-ui'
import { getPublicList } from '@/api/system/file'
import { SvgIcon } from '@/components/common'
import { set as savePanelConfig } from '@/api/panel/userConfig'
import { usePanelState } from '@/store'
import { copyToClipboard } from '@/utils/cmn'
import { t } from '@/locales'

const ms = useMessage()
const panelState = usePanelState()

const loading = ref(false)
const files = ref<File.Info[]>([])
const keyword = ref('')

/** 只保留图片（与图库选择器同一判定口径：ext 或文件名后缀任一成立） */
function isImage(f: File.Info): boolean {
  const IMG = ['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp', 'ico', 'avif']
  const norm = (s?: string) => (s || '').replace(/^\./, '').toLowerCase()
  if (IMG.includes(norm(f.ext)))
    return true
  for (const c of [f.src, f.fileName]) {
    const m = /\.([a-z0-9]+)(?:\?.*)?$/i.exec(c || '')
    if (m && IMG.includes(m[1].toLowerCase()))
      return true
  }
  return false
}

const images = computed(() => files.value.filter(isImage))

const filtered = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  if (!k)
    return images.value
  return images.value.filter(f => (f.fileName || '').toLowerCase().includes(k))
})

async function load() {
  loading.value = true
  try {
    const res = await getPublicList<{ list: File.Info[] }>()
    files.value = res?.code === 0 && res.data?.list ? res.data.list : []
  }
  catch {
    ms.warning(t('apps.publicGallery.loadFailed'))
    files.value = []
  }
  finally {
    loading.value = false
  }
}

async function copyImageUrl(src: string) {
  const ok = await copyToClipboard(window.location.origin + src)
  if (ok)
    ms.success(t('apps.uploadsFileManager.copySuccess'))
  else
    ms.error(t('apps.uploadsFileManager.copyFailed'))
}

/** 设为面板壁纸 —— 与「上传文件管理」里的行为一致 */
async function setWallpaper(src: string) {
  panelState.panelConfig.backgroundImageSrc = src
  // 只传 panel：userConfig 的 set 接口对 searchEngine 会单独处理，
  // 传 undefined 会被当成"清空"，导致用户的搜索引擎配置丢失。
  const res = await savePanelConfig({ panel: panelState.panelConfig })
  if (res.code === 0) {
    ms.success(t('apps.uploadsFileManager.setWallpaperSuccess'))
    panelState.recordState()
    // 清缓存，让首页下次加载拿到新壁纸
    try {
      localStorage.removeItem('USER_CONFIG_CACHE')
    }
    catch { /* 忽略 */ }
  }
  else {
    ms.error(t('common.saveFail'))
  }
}

onMounted(load)
</script>

<template>
  <div class="bg-slate-200 dark:bg-zinc-900 rounded-[10px] p-[8px] overflow-auto">
    <NCard style="border-radius:10px" size="small">
      <div class="flex items-center justify-between mb-[6px] flex-wrap gap-2">
        <div class="text-slate-500 font-bold text-base">
          {{ t('apps.publicGallery.title') }}
        </div>
        <div class="flex items-center gap-2">
          <NInput
            v-model:value="keyword"
            size="small"
            clearable
            style="width: 160px"
            :placeholder="t('apps.publicGallery.searchPlaceholder')"
          />
          <NButton size="small" :loading="loading" @click="load">
            {{ t('common.refreshPage') }}
          </NButton>
        </div>
      </div>

      <div class="text-sm text-gray-600 dark:text-gray-400 leading-relaxed mb-[10px]">
        {{ t('apps.publicGallery.desc') }}
      </div>

      <div v-if="loading" class="py-[30px] text-center">
        <NSpin size="small" />
      </div>

      <NEmpty
        v-else-if="!filtered.length"
        class="py-[30px]"
        :description="images.length ? t('apps.galleryPicker.noMatch') : t('apps.publicGallery.empty')"
      />

      <div v-else class="gallery-grid-public">
        <div v-for="f in filtered" :key="f.id" class="public-item">
          <div class="public-thumb">
            <img :src="f.src" :alt="f.fileName" loading="lazy">
          </div>
          <div class="public-meta">
            <span class="public-name" :title="f.fileName">{{ f.fileName }}</span>
            <NTag v-if="f.userId" size="tiny" :bordered="false">
              #{{ f.userId }}
            </NTag>
          </div>
          <div class="flex justify-center mt-[6px] gap-1">
            <NButton size="tiny" tertiary :title="t('apps.uploadsFileManager.copyLink')" @click="copyImageUrl(f.src)">
              <template #icon>
                <SvgIcon icon="ion-copy" />
              </template>
            </NButton>
            <NButton size="tiny" tertiary :title="t('apps.uploadsFileManager.setWallpaper')" @click="setWallpaper(f.src)">
              <template #icon>
                <SvgIcon icon="lucide:wallpaper" />
              </template>
            </NButton>
          </div>
        </div>
      </div>
    </NCard>
  </div>
</template>

<style scoped>
.gallery-grid-public {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 12px;
}

.public-item {
  background: rgba(128, 128, 128, 0.08);
  border-radius: 10px;
  padding: 8px;
}

.public-thumb {
  height: 96px;
  border-radius: 8px;
  overflow: hidden;
  background: rgba(128, 128, 128, 0.12);
  display: flex;
  align-items: center;
  justify-content: center;
}

.public-thumb img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

.public-meta {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 6px;
}

.public-name {
  font-size: 0.7rem;
  opacity: 0.8;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}
</style>