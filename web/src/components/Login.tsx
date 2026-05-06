import { useState } from 'preact/hooks';

import { Logo } from './Logo';

interface LoginProps {
  onLogin: (authHeader: string) => void;
}

export default function Login({ onLogin }: LoginProps) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleSubmit = async (e?: Event) => {
    if (e) e.preventDefault();
    setError('');
    setIsSubmitting(true);

    // Encode credentials for Basic Auth
    const authHeader = `Basic ${btoa(`${username}:${password}`)}`;

    try {
      const resp = await fetch('/api/status', {
        headers: { Authorization: authHeader },
      });

      if (resp.ok) {
        onLogin(authHeader);
      } else {
        setError('Usuário ou senha incorretos.');
      }
    } catch (_err) {
      setError('Erro ao conectar ao servidor.');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="login-page">
      <div className="login-card">
        <div className="login-header">
          <div className="login-logo-container">
            <Logo className="login-logo" />
          </div>
          <h1>TON-618</h1>
          <p>{'Jogue tudo no escuro. Atraia tudo. Nossa busca rompe o horizonte de eventos.'}</p>
        </div>

        <form onSubmit={handleSubmit} className="login-form">
          <div className="input-group">
            <label htmlFor="user">{'Usuário'}</label>
            <input
              id="user"
              type="text"
              value={username}
              onInput={(e: any) => setUsername(e.target.value)}
              placeholder={'Usuário'}
              required
            />
          </div>

          <div className="input-group">
            <label htmlFor="pass">{'Senha'}</label>
            <input
              id="pass"
              type="password"
              value={password}
              onInput={(e: any) => setPassword(e.target.value)}
              placeholder={'Senha'}
              required
            />
          </div>

          {error && <div className="login-error">{error}</div>}

          <button
            type="submit"
            className={`login-button ${isSubmitting ? 'loading' : ''}`}
            disabled={isSubmitting}
          >
            {isSubmitting ? 'Verificando...' : 'Entrar'}
          </button>
        </form>

        <div className="login-footer">
          <p>{'Auto-hospedado & Privado'}</p>
        </div>
      </div>

      <style>{`
        .login-page {
          height: 100vh;
          width: 100vw;
          display: flex;
          align-items: center;
          justify-content: center;
          background: #09090b;
          color: #f8fafc;
          font-family: 'Inter', system-ui, -apple-system, sans-serif;
        }

        .login-card {
          width: 100%;
          max-width: 400px;
          padding: 2.5rem;
          background: rgba(30, 41, 59, 0.4);
          backdrop-filter: blur(12px);
          border: 1px solid rgba(255, 255, 255, 0.1);
          border-radius: 24px;
          box-shadow: none;
          animation: slideUp 0.6s cubic-bezier(0.16, 1, 0.3, 1);
        }

        @keyframes slideUp {
          from { opacity: 0; transform: translateY(20px); }
          to { opacity: 1; transform: translateY(0); }
        }

        .login-header {
          text-align: center;
          margin-bottom: 2rem;
        }

        .login-logo-container {
          display: flex;
          justify-content: center;
          margin-bottom: 1rem;
          filter: drop-shadow(0 0 15px rgba(59, 130, 246, 0.5));
        }

        .login-logo {
          width: 60px;
          height: 60px;
          animation: pulseGlow 3s infinite alternate;
        }

        @keyframes pulseGlow {
          from { filter: drop-shadow(0 0 5px rgba(59, 130, 246, 0.3)); }
          to { filter: drop-shadow(0 0 20px rgba(59, 130, 246, 0.8)); }
        }

        h1 {
          font-size: 2rem;
          font-weight: 800;
          letter-spacing: -0.025em;
          margin: 0;
          background: linear-gradient(to right, #60a5fa, #3b82f6);
          -webkit-background-clip: text;
          -webkit-text-fill-color: transparent;
        }

        p {
          color: #94a3b8;
          font-size: 0.875rem;
          margin-top: 0.5rem;
        }

        .login-form {
          display: flex;
          flex-direction: column;
          gap: 1.25rem;
        }

        .input-group {
          display: flex;
          flex-direction: column;
          gap: 0.5rem;
        }

        label {
          font-size: 0.875rem;
          font-weight: 500;
          color: #cbd5e1;
          margin-left: 0.25rem;
        }

        input {
          background: rgba(15, 23, 42, 0.6);
          border: 1px solid rgba(255, 255, 255, 0.1);
          border-radius: 12px;
          padding: 0.75rem 1rem;
          color: white;
          font-size: 1rem;
          transition: all 0.2s;
        }

        input:focus {
          outline: none;
          border-color: #3b82f6;
          box-shadow: 0 0 0 4px rgba(59, 130, 246, 0.1);
          background: rgba(15, 23, 42, 0.8);
        }

        .login-error {
          color: #f87171;
          font-size: 0.875rem;
          text-align: center;
          background: rgba(248, 113, 113, 0.1);
          padding: 0.5rem;
          border-radius: 8px;
        }

        .login-button {
          margin-top: 0.5rem;
          background: #3b82f6;
          color: white;
          border: none;
          border-radius: 12px;
          padding: 0.875rem;
          font-size: 1rem;
          font-weight: 600;
          cursor: pointer;
          transition: all 0.2s;
        }

        .login-button:hover {
          background: #2563eb;
          transform: translateY(-1px);
          box-shadow: 0 4px 12px rgba(37, 99, 235, 0.3);
        }

        .login-button:active {
          transform: translateY(0);
        }

        .login-button:disabled {
          opacity: 0.7;
          cursor: not-allowed;
        }

        .login-footer {
          margin-top: 2rem;
          text-align: center;
          border-top: 1px solid rgba(255, 255, 255, 0.05);
          padding-top: 1.5rem;
        }

        .login-footer p {
          font-size: 0.75rem;
          color: #64748b;
          letter-spacing: 0.05em;
          text-transform: uppercase;
        }
      `}</style>
    </div>
  );
}
