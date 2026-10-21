<script setup lang="ts">
import { onMounted, ref } from 'vue'
import type { FormInst, FormRules } from 'naive-ui'
import { NButton, NCard, NColorPicker, NForm, NFormItem, NInput, NSelect, NSwitch, useDialog, useMessage } from 'naive-ui'
import { VueDraggable } from 'vue-draggable-plus'
import { deletes, edit, getList, saveSort } from '@/api/panel/itemIconGroup'
import { RoundCardModal, SvgIcon } from '@/components/common'
import { t } from '@/locales'
import { ss } from '@/utils/storage/local'
import {
  defaultGroupCardStyle, parseGroupCardStyle, stringifyGroupCardStyle,
  GroupCardStyleEnum, type GroupCardStyle,
} from '@/utils/groupCardStyle'

interface EditModalArg {
  show: boolean
  editStatus: number // 1.添加 2.编辑
  model: Panel.ItemIconGroup
  rules: FormRules
}

const formRef = ref<FormInst | null>(null)
const ms = useMessage()
const dialog = useDialog()
const sortStatus = ref(false)

const defaultMNodal = {
  title: '',
  icon: 'material-symbols:folder-outline',
  sort: 9999,
}

const editModalArg = ref<EditModalArg>({
  show: false,
  editStatus: 1,
  model: defaultMNodal,
  rules: {
    title: [
      {
        required: true,
        trigger: 'blur',
        message: t('form.required'),
      },
    ],
  },
})

const groups = ref<Panel.ItemIconGroup[]>([])

/* ---------- 分组的卡片风格 ----------
 *
 * 每个分组可单独设风格：NAS 应用用「详情图标」（能显示 CPU/内存），
 * 站点收藏用「小图标」（一屏放更多）。存在 card_style 字段里。
 *
 * 下面的 styleModel 用 -1 表示「跟随全局」—— 与后端默认行为一致
 * （未设过样式的老分组不回落到某个固定值，而是沿用全局设置，
 * 这样用户改全局风格时，没单独设过的分组会一起跟着变）。
 */
const FOLLOW_GLOBAL = -1

const styleModel = ref({
  style: FOLLOW_GLOBAL as number,
  textColor: '',
  infoHideDescription: false,
  iconHideTitle: false,
})

const styleOptions = [
  { label: '跟随全局设置', value: FOLLOW_GLOBAL },
  { label: '详情图标（大卡，可显示状态）', value: GroupCardStyleEnum.info },
  { label: '小图标（一屏更多）', value: GroupCardStyleEnum.icon },
]

/** 打开弹窗时，把该分组已有的 cardStyle 解析进表单 */
function loadStyleFrom(group?: Panel.ItemIconGroup) {
  if (!group || !group.cardStyle || !group.cardStyle.trim() || group.cardStyle.trim() === 'null') {
    // 从未设过 → 跟随全局
    styleModel.value = { style: FOLLOW_GLOBAL, textColor: '', infoHideDescription: false, iconHideTitle: false }
    return
  }
  const gs = parseGroupCardStyle(group.cardStyle)
  styleModel.value = {
    style: gs.style,
    textColor: gs.textColor,
    infoHideDescription: gs.textInfoHideDescription,
    iconHideTitle: gs.textIconHideTitle,
  }
}

/** 提交前把表单转回 card_style 字符串；跟随全局则存空串 */
function buildCardStyle(): string {
  if (styleModel.value.style === FOLLOW_GLOBAL)
    return ''
  const v: GroupCardStyle = {
    ...defaultGroupCardStyle(),
    style: styleModel.value.style as GroupCardStyleEnum,
    textColor: styleModel.value.textColor || '',
    textInfoHideDescription: styleModel.value.infoHideDescription,
    textIconHideTitle: styleModel.value.iconHideTitle,
  }
  return stringifyGroupCardStyle(v)
}

function handleAddGroup() {
  editModalArg.value.show = !editModalArg.value.show
  // 新建分组默认「跟随全局」
  loadStyleFrom(undefined)
  // Clear group list cache
  ss.remove('GROUP_LIST_CACHE')
}

function handleEditGroup(groupInfo: Panel.ItemIconGroup) {
  editModalArg.value.show = true
  editModalArg.value.model = groupInfo
  editModalArg.value.editStatus = 2
  // 把该分组已有的卡片风格解析进表单
  loadStyleFrom(groupInfo)
}

function handleDragSort() {
  sortStatus.value = true
}

function handleSaveSort() {
  const saveItems: Common.SortItemRequest[] = []
  for (let i = 0; i < groups.value.length; i++) {
    const element = groups.value[i]
    saveItems.push({
      id: element.id as number,
      sort: i + 1,
    })
  }
  saveSort(saveItems).then(({ code, msg }) => {
    if (code === 0) {
      ms.success(t('common.saveSuccess'))
      // 清除分组列表缓存
      ss.remove('GROUP_LIST_CACHE')
      sortStatus.value = false
    }
    else {
      ms.error(`${t('common.saveFail')}:${msg}`)
    }
  })
}

function handleDelete(groupInfo: Panel.ItemIconGroup) {
  dialog.warning({
    title: t('common.warning'),
    content: t('apps.itemGroupManage.deleteWarnText', { name: groupInfo.title }),
    positiveText: t('common.confirm'),
    negativeText: t('common.cancel'),
    onPositiveClick: () => {
      if (groupInfo.id) {
        deletes([groupInfo.id]).then(({ code, msg }) => {
          if (code !== 0)
            ms.error(t('common.deleteFail'))
          else {
            // 清除分组列表缓存
            ss.remove('GROUP_LIST_CACHE')
            refreshList()
          }
        })
      }
    },

  })
}

