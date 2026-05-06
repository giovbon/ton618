import { useEffect, useState } from "preact/hooks";

interface DocsPanelProps {
  isOpen: boolean;
  onClose: () => void;
  fetchWithAuth: (
    url: string,
    options?: RequestInit,
  ) => Promise<Response | null>;
}

const ICONS: Record<string, string> = {
  search: `<svg style="width:16px;height:16px;vertical-align:-2px;display:inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/></svg>`,
  compact: `<svg style="width:16px;height:16px;vertical-align:-2px;display:inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M4 6h16M4 12h16m-7 6h7"/></svg>`,
  map: `<svg style="width:16px;height:16px;vertical-align:-2px;display:inline" fill="none" viewBox="0 0 24 24"><path d="M9 20L3 17V4L9 7M9 20L15 17M9 20V7M15 17L21 20V7L15 4M15 17V4M9 7L15 4" stroke="#0ea5e9" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>`,
  note: `<svg style="width:16px;height:16px;vertical-align:-2px;display:inline;color:#38bdf8" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M12 4v16m8-8H4"/></svg>`,
  link: `<svg style="width:16px;height:16px;vertical-align:-2px;display:inline;color:#f59e0b" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"/></svg>`,
  camera: `<svg style="width:16px;height:16px;vertical-align:-2px;display:inline;color:#34d399" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M15 13a3 3 0 11-6 0 3 3 0 016 0z"/></svg>`,
  pdf: `<svg style="width:16px;height:16px;vertical-align:-2px;display:inline;color:#f87171" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"/></svg>`,
  bundle: `<svg style="width:16px;height:16px;vertical-align:-2px;display:inline;color:#818cf8" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/></svg>`,
  sync: `<svg style="width:16px;height:16px;vertical-align:-2px;display:inline;color:#a1a1aa" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>`,
  docs: `<svg style="width:16px;height:16px;vertical-align:-2px;display:inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253"/></svg>`,
  settings: `<svg style="width:16px;height:16px;vertical-align:-2px;display:inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/></svg>`,
};

