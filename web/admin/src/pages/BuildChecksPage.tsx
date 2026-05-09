import { BadgeCheck, Boxes, FileCheck2, PackageOpen, ScanSearch } from "lucide-react";
import { useEffect, useState } from "react";
import { SectionHeader } from "../components/SectionHeader";
import { buildCheckStatusMap } from "../store/labels";
import { isDesktopMode, runDesktopSelfCheck, type DesktopCheckItem, type DesktopSelfCheck } from "../utils/desktop-bridge";

const fallbackChecks: DesktopCheckItem[] = [
  { key: "assets", title: "管理后台构建资源", description: "等待 web/admin 构建产物同步到嵌入目录", status: "pending", detail: "执行 npm run build 后会同步 build/embed/admin" },
  { key: "portable", title: "便携目录结构", description: "确认打包目录和默认配置文件", status: "ready", detail: "浏览器模式下展示静态检查项" },
  { key: "runtime", title: "运行验证", description: "等待桌面运行环境执行真实自检", status: "blocked", detail: "浏览器模式下不会调用 Wails 自检" },
  { key: "release", title: "发布清单", description: "等待生成压缩包、版本说明和校验文件", status: "pending", detail: "当前尚未生成正式发布物" }
];

export function BuildChecksPage() {
  const [report, setReport] = useState<DesktopSelfCheck | null>(null);
  const checks = report?.checks?.length ? report.checks : fallbackChecks;

  useEffect(() => {
    if (isDesktopMode) void runDesktopSelfCheck().then(setReport);
  }, []);

  return (
    <section className="page-grid build-check-layout">
      <article className="panel">
        <SectionHeader
          eyebrow="构建检查"
          title="发布前检查清单"
          description="在发布桌面版或便携版前，逐项核对 Provider、数据库、资源、端口和打包产物。"
          actions={
            <button type="button" className="btn btn-primary" onClick={() => void runDesktopSelfCheck().then(setReport)}>
              <ScanSearch size={16} /> 重新执行检查
            </button>
          }
        />

        <div className="context-strip">
          <span className="metric-pill">检查模式：{isDesktopMode ? "桌面发布前检查器" : "浏览器预检视图"}</span>
          <span className="metric-pill">总体状态：{buildCheckStatusMap[report?.health ?? "pending"] ?? report?.health ?? "待检查"}</span>
          <span className="metric-pill">完成时间：{report?.completedAt ? new Date(report.completedAt).toLocaleString() : "尚未执行"}</span>
        </div>

        <div className="wizard-step-list list-animate">
          {checks.map((item, index) => {
            const iconMap = {
              providers: BadgeCheck,
              database: Boxes,
              assets: PackageOpen,
              port: ScanSearch,
              packaging: FileCheck2
            } as const;
            const Icon = iconMap[item.key as keyof typeof iconMap] ?? FileCheck2;
            return (
              <article key={item.key} className={`wizard-step-card build-check-card ${item.status}`} style={{ ["--index" as string]: index }}>
                <span className="wizard-index">0{index + 1}</span>
                <div>
                  <strong><Icon size={16} /> {item.title}</strong>
                  <p>{item.description}</p>
                  <p>{item.detail}</p>
                  <span className="pill pill-neutral mt-4">{buildCheckStatusMap[item.status] ?? item.status}</span>
                </div>
              </article>
            );
          })}
        </div>

        {report?.warnings?.length ? (
          <div className="stack-list mt-4">
            {report.warnings.map((warning) => (
              <article key={warning} className="alert-card warning">
                <strong>发布前注意</strong>
                <p>{warning}</p>
              </article>
            ))}
          </div>
        ) : null}
      </article>
    </section>
  );
}
