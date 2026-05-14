import { LockKeyhole, ShieldAlert } from "lucide-react";
import { SectionHeader } from "../components/SectionHeader";
import { useAdminStore } from "../store/admin-store";

export function SecurityPage() {
  const { settings, saveSettings, pushNotice } = useAdminStore();

  return (
    <section className="page-grid split-layout security-layout">
      <article className="panel">
        <SectionHeader
          eyebrow="安全中心"
          title="登录保护与安全基线"
          description="这里把默认本地访问、管理员入口、日志敏感边界和后续登录能力放在一个低噪声页面中，避免普通用户误开公网暴露。"
          actions={
            <button
              type="button"
              className="btn btn-primary"
              onClick={() => pushNotice({ tone: "info", title: "登录模块待接入", message: "当前版本保留入口与基线展示，真实登录会话将在后端认证能力接入后启用。" })}
            >
              <LockKeyhole size={16} /> 检查登录入口
            </button>
          }
        />
        <div className="form-grid">
          <div className="form-field">
            <label className="form-label">管理员账号</label>
            <input className="form-control" value={settings.adminUsername} onChange={(event) => saveSettings({ ...settings, adminUsername: event.target.value })} />
          </div>
          <div className="form-field">
            <label className="form-label">监听地址</label>
            <input className="form-control" value={settings.host} onChange={(event) => saveSettings({ ...settings, host: event.target.value })} />
            <span className="form-hint">生产默认推荐仅监听 127.0.0.1。</span>
          </div>
          <div className="form-field span-2">
            <label className="form-label">日志级别</label>
            <input className="form-control" value={settings.logLevel} onChange={(event) => saveSettings({ ...settings, logLevel: event.target.value })} />
          </div>
        </div>
      </article>

      <article className="panel">
        <SectionHeader eyebrow="安全清单" title="默认安全基线" description="下载即用不等于默认裸奔，关键风险必须明确展示。" />
        <div className="stack-list">
          <article className="alert-card warning">
            <strong>首次启动建议设置管理员账号</strong>
            <p>管理员账号当前为 {settings.adminUsername || "未配置"}，后续应接入强制改密和会话过期。</p>
          </article>
          <article className="alert-card success">
            <strong>默认仅本地访问</strong>
            <p>监听地址保持在 {settings.host}，如需开放局域网访问应在设置页显式修改。</p>
          </article>
          <article className="alert-card">
            <strong>密钥只展示脱敏值</strong>
            <p>Local Key 创建后只应短暂展示原文，列表长期展示 display key。</p>
          </article>
          <article className="alert-card danger">
            <strong><ShieldAlert size={16} /> Prompt 级完整日志默认不采集</strong>
            <p>避免本地排障能力变成敏感数据泄露入口。</p>
          </article>
        </div>
      </article>
    </section>
  );
}
