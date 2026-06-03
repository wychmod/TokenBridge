import { create } from "zustand";
import { api } from "../utils/api";
import {
  DistributionPlanRecord,
  LocalKeyRecord,
  ModelAliasRecord,
  ProviderRecord,
  RoutingRuleRecord,
  RoutingSimulation,
  SettingsRecord
} from "./entities";
import {
  localKeyRecordsLocalized,
  providerRecordsLocalized,
  routingRuleRecordsLocalized,
  settingsRecordLocalized
} from "./localized-data";
import {
  configValueLabelMap,
  labelFromMap,
  providerNameLabelMap,
  providerTypeLabelMap,
  valueFromLabel
} from "./labels";
import { providerLabel } from "../utils/routing-ui";

type NoticeTone = "success" | "warning" | "info";

type Notice = {
  id: number;
  createdAt: number;
  title: string;
  message: string;
  tone: NoticeTone;
};

export type ProviderTestResult = {
  status: string;
  latency_ms: number;
  models?: string[];
  message: string;
};

export type ProviderModelDiscoveryResult = {
  ok: boolean;
  models: string[];
  error?: string;
};

type AdminState = {
  providers: ProviderRecord[];
  keys: LocalKeyRecord[];
  rules: RoutingRuleRecord[];
  aliases: ModelAliasRecord[];
  distributionPlan: DistributionPlanRecord | null;
  settings: SettingsRecord;
  selectedProviderId?: string;
  selectedKeyId?: string;
  notices: Notice[];
  hydrate: () => Promise<void>;
  reloadProviders: () => Promise<void>;
  reloadKeys: () => Promise<void>;
  reloadRules: () => Promise<void>;
  reloadAliases: () => Promise<void>;
  reloadSettings: () => Promise<void>;
  reloadDistributionPlan: () => Promise<void>;
  setSelectedProvider: (id?: string) => void;
  setSelectedKey: (id?: string) => void;
  saveProvider: (record: ProviderRecord) => Promise<void>;
  deleteProvider: (id: string) => Promise<void>;
  reorderProviders: (ids: string[]) => Promise<void>;
  testProvider: (id: string) => Promise<ProviderTestResult>;
  testProviderDraft: (record: ProviderRecord) => Promise<ProviderTestResult>;
  discoverProviderModels: (id: string) => Promise<string[]>;
  discoverProviderModelsDraft: (record: ProviderRecord) => Promise<ProviderModelDiscoveryResult>;
  saveKey: (record: LocalKeyRecord) => Promise<void>;
  rotateKey: (id: string) => Promise<void>;
  revokeKey: (id: string) => Promise<void>;
  extendKey: (id: string, expiresAt: string | null) => Promise<void>;
  saveRule: (record: RoutingRuleRecord) => Promise<void>;
  deleteRule: (id: string) => Promise<void>;
  saveAlias: (record: ModelAliasRecord) => Promise<void>;
  testRouting: (input: { model: string; localKey: string; format: string; streaming?: boolean }) => Promise<RoutingSimulation | null>;
  saveSettings: (record: SettingsRecord) => Promise<void>;
  backupSettings: () => Promise<void>;
  pushNotice: (notice: Omit<Notice, "id" | "createdAt">) => void;
  dismissNotice: (id: number) => void;
};