function handleSaveGroup() {
  formRef.value?.validate((errors) => {
    if (!errors) {
      // 把卡面风格写回 card_style 字段（后端一直支持，此前前端未使用）
      editModalArg.value.model.cardStyle = buildCardStyle()
      edit(editModalArg.value.model).then(({ code, msg }) => {
        if (code !== 0)
          ms.error(msg)
        
        // 清除分组列表缓存
        ss.remove('GROUP_LIST_CACHE')
        refreshList()
        editModalArg.value.show = false
        editModalArg.value.model = { ...defaultMNodal }
      })
    }
    else { console.log(errors) }
  })
}

function refreshList() {
  getList<Common.ListResponse<Panel.ItemIconGroup[]>>().then(({ code, data }) => {
    groups.value = data.list
  })
}

onMounted(() => {
  refreshList()
})
</script>

<template>
  <div class="h-full">
    <div class="p-2">
      <NButton type="success" size="small" style="margin-right: 10px;" @click="handleAddGroup">
        {{ $t('common.add') }}
      </NButton>

      <NButton v-if="!sortStatus" size="small" @click="handleDragSort">
        {{ $t('common.sort') }}
      </NButton>

      <NButton v-else type="warning" size="small" @click="handleSaveSort">
        {{ $t('common.saveSort') }}
      </NButton>
    </div>

    <div class=" overflow-auto w-full mt-[20px]  bg-slate-200 dark:bg-zinc-900 rounded-xl" style="height:calc(100% - 65px)">
      <VueDraggable
        v-model="groups"
        item-key="sort" :animation="300"
        :style="{ padding: sortStatus ? '20px' : '10px' }"
        :disabled="!sortStatus"
      >
        <div v-for="(item, index) in groups" :key="index" class="w-full">
          <NCard size="small" style="border-radius:10px;margin-bottom: 10px;">
            <div class="flex" :class="sortStatus ? 'cursor-move' : ''">
              <div class="flex items-center">
                <span class="mr-[10px]">
                  <SvgIcon class="text-[20px]" icon="material-symbols:ad-group-outline-rounded" />
                  <!-- <SvgIcon class="text-[20px]" :icon="item.icon" /> -->
                </span>
                <span>
                  {{ item.title }}
                </span>
              </div>
              <div class="ml-auto">
                <span>
                  <NButton strong secondary type="success" size="small" @click="handleEditGroup(item)">
                    <template #icon>
                      <SvgIcon icon="basil:edit-solid" />
                    </template>
                  </NButton>
                </span>
                <span class="ml-[10px]">
                  <NButton strong secondary type="error" size="small" class="ml-[10px]" @click="handleDelete(item)">
                    <template #icon>
                      <SvgIcon icon="material-symbols:delete" />
                    </template>
                  </NButton>
                </span>
              </div>
            </div>
          </NCard>
        </div>
      </VueDraggable>
    </div>

    <RoundCardModal v-model:show="editModalArg.show" size="small" type="small" :title="editModalArg.editStatus === 1 ? '添加' : '编辑'" style="width: 400px;">
      <NForm ref="formRef" :model="editModalArg.model" :rules="editModalArg.rules">
        <NFormItem path="title" :label="$t('apps.itemGroupManage.groupName')">
          <NInput v-model:value="editModalArg.model.title" type="text" :maxlength="20" show-count />
        </NFormItem>

        <!-- 卡片风格：分组级设置。
             让一个面板里能混用「NAS 应用用大卡（显示状态）、站点用小图标（一屏更多）」。 -->
        <NFormItem :label="$t('apps.itemGroupManage.cardStyle')">
          <NSelect v-model:value="styleModel.style" :options="styleOptions" />
        </NFormItem>

        <!-- 仅当分组单独设了风格时才出现后续细节 ——
             跟随全局时这些值无意义，露出来只会让用户以为改了有用。 -->
        <template v-if="styleModel.style !== FOLLOW_GLOBAL">
          <NFormItem :label="$t('apps.itemGroupManage.textColor')">
            <NColorPicker
              v-model:value="styleModel.textColor"
              :show-alpha="false"
              size="small"
              :modes="['hex']"
              :swatches="['#ffffff', '#000000', '#f5f5f5', '#ffd666', '#69c0ff', '#95de64', '#ff9c6e']"
            />
          </NFormItem>
          <div class="text-xs text-gray-500 -mt-[8px] mb-[10px] leading-relaxed">
            留空表示跟随全局文字颜色
          </div>

          <NFormItem v-if="styleModel.style === GroupCardStyleEnum.info" :label="$t('apps.baseSettings.hideDescription')">
            <NSwitch v-model:value="styleModel.infoHideDescription" />
          </NFormItem>

          <NFormItem v-if="styleModel.style === GroupCardStyleEnum.icon" :label="$t('apps.baseSettings.hideTitle')">
            <NSwitch v-model:value="styleModel.iconHideTitle" />
          </NFormItem>
        </template>

        <!-- <NFormItem path="name" label="昵称">
          <NInput v-model:value="editModalArg.model" type="text" placeholder="请输入昵称" />
        </NFormItem> -->
      </NForm>
      <template #footer>
        <NButton type="success" size="small" class="float-right" @click="handleSaveGroup">
          {{ $t('common.confirm') }}
        </NButton>
      </template>
    </RoundCardModal>
  </div>
</template>
