import { Link } from "react-router-dom";
import { Sparkles, ShieldCheck } from "lucide-react";
import { SectionHeader } from "../components/SectionHeader";
import { useAdminStore } from "../store/admin-store";
import { useUIStore } from "../store/ui-store";

export function BootstrapSuccessPage() {
  const { setInitialized } = useUIStore();
  const { settings } = useAdminStore();

  return (
    <section className="page-grid success-layout">
      <article className="panel success-panel">
        <SectionHeader
          eyebrow="初始化完成"
          title="灵枢已进入正式管理模式"
          description="基础运行边界已经确认，接下来可以配置厂商、密钥、路由和日志观测。"
          actions={
            <Link className="btn btn-primary" to="/dashboard" onClick={() => setInitialized(true)}>
              <ShieldCheck size={16} /> 进入总台
            </Link>
          }
        />
        <div className="success-stack">
          <div className="metric-pill"><Sparkles size={16} /> 管理后台：{settings.adminPath}</div>
          <div className="metric-pill">监听地址：{settings.host}:{settings.port}</div>
          <div className="metric-pill">主题策略：{settings.theme}</div>
          <div className="metric-pill">数据保留：{settings.retentionDays} 天</div>
        </div>
      </article>
    </section>
  );
}
