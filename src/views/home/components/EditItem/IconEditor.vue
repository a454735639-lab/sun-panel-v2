<script setup lang="ts">
import { NButton, NColorPicker, NInput, NRadio, NUpload } from 'naive-ui'
import type { UploadFileInfo } from 'naive-ui'
import { computed, defineProps, ref } from 'vue'
import { GalleryPicker, ItemIcon } from '@/components/common'
import { useAuthStore } from '@/store'
import { apiRespErrMsg } from '@/utils/request/apiMessage'

const props = defineProps<{
  itemIcon: Panel.ItemIcon | null
}>()
const emit = defineEmits<{
  (e: 'update:itemIcon', visible: Panel.ItemIcon): void // 定义修改父组件（prop内）的值的事件
}>()
const authStore = useAuthStore()

// 默认图标背景色
const defautSwatchesBackground = [
  '#00000000',
  '#000000',
  '#ffffff',
  '#18A058',
  '#2080F0',
  '#F0A020',
  'rgba(208, 48, 80, 1)',
  '#C418D1FF',
]

const initData: Panel.ItemIcon = {
  itemType: 2,
  backgroundColor: '#2a2a2a6b',
}

const itemIconInfo = computed({
  get() {
    const v = {
      ...initData,
      ...props.itemIcon,
      backgroundColor: props.itemIcon?.backgroundColor || initData.backgroundColor,
    }
    return v
  },
  set() {
    handleChange()
  },
})

function handleIconTypeRadioChange(type: number) {
  // checkedValueRef.value = type
  itemIconInfo.value.itemType = type
  handleChange()
}

function handleChange() {
  emit('update:itemIcon', itemIconInfo.value || null)
}

function handleResetBackgroundColor() {
  itemIconInfo.value.backgroundColor = initData.backgroundColor
  handleChange()
}

const handleUploadFinish = ({
  file,
  event,
}: {
  file: UploadFileInfo
  event?: ProgressEvent
}) => {
  const res = JSON.parse((event?.target as XMLHttpRequest).response)
  if (res.code === 0) {
    const imageUrl = res.data.imageUrl
    itemIconInfo.value.src = imageUrl
    emit('update:itemIcon', itemIconInfo.value || null)
  }
  else {
    apiRespErrMsg(res)
    // ms.error(`${t('common.uploadFail')}:${res.msg}`)
  }

  return file
}

/* ---------- 从图库选择 ---------- */
const galleryShow = ref(false)

/**
 * 图库选中的图片回填。
 *
 * 与上传完成走同一条路径：都是把地址写进 src 并向父组件 emit。
 * 不额外引入"图片来源"这种状态 —— 图片一旦选定就只是一个 URL，
 * 区分来源没有意义，反而会让后续的编辑与保存逻辑分叉。
 */
function handleGallerySelect(src: string) {
  itemIconInfo.value.src = src
  emit('update:itemIcon', itemIconInfo.value || null)
}
</script>

<template>
  <div>
    <div class="mb-[10px]">
      <NRadio
        :checked="itemIconInfo.itemType === 1 "
        :value="1"
        name="iconType"
        @change="handleIconTypeRadioChange(1)"
      >
        {{ $t('common.text') }}
      </NRadio>

      <NRadio
        :checked="itemIconInfo.itemType === 2"
        :value="2"
        name="iconType"
        @change="handleIconTypeRadioChange(2)"
      >
        {{ $t('common.image') }}
      </NRadio>

      <NRadio
        :checked="itemIconInfo.itemType === 3"
        :value="3"
        name="iconType"
        @change="handleIconTypeRadioChange(3)"
      >
        {{ $t('iconItem.onlineIcon') }}
      </NRadio>
    </div>

    <div class=" h-[100px]">
      <div class="flex">
        <div>
          <div class="border rounded-2xl bg-slate-200 overflow-hidden rounded-2xl transparent-grid">
            <ItemIcon :item-icon="itemIconInfo" />
          </div>
        </div>
        <!-- 文字 -->
        <div class="ml-[20px]">
          <!-- <NImage :src="model.icon" preview-disabled /> -->
          <div v-if="itemIconInfo.itemType === 1">
            <NInput v-model:value="itemIconInfo.text" class="mb-[5px]" size="small" type="text" @input="handleChange" />
          </div>

          <div v-if="itemIconInfo.itemType === 3">
            <div>
              <NInput v-model:value="itemIconInfo.text" class="mb-[5px]" size="small" type="text" :placeholder="$t('iconItem.inputIconName')" @input="handleChange" />

              <NButton quaternary type="info">
                <a target="_blank" href="https://icon-sets.iconify.design/">{{ $t('iconItem.onlineIconLibrary') }}</a>
              </NButton>
            </div>
          </div>

          <!-- 图片 -->
          <div v-if="itemIconInfo.itemType === 2">
            <NInput v-model:value="itemIconInfo.src" class="mb-[5px] w-full" size="small" type="text" :placeholder="$t('iconItem.inputIconUrlOrUpload')" @input="handleChange" />
            <div class="flex items-center gap-2">
              <NUpload
                action="/api/file/uploadImg"
                :show-file-list="false"
                name="imgfile"
                :headers="{
                  token: authStore.token as string,
                }"
                @finish="handleUploadFinish"
              >
                <NButton size="small">
                  {{ $t('iconItem.selectUpload') }}
                </NButton>
              </NUpload>

              <!-- 从图库选：图片此前只能"填 URL"或"重新上传"，
                   图库里已有的图无法直接复用。 -->
              <NButton size="small" @click="galleryShow = true">
                {{ $t('iconItem.selectFromGallery') }}
              </NButton>
            </div>
          </div>
        </div>
      </div>

      <div class="flex items-center mt-[10px]">
        <div class="w-auto text-slate-500 mr-[10px]">
          {{ $t('common.backgroundColor') }}
        </div>
        <div class="w-[150px] flex items-center mr-[10px]">
          <NColorPicker
            v-model:value="itemIconInfo.backgroundColor"
            size="small"
            :modes="['hex']"
            :swatches="defautSwatchesBackground"
            @complete="handleChange"
            @update-value="handleChange"
          />
        </div>
        <div v-if="itemIconInfo.backgroundColor !== initData.backgroundColor" class="w-auto text-slate-500 mr-[10px] cursor-pointer">
          <NButton quaternary type="info" @click="handleResetBackgroundColor">
            {{ $t('common.reset') }}
          </NButton>
        </div>
      </div>
    </div>

    <!-- 图库选择器：选中后回填到 src，与手工填 URL 走同一条路径 -->
    <GalleryPicker v-model:show="galleryShow" @select="handleGallerySelect" />
  </div>
</template>

<style scoped>
.transparent-grid {
    background-image: linear-gradient(45deg, #fff 25%, transparent 25%, transparent 75%, #fff 75%),
                      linear-gradient(45deg, #fff 25%, transparent 25%, transparent 75%, #fff 75%);
    background-size: 16px 16px;
    background-position: 0 0, 8px 8px;
}
</style>
