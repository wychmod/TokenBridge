import { PropsWithChildren } from "react";
import { Link } from "react-router-dom";
import { Lock } from "lucide-react";
import { useUIStore } from "../store/ui-store";

export function InitializationGuard({ children }: PropsWithChildren) {
  const { initialized } = useUIStore();

  if (initialized) {
    return <>{children}</>;
  }

  return (
    <section className="panel empty-state initialization-guard">
      <div className="guard-icon">
        <Lock size={24} />
      </div>
      <h2 className="section-title">还未完成首次启用</h2>
      <p className="empty-state-desc">
        建议先确认管理员账号、安全基线和便携运行目录，再进入正式配置区。你也可以稍后再做，不会阻塞现有生产页面。
      </p>
      <div className="section-actions">
        <Link className="btn btn-primary" to="/bootstrap">开始启用</Link>
        <Link className="btn btn-secondary" to="/security">查看安全中心</Link>
      </div>
    </section>
  );
}
