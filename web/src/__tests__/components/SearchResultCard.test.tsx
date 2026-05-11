import { fireEvent, render, screen } from "@testing-library/preact";
import { describe, expect, it, vi } from "vitest";
import { CompactResultCard } from "../../components/CompactResultCard";
import { SearchResultCard } from "../../components/SearchResultCard";

// Mock de sub-componentes ou funções se necessário
vi.mock("../../utils/tagStyles", () => ({
  getTagStyle: () => ({ bgCol: "#000", mainCol: "#fff", borderCol: "#ccc" }),
}));

// Mock do IntersectionObserver para o JSDOM
const IntersectionObserverMock = vi.fn(() => ({
  disconnect: vi.fn(),
  observe: vi.fn(),
  takeRecords: vi.fn(),
  unobserve: vi.fn(),
}));
vi.stubGlobal("IntersectionObserver", IntersectionObserverMock);

describe("SearchResultCard - Tag Resilience", () => {
  const mockDoc = {
    id: "test-1",
    tipo: "markdown",
    arquivo: "test.md",
    texto: "Test content",
    tags: Array.from({ length: 15 }, (_, i) => `tag${i + 1}`),
    "@timestamp": new Date().toISOString(),
  } as any;

  it('should only show first 10 tags by default and a "more" button', () => {
    render(
      <SearchResultCard
        doc={mockDoc}
        query=""
        onEdit={() => {}}
        onDeleteFile={() => {}}
        fetchWithAuth={() => Promise.resolve(null)}
        index={0}
        searchTerms={[]}
        isCompact={false}
        isLastCollapsed={false}
        auth={null}
      />,
    );

    // Deve encontrar exatamente 10 tags (span com texto #tagN)
    const visibleTags = screen.queryAllByText(/#tag/i);
    expect(visibleTags.length).toBe(10);

    // Deve encontrar o botão "+ 5 tags"
    const moreButton = screen.getByText("+ 5 tags");
    expect(moreButton).toBeDefined();
  });

  it('should show all tags when "more" button is clicked', () => {
    render(
      <SearchResultCard
        doc={mockDoc}
        query=""
        onEdit={() => {}}
        onDeleteFile={() => {}}
        fetchWithAuth={() => Promise.resolve(null)}
        index={0}
        searchTerms={[]}
        isCompact={false}
        isLastCollapsed={false}
        auth={null}
      />,
    );

    const moreButton = screen.getByText("+ 5 tags");
    fireEvent.click(moreButton);

    // Agora deve encontrar as 15 tags
    const allTags = screen.queryAllByText(/#tag/i);
    expect(allTags.length).toBe(15);

    // Botão de colapsar deve aparecer
    expect(screen.getByText("colapsar")).toBeDefined();
  });

  it("should show count in COMPACT mode instead of list", () => {
    render(
      <CompactResultCard
        doc={mockDoc}
        index={0}
        query=""
        onEdit={() => {}}
        onDeleteFile={() => {}}
        fetchWithAuth={() => Promise.resolve(null)}
        searchTerms={[]}
        auth={null}
      />,
    );

    // No novo componente CompactResultCard, as 4 primeiras tags são mostradas
    expect(screen.getByText(/#tag3/i)).toBeDefined();
    expect(screen.queryByText(/#tag5/i)).toBeNull();
    // Exibe +11 (15 total - 4 visíveis)
    expect(screen.getByText("+11")).toBeDefined();
  });
});
