<script setup lang="ts">
/**
 * 诊断日志页面。
 *
 * 用途：面板在用户环境出错时，开发者看不到现场。
 * 这里把失败请求与 JS 异常汇总起来，用户一键复制后反馈即可。
 *
 * 界面上的几个刻意设计：
 *
 * 1. 顶部给出「问题摘要」
 *    不是甩给用户一堆日志让他自己找，而是先告诉他
 *    "有几个失败是不该弹提示的"——那正是 bug 的特征。
 *
 * 2. 每条日志标出「静默请求」
 *    这是区分问题的关键：静默请求（后台轮询）失败却弹了提示，
 *    就是 bug；主动操作失败弹提示则是正常行为。
 *
 * 3. 一键复制而非让用户截图
 *    手机上手打或截图都容易丢信息。复制出来是纯文本，
 *    可直接贴进聊天窗口。
 *
 * 4. 提供「复现」引导
 *    日志为空时提示用户先去复现问题，而不是让页面空着。
 */
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { NButton, NCard, NEmpty, NSpace, NTag, NAlert, useMessage } from 'naive-ui'
import {
  clearLogs, exportAsText, getLogs, subscribeLogs,
  type ErrorLogItem,
} from '@/utils/diagnostics'

const message = useMessage()
const logs = ref<ErrorLogItem[]>([])
const showRaw = ref(false)
// 复制失败时展示的全文文本框。需要它才能"展开即选中"。
const rawTextarea = ref<HTMLTextAreaElement | null>(null)

function refresh() {
  // 复制一份，避免视图直接引用内部数组（清空时会让列表失去响应）
  logs.value = [...getLogs()].reverse() // 最新的排在最前
}

let unsubscribe: (() => void) | null = null

onMounted(() => {
  refresh()
  unsubscribe = subscribeLogs(refresh)
})

onUnmounted(() => {
  unsubscribe?.()
})

const rawText = computed(() => exportAsText())

/** 统计：静默请求的失败次数 —— 这类失败本不应弹提示，是重点排查对象 */
const silentFailures = computed(() => logs.value.filter(l => l.kind === 'http' && l.silent))

/** 按类别统计，供顶部摘要展示 */
const stats = computed(() => ({
  http: logs.value.filter(l => l.kind === 'http').length,
  js: logs.value.filter(l => l.kind === 'js').length,
  unhandled: logs.value.filter(l => l.kind === 'unhandled').length,
}))

function kindLabel(k: ErrorLogItem['kind']) {
  return k === 'http' ? 'HTTP失败' : (k === 'js' ? 'JS异常' : 'Promise拒绝')
}

function kindType(k: ErrorLogItem['kind']) {
  return k === 'http' ? 'warning' : (k === 'js' ? 'error' : 'default')
}

async function copyAll() {
  const text = rawText.value

  // 优先用剪贴板 API（需要 https 或 localhost——即 isSecureContext）。
  //
  // 注意：内网 http 环境下 navigator.clipboard 通常**存在但被拒**，
  // 所以不能只判断"有没有这个对象"，必须真的 await 并捕获异常。
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      // 复制成功要把上次失败时展开的兜底面板收起来 ——
      // 否则用户会看到"已复制"的同时还挂着一块"请手动复制"，
      // 自相矛盾。
      showRaw.value = false
      message.success(`已复制 ${logs.value.length} 条日志，粘贴到对话里即可`)
      return
    }
    catch {
      // 落到下面的 execCommand 兜底
    }
  }

  // 兜底：execCommand('copy')。
  //
  // ★ 关键：它**失败时返回 false 而不抛异常** ——
  // 早先这里没有检查返回值，无论成功失败都提示"已复制"，
  // 用户看到成功的提示、粘贴出来却是空的（这正是本次反馈的问题）。
  // 必须显式判定返回值，失败时走降级路径。
  let ok = false
  try {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.top = '0'
    ta.style.left = '0'
    ta.style.opacity = '0'
    // 只读可避免移动端弹键盘
    ta.setAttribute('readonly', '')
    document.body.appendChild(ta)
    ta.focus()
    ta.select()
    // iOS Safari 需要显式设置选区
    ta.setSelectionRange(0, ta.value.length)
    ok = document.execCommand('copy')
    document.body.removeChild(ta)
  }
  catch {
    ok = false
  }

  if (ok) {
    // 成功后收起兜底面板（理由同上：不能同时显示"已复制"和"请手动复制"）
    showRaw.value = false
    message.success(`已复制 ${logs.value.length} 条日志，粘贴到对话里即可`)
  }
  else {
    // 复制失败：把全文展开并**自动全选**，用户直接 Ctrl+C 即可。
    //
    // 只展开不选中是不够的 —— 用户还得自己点进文本框、Ctrl+A、Ctrl+C，
    // 在手机上尤其麻烦。既然已经知道"自动复制失败"，
    // 就把能替他做的那一步做完。
    showRaw.value = true
    await nextTick()
    const ta = rawTextarea.value
    if (ta) {
      ta.focus()
      ta.select()
    }
    message.warning('浏览器拒绝了自动复制，已展开全文并选中，请按 Ctrl+C（或长按复制）')
  }
}

