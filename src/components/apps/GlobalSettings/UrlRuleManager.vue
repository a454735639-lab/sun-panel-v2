<script setup lang="ts">
/**
 * 地址匹配规则管理。
 *
 * 为什么需要它：卡片本身只区分"内网地址 / 外网地址"两个字段，
 * 靠 IP 判断二选一。但真实场景常常更细 ——
 * "这个站点内网走 A、外网走 B、另一个域名走 C"，
 * 甚至同一站点在不同端口下用不同地址。规则表就是为此而设。
 *
 * 匹配语义（与后端一致）：
 *   - 按 sort 升序逐条尝试，命中第一条即用其 target
 *   - pattern 支持通配符 * 与端口（如 *.example.com:8080）
 *   - 全部不命中则回落到内置的内外网 IP 判断
 */
import { ref, onMounted, computed } from 'vue'
import {
  NButton, NCard, NInput, NSwitch, NModal, NForm, NFormItem,
  NInputGroup, NEmpty, NSpace, useMessage, useDialog,
} from 'naive-ui'
import { getList, edit, deletes, test, type UrlRule } from '@/api/panel/urlRule'
import { t } from '@/locales'

const message = useMessage()
const dialog = useDialog()

const loading = ref(false)
const rules = ref<UrlRule[]>([])

// 编辑弹窗
const showEditor = ref(false)
const saving = ref(false)
const form = ref<Partial<UrlRule>>({
  id: undefined,
  pattern: '',
  target: '',
  description: '',
  enabled: 1,
  sort: 0,
})

// 试跑
const testUrl = ref('')
const testResult = ref<{ hit: boolean; target: string; rule?: UrlRule } | null>(null)
const testing = ref(false)

const isNew = computed(() => !form.value.id)

async function load() {
  loading.value = true
  try {
    const res = await getList()
    rules.value = res.data?.list || []
  } catch (e: any) {
    message.error(e?.message || '加载规则失败')
  } finally {
    loading.value = false
  }
}

function openNew() {
  form.value = { id: undefined, pattern: '', target: '', description: '', enabled: 1, sort: 0 }
  showEditor.value = true
}

function openEdit(r: UrlRule) {
  form.value = { ...r }
  showEditor.value = true
}