export const useAdminStore = create<AdminState>((set, get) => ({
  providers: providerRecordsLocalized,
  keys: localKeyRecordsLocalized,
  rules: routingRuleRecordsLocalized,
  aliases: [],
  distributionPlan: null,
  settings: settingsRecordLocalized,
  selectedProviderId: providerRecordsLocalized[0]?.id,
  selectedKeyId: localKeyRecordsLocalized[0]?.id,
  notices: [
    {
      id: 1,
      createdAt: Date.now(),
      tone: "info",
      title: "欢迎使用 TokenBridge 控制台",
      message: "当前界面已接入本地后端，配置变更会写入本地数据库。"
    }
  ],
  hydrate: async () => {
    await Promise.all([
      get().reloadProviders(),
      get().reloadKeys(),
      get().reloadRules(),
      get().reloadAliases(),
      get().reloadSettings(),
      get().reloadDistributionPlan()
    ]);
  },
  reloadProviders: async () => {
    const result = await api.get<ProviderApiRecord[]>("/providers");
    if (!result.ok || !result.data) {
      get().pushNotice({ tone: "warning", title: "供应商列表加载失败", message: result.error ?? "无法读取后端供应商配置。" });
      return;
    }
    const providers = result.data.map(mapProviderFromApi);
    set((state) => ({ providers, selectedProviderId: state.selectedProviderId ?? providers[0]?.id }));
  },
  reloadKeys: async () => {
    const result = await api.get<KeyApiRecord[]>("/keys");
    if (!result.ok || !result.data) {
      get().pushNotice({ tone: "warning", title: "密钥列表加载失败", message: result.error ?? "无法读取后端密钥配置。" });
      return;
    }
    const keys = result.data.map((record) => mapKeyFromApi(record));
    set((state) => ({ keys, selectedKeyId: state.selectedKeyId ?? keys[0]?.id }));
  },
  reloadRules: async () => {
    const result = await api.get<RoutingRuleApiRecord[]>("/routing");
    if (!result.ok || !result.data) {
      get().pushNotice({ tone: "warning", title: "路由规则加载失败", message: result.error ?? "无法读取后端路由配置。" });
      return;
    }
    set({ rules: result.data.map(mapRuleFromApi) });
  },
  reloadAliases: async () => {
    const result = await api.get<ModelAliasApiRecord[]>("/aliases");
    if (!result.ok || !result.data) {
      get().pushNotice({ tone: "warning", title: "模型别名加载失败", message: result.error ?? "无法读取后端模型别名。" });
      return;
    }
    set({ aliases: result.data.map(mapAliasFromApi) });
  },
  reloadSettings: async () => {
    const result = await api.get<SettingsApiRecord>("/settings");
    if (!result.ok || !result.data) {
      get().pushNotice({ tone: "warning", title: "系统设置加载失败", message: result.error ?? "无法读取后端系统设置。" });
      return;
    }
    set({ settings: mapSettingsFromApi(result.data) });
  },
  reloadDistributionPlan: async () => {
    const result = await api.get<DistributionPlanRecord>("/distribution");
    if (!result.ok || !result.data) {
      get().pushNotice({ tone: "warning", title: "分发计划加载失败", message: result.error ?? "无法读取后端分发计划。" });
      return;
    }
    set({ distributionPlan: result.data });
  },
  setSelectedProvider: (id) => set({ selectedProviderId: id }),
  setSelectedKey: (id) => set({ selectedKeyId: id }),
  saveProvider: async (record) => {
    const payload = mapProviderToApi(record);
    const isExisting = get().providers.some((item) => item.id === record.id);
    const result = isExisting
      ? await api.put<ProviderApiRecord>(`/providers/${record.id}`, payload)
      : await api.post<ProviderApiRecord>("/providers", payload);
    if (!result.ok || !result.data) {
      get().pushNotice({ tone: "warning", title: "供应商保存失败", message: result.error ?? "后端未接受当前供应商配置。" });
      return;
    }
    const saved = mapProviderFromApi(result.data);
    set({ selectedProviderId: saved.id });
    await get().reloadProviders();
    get().pushNotice({ tone: "success", title: "供应商配置已保存", message: `${saved.name} 已写入本地数据库。` });
  },
  deleteProvider: async (id) => {
    const result = await api.delete<Record<string, unknown>>(`/providers/${id}`);
    if (!result.ok) {
      get().pushNotice({ tone: "warning", title: "供应商删除失败", message: result.error ?? "后端未能删除当前供应商。" });
      return;
    }
    set((state) => {
      const providers = state.providers.filter((item) => item.id !== id);
      return { providers, selectedProviderId: providers[0]?.id };
    });
    get().pushNotice({ tone: "success", title: "供应商已删除", message: "该供应商配置已从本地数据库移除。" });
  },
  reorderProviders: async (ids) => {
    const result = await api.post<Record<string, unknown>>("/providers/reorder", { ids });
    if (!result.ok) {
      get().pushNotice({ tone: "warning", title: "供应商排序失败", message: result.error ?? "后端未能保存当前排序。" });
      return;
    }
    await get().reloadProviders();
    get().pushNotice({ tone: "success", title: "供应商排序已保存", message: "新的模型供应商优先级已经生效。" });
  },
  testProvider: async (id) => {
    const result = await api.post<ProviderTestResult>(`/providers/${id}/test`);
    const data = result.ok && result.data ? result.data : failedProviderTestResult(result.error);
    const healthy = result.ok && data.status === "healthy";
    get().pushNotice({
      tone: healthy ? "success" : "warning",
      title: healthy ? "连接测试通过" : "连接测试失败",
      message: formatProviderTestMessage(data)
    });
    return data;
  },
  testProviderDraft: async (record) => {
    const result = await api.post<ProviderTestResult>("/providers/test", mapProviderToApi(record));
    return result.ok && result.data ? result.data : failedProviderTestResult(result.error);
  },
  discoverProviderModels: async (id) => {
    const result = await api.post<string[]>(`/providers/${id}/discover-models`);
    if (!result.ok || !result.data) {
      get().pushNotice({ tone: "warning", title: "模型发现失败", message: result.error ?? "无法读取模型列表。" });
      return [];
    }
    get().pushNotice({ tone: "success", title: "模型发现完成", message: `共发现 ${result.data.length} 个模型。` });
    return result.data;
  },
  discoverProviderModelsDraft: async (record) => {
    const result = await api.post<string[]>("/providers/discover-models", mapProviderToApi(record));
    if (!result.ok || !result.data) {
      return { ok: false, models: [], error: result.error ?? "无法读取模型列表。" };
    }
    return { ok: true, models: result.data };
  },
  saveKey: async (record) => {
    const payload = mapKeyToApi(record);
    const isExisting = get().keys.some((item) => item.id === record.id);
    const result = isExisting
      ? await api.put<KeyApiRecord>(`/keys/${record.id}`, payload)
      : await api.post<KeyApiRecord>("/keys", payload);
    if (!result.ok || !result.data) {
      get().pushNotice({ tone: "warning", title: "密钥保存失败", message: result.error ?? "后端未接受当前密钥配置。" });
      return;
    }
    const saved = mapKeyFromApi(result.data, result.rawKey);
    set((state) => ({ keys: upsert(state.keys, saved), selectedKeyId: saved.id }));
    get().pushNotice({ tone: "success", title: isExisting ? "密钥已更新" : "密钥已创建", message: result.rawKey ? `请立即复制新密钥：${result.rawKey}` : `${saved.name} 已写入本地数据库。` });
  },
  rotateKey: async (id) => {
    const result = await api.post<KeyApiRecord>(`/keys/${id}/rotate`);
    if (!result.ok || !result.data) {
      get().pushNotice({ tone: "warning", title: "密钥轮换失败", message: result.error ?? "后端未能轮换密钥。" });
      return;
    }
    const saved = mapKeyFromApi(result.data, result.rawKey);
    set((state) => ({ keys: upsert(state.keys, saved), selectedKeyId: saved.id }));
    get().pushNotice({ tone: "success", title: "密钥已轮换", message: result.rawKey ? `新密钥：${result.rawKey}` : "密钥已轮换，请使用新的展示信息。" });
  },
  revokeKey: async (id) => {
    const result = await api.post<Record<string, unknown>>(`/keys/${id}/revoke`);
    if (!result.ok) {
      get().pushNotice({ tone: "warning", title: "密钥吊销失败", message: result.error ?? "后端未能吊销密钥。" });
      return;
    }
    await get().reloadKeys();
    get().pushNotice({ tone: "success", title: "密钥已吊销", message: "该本地密钥已禁用并标记为吊销。" });
  },
  extendKey: async (id, expiresAt) => {
    const result = await api.post<KeyApiRecord>(`/keys/${id}/extend`, { expires_at: expiresAt });
    if (!result.ok || !result.data) {
      get().pushNotice({ tone: "warning", title: "密钥续期失败", message: result.error ?? "后端未能更新密钥有效期。" });
      return;
    }
    const saved = mapKeyFromApi(result.data);
    set((state) => ({ keys: upsert(state.keys, saved), selectedKeyId: saved.id }));
    get().pushNotice({ tone: "success", title: "密钥有效期已更新", message: `${saved.name} 的到期时间已保存。` });
  },
  saveRule: async (record) => {
    const payload = mapRuleToApi(record);
    const isExisting = get().rules.some((item) => item.id === record.id);
    const result = isExisting
      ? await api.put<RoutingRuleApiRecord>(`/routing/${record.id}`, payload)
      : await api.post<RoutingRuleApiRecord>("/routing", payload);
    if (!result.ok || !result.data) {
      get().pushNotice({ tone: "warning", title: "路由规则保存失败", message: result.error ?? "后端未接受当前路由规则。" });
      return;
    }
    const saved = mapRuleFromApi(result.data);
    set((state) => ({ rules: upsert(state.rules, saved) }));
    get().pushNotice({ tone: "success", title: "路由规则已保存", message: `${saved.modelPattern} 已写入本地数据库。` });
  },
  deleteRule: async (id) => {
    const result = await api.delete<Record<string, unknown>>(`/routing/${id}`);
    if (!result.ok) {
      get().pushNotice({ tone: "warning", title: "路由规则删除失败", message: result.error ?? "后端未能删除当前规则。" });
      return;
    }
    set((state) => ({ rules: state.rules.filter((item) => item.id !== id) }));
    get().pushNotice({ tone: "success", title: "路由规则已删除", message: "该调度规则已从本地数据库移除。" });
  },
  saveAlias: async (record) => {
    const result = await api.put<ModelAliasApiRecord>("/aliases", mapAliasToApi(record));
    if (!result.ok || !result.data) {
      get().pushNotice({ tone: "warning", title: "模型别名保存失败", message: result.error ?? "后端未接受当前模型别名。" });
      return;
    }
    const saved = mapAliasFromApi(result.data);
    set((state) => ({ aliases: upsertByAlias(state.aliases, saved) }));
    get().pushNotice({ tone: "success", title: "模型别名已保存", message: `${saved.alias} 将指向 ${saved.target}。` });
  },
  testRouting: async (input) => {
    const result = await api.post<RoutingTestApiRecord>("/routing/test", {
      model: input.model,
      local_key: input.localKey,
      format: input.format,
      streaming: input.streaming ?? false
    });
    if (!result.ok || !result.data) {
      get().pushNotice({ tone: "warning", title: "路由模拟失败", message: result.error ?? "后端未能完成路由模拟。" });
      return null;
    }
    const simulation = mapRoutingSimulation(result.data, input, get().providers);
    get().pushNotice({ tone: "success", title: "路由模拟完成", message: `${input.model} 将分发到 ${simulation.target}。` });
    return simulation;
  },
  saveSettings: async (record) => {
    const result = await api.put<SettingsApiRecord>("/settings", mapSettingsToApi(record));
    if (!result.ok || !result.data) {
      get().pushNotice({ tone: "warning", title: "系统设置保存失败", message: result.error ?? "后端未接受当前系统设置。" });
      return;
    }
    const saved = mapSettingsFromApi(result.data);
    set({ settings: saved });
    get().pushNotice({ tone: "success", title: "系统设置已保存", message: `服务监听 ${saved.host}:${saved.port}，管理后台路径 ${saved.adminPath}。` });
  },
  backupSettings: async () => {
    const result = await api.post<{ message: string }>("/settings/backup");
    get().pushNotice({ tone: result.ok ? "success" : "warning", title: result.ok ? "备份已完成" : "备份失败", message: result.data?.message ?? result.error ?? "备份请求已返回。" });
  },
  pushNotice: (notice) =>
    set((state) => ({
      notices: prependNotice(state.notices, notice)
    })),
  dismissNotice: (id) =>
    set((state) => ({
      notices: state.notices.filter((notice) => notice.id !== id)
    }))
}));

