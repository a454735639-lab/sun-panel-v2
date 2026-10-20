import { createApp } from 'vue'
import App from './App.vue'
import { setupI18n } from './locales'
import { setupAssets, setupScrollbarStyle } from './plugins'
import { setupStore } from './store'
import { setupRouter } from './router'
import { installGlobalErrorCapture } from './utils/diagnostics'
import 'virtual:svg-icons-register' // svg图标注册

async function bootstrap() {
  // 尽早安装：要能捕获启动阶段就发生的异常，
  // 这类异常往往导致"白屏"而控制台里才有线索。
  installGlobalErrorCapture()

  const app = createApp(App)
  setupAssets()

  setupScrollbarStyle()

  setupStore(app)

  setupI18n(app)

  await setupRouter(app)
  app.mount('#app')
}

bootstrap()
