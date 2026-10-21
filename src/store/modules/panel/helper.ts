import { ss } from '@/utils/storage'
import { PanelPanelConfigStyleEnum, PanelStateNetworkModeEnum } from '@/enums'
import defaultBackground from '@/assets/defaultBackground.webp'
const LOCAL_NAME = 'panelStorage'

const defaultFooterHtml = '<div class="flex justify-center text-slate-300" style="margin-top:100px">Powered By <a href="https://github.com/75412701/sun-panel-v2" target="_blank" class="ml-[5px]">Sun-Panel</a></div>'

export function defaultStatePanelConfig(): Panel.panelConfig {
  return {
    backgroundImageSrc: defaultBackground,
    backgroundBlur: 0,
    backgroundMaskNumber: 0,
    iconStyle: PanelPanelConfigStyleEnum.icon,
    iconTextColor: '#ffffff',
    iconTextInfoHideDescription: false,
    iconTextIconHideTitle: false,
    // 小卡默认显示描述（与「隐藏标题」同为小卡专属开关）
    iconTextIconHideDescription: false,
    logoText: 'Sun-Panel',
    logoImageSrc: '',
    clockShowSecond: false,
    // 时钟文字颜色。默认取白色 —— 必须显式给默认值，
    // 否则颜色选择器会显示 #000000，而时钟实际继承的是白色（父级 iconTextColor），
    // 用户会以为"设成了黑色却不生效"。默认值要与实际渲染一致。
    clockColor: '#ffffff',
    searchBoxShow: false,
    searchBoxSearchIcon: false,
    marginBottom: 10,
    marginTop: 10,
    maxWidth: 1200,
    maxWidthUnit: 'px',
    marginX: 5,
    footerHtml: defaultFooterHtml,
    systemMonitorShow: false,
    systemMonitorShowTitle: true,
    systemMonitorPublicVisitModeShow: false,
    netModeChangeButtonShow: true,
    autoNetworkWallpaper: false, // 默认关闭自动获取网络壁纸
    autoNetworkWallpaperApi: 'https://img.xjh.me/random_img.php?return=302&type=bg&ctype=nature', // 默认自动获取网络壁纸API地址

  }
}

export function defaultState(): Panel.State {
  return {
    rightSiderCollapsed: false,
    leftSiderCollapsed: false,
    networkMode: PanelStateNetworkModeEnum.wan,
    panelConfig: { ...defaultStatePanelConfig() },
  }
}

export function getLocalState(): Panel.State {
  const localState = ss.get(LOCAL_NAME)
  return { ...defaultState(), ...localState }
}

export function setLocalState(state: Panel.State) {
  ss.set(LOCAL_NAME, state)
}

export function removeLocalState() {
  ss.remove(LOCAL_NAME)
}
