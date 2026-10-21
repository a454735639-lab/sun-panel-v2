<script setup lang="ts">
import { computed, defineEmits, defineProps, ref, watch } from 'vue'
import type { FormInst, FormRules } from 'naive-ui'
import { NButton, NForm, NFormItem, NGrid, NGridItem, NInput, NInputGroup, NModal, NSelect, NSwitch, NCollapse, NCollapseItem, useMessage } from 'naive-ui'
import IconEditor from './IconEditor.vue'
import { edit, getSiteFavicon } from '@/api/panel/itemIcon'
import { getList as getGroupList } from '@/api/panel/itemIconGroup'
import { getWidgetTypes, type WidgetTypeInfo } from '@/api/panel/widget'
import { t } from '@/locales'

interface Props {
  visible: boolean
  itemInfo: Panel.Info | null
  itemGroupId?: number
}

const props = defineProps<Props>()
const emit = defineEmits<Emit>()
const ms = useMessage()
const submitLoading = ref(false)
const getIconLoading = ref([false, false])
const itemIconGroupOptions = ref<{
  label: string
  value: number
}[]>([])

const restoreDefault: Panel.Info = {
  icon: null,
  title: '',
  url: '',
  lanUrl: '',
  description: '',
  openMethod: 2,
  lanOnly: 0,
}

// 表单数据。**必须在使用它的任何代码之前声明** ——
// <script setup> 是顺序执行的，而 watch/computed 的 getter 会在注册时
// 就被同步执行一次（watch 取初始值、computed 被模板求值）。
// 早先 model 声明在下面的 watch 之后，导致打开卡片编辑器直接抛
// "Cannot access 'model' before initialization"（TDZ）——
// 表现为「点编辑没反应」，且只在控制台可见。
const model = ref<Panel.Info>(props.itemInfo ? { ...props.itemInfo } : { ...restoreDefault })

/* ---------- 小组件配置 ----------
 *
 * 值存在卡片的 widgetType / widgetConfig 两个字段里（后端已支持）：
 *   widgetType   —— 类型标识，如 "qbittorrent"
 *   widgetConfig —— 该类型的参数，JSON 字符串，如 {"url":"...","apikey":"..."}
 *
 * 字段名与提示都由后端 /widget/types 提供，前端不写死 ——
 * 后端新增一种小组件时，这里会自动多出对应的输入框。
 */
const widgetTypes = ref<WidgetTypeInfo[]>([])
const widgetConfigMap = ref<Record<string, string>>({})

const widgetTypeOptions = computed(() =>
  widgetTypes.value.map(w => ({ label: w.label, value: w.value })),
)

/** 当前选中类型的元信息 */
const currentWidgetType = computed(() =>
  widgetTypes.value.find(w => w.value === model.value.widgetType),
)

/** 当前类型需要填哪些字段 */
const currentWidgetFields = computed(() => currentWidgetType.value?.fields || [])

/** 字段名 → 中文标签。后端只给字段名，这里做展示层映射。 */
function fieldLabel(f: string): string {
  const map: Record<string, string> = {
    url: '服务地址',
    apikey: 'API Key',
    username: '用户名',
    password: '密码',
  }
  return map[f] || f
}

/** 密码类字段用 password 输入框，避免明文显示 */
function isSecretField(f: string): boolean {
  return f === 'password' || f === 'apikey'
}

async function loadWidgetTypes() {
  try {
    const res = await getWidgetTypes()
    if (res?.code === 0 && Array.isArray(res.data))
      widgetTypes.value = res.data
  }
  catch {
    // 拉不到类型清单时静默降级：用户仍可编辑其他字段，
    // 只是看不到小组件选项，比弹错误提示更合适
  }
}

/** 把卡片的 widgetConfig（JSON 字符串）解析成表单可编辑的形式 */
function parseWidgetConfig(raw?: string) {
  const out: Record<string, string> = {}
  if (!raw)
    return out
  try {
    const obj = JSON.parse(raw)
    if (obj && typeof obj === 'object') {
      for (const k of Object.keys(obj)) {
        if (obj[k] != null)
          out[k] = String(obj[k])
      }
    }
  }
  catch {
    // 配置损坏时当作空配置，避免整个编辑弹窗打不开
  }
  return out
}

/** 切换类型时清空已填参数 —— 不同服务的字段含义完全不同，
 *  留着上一个类型的值会造成误导（比如把 qB 的地址填进 Emby） */
