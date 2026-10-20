/* Sun-Panel 服务端渲染版的少量胶水 JS
 * 只做三件必须客户端完成的事：时钟、在线图标、Docker 状态轮询。
 * 其余交互（表单、弹窗）交给 htmx。 */
(function () {
  'use strict'

  /* ---------- 时钟 ---------- */
  var WEEKDAYS = ['星期日', '星期一', '星期二', '星期三', '星期四', '星期五', '星期六']

  function tickClock() {
    var el = document.getElementById('clock')
    var dateEl = document.getElementById('clockDate')
    if (!el && !dateEl) return
    var d = new Date()
    var p = function (n) { return String(n).padStart(2, '0') }
    if (el) el.textContent = p(d.getHours()) + ':' + p(d.getMinutes()) + ':' + p(d.getSeconds())
    if (dateEl) {
      dateEl.textContent = (d.getMonth() + 1) + '-' + d.getDate() + ' ' + WEEKDAYS[d.getDay()]
    }
  }
  tickClock()
  setInterval(tickClock, 1000)

  /* ---------- 在线 SVG 图标（iconify） ---------- */
  var iconCache = {}
  var ICON_TIMEOUT = 4000 // 单个前缀请求超时，避免慢网络拖住 load 事件

  function collectIconNames() {
    var nodes = document.querySelectorAll('.svg-online[data-icon]')
    var names = []
    for (var i = 0; i < nodes.length; i++) {
      var n = nodes[i].getAttribute('data-icon')
      if (n && !iconCache[n] && names.indexOf(n) === -1) names.push(n)
    }
    return names
  }

  function fetchWithTimeout(url, ms) {
    var ctrl = typeof AbortController !== 'undefined' ? new AbortController() : null
    var timer = setTimeout(function () { if (ctrl) ctrl.abort() }, ms)
    return fetch(url, ctrl ? { signal: ctrl.signal } : undefined)
      .then(function (r) { return r.ok ? r.json() : null })
      .finally(function () { clearTimeout(timer) })
  }

  function fetchIcons(names) {
    // 按 prefix 分组批量请求：mdi:home -> https://api.iconify.design/mdi.json?icons=home
    var byPrefix = {}
    names.forEach(function (full) {
      var idx = full.indexOf(':')
      if (idx <= 0) return
      var prefix = full.slice(0, idx)
      var name = full.slice(idx + 1)
      ;(byPrefix[prefix] = byPrefix[prefix] || []).push(name)
    })
    Object.keys(byPrefix).forEach(function (prefix) {
      var url = 'https://api.iconify.design/' + prefix + '.json?icons=' + byPrefix[prefix].join(',')
      fetchWithTimeout(url, ICON_TIMEOUT)
        .then(function (data) {
          if (!data || !data.icons) return
          Object.keys(data.icons).forEach(function (name) {
            iconCache[prefix + ':' + name] = data.icons[name]
          })
          paintIcons()
        })
        .catch(function () { /* 图标服务不可达或超时，静默降级 */ })
    })
  }

  function paintIcons() {
    var nodes = document.querySelectorAll('.svg-online[data-icon]')
    for (var i = 0; i < nodes.length; i++) {
      var el = nodes[i]
      var full = el.getAttribute('data-icon')
      if (el.dataset.painted === '1') continue
      var icon = iconCache[full]
      if (!icon) continue
      var w = icon.width || 24
      var h = icon.height || 24
      var svg = '<svg xmlns="http://www.w3.org/2000/svg" width="1em" height="1em" viewBox="0 0 ' +
        w + ' ' + h + '" fill="currentColor">' + icon.body + '</svg>'
      el.innerHTML = svg
      el.dataset.painted = '1'
    }
  }

  function syncIcons() {
    var names = collectIconNames()
    if (names.length) fetchIcons(names)
    else paintIcons()
  }

  /* ---------- Docker 容器状态轮询 ---------- */
  var POLL_MS = 10000
  var stateMap = {}

  function containerIds() {
    var nodes = document.querySelectorAll('[data-container-id]')
    var ids = []
    for (var i = 0; i < nodes.length; i++) {
      var id = nodes[i].getAttribute('data-container-id')
      if (id && ids.indexOf(id) === -1) ids.push(id)
    }
    return ids
  }

  function applyStates(list) {
    list.forEach(function (s) { stateMap[s.id] = s })

    document.querySelectorAll('[data-container-id]').forEach(function (card) {
      var id = card.getAttribute('data-container-id')
      var s = stateMap[id]
      var dot = card.querySelector('.docker-dot')
      if (!dot) return
      var state = s ? s.state : 'unknown'
      dot.className = 'docker-dot docker-dot--' + state
      if (s) {
        dot.title = s.name + ' · ' + s.status +
          (s.running ? ' · CPU:' + s.cpuPercent.toFixed(1) + '% RAM:' + fmtBytes(s.memUsage) : '')
        // 服务端渲染的卡片里 metrics 是 .card-metrics
        var metrics = card.querySelector('.card-metrics')
        if (metrics && s.running) {
          metrics.textContent = 'CPU:' + s.cpuPercent.toFixed(1) + '% RAM:' + fmtBytes(s.memUsage)
        }
      }
    })

    var summary = document.getElementById('docker-summary')
    if (summary) {
      var running = list.filter(function (s) { return s.running }).length
      summary.textContent = running + ' / ' + list.length + ' 个容器运行中'
    }
  }

  function fmtBytes(v) {
    if (!v || v <= 0) return '0 B'
    var u = ['B', 'KB', 'MB', 'GB', 'TB'], i = 0
    while (v >= 1024 && i < u.length - 1) { v /= 1024; i++ }
    return (i === 0 ? v.toFixed(0) : v.toFixed(v >= 100 ? 0 : 1)) + u[i]
  }

  function pollDocker() {
    var ids = containerIds()
    if (!ids.length) return
    fetch('/api/docker/container/getContainerStates', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      body: JSON.stringify({ containerIds: ids, ts: Date.now(), withStats: true })
    })
      .then(function (r) { return r.ok ? r.json() : null })
      .then(function (res) {
        if (res && res.code === 0 && res.data && Array.isArray(res.data.list)) applyStates(res.data.list)
      })
      .catch(function () { /* 轮询失败静默，下轮重试 */ })
  }

  document.addEventListener('visibilitychange', function () {
    if (!document.hidden) { syncIcons(); pollDocker() }
  })

  function boot() {
    autoDetectNet()
    syncIcons()
    pollWidgets()
    if (widgetCardIds().length) {
      setInterval(function () { if (!document.hidden) pollWidgets() }, WIDGET_POLL_MS)
    }
    if (containerIds().length) { pollDocker(); setInterval(function () {
      if (!document.hidden) pollDocker()
    }, POLL_MS) }
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot)
  else boot()


  /* ---------- 拖拽排序 ---------- */
  var sortInstances = []

  window.toggleSort = function () {
    var body = document.body
    var on = body.classList.toggle('sorting')
    var btn = document.getElementById('sortToggle')
    if (btn) btn.classList.toggle('active', on)
    if (on && typeof showToast === 'function') showToast('拖拽卡片调整顺序，完成后再点菜单里的「卡片排序」退出')

    if (on) {
      if (typeof Sortable === 'undefined') { console.warn('Sortable 未加载'); return }
      document.querySelectorAll('.grid[id^="group-cards-"]').forEach(function (grid) {
        sortInstances.push(Sortable.create(grid, {
          animation: 180,
          draggable: '.card[data-card-id]',
          filter: '.card--add',
          ghostClass: 'sortable-ghost',
          chosenClass: 'sortable-chosen',
          onEnd: function () { saveSort(grid) }
        }))
      })
    } else {
      sortInstances.forEach(function (s) { s.destroy() })
      sortInstances = []
    }
  }

  /* 统一的 POST 请求出口：失败一定有反馈。
   * 此前这里用 .catch(function () {}) 把错误静默吞掉——会话过期或后端异常时，
   * 用户以为顺序已保存，刷新后却发现回退。改为统一走这里，复用页面已有的 showToast。 */
  function postJSON(url, payload, okMsg) {
    return fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      body: JSON.stringify(payload)
    }).then(function (r) {
      if (r.status === 401) {
        // 会话已失效：提示后回登录页，避免用户在无效会话里反复操作
        toast('登录已过期，正在跳转…', true)
        setTimeout(function () { window.location.href = '/login' }, 900)
        return
      }
      if (!r.ok) {
        toast('保存失败（HTTP ' + r.status + '），请重试', true)
        return
      }
      if (okMsg) toast(okMsg, false)
    }).catch(function () {
      toast('保存失败：网络异常，请重试', true)
    })
  }

  // toast 包装：showToast 定义在文件后段，这里用 typeof 守卫，避免加载顺序问题
  function toast(msg, isErr) {
    if (typeof window.showToast === 'function') {
      window.showToast(msg, isErr ? 'err' : 'ok')
    }
  }

  function saveSort(grid) {
    var groupId = parseInt(String(grid.id).replace('group-cards-', ''), 10)
    var ids = []
    grid.querySelectorAll('.card[data-card-id]').forEach(function (el) {
      var id = parseInt(el.getAttribute('data-card-id'), 10)
      if (!isNaN(id)) ids.push(id)
    })
    postJSON('/hx/card/sort', { groupId: groupId, ids: ids }, '顺序已保存')
  }


  /* ---------- 书签 / 便签弹窗 ---------- */
  window.openBookmarkForm = function (id, isFolder, parentId) {
    var url = '/hx/bookmark/form?id=' + (id || 0) + '&isFolder=' + (isFolder || 0) +
              '&parentId=' + (parentId || 0)
    htmx.ajax('GET', url, { target: '#bm-form-host', swap: 'innerHTML' })
  }

  window.openNoteForm = function (id) {
    htmx.ajax('GET', '/hx/notepad/form?id=' + (id || 0), { target: '#note-form-host', swap: 'innerHTML' })
  }

  /* ---------- 分组拖拽排序 ---------- */
  var groupSortable = null

  window.toggleGroupSort = function () {
    var body = document.body
    var on = body.classList.toggle('sorting-groups')
    var btn = document.getElementById('groupSortToggle')
    if (btn) btn.classList.toggle('active', on)
    if (on && typeof showToast === 'function') showToast('拖拽分组调整顺序，完成后再点菜单里的「分组排序」退出')

    if (on) {
      if (typeof Sortable === 'undefined') { console.warn('Sortable 未加载'); return }
      var container = document.querySelector('.panel')
      if (!container) return
      groupSortable = Sortable.create(container, {
        animation: 180,
        draggable: '.group',
        ghostClass: 'sortable-ghost',
        chosenClass: 'sortable-chosen',
        onEnd: function () {
          var ids = []
          container.querySelectorAll('.group[id^="group-"]').forEach(function (el) {
            var id = parseInt(String(el.id).replace('group-', ''), 10)
            if (!isNaN(id)) ids.push(id)
          })
          postJSON('/hx/group/sort', { ids: ids }, '分组顺序已保存')
        }
      })
    } else if (groupSortable) {
      groupSortable.destroy()
      groupSortable = null
    }
  }


  /* ---------- 内外网自动判定（连通性探测） ----------
   * 服务端已按客户端 IP 给了一次初判；这里用真实连通性做兜底：
   * 探测内网地址是否可达，可达就整页切到内网地址。
   * 与官方 netModeChangeAuto / netModeChangeAutoMaxPingMs 的语义一致。 */
  /* 探测地址是否可达。
   *
   * 用 fetch(mode:'no-cors')：
   *   - 连接层失败 → reject（真正不可达）
   *   - 连上了但读不了内容（opaque 响应）→ resolve
   * 这正是我们要的语义。
   *
   * 不要用 Image 加载：内网服务返回的多是 HTML 不是图片，
   * onerror 必然触发，会把所有可达地址误判成不可达。 */
  function probeReachable(url, timeoutMs) {
    return new Promise(function (resolve) {
      var done = false
      var ctrl = typeof AbortController !== 'undefined' ? new AbortController() : null
      var t = setTimeout(function () {
        if (done) return
        done = true
        if (ctrl) ctrl.abort()
        resolve(false)
      }, timeoutMs)

      fetch(url, {
        mode: 'no-cors',
        cache: 'no-store',
        credentials: 'omit',
        signal: ctrl ? ctrl.signal : undefined
      })
        .then(function () { if (!done) { done = true; clearTimeout(t); resolve(true) } })
        .catch(function () { if (!done) { done = true; clearTimeout(t); resolve(false) } })
    })
  }

  function applyNetMode(mode) {
    var nodes = document.querySelectorAll('[data-lan-url]')
    for (var i = 0; i < nodes.length; i++) {
      var el = nodes[i]
      var lan = el.getAttribute('data-lan-url')
      var wan = el.getAttribute('data-url')
      if (!lan || !wan) continue
      el.setAttribute('href', mode === 'lan' ? lan : wan)
    }
  }

  /* 内外网自动判定（客户端兜底）。
   *
   * 服务端只能按客户端 IP 猜，虚拟组网（NodeBabyLink / Tailscale）
   * 和反向代理场景都会猜错，所以这里用真实连通性做最终决定：
   *   探测内网地址 —— 可达就用内网，不可达就用外网。
   * 探测结果同时决定后续所有双地址卡片。 */
  function autoDetectNet() {
    var shell = document.querySelector('.shell[data-netmode]')
    if (!shell) return
    if (shell.getAttribute('data-netmode') !== 'auto') return
    if (shell.getAttribute('data-auto') !== '1') return

    var maxPing = parseInt(shell.getAttribute('data-ping-ms') || '700', 10)
    if (isNaN(maxPing) || maxPing <= 0) maxPing = 700

    // 收集去重后的内网地址，最多探 3 个
    var probes = [], seen = {}
    var nodes = document.querySelectorAll('[data-lan-url]')
    for (var i = 0; i < nodes.length && probes.length < 3; i++) {
      var u = nodes[i].getAttribute('data-lan-url')
      if (u && !seen[u]) { seen[u] = 1; probes.push(u) }
    }
    if (!probes.length) return

    Promise.all(probes.map(function (u) { return probeReachable(u, maxPing) }))
      .then(function (results) {
        var reachable = results.some(Boolean)
        applyNetMode(reachable ? 'lan' : 'wan')
      })
      .catch(function () { /* 探测异常保持服务端初判 */ })
  }

  /* ---------- 右键菜单 ---------- */
  var ctxTarget = null

  function hideCtxMenu() {
    var m = document.getElementById('ctx-menu')
    if (m) m.hidden = true
  }

  function showCtxMenu(x, y, card) {
    var m = document.getElementById('ctx-menu')
    if (!m) return
    ctxTarget = card

    var lan = card.getAttribute('data-lan-url')
    var wan = card.getAttribute('data-url')
    var type = card.getAttribute('data-card-type')

    // 按可用性显示/隐藏条目
    m.querySelector('[data-act="open-lan"]').hidden = !lan
    m.querySelector('[data-act="open-wan"]').hidden = !wan
    m.querySelector('[data-act="copy"]').hidden = !(lan || wan || card.getAttribute('href'))

    m.hidden = false
    // 先显示再量尺寸，避免超出视口
    var rect = m.getBoundingClientRect()
    var vw = window.innerWidth, vh = window.innerHeight
    if (x + rect.width > vw - 8) x = vw - rect.width - 8
    if (y + rect.height > vh - 8) y = vh - rect.height - 8
    m.style.left = Math.max(8, x) + 'px'
    m.style.top = Math.max(8, y) + 'px'
  }

  function openInNewTab(url) {
    if (!url) return
    window.open(url, '_blank', 'noopener')
  }

  document.addEventListener('contextmenu', function (e) {
    var card = e.target.closest('.card[data-card-id]')
    if (!card) { hideCtxMenu(); return }
    e.preventDefault()
    showCtxMenu(e.clientX, e.clientY, card)
  })

  document.addEventListener('click', function (e) {
    var menu = document.getElementById('ctx-menu')
    if (!menu || menu.hidden) return

    var btn = e.target.closest('[data-act]')
    if (!btn) { hideCtxMenu(); return }

    var act = btn.getAttribute('data-act')
    var card = ctxTarget
    hideCtxMenu()
    if (!card) return

    var id = card.getAttribute('data-card-id')
    var lan = card.getAttribute('data-lan-url')
    var wan = card.getAttribute('data-url')
    var cur = card.getAttribute('href')

    if (act === 'open') { openInNewTab(cur || wan || lan) }
    else if (act === 'open-lan') { openInNewTab(lan) }
    else if (act === 'open-wan') { openInNewTab(wan) }
    else if (act === 'copy') {
      var u = lan || wan || cur
      if (u && navigator.clipboard) navigator.clipboard.writeText(u)
    }
    else if (act === 'edit') {
      htmx.ajax('GET', '/hx/card/' + id + '/edit', { target: '#modal-host', swap: 'innerHTML' })
    }
    else if (act === 'delete') {
      if (confirm('确定删除这张卡片？')) {
        htmx.ajax('POST', '/hx/card/' + id + '/delete', { target: '#card-' + id, swap: 'outerHTML' })
      }
    }
  })

  document.addEventListener('scroll', hideCtxMenu, true)
  document.addEventListener('keydown', function (e) { if (e.key === 'Escape') hideCtxMenu() })


  /* ---------- 图标选择器 ---------- */
  var iconSearchTimer = null

  window.debouncedIconSearch = function (q) {
    clearTimeout(iconSearchTimer)
    iconSearchTimer = setTimeout(function () { window.iconSearch(q) }, 400)
  }

  window.switchPickerTab = function (tab) {
    ['lib', 'mine', 'url'].forEach(function (t) {
      var pane = document.getElementById('picker-' + t)
      if (pane) pane.style.display = (t === tab ? '' : 'none')
    })
    document.querySelectorAll('.picker-tab').forEach(function (b) {
      b.classList.toggle('active', b.getAttribute('data-tab') === tab)
    })
  }

  window.iconSearch = function (q) {
    q = (q || '').trim()
    var box = document.getElementById('icon-results')
    var status = document.getElementById('picker-status')
    if (!box) return
    if (!q) {
      box.innerHTML = ''
      if (status) status.textContent = '输入关键词搜索 iconify 图标库'
      return
    }
    if (status) status.textContent = '搜索中…'
    fetch('https://api.iconify.design/search?query=' + encodeURIComponent(q) + '&limit=96')
      .then(function (r) { return r.ok ? r.json() : null })
      .then(function (d) {
        var icons = (d && d.icons) || []
        box.innerHTML = ''
        if (!icons.length) {
          if (status) status.textContent = '没找到「' + q + '」相关的图标'
          return
        }
        if (status) status.textContent = '找到 ' + icons.length + ' 个，点击选用'
        icons.forEach(function (name) {
          var url = 'https://api.iconify.design/' + name.replace(':', '/') + '.svg'
          var b = document.createElement('button')
          b.type = 'button'
          b.className = 'icon-cell'
          b.title = name
          b.innerHTML = '<img class="iconify" src="' + url + '" alt="" loading="lazy">'
          b.onclick = function () { window.pickIcon(url) }
          box.appendChild(b)
        })
      })
      .catch(function () { if (status) status.textContent = '图标服务不可达，请用「手动输入」' })
  }

  window.pickIcon = function (src) {
    if (!src) return
    var el = document.getElementById('icon-src-input')
    var img = document.getElementById('icon-preview-img')
    if (el) el.value = src
    if (img) { img.src = src; img.style.display = '' }
    var backdrop = document.querySelector('.modal-backdrop')
    if (backdrop && backdrop.querySelector('.picker-tabs')) backdrop.remove()
  }


  /* ---------- 主题切换 ---------- */
  window.toggleTheme = function () {
    var cur = document.documentElement.getAttribute('data-theme')
    var next = cur === 'light' ? 'dark' : 'light'
    document.documentElement.setAttribute('data-theme', next)
    document.documentElement.setAttribute('data-theme-mode', next)
    // 记一年
    document.cookie = 'sunpanel_theme=' + next + ';path=/;max-age=31536000;SameSite=Lax'
  }


  /* ---------- Toast 反馈 ---------- */
  window.showToast = function (msg, type) {
    var host = document.getElementById('toast-host')
    if (!host) return
    var el = document.createElement('div')
    el.className = 'toast toast--' + (type || 'ok')
    el.textContent = msg
    host.appendChild(el)
    // 入场
    requestAnimationFrame(function () { el.classList.add('toast--in') })
    setTimeout(function () {
      el.classList.remove('toast--in')
      setTimeout(function () { el.remove() }, 260)
    }, 1800)
  }

  // 拦截 HX-Refresh：先弹 toast，延迟 600ms 再刷新，让用户看到反馈
  document.addEventListener('htmx:beforeSwap', function (e) {
    var xhr = e.detail.xhr
    if (!xhr) return
    var refresh = xhr.getResponseHeader('HX-Refresh')
    if (refresh !== 'true') return
    if (xhr.status >= 400) {
      window.showToast('操作失败：' + (xhr.responseText || xhr.status), 'err')
      e.detail.shouldSwap = false
      return
    }
    e.detail.shouldSwap = false
    window.showToast('已保存')
    setTimeout(function () { location.reload() }, 600)
  })

  // 非 GET 请求失败时也提示
  document.addEventListener('htmx:afterRequest', function (e) {
    var xhr = e.detail.xhr
    if (!xhr || e.detail.requestConfig.verb === 'get') return
    if (xhr.status >= 400 && xhr.getResponseHeader('HX-Refresh') !== 'true') {
      var msg = (xhr.responseText || '').replace(/<[^>]*>/g, '').trim()
      window.showToast(msg || '操作失败', 'err')
    }
  })


  /* ---------- 服务状态小组件 ---------- */
  var WIDGET_POLL_MS = 30000

  function widgetCardIds() {
    var ids = []
    document.querySelectorAll('[data-widget]').forEach(function (card) {
      var id = card.getAttribute('data-card-id')
      if (id) ids.push(parseInt(id, 10))
    })
    return ids
  }

  function renderWidget(card, info) {
    var box = card.querySelector('.card-widget')
    if (!box) return
    if (!info.ok) {
      box.className = 'card-widget card-widget--err'
      box.textContent = info.error || '不可用'
      box.title = info.error || ''
      return
    }
    box.className = 'card-widget'
    // 卡片宽度有限：优先显示摘要（下载/上传速度），指标放悬停浮层
    var html = ''
    if (info.text) {
      html += '<span class="cw-text">' + escapeHtml(info.text) + '</span>'
    } else if (info.stats && info.stats.length) {
      html += '<span class="cw-stats">'
      info.stats.slice(0, 2).forEach(function (st) {
        var tone = st.tone ? ' cw-tone--' + st.tone : ''
        html += '<span class="cw-stat' + tone + '">' +
          '<b>' + escapeHtml(st.value) + '</b>' + escapeHtml(st.label) + '</span>'
      })
      html += '</span>'
    }
    box.innerHTML = html
    if (info.detail && info.detail.length) {
      box.title = info.detail.map(function (d) { return d[0] + '：' + d[1] }).join('\n')
    }
  }

  function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]
    })
  }

  function pollWidgets() {
    var ids = widgetCardIds()
    if (!ids.length) return
    fetch('/api/widget/batch', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      body: JSON.stringify({ cardIds: ids })
    })
      .then(function (r) { return r.ok ? r.json() : null })
      .then(function (res) {
        if (!res || res.code !== 0 || !res.data || !res.data.list) return
        res.data.list.forEach(function (info) {
          var card = document.querySelector('[data-card-id="' + info.cardId + '"]')
          if (card) renderWidget(card, info)
        })
      })
      .catch(function () { /* 静默，下轮重试 */ })
  }

  // htmx 局部刷新后重新绑定图标
  document.addEventListener('htmx:afterSwap', function () { syncIcons() })
})()
/* ---------- 自定义二次确认 ---------- */
var confirmState = null

