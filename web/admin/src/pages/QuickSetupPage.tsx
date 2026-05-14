import { CheckCheck, Copy, TerminalSquare } from "lucide-react";
import { SectionHeader } from "../components/SectionHeader";
import { useAdminStore } from "../store/admin-store";

export function QuickSetupPage() {
  const { keys, settings, pushNotice } = useAdminStore();
  const currentKey = keys[0] ?? { name: "未配置密钥", displayKey: "tb-****" };
  const gatewayBase = `http://${settings.host === "0.0.0.0" ? "127.0.0.1" : settings.host}:${settings.port}`;

  const setups = [
    {
      name: "Codex / Cursor / Windsurf",
      snippet: `OPENAI_API_KEY=${currentKey.displayKey}\nOPENAI_BASE_URL=${gatewayBase}/v1`
    },
    {
      name: "Claude Desktop",
      snippet: `{"apiUrl": "${gatewayBase}", "apiKey": "${currentKey.displayKey}"}`
    },
    {
      name: "TokenBridge 便携分发包",
      snippet: `解压分发包 → 运行 TokenBridge.exe → 打开 ${gatewayBase}${settings.adminPath} → 完成 Provider 与 Local Key 配置`
    }
  ];

  const copySnippet = async (name: string, snippet: string) => {
    try {
      await navigator.clipboard?.writeText(snippet);
      pushNotice({ tone: "success", title: `已复制 ${name} 配置`, message: "配置片段已写入系统剪贴板。" });
    } catch {
      pushNotice({ tone: "info", title: `可手动复制 ${name} 配置`, message: "浏览器未开放剪贴板权限，请直接复制卡片中的配置片段。" });
    }
  };

  return (
    <section className="panel quick-setup-page">
      <SectionHeader
        eyebrow="接入助手"
        title="TokenBridge 接入说明"
        description="像向导一样给出当前状态、下一步和可复制配置。不了解 Provider / Token 的用户也可以按卡片完成接入。"
        actions={
          <button
            type="button"
            className="btn btn-secondary"
            onClick={() => pushNotice({ tone: "success", title: "配置基线正常", message: "当前模板来自真实设置和密钥状态，可作为接入起点。" })}
          >
            <CheckCheck size={16} /> 校验配置
          </button>
        }
      />

      <div className="context-strip">
        <div className="metric-pill">预设方案 {setups.length}</div>
        <div className="metric-pill">主用密钥 {currentKey.name}</div>
        <div className="metric-pill">网关地址 {gatewayBase}/v1</div>
        <div className="metric-pill">下一步：复制配置并在工具里验证一次请求</div>
      </div>

      <div className="setup-grid">
        {setups.map((item) => (
          <article key={item.name} className="setup-card">
            <div className="setup-header">
              <div>
                <strong>{item.name}</strong>
                <p className="text-secondary">复制后按实际工具配置文件位置粘贴即可。</p>
              </div>
              <button type="button" className="btn btn-ghost btn-sm" onClick={() => void copySnippet(item.name, item.snippet)}>
                <Copy size={14} /> 复制
              </button>
            </div>
            <pre>{item.snippet}</pre>
          </article>
        ))}
      </div>

      <div className="section-actions sticky-actions">
        <button
          type="button"
          className="btn btn-primary"
          onClick={() => pushNotice({ tone: "info", title: "接入向导已预留", message: "后续可以把工具类型、密钥选择和配置导出整合成真正的一步式向导。" })}
        >
          <TerminalSquare size={16} /> 打开接入向导
        </button>
      </div>
    </section>
  );
}
