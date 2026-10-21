<script setup lang="ts">
import { computed, ref } from 'vue'
import { NCard, NTabPane, NTabs } from 'naive-ui'
import { t } from '@/locales'
import AppearancePanel from './AppearancePanel.vue'
import NetworkPanel from './NetworkPanel.vue'
import UrlRuleManager from './UrlRuleManager.vue'
import DiagnosticsPanel from './DiagnosticsPanel.vue'
import ScanImport from './ScanImport.vue'

/**
 * 全局设置 —— 把原先分立的「风格设置」与「设置」合成一页。
 *
 * 为什么要合：
 * - 两者都是"影响整个面板的全局配置"，分开摆是历史遗留：
 *   风格设置来自旧版（外观），设置是后加的管理项。
 * - 用户实际使用时是**同一个心智模型**（"我要改面板的样/改它的行为"），
 *   分成两个并列菜单项，每次都要先猜"这个选项在哪一栏"。
 *
 * 合并方式：外层一个"全局设置"，内层用页签按主题分组。
 * 不按"原菜单名"分（外观/管理），因为那等于把两个旧菜单搬进来，
 * 用户还是要猜。按**对象**分更直观：外观 / 网络 / 规则 / 诊断 / 导入。
 */
type TabKey = 'appearance' | 'network' | 'rules' | 'diagnostics' | 'scan'

const activeTab = ref<TabKey>('appearance')

/**
 * 页签定义。
 *
 * `component` 直接持有组件引用而非动态 import：
 * 这里只有 5 个子面板、总量不大，一并打进这个 chunk 更省事，
 * 也避免在页签间切换时出现加载闪烁（用户会以为卡住了）。
 */
const tabs = computed(() => [
  { key: 'appearance' as TabKey, label: t('apps.globalSettings.tabAppearance'), component: AppearancePanel },
  { key: 'network' as TabKey, label: t('apps.globalSettings.tabNetwork'), component: NetworkPanel },
  { key: 'rules' as TabKey, label: t('apps.globalSettings.tabRules'), component: UrlRuleManager },
  { key: 'diagnostics' as TabKey, label: t('apps.globalSettings.tabDiagnostics'), component: DiagnosticsPanel },
  { key: 'scan' as TabKey, label: t('apps.globalSettings.tabScan'), component: ScanImport },
])

const currentComponent = computed(() => tabs.value.find(x => x.key === activeTab.value)?.component)
</script>

<template>
  <div class="bg-slate-200 dark:bg-zinc-900 rounded-[10px] p-[8px] overflow-auto h-full">
    <!-- 合并说明：只在最上方出现一次，避免用户以为"风格设置不见了" -->
    <NCard style="border-radius:10px" size="small">
      <div class="text-slate-500 font-bold text-base">
        {{ t('apps.globalSettings.title') }}
      </div>
      <div class="mt-[6px] text-xs text-gray-500 leading-relaxed">
        {{ t('apps.globalSettings.desc') }}
      </div>
    </NCard>

    <NTabs v-model:value="activeTab" type="line" size="small" class="mt-2" :animated="false">
      <NTabPane
        v-for="item in tabs"
        :key="item.key"
        :name="item.key"
        :tab="item.label"
      />
    </NTabs>

    <!-- 内容区。各子面板自带 van-card 与间距，这里只负责切换。 -->
    <div class="mt-1">
      <component :is="currentComponent" />
    </div>
  </div>
</template>

<style scoped>
.text-shadow {
  text-shadow: 0px 0px 5px gray;
}
</style>