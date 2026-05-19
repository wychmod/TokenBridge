import { Link } from "react-router-dom";
import { ShieldCheck, Sparkles, Wand2 } from "lucide-react";
import { SectionHeader } from "../components/SectionHeader";
import { useAdminStore } from "../store/admin-store";
import { useUIStore } from "../store/ui-store";

const setupSteps = [
  "确认本地监听地址",
  "设置管理员账号",
  "检查 Provider 与密钥状态",
  "确认桌面 / 浏览器访问路径",
  "完成首次启用标记"
];

export function BootstrapPage() {
  const { settings, providers, keys, pushNotice } = useAdminStore();
  const { setInitialized } = useUIStore();
  const enabledProviders = providers.filter((provider) => provider.enabled).length;
  const activeKeys = keys.filter((key) => key.enabled && key.status !== "revoked").length;

  const completeBootstrap = () => {
    setInitialized(true);
    pushNotice({
      tone: "success",
      title: "首次启用已确认",
      message: "初始化标记已写入当前前端会话；正式配置仍由后端设置页保存。"
    });
  };

  return (
    <section className="page-grid bootstrap-layout">
      <article className="panel">
        <SectionHeader
          eyebrow="启用引导"
          title="TokenBridge 首次启用向导"
          description="用少量步骤确认“本地服务是否开着、账号是否明确、接下来该配什么”，再进入完整控制台。"
          actions={
            <button type="button" className="btn btn-primary" onClick={completeBootstrap}>
              <Wand2 size={16} /> 标记为已启用
            </button>
          }
        />
        <div className="wizard-step-list list-animate">
          {setupSteps.map((step, index) => (
            <article key={step} className="wizard-step-card" style={{ ["--index" as string]: index }}>
              <span className="wizard-index">0{index + 1}</span>
              <div>
                <strong>{step}</strong>
                <p>{index === 0 ? `当前监听 ${settings.host}:${settings.port}，管理后台路径 ${settings.adminPath}。` : "完成后进入正式配置页继续细化参数。"}</p>
              </div>
            </article>
          ))}
        </div>
      </article>

      <article className="panel">
        <SectionHeader eyebrow="初始化状态" title="当前运行基线" description="这里读取当前真实管理状态，不再使用独立演示数据。" />
        <div className="status-grid">
          <div className="metric-pill"><ShieldCheck size={16} /> 管理员账号：{settings.adminUsername || "未配置"}</div>
          <div className="metric-pill"><Sparkles size={16} /> 可用供应商：{enabledProviders}/{providers.length}</div>
          <div className="metric-pill">可用密钥：{activeKeys}/{keys.length}</div>
          <div className="metric-pill">主题策略：{settings.theme}</div>
        </div>
        <div className="section-actions mt-4">
          <Link className="btn btn-secondary" to="/settings">去设置页确认</Link>
          <Link className="btn btn-ghost" to="/bootstrap/success">查看完成页</Link>
        </div>
      </article>
    </section>
  );
}