function esc(s) {
  return String(s == null ? '' : s)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;').replace(/'/g, '&#39;')
}

window.showConfirm = function (opts, onOk) {
  var host = document.getElementById('confirm-host')
  if (!host) return
  confirmState = { opts: opts || {}, onOk: onOk }
  var o = confirmState.opts
  var html = '<div class="confirm-backdrop" onclick="if(event.target===this)closeConfirm()">'
  html += '<div class="confirm-box">'
  html += '<h3 class="confirm-title">'
  if ((o.type || 'danger') === 'danger') html += '<span class="confirm-danger-ico">!</span>'
  html += esc(o.title || '确认操作') + '</h3>'
  html += '<div class="confirm-msg">' + esc(o.message || '此操作不可撤销。') + '</div>'
  if (o.requireInput) {
    html += '<label class="field"><span class="field-label">请输入 <strong>' +
      esc(o.requireInput) + '</strong> 以确认</span>'
    html += '<input type="text" id="confirm-input" class="confirm-input" autocomplete="off" spellcheck="false"></label>'
    html += '<p class="confirm-hint">此操作不可撤销。</p>'
  }
  html += '<div class="confirm-actions">'
  html += '<button type="button" class="btn" onclick="closeConfirm()">取消</button>'
  html += '<button type="button" class="btn btn--danger" id="confirm-ok" onclick="submitConfirm()">' +
    esc(o.okText || '确认') + '</button>'
  html += '</div></div></div>'
  host.innerHTML = html
  var inp = document.getElementById('confirm-input')
  if (inp) {
    inp.focus()
    inp.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') submitConfirm()
      if (e.key === 'Escape') closeConfirm()
    })
  }
}