async function save() {
  const pattern = (form.value.pattern || '').trim()
  const target = (form.value.target || '').trim()
  if (!pattern || !target) {
    message.warning(t('apps.urlRule.patternTargetRequired'))
    return
  }
  saving.value = true
  try {
    await edit({ ...form.value, pattern, target })
    message.success(isNew.value ? t('apps.urlRule.added') : t('apps.urlRule.saved'))
    showEditor.value = false
    await load()
  } catch (e: any) {
    message.error(e?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(r: UrlRule) {
  try {
    await edit({ ...r, enabled: r.enabled === 1 ? 0 : 1 })
    await load()
  } catch (e: any) {
    message.error(e?.message || '操作失败')
  }
}

function remove(r: UrlRule) {
  dialog.warning({
    title: t('apps.urlRule.confirmDelete'),
    content: t('apps.urlRule.confirmDeleteDesc', { pattern: r.pattern }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deletes({ ids: [r.id] })
        message.success(t('apps.urlRule.deleted'))
        await load()
      } catch (e: any) {
        message.error(e?.message || '删除失败')
      }
    },
  })
}

async function runTest() {
  const url = testUrl.value.trim()
  if (!url) {
    message.warning(t('apps.urlRule.testUrlRequired'))
    return
  }
  testing.value = true
  testResult.value = null
  try {
    const res = await test({ url })
    testResult.value = res.data || null
  } catch (e: any) {
    message.error(e?.message || '测试失败')
  } finally {
    testing.value = false
  }
}

onMounted(load)
</script>

<template>
  <NCard style="border-radius:10px" class="mt-[10px]" size="small">
    <div class="flex items-center justify-between mb-[5px]">
      <div class="text-slate-500 font-bold text-base">
        {{ t('apps.urlRule.title') }}
      </div>
      <NButton size="small" type="primary" @click="openNew">
        {{ t('apps.urlRule.add') }}
      </NButton>
    </div>

    <div class="mt-[10px] text-sm text-gray-600 dark:text-gray-400 leading-relaxed">
      {{ t('apps.urlRule.desc') }}
    </div>

    <!-- 规则列表 -->
    <div v-if="loading" class="mt-[12px] text-sm text-gray-500">
      {{ t('common.loading') }}
    </div>

    <NEmpty
      v-else-if="!rules.length"
      class="mt-[16px]"
      :description="t('apps.urlRule.empty')"
    />

    <div v-else class="mt-[12px] overflow-x-auto">
      <table class="w-full text-sm">
        <thead>
          <tr class="text-left text-gray-500 dark:text-gray-400 border-b border-gray-200 dark:border-zinc-700">
            <th class="py-[8px] pr-[8px] font-medium">{{ t('apps.urlRule.pattern') }}</th>
            <th class="py-[8px] pr-[8px] font-medium">{{ t('apps.urlRule.target') }}</th>
            <th class="py-[8px] pr-[8px] font-medium w-[90px]">{{ t('apps.urlRule.enabled') }}</th>
            <th class="py-[8px] font-medium w-[120px]"></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="r in rules"
            :key="r.id"
            class="border-b border-gray-100 dark:border-zinc-800"
          >
            <td class="py-[8px] pr-[8px]">
              <span class="font-mono">{{ r.pattern }}</span>
              <div v-if="r.description" class="text-xs text-gray-500 mt-[2px]">
                {{ r.description }}
              </div>
            </td>
            <td class="py-[8px] pr-[8px]">
              <span class="font-mono break-all">{{ r.target }}</span>
            </td>
            <td class="py-[8px] pr-[8px]">
              <NSwitch
                size="small"
                :value="r.enabled === 1"
                @update:value="toggleEnabled(r)"
              />
            </td>
            <td class="py-[8px]">
              <NSpace :size="6">
                <NButton size="tiny" @click="openEdit(r)">
                  {{ t('common.edit') }}
                </NButton>
                <NButton size="tiny" type="error" ghost @click="remove(r)">
                  {{ t('common.delete') }}
                </NButton>
              </NSpace>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 试跑：规则支持通配符，肉眼很难确认是否写对，给一个直接验证的入口 -->
    <div class="mt-[16px] pt-[14px] border-t border-gray-200 dark:border-zinc-700">
      <div class="text-sm font-medium text-gray-600 dark:text-gray-300 mb-[8px]">
        {{ t('apps.urlRule.testTitle') }}
      </div>
      <NInputGroup>
        <NInput
          v-model:value="testUrl"
          :placeholder="t('apps.urlRule.testPlaceholder')"
          @keyup.enter="runTest"
        />
        <NButton type="primary" :loading="testing" @click="runTest">
          {{ t('apps.urlRule.testRun') }}
        </NButton>
      </NInputGroup>

      <div v-if="testResult" class="mt-[10px] p-[10px] rounded-lg text-sm"
           :class="testResult.hit
             ? 'bg-emerald-50 dark:bg-emerald-900/20 text-emerald-700 dark:text-emerald-300'
             : 'bg-gray-100 dark:bg-zinc-800 text-gray-600 dark:text-gray-300'">
        <template v-if="testResult.hit">
          <div>
            ✓ {{ t('apps.urlRule.testHit') }}
            <span class="font-mono ml-1">{{ testResult.rule?.pattern }}</span>
          </div>
          <div class="mt-[4px]">
            {{ t('apps.urlRule.testTarget') }}
            <span class="font-mono ml-1 break-all">{{ testResult.target }}</span>
          </div>
        </template>
        <template v-else>
          {{ t('apps.urlRule.testMiss') }}
        </template>
      </div>
    </div>
  </NCard>

  <!-- 编辑器 -->
  <NModal
    v-model:show="showEditor"
    preset="card"
    :title="isNew ? t('apps.urlRule.add') : t('apps.urlRule.edit')"
    style="max-width: 520px"
  >
    <NForm label-placement="top" size="small">
      <NFormItem :label="t('apps.urlRule.pattern')">
        <NInput
          v-model:value="form.pattern"
          placeholder="*.example.com:8080"
        />
      </NFormItem>

      <div class="text-xs text-gray-500 -mt-[10px] mb-[12px] leading-relaxed">
        {{ t('apps.urlRule.patternHint') }}
      </div>

      <NFormItem :label="t('apps.urlRule.target')">
        <NInput
          v-model:value="form.target"
          placeholder="http://192.168.1.10:8080"
        />
      </NFormItem>

      <div class="text-xs text-gray-500 -mt-[10px] mb-[12px] leading-relaxed">
        {{ t('apps.urlRule.targetHint') }}
      </div>

      <NFormItem :label="t('apps.urlRule.descLabel')">
        <NInput v-model:value="form.description" :placeholder="t('common.optional')" />
      </NFormItem>

      <NFormItem :label="t('apps.urlRule.enabled')">
        <NSwitch
          :value="form.enabled === 1"
          @update:value="(v: boolean) => (form.enabled = v ? 1 : 0)"
        />
      </NFormItem>
    </NForm>

    <template #footer>
      <div class="flex justify-end gap-2">
        <NButton size="small" @click="showEditor = false">
          {{ t('common.cancel') }}
        </NButton>
        <NButton size="small" type="primary" :loading="saving" @click="save">
          {{ t('common.save') }}
        </NButton>
      </div>
    </template>
  </NModal>
</template>