function renderMarkdown(md: string): string {
  // 1. Salvar icones (substitui :icon-nome: por placeholder)
  const saved: string[] = [];
  let html = md.replace(/:icon-(\w+):/g, (_, name: string) => {
    saved.push(ICONS[name] || name);
    return "\x00I" + (saved.length - 1) + "\x00";
  });

  // 2. Escapar HTML
  html = html
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");

  // 3. Tabelas
  html = html.replace(/((?:^\|.+\|$\n?)+)/gm, (block) => {
    const lines = block.trim().split("\n");
    if (lines.length < 2) return block;
    const cells = (l: string) =>
      l
        .replace(/^\||\|$/g, "")
        .split("|")
        .map((c) => c.trim());
    const hdr = cells(lines[0]);
    const body = lines.slice(lines[1].match(/^[\|\s\-:]+$/) ? 2 : 1);
    if (!body.length) return block;
    const th = hdr
      .map(
        (c) =>
          `<th style="background:#27272a;color:#e4e4e7;padding:8px 12px;text-align:left;font-size:13px;font-weight:600;border-bottom:2px solid #3f3f46">${c}</th>`,
      )
      .join("");
    const trs = body
      .map(
        (l) =>
          `<tr style="border-bottom:1px solid #27272a">${cells(l)
            .map(
              (c) =>
                `<td style="padding:6px 12px;color:#a1a1aa;font-size:13px">${c}</td>`,
            )
            .join("")}</tr>`,
      )
      .join("");
    return `<table style="width:100%;border-collapse:collapse;margin:16px 0;border:1px solid #27272a;border-radius:8px;overflow:hidden"><thead><tr>${th}</tr></thead><tbody>${trs}</tbody></table>`;
  });

  // 4. Code blocks (pre) e inline code
  html = html.replace(
    /```(\w*)\n([\s\S]*?)```/g,
    (_, lang, code) =>
      `<pre style="background:#1e1e2e;color:#cdd6f4;padding:16px;border-radius:8px;overflow-x:auto;font-size:13px;font-family:monospace;line-height:1.5;margin:12px 0">${code.trim()}</pre>`,
  );
  html = html.replace(
    /`([^`]+)`/g,
    '<code style="color:#38bdf8;background:#1e293b;padding:1px 6px;border-radius:4px;font-size:13px;font-family:monospace">$1</code>',
  );

  // 5. Headers
  html = html
    .replace(
      /^### (.+)$/gm,
      '<h3 style="color:#e4e4e7;font-size:16px;margin:20px 0 8px;font-weight:700">$1</h3>',
    )
    .replace(
      /^## (.+)$/gm,
      '<h2 style="color:#f4f4f5;font-size:18px;margin:24px 0 10px;font-weight:800;padding-bottom:6px;border-bottom:1px solid #27272a">$1</h2>',
    )
    .replace(
      /^# (.+)$/gm,
      '<h1 style="color:#fff;font-size:22px;margin:32px 0 12px;font-weight:900">$1</h1>',
    );

  // 6. Lists (depois do code, antes de bold/italic)
  html = html
    .replace(
      /^\- (.+)$/gm,
      '<li style="color:#a1a1aa;margin:4px 0 4px 16px">$1</li>',
    )
    .replace(
      /^(\d+)\. (.+)$/gm,
      '<li style="color:#a1a1aa;margin:4px 0 4px 16px">$1. $2</li>',
    );

  // 7. Bold, italic
  html = html
    .replace(
      /\*\*(.+?)\*\*/g,
      '<strong style="color:#e4e4e7;font-weight:700">$1</strong>',
    )
    .replace(/\*(.+?)\*/g, '<em style="color:#d4d4d8">$1</em>');

  // 8. Horizontal rule, blockquote, links
  html = html
    .replace(
      /^---$/gm,
      '<hr style="border:none;border-top:1px solid #27272a;margin:20px 0">',
    )
    .replace(
      /^> (.+)$/gm,
      '<blockquote style="border-left:3px solid #0ea5e9;padding:8px 16px;margin:12px 0;color:#a1a1aa;background:rgba(14,165,233,0.05);border-radius:0 8px 8px 0">$1</blockquote>',
    )
    .replace(
      /\[([^\]]+)\]\(([^)]+)\)/g,
      '<a href="$2" style="color:#38bdf8;text-decoration:none" target="_blank">$1</a>',
    );

  // 9. Paragrafos
  html = html
    .replace(
      /\n\n/g,
      "</p><p style='color:#a1a1aa;line-height:1.7;margin:8px 0'>",
    )
    .replace(/^(.+)$/gm, (line) => {
      if (
        /^<\/?(h[1-3]|li|hr|blockquote|table|thead|tbody|th|td|tr|pre|code|ul|ol|p)/.test(
          line,
        )
      )
        return line;
      return `<p style="color:#a1a1aa;line-height:1.7;margin:8px 0">${line}</p>`;
    });

  // 10. Restaurar icones
  html = html.replace(/\x00I(\d+)\x00/g, (_, i) => saved[+i] || "");

  return html;
}

export const DocsPanel = ({
  isOpen,
  onClose,
  fetchWithAuth,
}: DocsPanelProps) => {
  const [html, setHtml] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!isOpen) return;
    let cancel = false;
    setLoading(true);
    setError("");
    fetchWithAuth("/help/README.md")
      .then((r) => (r.ok ? r.text() : Promise.reject("Status " + r.status)))
      .then((md) => {
        if (!cancel) setHtml(renderMarkdown(md));
      })
      .catch((e) => {
        if (!cancel) setError(String(e));
      })
      .finally(() => {
        if (!cancel) setLoading(false);
      });
    return () => {
      cancel = true;
    };
  }, [isOpen, fetchWithAuth]);

  useEffect(() => {
    if (!isOpen) return;
    const h = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  return (
    <div
      style={{
        position: "fixed",
        inset: 0,
        zIndex: 200,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        padding: 16,
        background: "rgba(0,0,0,0.7)",
        backdropFilter: "blur(4px)",
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        style={{
          background: "#18181b",
          border: "1px solid #27272a",
          borderRadius: 16,
          width: "100%",
          maxWidth: 800,
          maxHeight: "85vh",
          display: "flex",
          flexDirection: "column",
          overflow: "hidden",
        }}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            padding: "14px 20px",
            borderBottom: "1px solid #27272a",
            flexShrink: 0,
          }}
        >
          <span style={{ fontSize: 18, fontWeight: 700, color: "#f4f4f5" }}>
            📖 Documentação
          </span>
          <button
            onClick={onClose}
            style={{
              background: "none",
              border: "none",
              color: "#71717a",
              cursor: "pointer",
              fontSize: 18,
              padding: "4px 8px",
              borderRadius: 6,
            }}
          >
            ✕
          </button>
        </div>
        <div
          style={{ flex: 1, overflowY: "auto", padding: 24 }}
          className="custom-scrollbar"
        >
          {loading && !html && (
            <div style={{ textAlign: "center", padding: 40, color: "#71717a" }}>
              Carregando...
            </div>
          )}
          {error && (
            <div style={{ textAlign: "center", padding: 40, color: "#ef4444" }}>
              Erro: {error}
            </div>
          )}
          {html && <div dangerouslySetInnerHTML={{ __html: html }} />}
        </div>
      </div>
    </div>
  );
};
