import { useState, useRef, useEffect } from "preact/hooks";

interface LinksEditorProps {
  links: string[];
  onLinksChange: (links: string[]) => void;
  suggestions: string[];
}

/**
 * LinksEditor — Componente visual para gerenciar links semânticos (@[]) no frontmatter.
 *
 * - Exibe links atuais como pills removíveis
 * - Input com autocomplete de tópicos existentes
 * - Adiciona ao pressionar Enter
 * - Suporta navegação por setas no dropdown
 */
export function LinksEditor({
  links,
  onLinksChange,
  suggestions,
}: LinksEditorProps) {
  const [input, setInput] = useState("");
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(-1);
  const inputRef = useRef<HTMLInputElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);

  // Filtra sugestões: só mostra as que dão match e não estão já adicionadas
  const filtered = suggestions.filter(
    (s) => s.toLowerCase().includes(input.toLowerCase()) && !links.includes(s),
  );

  const addLink = (topic: string) => {
    const trimmed = topic.trim();
    if (trimmed && !links.includes(trimmed)) {
      onLinksChange([...links, trimmed]);
    }
    setInput("");
    setShowSuggestions(false);
    setSelectedIndex(-1);
    inputRef.current?.focus();
  };

  const removeLink = (topic: string) => {
    onLinksChange(links.filter((l) => l !== topic));
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      if (selectedIndex >= 0 && filtered[selectedIndex]) {
        addLink(filtered[selectedIndex]);
      } else if (input.trim()) {
        addLink(input);
      }
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      setSelectedIndex((prev) => Math.min(prev + 1, filtered.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setSelectedIndex((prev) => Math.max(prev - 1, -1));
    } else if (e.key === "Escape") {
      setShowSuggestions(false);
    } else if (e.key === "Backspace" && !input && links.length > 0) {
      // Backspace no input vazio remove o último link
      removeLink(links[links.length - 1]);
    }
  };

  // Fecha o dropdown ao clicar fora
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(e.target as Node)
      ) {
        setShowSuggestions(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  return (
    <div className="space-y-2">
      <label className="text-xs font-mono text-zinc-500 uppercase tracking-wider">
        Links Semânticos (@[])
      </label>

      {/* Links atuais como pills removíveis */}
      <div className="flex flex-wrap gap-1.5 min-h-[28px]">
        {links.length === 0 && (
          <span className="text-xs text-zinc-600 italic self-center">
            Nenhum link definido
          </span>
        )}
        {links.map((link) => (
          <span
            key={link}
            className="inline-flex items-center gap-1 text-violet-400 bg-violet-500/10 px-2 py-0.5 rounded-md border border-violet-500/20 text-xs font-bold group"
          >
            <span>@[{link}]</span>
            <button
              onClick={() => removeLink(link)}
              className="text-violet-500/40 hover:text-violet-300 transition-colors ml-0.5 opacity-0 group-hover:opacity-100 focus:opacity-100 text-sm leading-none"
              title="Remover link"
              type="button"
            >
              ×
            </button>
          </span>
        ))}
      </div>

      {/* Input para adicionar novo link com autocomplete */}
      <div className="relative" ref={dropdownRef}>
        <div className="flex items-center gap-1 bg-zinc-900/50 border border-zinc-800 rounded-lg px-2 py-1.5 transition-all focus-within:border-violet-500/50 focus-within:bg-zinc-900">
          <span className="text-violet-400 font-bold text-xs select-none shrink-0">
            @[
          </span>
          <input
            ref={inputRef}
            type="text"
            value={input}
            onInput={(e: any) => {
              setInput(e.target.value);
              setShowSuggestions(true);
              setSelectedIndex(-1);
            }}
            onFocus={() => {
              if (input) setShowSuggestions(true);
            }}
            onKeyDown={handleKeyDown}
            placeholder="Nome do tópico..."
            className="flex-1 bg-transparent border-none outline-none text-xs text-zinc-300 placeholder-zinc-600 min-w-[120px]"
          />
          <span className="text-violet-400 font-bold text-xs select-none shrink-0">
            ]
          </span>
        </div>

        {/* Dropdown de autocomplete */}
        {showSuggestions && input && (
          <div className="absolute top-full left-0 right-0 mt-1 bg-zinc-900 border border-zinc-700/80 rounded-lg shadow-2xl z-[200] overflow-hidden animate-in fade-in slide-in-from-top-1 duration-150">
            {filtered.length === 0 ? (
              <div className="px-3 py-2.5 text-zinc-500 text-xs italic text-center">
                {input.trim()
                  ? "Pressione Enter para criar novo link"
                  : "Digite para buscar tópicos existentes"}
              </div>
            ) : (
              filtered.map((s, i) => (
                <button
                  key={s}
                  onClick={() => addLink(s)}
                  onMouseEnter={() => setSelectedIndex(i)}
                  type="button"
                  className={`w-full text-left px-3 py-2 text-xs font-mono transition-colors flex items-center gap-1 ${
                    i === selectedIndex
                      ? "bg-violet-500/20 text-violet-300"
                      : "text-zinc-300 hover:bg-zinc-800"
                  }`}
                >
                  <span className="text-violet-500/70">@[</span>
                  <span>{s}</span>
                  <span className="text-violet-500/70">]</span>
                  {i === selectedIndex && (
                    <span className="ml-auto text-[9px] font-mono text-zinc-600">
                      ↵
                    </span>
                  )}
                </button>
              ))
            )}
          </div>
        )}
      </div>
    </div>
  );
}

// ── Helpers para sincronizar com YAML frontmatter ──────────────────────

/**
 * Extrai a lista de links do frontmatter YAML (string).
 * Suporta formato:
 *   links:
 *     - topico1
 *     - topico2
 */
export function parseLinksFromFrontmatter(fm: string): string[] {
  const links: string[] = [];
  let inLinks = false;

  for (const line of fm.split("\n")) {
    const trimmed = line.trim();

    if (trimmed.startsWith("links:")) {
      inLinks = true;
      // Verifica se há inline após "links:" (ex: links: [a, b] ou links: a)
      const afterColon = trimmed.slice(6).trim();
      if (afterColon) {
        // Tenta detectar array inline: links: [a, b]
        const bracketMatch = afterColon.match(/^\[([\s\S]*)\]$/);
        if (bracketMatch) {
          links.push(
            ...bracketMatch[1]
              .split(",")
              .map((s) => s.trim().replace(/^["']|["']$/g, ""))
              .filter(Boolean),
          );
          inLinks = false; // Já consumiu tudo na mesma linha
        } else if (afterColon.startsWith("- ")) {
          // Formato: links:\n  - item  (mas na mesma linha — YAML suporta)
          // Na prática não ocorre, mas tratamos por segurança
          links.push(afterColon.slice(2).trim());
        }
        // Se for só o valor escalar (links: algo), trata como string única
        // mas inLinks = true para não pegar linhas seguintes acidentalmente
      }
      continue;
    }

    if (inLinks) {
      if (trimmed.startsWith("- ")) {
        links.push(trimmed.slice(2).trim());
      } else if (trimmed.startsWith("-")) {
        links.push(trimmed.slice(1).trim());
      } else if (trimmed === "" || trimmed.startsWith("#")) {
        // Linha vazia ou comentário — continua
        continue;
      } else {
        // Linha que não parece ser continuação da lista → saiu do bloco links
        inLinks = false;
      }
    }
  }

  return links;
}

/**
 * Gera a string YAML para a chave "links" a partir de um array.
 * Usa o formato com hífen:
 *   links:
 *     - topico1
 *     - topico2
 */
export function linksToYaml(links: string[]): string {
  if (links.length === 0) return "";
  const items = links.map((l) => `  - ${l}`).join("\n");
  return `links:\n${items}`;
}

/**
 * Substitui ou insere o bloco "links" no frontmatter YAML existente.
 * Se "links" já existir, substitui. Caso contrário, insere antes do final.
 */
export function setLinksInFrontmatter(fm: string, links: string[]): string {
  const linksBlock = linksToYaml(links);

  // Se não há links e não há bloco links no frontmatter, retorna como está
  if (!linksBlock && !fm.includes("links:")) return fm;

  // Remove bloco "links" existente (linha "links:" + linhas seguintes indentadas)
  const lines = fm.split("\n");
  const result: string[] = [];
  let skippingLinks = false;

  for (const line of lines) {
    if (line.trim().startsWith("links:")) {
      skippingLinks = true;
      continue;
    }
    if (skippingLinks) {
      // Pula linhas indentadas (continuação do bloco links)
      if (line.startsWith(" ") || line.startsWith("\t") || line.trim() === "") {
        continue;
      }
      skippingLinks = false;
    }
    result.push(line);
  }

  // Se ainda estamos pulando (links era a última seção), finaliza
  if (skippingLinks) {
    skippingLinks = false;
  }

  // Insere o novo bloco links no final (antes de qualquer linha em branco final)
  if (linksBlock) {
    // Encontra onde inserir: antes do último \n\n ou no final
    let insertIdx = result.length;
    // Procura linha em branco no final
    while (insertIdx > 0 && result[insertIdx - 1].trim() === "") {
      insertIdx--;
    }
    result.splice(insertIdx, 0, linksBlock);
  }

  return result.join("\n");
}
