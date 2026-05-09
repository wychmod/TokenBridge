import {
  LocalKeyRecord,
  ProviderRecord,
  RoutingRuleRecord,
  SettingsRecord,
  localKeyRecords,
  providerRecords,
  routingRuleRecords,
  settingsRecord
} from "./entities";

export const providerRecordsLocalized: ProviderRecord[] = providerRecords.map((provider) => {
  const nameMap: Record<string, string> = {
    "OpenAI Primary": "OpenAI 主线路",
    "Claude Premium": "Claude 高级线路",
    "DeepSeek Saver": "DeepSeek 节省线路"
  };

  return {
    ...provider,
    name: nameMap[provider.name] ?? provider.name
  };
});

export const localKeyRecordsLocalized: LocalKeyRecord[] = localKeyRecords.map((key) => {
  const nameMap: Record<string, string> = {
    "Codex Pro": "Codex 专用密钥",
    "Cursor Team": "Cursor 团队密钥"
  };

  const providerNameMap: Record<string, string> = {
    "OpenAI Primary": "OpenAI 主线路",
    "Claude Premium": "Claude 高级线路",
    "DeepSeek Saver": "DeepSeek 节省线路"
  };

  return {
    ...key,
    name: nameMap[key.name] ?? key.name,
    allowedProviders: key.allowedProviders.map((provider) => providerNameMap[provider] ?? provider)
  };
});

export const routingRuleRecordsLocalized: RoutingRuleRecord[] = routingRuleRecords.map((rule) => {
  const providerNameMap: Record<string, string> = {
    "OpenAI Primary": "OpenAI 主线路",
    "Azure Backup": "Azure 备用线路",
    OpenRouter: "OpenRouter 备用出口",
    "Claude Premium": "Claude 高级线路",
    "DeepSeek Saver": "DeepSeek 节省线路"
  };

  const strategyMap: Record<string, string> = {
    "Priority + Fallback": "优先转发 + 备用切换"
  };

  return {
    ...rule,
    strategy: strategyMap[rule.strategy] ?? rule.strategy,
    providerChain: rule.providerChain.map((provider) => providerNameMap[provider] ?? provider),
    fallbackChain: rule.fallbackChain.map((provider) => providerNameMap[provider] ?? provider)
  };
});

export const settingsRecordLocalized: SettingsRecord = {
  ...settingsRecord,
  theme: settingsRecord.theme,
  updateChannel: settingsRecord.updateChannel === "stable" ? "稳定版" : settingsRecord.updateChannel,
  backupInterval: settingsRecord.backupInterval === "24h" ? "每 24 小时" : settingsRecord.backupInterval,
  logLevel: settingsRecord.logLevel === "standard" ? "标准" : settingsRecord.logLevel,
  bundleMode: settingsRecord.bundleMode === "single-binary" ? "单文件便携版" : settingsRecord.bundleMode
};
