import { h } from "preact";
import { render, screen, fireEvent, waitFor } from "@testing-library/preact";
import { describe, it, expect, vi, beforeEach } from "vitest";
import Login from "../../components/Login";

describe("Login Component", () => {
  const mockOnLogin = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  it("deve renderizar o formulario de login", () => {
    render(<Login onLogin={mockOnLogin} />);
    expect(screen.getByPlaceholderText("Usuário")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Senha")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /entrar/i })).toBeInTheDocument();
  });

  it("deve chamar onLogin com credenciais corretas", async () => {
    (global.fetch as any).mockResolvedValueOnce({ ok: true });

    render(<Login onLogin={mockOnLogin} />);

    fireEvent.input(screen.getByPlaceholderText("Usuário"), {
      target: { value: "admin" },
    });
    fireEvent.input(screen.getByPlaceholderText("Senha"), {
      target: { value: "pass123" },
    });
    fireEvent.click(screen.getByRole("button", { name: /entrar/i }));

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith("/api/status", {
        headers: {
          Authorization: `Basic ${btoa("admin:pass123")}`,
        },
      });
      expect(mockOnLogin).toHaveBeenCalledWith(
        `Basic ${btoa("admin:pass123")}`,
      );
    });
  });

  it("deve mostrar erro com credenciais invalidas (401)", async () => {
    (global.fetch as any).mockResolvedValueOnce({
      ok: false,
      status: 401,
    });

    render(<Login onLogin={mockOnLogin} />);

    fireEvent.input(screen.getByPlaceholderText("Usuário"), {
      target: { value: "test" },
    });
    fireEvent.input(screen.getByPlaceholderText("Senha"), {
      target: { value: "wrong" },
    });
    fireEvent.click(screen.getByRole("button", { name: /entrar/i }));

    await waitFor(() => {
      expect(
        screen.getByText("Usuário ou senha incorretos."),
      ).toBeInTheDocument();
    });
    expect(mockOnLogin).not.toHaveBeenCalled();
  });

  it("deve tentar novamente em erro de rede", async () => {
    // Simula 2 falhas e depois sucesso na 3a tentativa
    (global.fetch as any)
      .mockRejectedValueOnce(new Error("Network error"))
      .mockResolvedValueOnce({ ok: true });

    render(<Login onLogin={mockOnLogin} />);

    fireEvent.input(screen.getByPlaceholderText("Usuário"), {
      target: { value: "admin" },
    });
    fireEvent.input(screen.getByPlaceholderText("Senha"), {
      target: { value: "pass" },
    });
    fireEvent.click(screen.getByRole("button", { name: /entrar/i }));

    // Deve chamar fetch pelo menos 2x (1a falha + retry)
    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledTimes(2);
      expect(mockOnLogin).toHaveBeenCalled();
    }, { timeout: 3000 });
  });

});