function upsert<T extends { id: string }>(items: T[], next: T): T[] {
  const exists = items.some((item) => item.id === next.id);
  if (!exists) {
    return [next, ...items];
  }
  return items.map((item) => (item.id === next.id ? next : item));
}

function upsertByAlias(items: ModelAliasRecord[], next: ModelAliasRecord): ModelAliasRecord[] {
  const exists = items.some((item) => item.alias === next.alias);
  if (!exists) {
    return [next, ...items];
  }
  return items.map((item) => (item.alias === next.alias ? next : item));
}

function prependNotice(notices: Notice[], notice: Omit<Notice, "id" | "createdAt">): Notice[] {
  const createdAt = Date.now();
  return [{ id: nextNoticeId(createdAt), createdAt, ...notice }, ...notices].slice(0, 4);
}

let noticeSequence = 1;

function nextNoticeId(createdAt: number): number {
  noticeSequence = (noticeSequence + 1) % 1000;
  return createdAt * 1000 + noticeSequence;
}

function failedProviderTestResult(error?: string): ProviderTestResult {
  return {
    status: "warning",
    latency_ms: 0,
    message: error ?? "测试请求失败。"
  };
}

function formatProviderTestMessage(result: ProviderTestResult): string {
  const latency = Number.isFinite(result.latency_ms) ? result.latency_ms : 0;
  return `${result.message || "连接测试已返回。"}，延迟 ${latency} 毫秒。`;
}

