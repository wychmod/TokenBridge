// 状态标签中文映射 —— 所有 UI 上展示的状态文本统一收口在这里

export const providerStatusMap: Record<string, string> = {
  healthy: "运行正常",
  warning: "异常警告",
  disabled: "已停用"
};

export const keyStatusMap: Record<string, string> = {
  active: "启用中",
  warning: "预算警告",
  revoked: "已吊销"
};

export const buildCheckStatusMap: Record<string, string> = {
  pending: "待处理",
  ready: "已就绪",
  blocked: "阻塞中",
  healthy: "检查通过",
  warning: "需要关注"
};

export const themeLabelMap: Record<string, string> = {
  light: "浅色模式",
  dark: "深色模式",
  system: "跟随系统"
};

export const runtimeHealthMap: Record<string, string> = {
  browser: "浏览器模式",
  healthy: "运行正常",
  warning: "需要关注",
  blocked: "阻塞中"
};

export const platformLabelMap: Record<string, string> = {
  web: "网页端",
  windows: "Windows",
  darwin: "macOS",
  linux: "Linux"
};

export const configValueLabelMap: Record<string, string> = {
  browser: "浏览器模式",
  stable: "稳定版",
  standard: "标准",
  "single-binary": "单文件便携版"
};

export const providerNameLabelMap: Record<string, string> = {
  "OpenAI Primary": "OpenAI 主线路",
  "Claude Premium": "Claude 高级线路",
  "DeepSeek Saver": "DeepSeek 节省线路",
  "Azure Backup": "Azure 备用线路",
  OpenRouter: "OpenRouter 备用出口"
};

export const providerTypeLabelMap: Record<string, string> = {
  openai: "OpenAI",
  "openai-compatible": "OpenAI 兼容",
  anthropic: "Anthropic (Claude)"
};

export function labelFromMap(map: Record<string, string>, value?: string): string {
  if (!value) return "未配置";
  return map[value] ?? value;
}

export function valueFromLabel(map: Record<string, string>, label?: string): string {
  if (!label) return "";
  const entry = Object.entries(map).find(([, text]) => text === label);
  return entry?.[0] ?? label;
}
