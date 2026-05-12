import { Routes, Route, Navigate } from "react-router-dom";
import { AppShell } from "./layouts/AppShell";
import { DashboardPage } from "./pages/DashboardPage";
import { ProvidersPage } from "./pages/ProvidersPage";
import { KeysPage } from "./pages/KeysPage";
import { RoutingPage } from "./pages/RoutingPage";
import { AnalyticsPage } from "./pages/AnalyticsPage";
import { AIToolUsagePage } from "./pages/AIToolUsagePage";
import { LogsPage } from "./pages/LogsPage";
import { PricingPage } from "./pages/PricingPage";
import { SettingsPage } from "./pages/SettingsPage";
import { BootstrapPage } from "./pages/BootstrapPage";
import { BootstrapSuccessPage } from "./pages/BootstrapSuccessPage";
import { SecurityPage } from "./pages/SecurityPage";
import { ReleaseStatusPage } from "./pages/ReleaseStatusPage";
import { VersionInfoPage } from "./pages/VersionInfoPage";
import { BuildChecksPage } from "./pages/BuildChecksPage";
import { QuickSetupPage } from "./pages/QuickSetupPage";

export default function App() {
  return (
    <AppShell>
      <Routes>
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="/dashboard" element={<DashboardPage />} />
        <Route path="/providers" element={<ProvidersPage />} />
        <Route path="/keys" element={<KeysPage />} />
        <Route path="/routing" element={<RoutingPage />} />
        <Route path="/analytics" element={<AnalyticsPage />} />
        <Route path="/ai-tool-usage" element={<AIToolUsagePage />} />
        <Route path="/logs" element={<LogsPage />} />
        <Route path="/pricing" element={<PricingPage />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="/bootstrap" element={<BootstrapPage />} />
        <Route path="/bootstrap/success" element={<BootstrapSuccessPage />} />
        <Route path="/security" element={<SecurityPage />} />
        <Route path="/release-status" element={<ReleaseStatusPage />} />
        <Route path="/version" element={<VersionInfoPage />} />
        <Route path="/build-checks" element={<BuildChecksPage />} />
        <Route path="/quick-setup" element={<QuickSetupPage />} />
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </AppShell>
  );
}
