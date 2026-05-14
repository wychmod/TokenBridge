import { CircleCheckBig, Hash, Shield, TimerReset } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { SectionHeader } from "../components/SectionHeader";
import { labelFromMap, platformLabelMap } from "../store/labels";
import { fetchDesktopStatus, fetchDesktopVersion, isDesktopMode } from "../utils/desktop-bridge";

type VersionBlock = {
  label: string;
  value: string;
  icon: typeof Hash;
};

export function VersionInfoPage() {
  const [desktopVersion, setDesktopVersion] = useState("0.1.0-alpha");
  const [platform, setPlatform] = useState("web");

  useEffect(() => {
    void fetchDesktopVersion().then((version) => {
      if (version && version !== "browser") setDesktopVersion(version);
    });
    void fetchDesktopStatus().then((status) => {
      if (status.platform) setPlatform(status.platform);
    });
  }, []);

  const versionBlocks = useMemo<VersionBlock[]>(() => [
    { label: "版本号", value: isDesktopMode ? desktopVersion : "浏览器控制台", icon: Hash },
    { label: "发布通道", value: isDesktopMode ? `桌面版 · ${labelFromMap(platformLabelMap, platform)}` : "Web Admin", icon: TimerReset },
    { label: "安全状态", value: "默认本地访问 · Key 脱敏展示", icon: Shield },
    { label: "当前状态", value: isDesktopMode ? "桌面特性已启用" : "网页端与桌面端共用 web/admin", icon: CircleCheckBig }
  ], [desktopVersion, platform]);

  return (
    <section className="page-grid version-layout">
      <article className="panel">
        <SectionHeader
          eyebrow="版本概览"
          title="TokenBridge 版本与产品状态"
          description="展示当前运行形态、平台、安全基线和发布通道，方便判断桌面端与网页端是否处在同一交付源。"
        />
        <div className="kpi-grid version-grid">
          {versionBlocks.map((item) => {
            const Icon = item.icon;
            return (
              <article key={item.label} className="kpi-card">
                <span className="kpi-label">{item.label}</span>
                <strong className="kpi-value"><Icon size={18} /> {item.value}</strong>
              </article>
            );
          })}
        </div>
      </article>
    </section>
  );
}