watch(() => model.value.widgetType, (val, old) => {
  if (val !== old)
    widgetConfigMap.value = {}
})

interface Emit {
  (e: 'update:visible', visible: boolean): void
  (e: 'done', item: Panel.Info): void// 创建完成
}

const formRef = ref<FormInst | null>(null)

const rules: FormRules = {
  title: {
    required: true,
    trigger: 'blur',
    message: t('form.required'),
  },
  url: {
    required: true,
    trigger: 'blur',
    type: 'string',
    message: t('form.required'),
  },
  // itemIconGroupId: {
  //   required: true,
  //   trigger: ['blur', 'change'],
  //   message: t('form.required'),
  // },
}

const options = [
  {
    default: true,
    label: t('iconItem.currentPageOpen'),
    value: 1,
  },
  {
    label: t('iconItem.newWindowOpen'),
    value: 2,
  },
  {
    label: t('iconItem.currentPageLayerOpen'),
    value: 3,
  },
]

// 更新值父组件传来的值
const show = computed({
  get: () => props.visible,
  set: (visible: boolean) => {
    emit('update:visible', visible)
  },
})

async function editApi() {
  submitLoading.value = true
  try {
    // 提交前把小组件参数序列化成 JSON 字符串。
    //
    // 只保留当前类型实际会用到的字段：用户可能先填了 A 类型的参数、
    // 又改成 B 类型，残留的旧字段会被一并提交，造成配置里混入无意义的键。
    // 筛掉既能让存储干净，也避免后端拿到无关参数。
    if (model.value.widgetType) {
      const picked: Record<string, string> = {}
      for (const f of currentWidgetFields.value) {
        const v = widgetConfigMap.value[f]
        if (v !== undefined && v !== '')
          picked[f] = v
      }
      model.value.widgetConfig = Object.keys(picked).length ? JSON.stringify(picked) : ''
    }
    else {
      // 未选类型时清空配置，避免留下孤儿数据
      model.value.widgetConfig = ''
    }

    const { code, data, msg } = await edit<Panel.ItemInfo>(model.value)
    if (code === 0) {
      show.value = false
      model.value = { ...restoreDefault }

      emit('done', data)
    }
    else {
      ms.error(`${t('common.saveFail')}:${msg}`)
    }
  }
  catch (error) {
    ms.error(t('common.saveFail'))
  }
  submitLoading.value = false
}

const handleValidateButtonClick = (e: MouseEvent) => {
  e.preventDefault()
  formRef.value?.validate((errors) => {
    if (!errors)
      editApi()
  })
}

async function getIconByUrl(url: string, loadingIndex: number) {
  getIconLoading.value[loadingIndex] = true
  try {
    const { code, data } = await getSiteFavicon<{ iconUrl: string }>(url)
    if (code === 0) {
      model.value.icon = {
        itemType: 2,
        src: data.iconUrl,
      }
    }
    else {
      ms.error(t('iconItem.geticonFail'))
    }
  }
  catch (error) {
    ms.error(t('iconItem.geticonFail'))
  }
  getIconLoading.value[loadingIndex] = false
}

watch(() => props.visible, (newValue) => {
  if (newValue === true) {
    model.value = props.itemInfo ? { ...props.itemInfo } : { ...restoreDefault }
    if (props.itemGroupId)
      model.value.itemIconGroupId = props.itemGroupId
    // 载入该卡片已有的小组件配置，让用户能看到并修改
    widgetConfigMap.value = parseWidgetConfig(model.value.widgetConfig)
    loadWidgetTypes()
  }

  getGroupListOptions()
})

function getGroupListOptions() {
  getGroupList<Common.ListResponse<Panel.ItemIconGroup[]>>().then(({ data, code, msg }) => {
    if (code === 0) {
      itemIconGroupOptions.value = []

      for (let i = 0; i < data.list.length; i++) {
        const element = data.list[i]
        if (i === 0 && !model.value.itemIconGroupId) {
          model.value.itemIconGroupId = element.id
          restoreDefault.itemIconGroupId = element.id
        }

        itemIconGroupOptions.value.push({
          value: element.id as number,
          label: element.title as string,
        })
      }
    }
    else {
      ms.error(`${t('iconItem.getGroupFail')}:${msg}`)
    }
  })
}
</script>