type ProviderApiRecord = {
  id: string;
  name: string;
  type: string;
  base_url?: string;
  organization_id?: string;
  enabled?: boolean;
  priority?: number;
  status?: string;
  models_json?: string;
  rate_limit_rpm?: number;
  rate_limit_tpm?: number;
  has_api_key?: boolean;
  api_key_masked?: string;
};

type KeyApiRecord = {
  id: string;
  name: string;
  display_key?: string;
  allowed_models_json?: string;
  allowed_providers_json?: string;
  monthly_budget?: number;
  current_spend?: number;
  token_budget?: number;
  current_tokens?: number;
  enabled?: boolean;
  revoked_at?: string | null;
  expires_at?: string | null;
};

type RoutingRuleApiRecord = {
  id: string;
  model_pattern?: string;
  strategy?: string;
  provider_chain?: string;
  fallback_chain?: string;
  enabled?: boolean;
};

type RoutingTestApiRecord = {
  resolved_model: string;
  provider_id: string;
  provider_name?: string;
  strategy: string;
  fallback_chain: string[];
  estimated_cost: string;
  estimated_ttft: string;
  format_compatible?: boolean;
  format_warning?: string;
  simulation_scope?: string;
};

type ModelAliasApiRecord = {
  id?: string;
  alias: string;
  target: string;
  fallback_chain?: string;
};

