import { EventsOn } from "../wailsjs/runtime/runtime";
import {
  CloseWindow,
  GetConfigSummary,
  GetDesktopStatus,
  GetRuntimeSummary,
  GetVersion,
  GetWindowState,
  HideToTray,
  MinimiseWindow,
  OpenAdminInBrowser,
  RestoreLastRoute,
  RunSelfCheck,
  SaveWindowState,
  SendNativeNotice,
  ShowMainWindow,
  ToggleMaximiseWindow
} from "../wailsjs/go/main/DesktopApp";
import { isDesktopMode } from "./desktop-env";

export type DesktopCheckItem = {
  key: string;
  title: string;
  description: string;
  status: string;
  detail: string;
};

export type DesktopWindowState = {
  width: number;
  height: number;
  x: number;
  y: number;
  maximised: boolean;
  lastRoute: string;
  hiddenToTray: boolean;
};

export type DesktopStatus = {
  version: string;
  platform: string;
  serverAddr: string;
  adminUrl: string;
  windowTitle: string;
  desktopMode: boolean;
  notifications: boolean;
  customChrome: boolean;
  trayEnabled: boolean;
  hideToTrayEnabled: boolean;
  stateRestore: boolean;
  windowState: DesktopWindowState;
  runtime: DesktopRuntimeSummary;
  configSummary: DesktopConfigSummary;
};

export type DesktopRuntimeSummary = {
  providers: number;
  keys: number;
  rules: number;
  health: string;
};

export type DesktopConfigSummary = {
  host: string;
  port: number;
  adminPath: string;
  theme: string;
  bundleMode: string;
  updateChannel: string;
};

export type DesktopSelfCheck = {
  health: string;
  checks: DesktopCheckItem[];
  warnings: string[];
  completedAt: string;
  serverReachable: boolean;
};

export type DesktopNotice = {
  title: string;
  message: string;
};

type DesktopRuntimeBridge = Window & {
  go?: {
    main?: {
      DesktopApp?: {
        ToggleAIStatsWidget?: () => Promise<unknown>;
      };
    };
  };
};

const emptyWindowState: DesktopWindowState = {
  width: 1360,
  height: 860,
  x: 120,
  y: 80,
  maximised: false,
  lastRoute: "/dashboard",
  hiddenToTray: false
};

const emptyStatus: DesktopStatus = {
  version: "browser",
  platform: "web",
  serverAddr: "",
  adminUrl: "",
  windowTitle: "TokenBridge",
  desktopMode: false,
  notifications: false,
  customChrome: false,
  trayEnabled: false,
  hideToTrayEnabled: false,
  stateRestore: false,
  windowState: emptyWindowState,
  runtime: {
    providers: 0,
    keys: 0,
    rules: 0,
    health: "browser"
  },
  configSummary: {
    host: "127.0.0.1",
    port: 0,
    adminPath: "/admin",
    theme: "system",
    bundleMode: "browser",
    updateChannel: "stable"
  }
};

export async function fetchDesktopStatus(): Promise<DesktopStatus> {
  if (!isDesktopMode) return emptyStatus;
  try { return await GetDesktopStatus(); } catch { return emptyStatus; }
}
export async function fetchDesktopVersion(): Promise<string> {
  if (!isDesktopMode) return "browser";
  try { return await GetVersion(); } catch { return "browser"; }
}
export async function fetchRuntimeSummary(): Promise<DesktopRuntimeSummary> {
  if (!isDesktopMode) return emptyStatus.runtime;
  try { return await GetRuntimeSummary(); } catch { return emptyStatus.runtime; }
}
export async function fetchConfigSummary(): Promise<DesktopConfigSummary> {
  if (!isDesktopMode) return emptyStatus.configSummary;
  try { return await GetConfigSummary(); } catch { return emptyStatus.configSummary; }
}
export async function runDesktopSelfCheck(): Promise<DesktopSelfCheck> {
  if (!isDesktopMode) {
    return { health: "browser", checks: [], warnings: ["当前为浏览器模式，发布前检查器不可用"], completedAt: new Date().toISOString(), serverReachable: true };
  }
  try { return await RunSelfCheck(); } catch { return { health: "error", checks: [], warnings: ["自检执行异常"], completedAt: new Date().toISOString(), serverReachable: false }; }
}
export async function fetchWindowState(): Promise<DesktopWindowState> {
  if (!isDesktopMode) return emptyWindowState;
  try { return await GetWindowState(); } catch { return emptyWindowState; }
}
export function persistWindowState(next: DesktopWindowState) { if (!isDesktopMode) return; void SaveWindowState(next); }
export async function restoreDesktopRoute() { if (!isDesktopMode) return "/dashboard"; return RestoreLastRoute(); }
export function minimiseDesktopWindow() { if (!isDesktopMode) return; void MinimiseWindow(); }
export function toggleDesktopMaximise() { if (!isDesktopMode) return; void ToggleMaximiseWindow(); }
export function closeDesktopWindow() { if (!isDesktopMode) return; void CloseWindow(); }
export function hideDesktopToTray() { if (!isDesktopMode) return; void HideToTray(); }
export function openDesktopAdminInBrowser() { if (!isDesktopMode) { window.open("/admin/dashboard", "_blank"); return; } void OpenAdminInBrowser(); }
export function toggleDesktopAIStatsWidget() {
  if (!isDesktopMode) return;
  void (window as DesktopRuntimeBridge).go?.main?.DesktopApp?.ToggleAIStatsWidget?.();
}
export function sendDesktopNotice(title: string, message: string) { if (!isDesktopMode) return; void SendNativeNotice(title, message); }

function subscribe<T>(eventName: string, handler: (payload: T) => void) {
  if (!isDesktopMode) return () => undefined;
  const unsubscribe = EventsOn(eventName, (payload) => handler(payload as T));
  return () => unsubscribe();
}

export const onDesktopStatus = (handler: (payload: DesktopStatus) => void) => subscribe<DesktopStatus>("desktop:server-ready", handler);
export const onDesktopDomReady = (handler: (payload: DesktopStatus) => void) => subscribe<DesktopStatus>("desktop:dom-ready", handler);
export const onDesktopNotice = (handler: (payload: DesktopNotice) => void) => subscribe<DesktopNotice>("desktop:notice", handler);
export const onDesktopSelfCheck = (handler: (payload: DesktopSelfCheck) => void) => subscribe<DesktopSelfCheck>("desktop:self-check", handler);
export const onDesktopRestoreRoute = (handler: (payload: string) => void) => subscribe<string>("desktop:restore-route", handler);
export const onDesktopWindowHidden = (handler: (payload: DesktopWindowState) => void) => subscribe<DesktopWindowState>("desktop:window-hidden", handler);
export const onDesktopWindowShown = (handler: (payload: DesktopWindowState) => void) => subscribe<DesktopWindowState>("desktop:window-shown", handler);

export { isDesktopMode };