<template>
  <NModal v-model:show="show" preset="card" size="small" style="width: 600px;border-radius: 1rem;" :title="itemInfo ? t('iconItem.edit') : t('iconItem.add')">
    <div class="h-[600px] overflow-auto p-[5px]">
      <NForm ref="formRef" :model="model" :rules="rules">
        <NGrid cols="2" :x-gap="10" item-responsive>
          <NGridItem span="2 500:1">
            <NFormItem path="itemIconGroupId" :label="t('iconItem.iconGroup')">
              <NSelect v-model:value="model.itemIconGroupId" :options="itemIconGroupOptions" />
            </NFormItem>
          </NGridItem>
          <NGridItem span="2 500:1">
            <NFormItem path="title" :label="$t('common.title')">
              <NInput v-model:value="model.title" type="text" show-count :maxlength="20" />
            </NFormItem>
          </NGridItem>
        </NGrid>

        <NFormItem path="icon" :label="$t('common.icon')">
          <IconEditor v-model:item-icon="model.icon" />
        </NFormItem>
        <NFormItem path="url" :label="$t('iconItem.url')">
          <!-- <NSelect :style="{ width: '100px' }" :options="urlProtocolOptions" /> -->
          <NInputGroup>
            <NInput v-model:value="model.url" type="text" :maxlength="1000" placeholder="http(s)://" />
            <NButton :disabled="!model.url" :loading="getIconLoading[0]" @click="getIconByUrl(model.url, 0)">
              {{ $t('iconItem.getIcon') }}
            </NButton>
          </NInputGroup>
        </NFormItem>
        <NFormItem path="lanUrl" :label="$t('iconItem.lanUrl')">
          <NInputGroup>
            <NInput v-model:value="model.lanUrl" type="text" :maxlength="1000" :placeholder="$t('iconItem.lanUrlInputPlaceholder')" />
            <NButton :disabled="!model.lanUrl" :loading="getIconLoading[1]" @click="getIconByUrl(model.lanUrl || '', 1)">
              {{ $t('iconItem.getIcon') }}
            </NButton>
          </NInputGroup>
        </NFormItem>
        <NFormItem path="description" :label="$t('common.description')">
          <NInput v-model:value="model.description" type="text" show-count :maxlength="100" />
        </NFormItem>
        <NFormItem path="openMethod" :label="$t('iconItem.openMethod')">
          <NSelect v-model:value="model.openMethod" :options="options" />
        </NFormItem>
        <NFormItem path="lanOnly" :label="$t('iconItem.lanOnly')">
          <NSwitch v-model:value="model.lanOnly" :checked-value="1" :unchecked-value="0" />
        </NFormItem>

        <!-- 小组件：在卡片下方显示实时状态（下载进度、媒体库统计等）。
             默认折叠 —— 多数卡片不需要，展开会拉长表单。 -->
        <NCollapse class="mt-[4px]">
          <NCollapseItem :title="$t('iconItem.widget')" name="widget">
            <NFormItem :label="$t('iconItem.widgetType')">
              <NSelect
                v-model:value="model.widgetType"
                :options="widgetTypeOptions"
                :placeholder="$t('iconItem.widgetTypePlaceholder')"
                clearable
              />
            </NFormItem>

            <!-- 类型说明：由后端提供，告诉用户该填什么、去哪拿 Key -->
            <div
              v-if="currentWidgetType?.hint"
              class="text-xs text-gray-500 mb-[10px] -mt-[6px] leading-relaxed"
            >
              {{ currentWidgetType.hint }}
            </div>

            <!-- 参数字段按后端声明的 fields 动态生成。
                 后端加新类型时这里自动出现对应输入框，前端无需改动。 -->
            <NFormItem
              v-for="f in currentWidgetFields"
              :key="f"
              :label="fieldLabel(f)"
            >
              <NInput
                v-model:value="widgetConfigMap[f]"
                :type="isSecretField(f) ? 'password' : 'text'"
                :show-password-on="isSecretField(f) ? 'click' : undefined"
                :placeholder="isSecretField(f) ? '••••••' : ''"
                clearable
              />
            </NFormItem>

            <div v-if="model.widgetType" class="text-xs text-gray-400 leading-relaxed">
              {{ $t('iconItem.widgetHint') }}
            </div>
          </NCollapseItem>
        </NCollapse>
      </NForm>
    </div>

    <template #footer>
      <NButton type="success" :loading="submitLoading" style="float: right;" @click="handleValidateButtonClick">
        {{ $t('common.save') }}
      </NButton>
    </template>
  </NModal>
</template>