type SettingsApiRecord = {
  host?: string;
  port?: number;
  admin_path?: string;
  admin_username?: string;
  theme?: string;
  update_channel?: string;
  backup_interval?: string;
  log_level?: string;
  retention_days?: number;
  bundle_mode?: string;
};

function parseJSONList(raw?: string): string[] {
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed.map(String) : [];
  } catch {
    return [];
  }
}

function mapProviderStatus(record: ProviderApiRecord): ProviderRecord["status"] {
  if (record.enabled === false || record.status === "disabled" || record.status === "deleted") return "disabled";
  if (record.status === "warning") return "warning";
  return "healthy";
}

function normalizeProviderType(type?: string): string {
  const value = (type ?? "").trim().toLowerCase();
  if (["openai", "openai official", "openai官方", "openai 官方"].includes(value)) return "openai";
  if (["openai-compatible", "openai compatible", "openai_compatible", "openai 兼容", "openai兼容", "deepseek", "deepseek compatible", "deepseek 兼容", "deepseek兼容"].includes(value)) return "openai-compatible";
  if (["anthropic", "claude", "anthropic compatible", "anthropic 兼容", "anthropic兼容", "anthropic official", "anthropic 官方", "anthropic官方"].includes(value)) return "anthropic";
  return type ?? "openai-compatible";
}

