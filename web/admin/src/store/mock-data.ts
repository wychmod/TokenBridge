export const statCards = [
  {
    title: "今日请求",
    value: "12,846 次",
    delta: "+18.4%",
    tone: "violet"
  },
  {
    title: "本月费用",
    value: "1,284.73 美元",
    delta: "-6.8%",
    tone: "emerald"
  },
  {
    title: "平均首字返回时间",
    value: "218 毫秒",
    delta: "-14 毫秒",
    tone: "sky"
  },
  {
    title: "可用厂商",
    value: "8 家",
    delta: "+2",
    tone: "amber"
  }
];

export const providerHealth = [
  { name: "OpenAI 主线路", latency: 182, status: "healthy", share: 38 },
  { name: "Claude 高级线路", latency: 246, status: "healthy", share: 26 },
  { name: "DeepSeek 节省线路", latency: 128, status: "healthy", share: 21 },
  { name: "Azure 备用线路", latency: 311, status: "warning", share: 15 }
];

export const costTrend = [
  { day: "周一", cost: 132, requests: 1840, tokens: 420000 },
  { day: "周二", cost: 148, requests: 2030, tokens: 458000 },
  { day: "周三", cost: 141, requests: 1964, tokens: 436000 },
  { day: "周四", cost: 169, requests: 2210, tokens: 492000 },
  { day: "周五", cost: 188, requests: 2450, tokens: 538000 },
  { day: "周六", cost: 124, requests: 1630, tokens: 362000 },
  { day: "周日", cost: 117, requests: 1520, tokens: 349000 }
];

export const alertFeed = [
  { level: "warning", title: "Claude 高级线路延迟升高", description: "最近 5 分钟首字返回时间超过 320 毫秒，已触发备用链路观察。" },
  { level: "info", title: "Cursor 团队密钥预算达到 35%", description: "月度使用趋势正常，无需限流。" },
  { level: "critical", title: "Azure 备用线路最近 1 小时切换次数增加", description: "建议检查 OpenAI 主线路出站稳定性。" }
];

export const quickActions = [
  "新增厂商",
  "新建本地密钥",
  "执行路由测试",
  "打开便携版打包",
  "查看失败日志"
];

export const distributionStatus = {
  artifact: "tokenbridge.zip",
  mode: "便携版 · 下载即用",
  bundle: "管理后台前端将内嵌至单一可执行文件",
  init: "首次启动自动初始化配置、数据与日志目录"
};

export const liveRequests = [
  { tool: "Codex", model: "gpt-4o", provider: "OpenAI 主线路", latency: "182 毫秒", status: "流式返回中" },
  { tool: "Claude Desktop", model: "claude-sonnet-4", provider: "Claude 高级线路", latency: "246 毫秒", status: "正常" },
  { tool: "Cursor", model: "gpt-4o-mini", provider: "DeepSeek 节省线路", latency: "128 毫秒", status: "正常" }
];
