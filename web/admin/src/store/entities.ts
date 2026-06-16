export type ProviderRecord = {
  id: string;
  name: string;
  type: string;
  base_url?: string;
  baseURL: string;
  apiKey?: string;
  has_api_key?: boolean;
  hasApiKey?: boolean;
  api_key_masked?: string;
  apiKeyMasked?: string;
  organization_id?: string;
  enabled: boolean;
  status: "healthy" | "warning" | "disabled" | "active" | string;
  priority: number;
  models_json?: string;
  models: string[];
  rate_limit_rpm?: number;
  rate_limit_tpm?: number;
  rpm: number;
  tpm: number;
};

export type LocalKeyRecord = {
  id: string;
  name: string;
  display_key?: string;
  displayKey: string;
  allowed_models_json?: string;
  allowed_providers_json?: string;
  allowedModels: string[];
  allowedProviders: string[];
  monthly_budget?: number;
  monthlyBudget: number;
  current_spend?: number;
  currentSpend: number;
  token_budget?: number;
  tokenBudget: number;
  current_tokens?: number;
  currentTokens: number;
  enabled: boolean;
  revoked_at?: string | null;
  expires_at?: string | null;
  status: "active" | "warning" | "revoked";
};

export type RoutingRuleRecord = {
  id: string;
  model_pattern?: string;
  modelPattern: string;
  strategy: string;
  provider_chain?: string;
  providerChain: string[];
  fallback_chain?: string;
  fallbackChain: string[];
  enabled: boolean;
};

export type ModelAliasRecord = {
  id?: string;
  alias: string;
  target: string;
  fallback_chain?: string;
  fallbackChain: string[];
};

export type DistributionPlanRecord = {
  package_name?: string;
  mode?: string;
  includes?: string[];
  notes?: string[];
};

export type RoutingSimulation = {
  model: string;
  requestedModel?: string;
  key: string;
  format: string;
  target: string;
  fallback: string;
  cost: string;
  ttft: string;
  providerId?: string;
  providerName?: string;
  fallbackChain?: string[];
  formatCompatible?: boolean;
  formatWarning?: string;
  scope?: string;
};

export type SettingsRecord = {
  host: string;
  port: number;
  admin_path?: string;
  adminPath: string;
  admin_username?: string;
  adminUsername: string;
  theme: "light" | "dark" | "system" | string;
  update_channel?: string;
  updateChannel: string;
  backup_interval?: string;
  backupInterval: string;
  log_level?: string;
  logLevel: string;
  retention_days?: number;
  retentionDays: number;
  bundle_mode?: string;
  bundleMode: string;
  start_at_login?: boolean;
  startAtLogin: boolean;
};

export const providerRecords: ProviderRecord[] = [
  {
    id: "prov-openai-primary",
    name: "OpenAI Primary",
    type: "openai",
    baseURL: "https://api.openai.com",
    enabled: true,
    status: "healthy",
    priority: 1,
    models: ["gpt-4o", "gpt-4o-mini", "o3-mini"],
    rpm: 120,
    tpm: 400000
  },
  {
    id: "prov-claude-premium",
    name: "Claude Premium",
    type: "anthropic",
    baseURL: "https://api.anthropic.com",
    enabled: true,
    status: "healthy",
    priority: 2,
    models: ["claude-sonnet-4", "claude-haiku-4"],
    rpm: 90,
    tpm: 320000
  },
  {
    id: "prov-deepseek-saver",
    name: "DeepSeek Saver",
    type: "openai-compatible",
    baseURL: "https://api.deepseek.com",
    enabled: true,
    status: "warning",
    priority: 3,
    models: ["deepseek-chat", "deepseek-reasoner"],
    rpm: 160,
    tpm: 600000
  }
];

export const localKeyRecords: LocalKeyRecord[] = [
  {
    id: "key-codex",
    name: "Codex Pro",
    displayKey: "tb-a3f8****9k2m",
    allowedModels: ["gpt-4o", "claude-sonnet-4"],
    allowedProviders: ["OpenAI Primary", "Claude Premium"],
    monthlyBudget: 150,
    currentSpend: 42.8,
    tokenBudget: 10000000,
    currentTokens: 2340000,
    enabled: true,
    status: "active"
  },
  {
    id: "key-cursor",
    name: "Cursor Team",
    displayKey: "tb-b8d2****c1q9",
    allowedModels: ["gpt-4o-mini", "deepseek-chat"],
    allowedProviders: ["OpenAI Primary", "DeepSeek Saver"],
    monthlyBudget: 80,
    currentSpend: 28.1,
    tokenBudget: 8500000,
    currentTokens: 1720000,
    enabled: true,
    status: "warning"
  }
];

export const routingRuleRecords: RoutingRuleRecord[] = [
  {
    id: "route-gpt4o",
    modelPattern: "gpt-4o*",
    strategy: "Priority + Fallback",
    providerChain: ["OpenAI Primary", "Azure Backup"],
    fallbackChain: ["Azure Backup", "OpenRouter"],
    enabled: true
  },
  {
    id: "route-claude",
    modelPattern: "claude-*",
    strategy: "Priority + Fallback",
    providerChain: ["Claude Premium", "OpenRouter"],
    fallbackChain: ["OpenRouter", "DeepSeek Saver"],
    enabled: true
  }
];

export const settingsRecord: SettingsRecord = {
  host: "127.0.0.1",
  port: 18743,
  adminPath: "/admin",
  adminUsername: "admin",
  theme: "system",
  updateChannel: "stable",
  backupInterval: "24h",
  logLevel: "standard",
  retentionDays: 30,
  bundleMode: "single-binary",
  startAtLogin: false
};