window.closeConfirm = function () {
  var host = document.getElementById('confirm-host')
  if (host) host.innerHTML = ''
  confirmState = null
}

window.submitConfirm = function () {
  if (!confirmState) return
  var o = confirmState.opts
  if (o.requireInput) {
    var inp = document.getElementById('confirm-input')
    var v = inp ? inp.value.trim() : ''
    if (v !== o.requireInput) {
      if (inp) {
        inp.style.borderColor = '#ff6b81'
        inp.placeholder = '输入不匹配，请重新输入'
        inp.value = ''
      }
      return
    }
  }
  var cb = confirmState.onOk
  closeConfirm()
  if (cb) cb()
}

document.addEventListener('keydown', function (e) {
  if (e.key === 'Escape' && confirmState) closeConfirm()
})

/* htmx 删除/危险按钮拦截：先弹自定义确认，确认后才发请求 */
window.confirmHx = function (btn, ev) {
  if (ev) { ev.preventDefault(); ev.stopPropagation() }
  var raw = btn.getAttribute('data-confirm-action') || '{}'
  var o = {}
  try { o = JSON.parse(raw) } catch (e) { o = {} }
  showConfirm({
    title: o.title || '确认操作',
    message: o.message || '此操作不可撤销。',
    okText: o.okText || '确认',
    requireInput: o.requireInput || '',
    type: o.type || 'danger'
  }, function () {
    var method = 'POST'
    var url = btn.getAttribute('hx-post') || btn.getAttribute('hx-get') || ''
    if (btn.hasAttribute('hx-get') && !btn.hasAttribute('hx-post')) method = 'GET'
    if (btn.hasAttribute('hx-delete')) method = 'DELETE'
    var target = btn.getAttribute('hx-target') || btn
    var swap = btn.getAttribute('hx-swap') || 'outerHTML'
    if (url) htmx.ajax(method, url, { target: target, swap: swap })
  })
}
