import { Download, PackageCheck, TimerReset } from "lucide-react";
import { SectionHeader } from "../components/SectionHeader";
import { useAdminStore } from "../store/admin-store";

const releaseChecklist = [
  "构建 web/admin 并同步嵌入资源",
  "执行 Go 测试与桌面自检",
  "生成 Windows / macOS 桌面产物",
  "输出便携目录与默认配置",
  "补齐版本说明和校验信息"
];

export function ReleaseStatusPage() {
  const { distributionPlan, settings, pushNotice } = useAdminStore();
  const packageName = distributionPlan?.package_name ?? "Lingshu-Portable.zip";
  const releaseMode = distributionPlan?.mode ?? settings.bundleMode ?? "single-binary";

  return (
    <section className="page-grid release-layout">
      <article className="panel">
        <SectionHeader
          eyebrow="发布状态"
          title="便携版分发与发布状态"
          description="围绕“下载后直接用”的桌面交付目标，把构建、嵌入资源和运行自检状态集中展示。"
          actions={
            <button
              type="button"
              className="btn btn-primary"
              onClick={() => pushNotice({ tone: "info", title: "构建命令", message: "请在本地执行 build/desktop.ps1 或 Wails 构建命令生成正式产物。" })}
            >
              <Download size={16} /> 查看构建命令
            </button>
          }
        />
        <div className="status-grid">
          <div className="metric-pill"><PackageCheck size={16} /> 分发包：{packageName}</div>
          <div className="metric-pill">分发模式：{releaseMode}</div>
          <div className="metric-pill">资源路径：web/admin → build/embed/admin</div>
          <div className="metric-pill"><TimerReset size={16} /> 发布状态：待本机构建验证</div>
        </div>
      </article>

      <article className="panel">
        <SectionHeader eyebrow="待办清单" title="发布待办清单" description="合并后桌面与网页统一从同一套前端产物进入发布链路。" />
        <div className="wizard-step-list list-animate">
          {releaseChecklist.map((step, index) => (
            <article key={step} className="wizard-step-card" style={{ ["--index" as string]: index }}>
              <span className="wizard-index">0{index + 1}</span>
              <div>
                <strong>{step}</strong>
                <p>完成该项后，发布链路更接近可下载即运行的工业级交付标准。</p>
              </div>
            </article>
          ))}
        </div>
      </article>
    </section>
  );
}
