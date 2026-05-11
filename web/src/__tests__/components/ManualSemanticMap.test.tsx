import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, waitFor } from "@testing-library/preact";
import { ManualSemanticMap } from "../../components/ManualSemanticMap";

// ─── Minimal mocks para nao quebrar imports ───────────────────────

vi.mock("d3-force", () => ({
  forceSimulation: () => ({
    force: () => ({
      force: () => ({
        force: () => ({
          force: () => ({
            force: () => ({ nodes: () => {} }),
          }),
        }),
      }),
    }),
    on: () => ({}),
    alpha: () => 0,
    alphaTarget: () => {},
    restart: () => {},
    stop: () => {},
    find: () => null,
  }),
  forceLink: () => ({ id: () => ({ distance: () => {} }) }),
  forceManyBody: () => ({ strength: () => {} }),
  forceCenter: () => ({}),
  forceCollide: () => ({ radius: () => {} }),
}));

vi.mock("d3-zoom", () => ({
  zoom: () => {
    const b: any = () => b;
    b.scaleExtent = () => b;
    b.filter = () => b;
    b.on = () => b;
    return b;
  },
  zoomIdentity: {
    x: 0,
    y: 0,
    k: 1,
    invertX: (v: number) => v,
    invertY: (v: number) => v,
  },
  ZoomTransform: class {
    x = 0;
    y = 0;
    k = 1;
    invertX = (v: number) => (v - this.x) / this.k;
    invertY = (v: number) => (v - this.y) / this.k;
  },
}));

vi.mock("d3-selection", () => ({
  select: () => ({ call: () => ({ call: () => {} }) }),
}));

// ─── Tests ──────────────────────────────────────────────────────────

describe("ManualSemanticMap", () => {
  beforeEach(() => {
    HTMLCanvasElement.prototype.getContext = vi.fn(
      () =>
        ({
          save: vi.fn(),
          restore: vi.fn(),
          clearRect: vi.fn(),
          setTransform: vi.fn(),
          beginPath: vi.fn(),
          arc: vi.fn(),
          fill: vi.fn(),
          moveTo: vi.fn(),
          lineTo: vi.fn(),
          stroke: vi.fn(),
          setLineDash: vi.fn(),
          fillText: vi.fn(),
          measureText: vi.fn(() => ({ width: 10 })),
          font: "",
          fillStyle: "",
          strokeStyle: "",
          lineWidth: 1,
          textAlign: "",
          textBaseline: "",
        }) as any,
    );
    vi.spyOn(window, "requestAnimationFrame").mockImplementation(
      (cb) => window.setTimeout(() => cb(Date.now()), 16) as any,
    );
    vi.spyOn(window, "cancelAnimationFrame").mockImplementation((id) =>
      clearTimeout(id),
    );
  });

  afterEach(() => vi.restoreAllMocks());

  it("renderiza canvas e botao voltar", async () => {
    (global.fetch as any).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ topics: [], links: [] }),
    });
    const { container, findByText } = render(
      <ManualSemanticMap auth="test" onOpenNote={vi.fn()} onClose={vi.fn()} />,
    );
    await waitFor(() =>
      expect(container.textContent).not.toContain("Sincronizando"),
    );
    expect(container.querySelector("canvas")).toBeTruthy();
    expect(await findByText("VOLTAR")).toBeTruthy();
  });

  it("mostra erro quando fetch falha", async () => {
    (global.fetch as any).mockRejectedValue(new Error("Falha"));
    const { findByText } = render(
      <ManualSemanticMap auth="test" onOpenNote={vi.fn()} onClose={vi.fn()} />,
    );
    expect(await findByText(/Erro ao carregar/)).toBeTruthy();
  });

  it("chama onClose ao clicar em VOLTAR", async () => {
    const onClose = vi.fn();
    (global.fetch as any).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ topics: [], links: [] }),
    });
    const { findByText } = render(
      <ManualSemanticMap auth="test" onOpenNote={vi.fn()} onClose={onClose} />,
    );
    (await findByText("VOLTAR")).click();
    expect(onClose).toHaveBeenCalled();
  });
});
