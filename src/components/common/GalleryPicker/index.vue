<script setup lang="ts">
/**
 * 图库选择器：从已上传的图片里挑一张。
 *
 * 存在意义：卡片图标此前只能"填 URL"或"重新上传"。
 * 用户图库里可能已经有合适的图（甚至就是为这张卡传的），
 * 却只能再传一遍 —— 既浪费存储，也让图库里堆重复文件。
 *
 * 设计取舍：
 *
 * 1. 只在需要时才加载列表（`show` 变为 true 时拉取）
 *    图库可能有很多图，每次打开编辑弹窗都拉一遍没必要。
 *
 * 2. 只显示图片类文件
 *    图库接口返回的是所有上传文件，但这里选的是图标，
 *    过滤掉非图片避免用户误选到别的类型。
 *
 * 3. 选中即回填 URL 并关闭
 *    不引入"多选"或"确认"步骤 —— 选图这个动作本身就是确认。
 */
import { ref, watch, computed } from 'vue'
import { NEmpty, NSpin, NModal, NInput, useMessage } from 'naive-ui'
import { getList as getFileList } from '@/api/system/file'
import { t } from '@/locales'

const props = defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', v: boolean): void
  (e: 'select', src: string): void
}>()

const ms = useMessage()
const loading = ref(false)
const files = ref<File.Info[]>([])
const keyword = ref('')

/**
 * 是否为图片。
 *
 * 判断依据取「ext 字段」与「文件名后缀」中**任意一个**成立，
 * 而不是只信 ext —— 后端 GetList 原先手工拼返回结构时漏掉了 ext，
 * 导致所有文件都被判为非图片、选择器一片空白。
 * 多一个来源更稳：任一处能看出来是图片即可。
 */
function isImage(f: File.Info): boolean {
  const IMG = ['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp', 'ico', 'avif']

  const norm = (s?: string) => (s || '').replace(/^\./, '').toLowerCase()

  const byExtField = norm(f.ext)
  if (byExtField && IMG.includes(byExtField))
    return true

  // 回落到解析 src / fileName 的后缀
  for (const candidate of [f.src, f.fileName]) {
    const m = /\.([a-z0-9]+)(?:\?.*)?$/i.exec(candidate || '')
    if (m && IMG.includes(m[1].toLowerCase()))
      return true
  }
  return false
}

const images = computed(() => files.value.filter(isImage))

/** 按文件名过滤，便于在图多时快速定位 */
const filtered = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  if (!k)
    return images.value
  return images.value.filter(f => (f.fileName || '').toLowerCase().includes(k))
})

async function load() {
  loading.value = true
  try {
    const res = await getFileList<{ list: File.Info[] }>()
    if (res?.code === 0 && res.data?.list)
      files.value = res.data.list
    else
      files.value = []
  }
  catch {
    // silent 未开，这里给用户一个提示即可
    ms.warning(t('apps.galleryPicker.loadFailed'))
    files.value = []
  }
  finally {
    loading.value = false
  }
}

// 打开时才加载；关闭时清空搜索词，下次打开是干净状态
watch(() => props.show, (v) => {
  if (v) {
    keyword.value = ''
    load()
  }
})

function pick(f: File.Info) {
  emit('select', f.src)
  emit('update:show', false)
}

function close() {
  emit('update:show', false)
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="t('apps.galleryPicker.title')"
    style="max-width: 720px"
    @update:show="close"
  >
    <NInput
      v-model:value="keyword"
      clearable
      size="small"
      :placeholder="t('apps.galleryPicker.searchPlaceholder')"
      class="mb-[10px]"
    />

    <div v-if="loading" class="py-[30px] text-center">
      <NSpin size="small" />
    </div>

    <NEmpty
      v-else-if="!filtered.length"
      class="py-[30px]"
      :description="images.length ? t('apps.galleryPicker.noMatch') : t('apps.galleryPicker.empty')"
    />

    <div v-else class="gallery-pick-grid">
      <div
        v-for="f in filtered"
        :key="f.id"
        class="gallery-pick-item"
        :title="f.fileName"
        @click="pick(f)"
      >
        <img :src="f.src" :alt="f.fileName" loading="lazy">
        <div class="gallery-pick-name">{{ f.fileName }}</div>
      </div>
    </div>
  </NModal>
</template>

<style scoped>
/* 固定列宽的自适应网格：图标是方形的，用等宽格子最整齐 */
.gallery-pick-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(96px, 1fr));
  gap: 10px;
  max-height: 52vh;
  overflow-y: auto;
  padding-right: 4px;
}

.gallery-pick-item {
  cursor: pointer;
  border-radius: 10px;
  padding: 6px;
  transition: background 0.15s;
  text-align: center;
}

.gallery-pick-item:hover {
  background: rgba(128, 128, 128, 0.16);
}

.gallery-pick-item img {
  width: 100%;
  height: 64px;
  object-fit: contain;
  display: block;
  border-radius: 6px;
  background: rgba(128, 128, 128, 0.1);
}

.gallery-pick-name {
  margin-top: 4px;
  font-size: 0.68rem;
  opacity: 0.75;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>