function mapProviderFromApi(record: ProviderApiRecord): ProviderRecord {
  const models = parseJSONList(record.models_json);
  return {
    id: record.id,
    name: labelFromMap(providerNameLabelMap, record.name),
    type: normalizeProviderType(record.type),
    base_url: record.base_url,
    baseURL: record.base_url ?? "",
    apiKey: "",
    has_api_key: record.has_api_key,
    hasApiKey: record.has_api_key ?? Boolean(record.api_key_masked),
    api_key_masked: record.api_key_masked,
    apiKeyMasked: record.api_key_masked,
    organization_id: record.organization_id,
    enabled: record.enabled ?? true,
    status: mapProviderStatus(record),
    priority: record.priority ?? 0,
    models_json: record.models_json,
    models,
    rate_limit_rpm: record.rate_limit_rpm,
    rate_limit_tpm: record.rate_limit_tpm,
    rpm: record.rate_limit_rpm ?? 0,
    tpm: record.rate_limit_tpm ?? 0
  };
}

function mapProviderToApi(record: ProviderRecord) {
  return {
    id: record.id,
    name: record.name,
    type: normalizeProviderType(valueFromLabel(providerTypeLabelMap, record.type)),
    base_url: record.baseURL,
    api_key: record.apiKey?.trim() ?? "",
    organization_id: record.organization_id ?? "",
    enabled: record.status !== "disabled" && record.enabled !== false,
    priority: record.priority,
    status: record.status === "warning" ? "warning" : "active",
    models: record.models,
    rate_limit_rpm: record.rpm,
    rate_limit_tpm: record.tpm
  };
}

function mapKeyStatus(record: KeyApiRecord): LocalKeyRecord["status"] {
  if (record.revoked_at || record.enabled === false) return "revoked";
  const budget = record.monthly_budget ?? 0;
  const spend = record.current_spend ?? 0;
  if (budget > 0 && spend / budget >= 0.8) return "warning";
  return "active";
}

function mapKeyFromApi(record: KeyApiRecord, rawKey?: string): LocalKeyRecord {
  return {
    id: record.id,
    name: record.name,
    display_key: record.display_key,
    displayKey: rawKey ?? record.display_key ?? "tb-****",
    allowed_models_json: record.allowed_models_json,
    allowed_providers_json: record.allowed_providers_json,
    allowedModels: parseJSONList(record.allowed_models_json),
    allowedProviders: parseJSONList(record.allowed_providers_json).map((provider) => labelFromMap(providerNameLabelMap, provider)),
    monthly_budget: record.monthly_budget,
    monthlyBudget: record.monthly_budget ?? 0,
    current_spend: record.current_spend,
    currentSpend: record.current_spend ?? 0,
    token_budget: record.token_budget,
    tokenBudget: record.token_budget ?? 0,
    current_tokens: record.current_tokens,
    currentTokens: record.current_tokens ?? 0,
    enabled: record.enabled ?? true,
    revoked_at: record.revoked_at,
    expires_at: record.expires_at,
    status: mapKeyStatus(record)
  };
}

function mapKeyToApi(record: LocalKeyRecord) {
  return {
    name: record.name,
    allowed_models: record.allowedModels,
    allowed_providers: record.allowedProviders.map((provider) => valueFromLabel(providerNameLabelMap, provider)),
    monthly_budget: record.monthlyBudget,
    token_budget: record.tokenBudget,
    enabled: record.status !== "revoked" && record.enabled !== false,
    expires_at: record.expires_at ?? null
  };
}

