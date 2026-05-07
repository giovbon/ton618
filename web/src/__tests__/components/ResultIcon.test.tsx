import { render } from "@testing-library/preact";
import { describe, expect, it } from "vitest";
import { ResultIcon } from "../../components/ResultIcon";

function makeDoc(overrides: {
  tipo?: string;
  arquivo?: string;
  tags?: string[];
  is_indexing?: boolean;
} = {}): any {
  return {
    id: "test-1",
    tipo: overrides.tipo ?? "markdown",
    arquivo: overrides.arquivo ?? "nota.md",
    tags: overrides.tags ?? [],
    is_indexing: overrides.is_indexing ?? false,
    texto: "",
    secao: "",
    "@timestamp": new Date().toISOString(),
  };
}

describe("ResultIcon", () => {
  it("PDF tipo=pdf → vermelho", () => {
    const { container } = render(<ResultIcon doc={makeDoc({ tipo: "pdf" })} />);
    expect(container.querySelector('[title="Documento PDF"]')).toBeTruthy();
    expect(container.innerHTML).toContain("text-red-500");
  });

  it("PDF tag=documento → vermelho", () => {
    const { container } = render(<ResultIcon doc={makeDoc({ tags: ["documento"] })} />);
    expect(container.querySelector('[title="Documento PDF"]')).toBeTruthy();
  });

  it("Imagem tipo=image → emerald", () => {
    const { container } = render(<ResultIcon doc={makeDoc({ tipo: "image" })} />);
    expect(container.innerHTML).toContain("text-emerald-400");
  });

  it("Imagem tipo=imagem → emerald", () => {
    const { container } = render(<ResultIcon doc={makeDoc({ tipo: "imagem" })} />);
    expect(container.innerHTML).toContain("text-emerald-400");
  });

  it("Imagem tag=imagem (nota OCR) → emerald", () => {
    const { container } = render(<ResultIcon doc={makeDoc({ tags: ["imagem"] })} />);
    expect(container.innerHTML).toContain("text-emerald-400");
  });

  it("Link arquivo comeca com links/ → amber", () => {
    const { container } = render(<ResultIcon doc={makeDoc({ arquivo: "links/artigo.md" })} />);
    expect(container.querySelector('[title="Link Capturado"]')).toBeTruthy();
  });

  it("Arquivos tag=arquivos → indigo", () => {
    const { container } = render(<ResultIcon doc={makeDoc({ tags: ["arquivos"] })} />);
    expect(container.querySelector('[title="Arquivos (Temporário)"]')).toBeTruthy();
  });

  it("Desenho tipo=desenho → purple", () => {
    const { container } = render(<ResultIcon doc={makeDoc({ tipo: "desenho" })} />);
    expect(container.querySelector('[title="Desenho Tldraw"]')).toBeTruthy();
  });

  it("Markdown padrão → sky", () => {
    const { container } = render(<ResultIcon doc={makeDoc({ tipo: "markdown" })} />);
    expect(container.querySelector('[title="Nota Markdown"]')).toBeTruthy();
  });

  it("Bolinha vermelha quando isIndexing=true (image)", () => {
    const { container } = render(<ResultIcon doc={makeDoc({ tipo: "image" })} isIndexing={true} />);
    expect(container.innerHTML).toContain("bg-red-500");
  });

  it("Bolinha verde quando isIndexing=false (image)", () => {
    const { container } = render(<ResultIcon doc={makeDoc({ tipo: "image" })} isIndexing={false} />);
    expect(container.innerHTML).toContain("bg-emerald-500");
  });

  // Regressao: nota OCR criada como .md com tag=imagem deve mostrar icone de imagem
  it("Nota OCR (.md + tag=imagem) → icone de imagem emerald", () => {
    const { container } = render(
      <ResultIcon
        doc={makeDoc({
          tipo: "markdown",
          tags: ["imagem"],
          arquivo: "notes/ocr_20260506_foto.md",
        })}
      />
    );
    expect(container.innerHTML).toContain("text-emerald-400");
    expect(container.querySelector('[title*="Imagem"]')).toBeTruthy();
  });
});