function doClear() {
  clearLogs()
  message.success('已清空')
}
</script>

<template>
  <NCard style="border-radius:10px" class="mt-[10px]" size="small">
    <div class="flex items-center justify-between mb-[8px] flex-wrap gap-2">
      <div class="text-slate-500 font-bold text-base">
        诊断日志
      </div>
      <NSpace :size="8">
        <NButton size="small" @click="refresh">
          刷新
        </NButton>
        <NButton size="small" @click="doClear" :disabled="!logs.length">
          清空
        </NButton>
        <NButton size="small" type="primary" @click="copyAll" :disabled="!logs.length">
          一键复制
        </NButton>
      </NSpace>
    </div>

    <div class="text-sm text-gray-600 dark:text-gray-400 leading-relaxed mb-[10px]">
      记录面板运行中的<strong>请求失败</strong>与 <strong>JS 异常</strong>，
      便于定位「网络错误」这类提示的来源。仅记录失败，不含任何密码或令牌。
    </div>

    <!-- 重点提示：静默请求失败却出现提示 = bug 特征 -->
    <NAlert
      v-if="silentFailures.length"
      type="warning"
      :bordered="false"
      class="mb-[10px]"
      title="发现可疑失败"
    >
      有 <strong>{{ silentFailures.length }}</strong> 条失败来自「静默请求」
      （后台轮询）。这类请求<strong>本不应弹任何提示</strong> ——
      如果你同时看到「网络错误」提示，那这些就是线索。
    </NAlert>

    <!-- 统计摘要 -->
    <div v-if="logs.length" class="mb-[10px] text-sm text-gray-600 dark:text-gray-400">
      共 {{ logs.length }} 条：
      HTTP 失败 {{ stats.http }} ·
      JS 异常 {{ stats.js }} ·
      Promise 拒绝 {{ stats.unhandled }}
    </div>

    <NEmpty
      v-if="!logs.length"
      class="my-[16px]"
      description="暂无记录。请先复现问题（例如等待提示出现），再回来点「刷新」。"
    />

    <!-- 日志列表 -->
    <div v-else class="max-h-[420px] overflow-y-auto">
      <div
        v-for="(it, i) in logs"
        :key="i"
        class="mb-[8px] p-[10px] rounded-lg bg-gray-100 dark:bg-zinc-800 text-xs"
      >
        <div class="flex items-center gap-2 flex-wrap mb-[4px]">
          <NTag size="tiny" :type="kindType(it.kind)">
            {{ kindLabel(it.kind) }}
          </NTag>
          <span class="text-gray-500 font-mono">{{ it.time }}</span>
          <NTag v-if="it.silent" size="tiny" type="info">静默请求</NTag>
          <span class="text-gray-500 truncate">{{ it.route }}</span>
        </div>
        <div class="font-mono break-all text-gray-800 dark:text-gray-200">
          {{ it.summary }}
        </div>
        <details class="mt-[6px]">
          <summary class="cursor-pointer text-gray-500 select-none">详情</summary>
          <pre class="mt-[4px] whitespace-pre-wrap break-all text-gray-600 dark:text-gray-400 font-mono">{{ it.detail }}</pre>
        </details>
      </div>
    </div>

    <!-- 复制失败时的兜底：展示纯文本供手动全选。
         这条提示做成**页面内常驻**而不是只靠 toast ——
         toast 会消失，而"复制没成功、请手动复制"是需要用户照做的指令，
         错过就以为复制成功了（这正是用户反馈"一键复制没用"的体感来源）。 -->
    <div v-if="showRaw" class="mt-[10px]">
      <NAlert type="warning" :bordered="false" class="mb-[8px]">
        浏览器拒绝了自动复制（内网 http 环境下常见）。下方全文<strong>已自动选中</strong>，
        直接按 <strong>Ctrl+C</strong>（手机上长按 → 复制）即可。
      </NAlert>
      <div class="text-sm text-gray-600 mb-[6px]">
        完整日志：
      </div>
      <textarea
        ref="rawTextarea"
        readonly
        class="w-full h-[240px] p-[8px] text-xs font-mono rounded-lg bg-gray-100 dark:bg-zinc-800 border border-gray-200 dark:border-zinc-700"
        :value="rawText"
        @focus="($event.target as HTMLTextAreaElement).select()"
      />
    </div>
  </NCard>
</template>