function mapRuleFromApi(record: RoutingRuleApiRecord): RoutingRuleRecord {
  return {
    id: record.id,
    model_pattern: record.model_pattern,
    modelPattern: record.model_pattern ?? "*",
    strategy: record.strategy ?? "priority",
    provider_chain: record.provider_chain,
    providerChain: parseJSONList(record.provider_chain),
    fallback_chain: record.fallback_chain,
    fallbackChain: parseJSONList(record.fallback_chain),
    enabled: record.enabled ?? true
  };
}

function mapRuleToApi(record: RoutingRuleRecord) {
  return {
    model_pattern: record.modelPattern,
    strategy: record.strategy,
    provider_chain: record.providerChain,
    fallback_chain: record.fallbackChain,
    enabled: record.enabled
  };
}

function mapAliasFromApi(record: ModelAliasApiRecord): ModelAliasRecord {
  return {
    id: record.id,
    alias: record.alias,
    target: record.target,
    fallback_chain: record.fallback_chain,
    fallbackChain: parseJSONList(record.fallback_chain)
  };
}

function mapAliasToApi(record: ModelAliasRecord) {
  return {
    alias: record.alias,
    target: record.target,
    fallback_chain: record.fallbackChain
  };
}

function mapSettingsFromApi(record: SettingsApiRecord): SettingsRecord {
  return {
    host: record.host ?? "127.0.0.1",
    port: record.port ?? 18743,
    admin_path: record.admin_path,
    adminPath: record.admin_path ?? "/admin",
    admin_username: record.admin_username,
    adminUsername: record.admin_username ?? "admin",
    theme: record.theme ?? "system",
    update_channel: record.update_channel,
    updateChannel: labelFromMap(configValueLabelMap, record.update_channel ?? "stable"),
    backup_interval: record.backup_interval,
    backupInterval: record.backup_interval ?? "24h",
    log_level: record.log_level,
    logLevel: labelFromMap(configValueLabelMap, record.log_level ?? "standard"),
    retention_days: record.retention_days,
    retentionDays: record.retention_days ?? 30,
    bundle_mode: record.bundle_mode,
    bundleMode: labelFromMap(configValueLabelMap, record.bundle_mode ?? "single-binary")
  };
}

function mapSettingsToApi(record: SettingsRecord) {
  return {
    host: record.host,
    port: record.port,
    admin_path: record.adminPath,
    admin_username: record.adminUsername,
    theme: record.theme,
    update_channel: valueFromLabel(configValueLabelMap, record.updateChannel),
    backup_interval: record.backupInterval,
    log_level: valueFromLabel(configValueLabelMap, record.logLevel),
    retention_days: record.retentionDays,
    bundle_mode: valueFromLabel(configValueLabelMap, record.bundleMode)
  };
}

function mapRoutingSimulation(record: RoutingTestApiRecord, input: { model: string; localKey: string; format: string }, providers: ProviderRecord[]): RoutingSimulation {
  const fallbackChain = record.fallback_chain ?? [];
  const fallbackLabels = fallbackChain.map((provider) => providerLabel(provider, providers));
  const targetLabel = providerLabel(record.provider_id || record.provider_name || "", providers);
  return {
    model: record.resolved_model || input.model,
    requestedModel: input.model,
    key: input.localKey,
    format: input.format,
    target: targetLabel === record.provider_id && record.provider_name ? record.provider_name : targetLabel,
    fallback: fallbackLabels.length ? fallbackLabels.join(" -> ") : "无",
    cost: record.estimated_cost || "未实际请求",
    ttft: record.estimated_ttft || "未实际请求",
    providerId: record.provider_id,
    providerName: record.provider_name,
    fallbackChain: fallbackLabels,
    formatCompatible: record.format_compatible ?? true,
    formatWarning: record.format_warning ?? "",
    scope: record.simulation_scope ?? "仅模拟路由决策，未发送上游请求。"
  };
}

