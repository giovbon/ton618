import { useCallback, useEffect, useState } from 'preact/hooks';

import type { RankingWeights } from '../types';

interface WeightInputProps {
  label: string;
  value: number;
  description: string;
  onChange: (name: string, val: number) => void;
  name: string;
  icon: any;
  color?: string;
}

const WeightInput = ({
  label,
  value,
  description,
  onChange,
  name,
  icon,
  color = 'sky',
}: WeightInputProps) => {
  const [inputValue, setInputValue] = useState(value.toString().replace('.', ','));

  const colorStyles: Record<string, string> = {
    sky: 'group-hover:text-sky-400 group-hover:bg-sky-500/10 group-hover:border-sky-500/20 shadow-sky-500/5',
    amber:
      'group-hover:text-amber-400 group-hover:bg-amber-500/10 group-hover:border-amber-500/20 shadow-amber-500/5',
    indigo:
      'group-hover:text-indigo-400 group-hover:bg-indigo-500/10 group-hover:border-indigo-500/20 shadow-indigo-500/5',
    violet:
      'group-hover:text-violet-400 group-hover:bg-violet-500/10 group-hover:border-violet-500/20 shadow-violet-500/5',
    emerald:
      'group-hover:text-emerald-400 group-hover:bg-emerald-500/10 group-hover:border-emerald-500/20 shadow-emerald-500/5',
    cyan: 'group-hover:text-cyan-400 group-hover:bg-cyan-500/10 group-hover:border-cyan-500/20 shadow-cyan-500/5',
    fuchsia:
      'group-hover:text-fuchsia-400 group-hover:bg-fuchsia-500/10 group-hover:border-fuchsia-500/20 shadow-fuchsia-500/5',
    orange:
      'group-hover:text-orange-400 group-hover:bg-orange-500/10 group-hover:border-orange-500/20 shadow-orange-500/5',
    rose: 'group-hover:text-rose-400 group-hover:bg-rose-500/10 group-hover:border-rose-500/20 shadow-rose-500/5',
  };

  const currentStyle = colorStyles[color] || colorStyles.sky;

  useEffect(() => {
    setInputValue(value.toString().replace('.', ','));
  }, [value]);

  const handleChange = (e: any) => {
    const rawValue = e.target.value;
    setInputValue(rawValue);
    const normalizedValue = rawValue.replace(',', '.');
    const parsed = parseFloat(normalizedValue);
    if (!Number.isNaN(parsed)) onChange(name, parsed);
  };

  const labelColors: Record<string, string> = {
    sky: 'text-sky-400/70 group-hover:text-sky-400',
    amber: 'text-amber-400/70 group-hover:text-amber-400',
    indigo: 'text-indigo-400/70 group-hover:text-indigo-400',
    violet: 'text-violet-400/70 group-hover:text-violet-400',
    emerald: 'text-emerald-400/70 group-hover:text-emerald-400',
    cyan: 'text-cyan-400/70 group-hover:text-cyan-400',
    fuchsia: 'text-fuchsia-400/70 group-hover:text-fuchsia-400',
    orange: 'text-orange-400/70 group-hover:text-orange-400',
    rose: 'text-rose-400/70 group-hover:text-rose-400',
  };

  const currentLabelColor = labelColors[color] || labelColors.sky;

  return (
    <div className="flex flex-col gap-1 p-3 bg-zinc-900/40 border border-zinc-800/60 rounded-xl hover:border-zinc-700/60 transition-all group relative overflow-hidden">
      <div
        className={`absolute top-0 right-0 w-20 h-20 blur-2xl rounded-full -mr-10 -mt-10 transition-opacity opacity-0 group-hover:opacity-100 ${currentStyle.split(' ').pop()}`}
      />

      <div className="flex items-center justify-between gap-3 relative z-10">
        <div className="flex items-center gap-2">
          <div
            className={`w-6 h-6 rounded-lg bg-zinc-800 flex items-center justify-center text-zinc-500 transition-all border border-zinc-700/50 ${currentStyle}`}
          >
            {icon}
          </div>
          <label
            className={`text-[9px] font-black uppercase tracking-widest transition-colors ${currentLabelColor}`}
          >
            {label}
          </label>
        </div>
        <input
          type="text"
          inputMode="decimal"
          value={inputValue}
          onInput={handleChange}
          className="w-14 bg-zinc-950 border border-zinc-800 text-zinc-100 text-right px-2 py-1 rounded-lg focus:ring-1 focus:ring-sky-500 outline-none font-mono text-xs shadow-inner group-hover:border-zinc-700"
        />
      </div>
      <p className="text-[11px] text-zinc-500 leading-tight mt-1 group-hover:text-zinc-400 transition-colors">
        {description}
      </p>
    </div>
  );
};

interface WeightsSettingsProps {
  isOpen: boolean;
  onClose: () => void;
  fetchWithAuth: (url: string, init?: RequestInit) => Promise<Response | null>;
  onUpdate?: () => void;
  onLogout?: () => void;
}

export const WeightsSettings = ({
  isOpen,
  onClose,
  fetchWithAuth,
  onUpdate,
  onLogout,
}: WeightsSettingsProps) => {
  const [weights, setWeights] = useState<RankingWeights | null>(null);
  const [settings, setSettings] = useState({
    google_vision_key: '',
    semantic_threshold: 0.2,
    semantic_enable: true,
    semantic_strategy: 'whitelist',
    language: 'pt-BR',
  });
  const [isSaving, setIsSaving] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [activeTab, setActiveTab] = useState('backup'); // 'backup' | 'pesos' | 'apis' | 'manutencao'

  const [isResetConfirmOpen, setIsResetConfirmOpen] = useState(false);

  // Manutenção / Cleanup State
  const [cleanupSettings, setCleanupSettings] = useState({
    years: 1,
    photos: true,
    notes: false,
    pdfs: false,
    zombies: false,
    abandoned: false,
    captures: false,
    inactivity: true,
    minSizeMb: 0,
    targetTags: '',
    tagMode: 'any', // "any" | "only"
  });
  const [staleInfo, setStaleInfo] = useState<any>(null);
  const [backupSize, setBackupSize] = useState<number | null>(null);
  const [isAnalyzing, setIsAnalyzing] = useState(false);
  const [isCleaning, setIsCleaning] = useState(false);
  const [isCleanupConfirmOpen, setIsCleanupConfirmOpen] = useState(false);
  const [isSuccess, setIsSuccess] = useState(false);

  const fetchBackupSize = useCallback(async () => {
    try {
      const res = await fetchWithAuth('/api/backup/size');
      if (res?.ok) {
        const data = await res.json();
        setBackupSize(data.totalSize);
      }
    } catch (err) {
      console.error('Erro ao carregar tamanho do backup:', err);
    }
  }, [fetchWithAuth]);

  const fetchSettings = useCallback(async () => {
    try {
      const res = await fetchWithAuth('/api/settings');
      if (res?.ok) {
        const data = await res.json();
        setSettings(data);
      }
    } catch (err) {
      console.error('Erro ao carregar APIs:', err);
    }
  }, [fetchWithAuth]);

  const fetchWeights = useCallback(async () => {
    setIsLoading(true);
    try {
      const res = await fetchWithAuth('/api/weights');
      if (res?.ok) {
        const data = await res.json();
        setWeights(data);
      }
    } catch (err) {
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  }, [fetchWithAuth]);

  useEffect(() => {
    if (isOpen) {
      fetchWeights();
      fetchSettings();
      if (activeTab === 'backup') fetchBackupSize();
    }
  }, [isOpen, activeTab, fetchWeights, fetchSettings, fetchBackupSize]);

  const handleUpdateWeight = (name: string, val: number) => {
    setWeights((prev: any) => ({ ...prev, [name]: val }));
  };

  const handleSave = async () => {
    setIsSaving(true);
    try {
      // Salvar Pesos
      const resWeights = await fetchWithAuth('/api/weights', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(weights),
      });

      // Salvar Configurações de API
      const resSettings = await fetchWithAuth('/api/settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(settings),
      });

      if (resWeights?.ok && resSettings?.ok) {
        if (onUpdate) onUpdate();
        onClose();
      } else {
        alert('Erro ao salvar algumas configurações.');
      }
    } catch (_err) {
      alert('Erro de conexão ao salvar.');
    } finally {
      setIsSaving(false);
    }
  };

  const handleReset = async () => {
    setIsResetConfirmOpen(false);
    setIsLoading(true);
    try {
      const res = await fetchWithAuth('/api/weights', { method: 'DELETE' });
      if (res?.ok) {
        const data = await res.json();
        setWeights(data);
        if (onUpdate) onUpdate();
      }
    } catch (_err) {
      alert('Erro ao resetar.');
    } finally {
      setIsLoading(false);
    }
  };

  if (!isOpen) return null;

  const ICONS = {
    exact: (
      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth="2.5"
          d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
        />
      </svg>
    ),
    partial: (
      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth="2.5"
          d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
        />
      </svg>
    ),
    phrase: (
      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth="2.5"
          d="M7 8h10M7 12h4m1 8l-4-4H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-3l-4 4z"
        />
      </svg>
    ),
    path: (
      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth="2.5"
          d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"
        />
      </svg>
    ),
    h1: (
      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth="2.5"
          d="M7 20V4m0 8h10M17 4v16"
        />
      </svg>
    ),
    clock: (
      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth="2.5"
          d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
        />
      </svg>
    ),
    semantic: (
      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth="2.5"
          d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2-2v10a2 2 0 002 2z"
        />
      </svg>
    ),
  };

  return (
    <div className="fixed inset-0 z-[100] flex flex-col bg-zinc-950 animate-in fade-in duration-300 overflow-hidden">
      <div className="flex-1 flex flex-col md:flex-row w-full h-full bg-zinc-950 overflow-hidden relative">
        {/* Grainy Texture Background */}
        <div className="absolute inset-0 pointer-events-none opacity-[0.03] grainy" />

        <aside className="w-full md:w-72 h-fit md:h-screen bg-zinc-900/10 border-b md:border-b-0 md:border-r border-zinc-800 flex flex-col p-4 md:p-6 z-20 shrink-0 sticky top-0 md:static">
          <button
            onClick={onClose}
            className="hidden md:flex w-fit items-center gap-2 py-2 px-3 mb-8 rounded-xl bg-zinc-800/50 hover:bg-zinc-800 text-zinc-400 hover:text-zinc-200 text-[10px] font-black uppercase tracking-widest transition-all border border-zinc-700/30 active:scale-95"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth="2.5"
                d="M10 19l-7-7m0 0l7-7m-7 7h18"
              />
            </svg>
            {'Cancelar'}
          </button>

          <div className="mb-4 md:mb-6 px-1 md:px-2">
            <h2 className="text-zinc-100 font-black text-[10px] md:text-xs uppercase tracking-[0.2em] md:tracking-[0.25em]">
              {'Backup'}
            </h2>
            <p className="text-zinc-600 text-[8px] md:text-[10px] font-bold mt-1 uppercase tracking-widest">
              {'Segurança de Dados'}
            </p>
          </div>

          <nav className="flex md:flex-col gap-2 overflow-x-auto md:overflow-visible no-scrollbar pb-2 md:pb-0">
            <button
              onClick={() => setActiveTab('backup')}
              className={`flex-shrink-0 flex items-center gap-2 md:gap-3 px-3 md:px-4 py-2 md:py-3 rounded-lg md:rounded-xl text-[8px] md:text-[10px] font-black uppercase tracking-widest transition-all ${activeTab === 'backup' ? 'bg-sky-500/10 text-sky-400 border border-sky-500/20 shadow-lg' : 'text-zinc-500 hover:text-zinc-400'}`}
            >
              <svg
                className="w-3.5 h-3.5 md:w-4 md:h-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2.5"
                  d="M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l-3-3m3 3V4"
                />
              </svg>
              {'Backup'}
            </button>
            <button
              onClick={() => setActiveTab('pesos')}
              className={`flex-shrink-0 flex items-center gap-2 md:gap-3 px-3 md:px-4 py-2 md:py-3 rounded-lg md:rounded-xl text-[8px] md:text-[10px] font-black uppercase tracking-widest transition-all ${activeTab === 'pesos' ? 'bg-sky-500/10 text-sky-400 border border-sky-500/20 shadow-lg' : 'text-zinc-500 hover:text-zinc-400'}`}
            >
              <svg
                className="w-3.5 h-3.5 md:w-4 md:h-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2.5"
                  d="M3 6l3 12h12l3-12H3z"
                />
              </svg>
              {'Pesos e Semântica'}
            </button>
            <button
              onClick={() => setActiveTab('apis')}
              className={`flex-shrink-0 flex items-center gap-2 md:gap-3 px-3 md:px-4 py-2 md:py-3 rounded-lg md:rounded-xl text-[8px] md:text-[10px] font-black uppercase tracking-widest transition-all ${activeTab === 'apis' ? 'bg-amber-500/10 text-amber-400 border border-amber-500/20 shadow-lg' : 'text-zinc-500 hover:text-zinc-400'}`}
            >
              <svg
                className="w-3.5 h-3.5 md:w-4 md:h-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2.5"
                  d="M13 10V3L4 14h7v7l9-11h-7z"
                />
              </svg>
              APIs
            </button>
            <button
              onClick={() => setActiveTab('manutencao')}
              className={`flex-shrink-0 flex items-center gap-2 md:gap-3 px-3 md:px-4 py-2 md:py-3 rounded-lg md:rounded-xl text-[8px] md:text-[10px] font-black uppercase tracking-widest transition-all ${activeTab === 'manutencao' ? 'bg-rose-500/10 text-rose-400 border border-rose-500/20 shadow-lg' : 'text-zinc-500 hover:text-zinc-400'}`}
            >
              <svg
                className="w-3.5 h-3.5 md:w-4 md:h-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2.5"
                  d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                />
              </svg>
              {'Limpeza'}
            </button>
          </nav>

          <div className="hidden md:flex flex-1" />

          {/* Desktop Action Buttons */}
          <div className="hidden md:flex flex-col gap-3 pt-6 border-t border-zinc-800/50">
            <button
              onClick={handleSave}
              disabled={isSaving}
              className="w-full py-4 rounded-2xl bg-sky-500 text-white text-[10px] font-black uppercase tracking-[0.2em] shadow-lg shadow-sky-500/20 hover:bg-sky-400 active:scale-[0.95] transition-all disabled:bg-zinc-800 disabled:text-zinc-600"
            >
              {isSaving ? 'Salvando...' : 'Salvar'}
            </button>
            <div className="flex gap-2">
              <button
                onClick={onClose}
                className="flex-1 py-2.5 rounded-xl text-[9px] font-black uppercase tracking-widest text-zinc-500 hover:text-zinc-300 transition-all border border-zinc-800/50 hover:bg-zinc-800"
              >
                {'Cancelar'}
              </button>
              <button
                onClick={() => setIsResetConfirmOpen(true)}
                className="flex-1 py-2.5 rounded-xl border border-zinc-800/50 text-zinc-600 text-[9px] font-black uppercase tracking-widest hover:bg-zinc-800 hover:text-rose-400 transition-all active:scale-95"
              >
                {'Resetar'}
              </button>
            </div>

            <button
              onClick={onLogout}
              className="w-full mt-2 py-2.5 rounded-xl border border-rose-500/10 text-rose-500/40 text-[9px] font-black uppercase tracking-widest hover:bg-rose-500/10 hover:text-rose-400 transition-all active:scale-95 flex items-center justify-center gap-2 group"
            >
              <svg
                className="w-3.5 h-3.5 opacity-50 group-hover:opacity-100 transition-opacity"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2.5"
                  d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
                />
              </svg>
              {'Sair da Conta'}
            </button>
          </div>
        </aside>

        {/* Mobile Fixed Bottom Action Bar */}
        <div className="md:hidden fixed bottom-0 left-0 right-0 p-3 bg-zinc-950/80 backdrop-blur-xl border-t border-zinc-800 z-50 flex flex-col gap-2">
          <button
            onClick={handleSave}
            disabled={isSaving}
            className="w-full py-3 rounded-xl bg-sky-500 text-white text-[10px] font-black uppercase tracking-[0.2em] shadow-lg shadow-sky-500/20 active:scale-[0.95] transition-all disabled:bg-zinc-800"
          >
            {isSaving ? 'Salvando...' : 'Salvar'}
          </button>
          <div className="flex gap-2">
            <button
              onClick={onClose}
              className="flex-1 py-2 rounded-lg text-[9px] font-black uppercase tracking-widest text-zinc-400 border border-zinc-800 bg-zinc-900/50"
            >
              {'Cancelar'}
            </button>
            <button
              onClick={() => setIsResetConfirmOpen(true)}
              className="flex-1 py-2 rounded-lg border border-zinc-800 text-zinc-600 text-[9px] font-black uppercase tracking-widest bg-zinc-900/50"
            >
              {'Resetar'}
            </button>
            <button
              onClick={onLogout}
              className="flex-1 py-2 rounded-lg border border-rose-500/20 text-rose-500/50 bg-rose-500/5 flex items-center justify-center"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2.5"
                  d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
                />
              </svg>
            </button>
          </div>
        </div>

        <main className="flex-1 flex flex-col min-w-0 bg-zinc-950/20 overflow-hidden pb-24 md:pb-0">
          <header className="px-4 py-3 md:p-10 flex justify-between items-start">
            <div>
              <h1 className="text-xl md:text-5xl font-black text-zinc-100 tracking-tighter uppercase">
                {activeTab === 'pesos'
                  ? 'Pesos e Semântica'
                  : activeTab === 'apis'
                    ? 'Integração de APIs'
                    : activeTab === 'backup'
                      ? 'Backup e Segurança'
                      : 'Manutenção de Disco'}
              </h1>
              <p className="hidden md:block text-zinc-500 text-sm mt-3 font-medium max-w-2xl">
                {activeTab === 'pesos'
                  ? 'Ajuste como os resultados são priorizados pelo motor de busca.'
                  : activeTab === 'apis'
                    ? 'Configure chaves de serviços externos para expandir as capacidades do PKM.'
                    : activeTab === 'backup'
                      ? 'Exporte seu conhecimento para um arquivo seguro.'
                      : 'Remova conteúdo antigo ou não utilizado para otimizar o espaço e a performance.'}
              </p>
            </div>
          </header>

          <div className="flex-1 overflow-y-auto px-4 md:px-6 pb-4 md:pb-6 custom-scrollbar">
            {isLoading || !weights ? (
              <div className="flex items-center justify-center h-full text-zinc-700 animate-pulse text-[10px] font-black uppercase tracking-widest">
                {'Carregando...'}
              </div>
            ) : activeTab === 'backup' ? (
              <div className="flex flex-col gap-6">
                <div className="p-10 bg-zinc-900/60 border border-zinc-800 rounded-[40px] relative overflow-hidden group">
                  <div className="absolute top-0 right-0 p-10 opacity-5 group-hover:opacity-10 transition-opacity">
                    <svg className="w-32 h-32 text-sky-500" fill="currentColor" viewBox="0 0 24 24">
                      <path d="M19.35 10.04C18.67 6.59 15.64 4 12 4 9.11 4 6.6 5.64 5.35 8.04 2.34 8.36 0 10.91 0 14c0 3.31 2.69 6 6 6h13c2.76 0 5-2.24 5-5 0-2.64-2.05-4.78-4.65-4.96zM17 13l-5 5-5-5h3V9h4v4h3z" />
                    </svg>
                  </div>

                  <div className="max-w-md relative z-10">
                    <h3 className="text-zinc-100 font-black text-2xl uppercase tracking-tighter mb-4 flex items-center gap-3">
                      {'Baixar Backup Completo'}
                    </h3>
                    <p className="text-zinc-500 text-sm leading-relaxed mb-8">
                      {
                        'Gera um arquivo compactado (.zip) contendo todas as suas notas, anexos e imagens da pasta /docs. Seus dados são seus, leve-os para onde quiser.'
                      }
                    </p>

                    <button
                      onClick={async () => {
                        try {
                          const res = await fetchWithAuth('/api/backup/download');
                          if (res?.ok) {
                            const blob = await res.blob();
                            const url = window.URL.createObjectURL(blob);
                            const a = document.createElement('a');
                            a.href = url;
                            a.download = `ton618_backup_${new Date().toISOString().split('T')[0]}.zip`;
                            document.body.appendChild(a);
                            a.click();
                            window.URL.revokeObjectURL(url);
                          }
                        } catch (_err) {
                          alert('Erro ao gerar backup.');
                        }
                      }}
                      className="group relative flex items-center gap-3 px-8 py-4 bg-sky-500 hover:bg-sky-400 text-white rounded-2xl font-black text-[11px] uppercase tracking-[0.2em] transition-all shadow-xl shadow-sky-500/20 active:scale-95"
                    >
                      <svg
                        className="w-5 h-5 group-hover:translate-y-0.5 transition-transform"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth="2.5"
                          d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
                        />
                      </svg>
                      {'Gerar ZIP Agora'}
                    </button>

                    <div className="mt-8 flex items-center gap-3 p-4 bg-zinc-950/50 border border-zinc-800 rounded-2xl">
                      <div className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse" />
                      <p className="text-[10px] font-black text-zinc-400 uppercase tracking-widest">
                        {'Tamanho estimado: '}
                        <span className="text-zinc-100">
                          {backupSize === null
                            ? 'Calculando...'
                            : backupSize < 1024 * 1024
                              ? `${(backupSize / 1024).toFixed(1)} KB`
                              : backupSize < 1024 * 1024 * 1024
                                ? `${(backupSize / (1024 * 1024)).toFixed(1)} MB`
                                : `${(backupSize / (1024 * 1024 * 1024)).toFixed(2)} GB`}
                        </span>
                      </p>
                    </div>
                  </div>
                </div>
              </div>
            ) : activeTab === 'pesos' ? (
              <div className="flex flex-col gap-6">
                {/* Grid de Pesos de Relevância */}
                <div className="p-6 bg-zinc-900/40 border border-zinc-800/60 rounded-3xl relative overflow-hidden group">
                  <h3 className="text-zinc-100 font-black text-xs uppercase tracking-[0.2em] mb-6 flex items-center gap-2">
                    {'Pesos e Semântica'}
                    <span className="text-[10px] bg-sky-500/10 text-sky-400 px-2 py-0.5 rounded-full border border-sky-500/20 font-black tracking-normal">
                      Fine Tuning
                    </span>
                  </h3>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
                    <WeightInput
                      label={'Peso da Base'}
                      name="base_multiplier"
                      value={weights?.base_multiplier}
                      onChange={handleUpdateWeight}
                      icon={
                        <svg
                          className="w-4 h-4"
                          fill="none"
                          stroke="currentColor"
                          viewBox="0 0 24 24"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth="2.5"
                            d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"
                          />
                        </svg>
                      }
                      color="sky"
                      description={'Influência do motor estatístico (Bleve) no score final.'}
                    />
                    <WeightInput
                      label={'Título (Bônus %)'}
                      name="boost_title_exact"
                      value={weights?.boost_title_exact}
                      onChange={handleUpdateWeight}
                      icon={ICONS.exact}
                      color="amber"
                      description={
                        'Multiplicador aplicado se o termo for idêntico ao título da seção.'
                      }
                    />
                    <WeightInput
                      label={'Título Parcial (%)'}
                      name="boost_title_partial"
                      value={weights?.boost_title_partial}
                      onChange={handleUpdateWeight}
                      icon={ICONS.partial}
                      color="orange"
                      description={'Multiplicador aplicado se o termo estiver contido no título.'}
                    />
                    <WeightInput
                      label={'Frase Exata (%)'}
                      name="boost_phrase"
                      value={weights?.boost_phrase}
                      onChange={handleUpdateWeight}
                      icon={ICONS.phrase}
                      color="indigo"
                      description={'Multiplicador massivo para encontrar a frase exata pesquisada.'}
                    />
                    <WeightInput
                      label={'Nome Arquivo (%)'}
                      name="boost_path_context"
                      value={weights?.boost_path_context}
                      onChange={handleUpdateWeight}
                      icon={ICONS.path}
                      color="cyan"
                      description={'Prioridade extra se o termo estiver no nome do arquivo.'}
                    />
                    <WeightInput
                      label={'Recência (Bônus %)'}
                      name="boost_freshness_max"
                      value={weights?.boost_freshness_max}
                      onChange={handleUpdateWeight}
                      icon={ICONS.clock}
                      color="emerald"
                      description={
                        'Multiplicador máximo para notas criadas ou editadas recentemente.'
                      }
                    />
                    <WeightInput
                      label={'Riqueza Técnica'}
                      name="boost_technical"
                      value={weights?.boost_technical}
                      onChange={handleUpdateWeight}
                      icon={
                        <svg
                          className="w-4 h-4"
                          fill="none"
                          stroke="currentColor"
                          viewBox="0 0 24 24"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth="2.5"
                            d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4"
                          />
                        </svg>
                      }
                      color="fuchsia"
                      description={
                        'Bônus cumulativo por categoria (tabelas, código, links internos e externos).'
                      }
                    />
                    <WeightInput
                      label={'Autoridade de Links'}
                      name="boost_link_authority"
                      value={weights?.boost_link_authority}
                      onChange={handleUpdateWeight}
                      icon={
                        <svg
                          className="w-4 h-4"
                          fill="none"
                          stroke="currentColor"
                          viewBox="0 0 24 24"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth="2.5"
                            d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"
                          />
                        </svg>
                      }
                      color="violet"
                      description={
                        'Bônus logarítmico para notas que são muito citadas por outras notas.'
                      }
                    />
                  </div>
                </div>
              </div>
            ) : activeTab === 'apis' ? (
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                <div className="p-6 bg-zinc-900/60 border border-zinc-800 rounded-3xl relative overflow-hidden group">
                  <div className="absolute top-0 right-0 p-4 opacity-10 group-hover:opacity-20 transition-opacity">
                    <svg
                      className="w-12 h-12 text-blue-500"
                      fill="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z" />
                    </svg>
                  </div>

                  <h3 className="text-zinc-100 font-black text-xs uppercase tracking-[0.2em] mb-2 flex items-center gap-2">
                    {'Google Cloud Vision'}
                    <span className="text-[10px] bg-sky-500/20 text-sky-400 px-2 py-0.5 rounded-full border border-sky-500/20 font-black tracking-normal">
                      OCR
                    </span>
                  </h3>
                  <p className="text-zinc-500 text-sm leading-relaxed mb-6">
                    {
                      'Extrai texto de imagens e fotos automaticamente usando IA avançada. Sem chave, o processamento de texto é ignorado.'
                    }
                  </p>

                  <div className="flex flex-col gap-2">
                    <label className="text-[10px] font-black text-zinc-400 uppercase tracking-widest">
                      {'API Key'}
                    </label>
                    <input
                      type="password"
                      value={settings.google_vision_key}
                      onInput={(e: any) =>
                        setSettings({ ...settings, google_vision_key: e.target.value })
                      }
                      placeholder="AIzaSy..."
                      className="w-full bg-zinc-950 border border-zinc-800 text-zinc-100 px-4 py-3 rounded-xl focus:ring-1 focus:ring-sky-500 outline-none font-mono text-sm shadow-inner transition-all hover:border-zinc-700"
                    />
                    <p className="text-[9px] text-zinc-600 mt-1 italic">
                      {"Dica: Ative a 'Cloud Vision API' no console do Google Cloud."}
                    </p>
                  </div>
                </div>

                <div className="p-10 bg-zinc-900/30 border border-dashed border-zinc-800 rounded-3xl flex flex-col items-center justify-center gap-4 group hover:bg-zinc-900/40 transition-all">
                  <div className="w-12 h-12 rounded-full bg-zinc-800 flex items-center justify-center text-zinc-600 group-hover:scale-110 transition-transform">
                    <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth="2"
                        d="M12 6v6m0 0v6m0-6h6m-6 0H6"
                      />
                    </svg>
                  </div>
                  <p className="text-zinc-600 text-[10px] font-black uppercase tracking-widest italic">
                    {'Novas Integrações em Breve'}
                  </p>
                </div>
              </div>
            ) : (
              <div className="grid grid-cols-1 xl:grid-cols-12 gap-8 overflow-hidden">
                {/* Coluna de Configuração (Esquerda) */}
                <div className="xl:col-span-5 flex flex-col gap-4">
                  <div className="p-5 bg-zinc-900/60 border border-zinc-800 rounded-[24px]">
                    <div className="flex items-center justify-between mb-4">
                      <label className="text-[10px] font-black text-zinc-400 uppercase tracking-widest block">
                        {'Corte de Inatividade'}
                      </label>
                      <label className="relative inline-flex items-center cursor-pointer">
                        <input
                          type="checkbox"
                          className="sr-only peer"
                          checked={cleanupSettings.inactivity}
                          onChange={(e: any) =>
                            setCleanupSettings({ ...cleanupSettings, inactivity: e.target.checked })
                          }
                        />
                        <div className="w-9 h-5 bg-zinc-800 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-zinc-400 peer-checked:after:bg-white after:border-zinc-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-rose-500/50"></div>
                      </label>
                    </div>
                    <div
                      className={`flex flex-col gap-4 transition-opacity duration-300 ${cleanupSettings.inactivity ? 'opacity-100' : 'opacity-40 pointer-events-none'}`}
                    >
                      <input
                        type="range"
                        min="0"
                        max="20"
                        value={cleanupSettings.years}
                        onInput={(e: any) =>
                          setCleanupSettings({
                            ...cleanupSettings,
                            years: parseInt(e.target.value, 10),
                          })
                        }
                        className="w-full accent-rose-500 h-1.5 bg-zinc-800 rounded-lg appearance-none cursor-pointer"
                      />
                      <div className="flex justify-between items-center">
                        <span className="text-xl font-black text-zinc-100 italic tracking-tighter">
                          {cleanupSettings.years === 0 ? 'Qualquer' : cleanupSettings.years}
                          <span className="text-xs text-zinc-500 uppercase ml-1 not-italic">
                            {cleanupSettings.years === 0
                              ? ''
                              : cleanupSettings.years === 1
                                ? 'ano'
                                : 'anos'}
                          </span>
                        </span>
                        <span className="text-[9px] text-zinc-600 font-bold uppercase tracking-tighter italic">
                          {'Última edição'}
                        </span>
                      </div>

                      <div className="pt-4 border-t border-zinc-800/50 mt-2">
                        <div className="flex justify-between items-center mb-3">
                          <label className="text-[10px] font-black text-zinc-400 uppercase tracking-widest block">
                            {'Tamanho Mínimo'}
                          </label>
                          <span className="text-[9px] text-zinc-600 font-bold uppercase tracking-tighter italic">
                            {'Ignorar menores'}
                          </span>
                        </div>
                        <div className="flex items-center gap-3">
                          <input
                            type="number"
                            min="0"
                            step="0.5"
                            value={cleanupSettings.minSizeMb}
                            onChange={(e: any) =>
                              setCleanupSettings({
                                ...cleanupSettings,
                                minSizeMb: parseFloat(e.target.value) || 0,
                              })
                            }
                            className="flex-1 bg-zinc-950 border border-zinc-800 rounded-xl px-4 py-2.5 text-zinc-200 text-sm font-black focus:outline-none focus:border-rose-500/50 transition-all"
                          />
                          <span className="text-zinc-600 text-[10px] font-black tracking-widest uppercase">
                            MB
                          </span>
                        </div>
                      </div>
                    </div>
                  </div>

                  <div className="p-5 bg-zinc-900/60 border border-zinc-800 rounded-[24px] flex flex-col gap-3">
                    <label className="text-[10px] font-black text-zinc-400 uppercase tracking-widest block mb-1">
                      {'Tipos de Arquivo'}
                    </label>
                    <div className="grid grid-cols-1 gap-2">
                      <label className="flex items-center gap-3 cursor-pointer group p-2 rounded-xl hover:bg-zinc-800/30 transition-all border border-transparent hover:border-zinc-800">
                        <input
                          type="checkbox"
                          checked={cleanupSettings.photos}
                          onChange={(e: any) =>
                            setCleanupSettings({ ...cleanupSettings, photos: e.target.checked })
                          }
                          className="w-4 h-4 rounded border-zinc-700 bg-zinc-800 text-rose-500 focus:ring-rose-500"
                        />
                        <span className="text-xs font-bold text-zinc-400 group-hover:text-zinc-200 transition-colors flex-1">
                          {'Anexos & Fotos (.png, .jpg)'}
                        </span>
                      </label>
                      <label className="flex items-center gap-3 cursor-pointer group p-2 rounded-xl hover:bg-zinc-800/30 transition-all border border-transparent hover:border-zinc-800">
                        <input
                          type="checkbox"
                          checked={cleanupSettings.pdfs}
                          onChange={(e: any) =>
                            setCleanupSettings({ ...cleanupSettings, pdfs: e.target.checked })
                          }
                          className="w-4 h-4 rounded border-zinc-700 bg-zinc-800 text-rose-500 focus:ring-rose-500"
                        />
                        <span className="text-xs font-bold text-zinc-400 group-hover:text-zinc-200 transition-colors flex-1">
                          {'Relatórios & PDFs (.pdf)'}
                        </span>
                      </label>
                      <label className="flex items-center gap-3 cursor-pointer group p-2 rounded-xl hover:bg-zinc-800/30 transition-all border border-transparent hover:border-zinc-800">
                        <input
                          type="checkbox"
                          checked={cleanupSettings.notes}
                          onChange={(e: any) =>
                            setCleanupSettings({ ...cleanupSettings, notes: e.target.checked })
                          }
                          className="w-4 h-4 rounded border-zinc-700 bg-zinc-800 text-rose-500 focus:ring-rose-500"
                        />
                        <span className="text-xs font-bold text-zinc-400 group-hover:text-zinc-200 transition-colors flex-1">
                          {'Notas Markdown (.md)'}
                        </span>
                      </label>
                    </div>

                    {cleanupSettings.notes && (
                      <div className="pl-4 ml-4 flex flex-col gap-2 border-l-2 border-zinc-800 animate-in fade-in duration-200">
                        <label className="flex items-center gap-2 cursor-pointer group">
                          <input
                            type="checkbox"
                            checked={cleanupSettings.zombies}
                            onChange={(e: any) =>
                              setCleanupSettings({ ...cleanupSettings, zombies: e.target.checked })
                            }
                            className="w-3 h-3 rounded border-zinc-700 bg-zinc-800 text-amber-500 focus:ring-amber-500"
                          />
                          <span className="text-[10px] font-bold text-zinc-500 group-hover:text-zinc-300 transition-colors tracking-tighter">
                            {'Notas Zumbis (Vazias)'}
                          </span>
                        </label>
                        <label className="flex items-center gap-2 cursor-pointer group">
                          <input
                            type="checkbox"
                            checked={cleanupSettings.abandoned}
                            onChange={(e: any) =>
                              setCleanupSettings({
                                ...cleanupSettings,
                                abandoned: e.target.checked,
                              })
                            }
                            className="w-3 h-3 rounded border-zinc-700 bg-zinc-800 text-amber-500 focus:ring-amber-500"
                          />
                          <span className="text-[10px] font-bold text-zinc-500 group-hover:text-zinc-300 transition-colors tracking-tighter">
                            {'Tarefas Abandonadas'}
                          </span>
                        </label>
                        <label className="flex items-center gap-2 cursor-pointer group">
                          <input
                            type="checkbox"
                            checked={cleanupSettings.captures}
                            onChange={(e: any) =>
                              setCleanupSettings({ ...cleanupSettings, captures: e.target.checked })
                            }
                            className="w-3 h-3 rounded border-zinc-700 bg-zinc-800 text-amber-500 focus:ring-amber-500"
                          />
                          <span className="text-[10px] font-bold text-zinc-500 group-hover:text-zinc-300 transition-colors tracking-tighter">
                            {'Capturas (Artigos + YouTube)'}
                          </span>
                        </label>
                      </div>
                    )}
                  </div>

                  {/* Exclusão por Hashtag - Somente Visível se Notas .md estiverem selecionadas */}
                  {cleanupSettings.notes && (
                    <div className="p-5 bg-rose-500/5 border border-rose-500/10 rounded-[24px] flex flex-col gap-4 animate-in fade-in duration-200">
                      <div className="flex items-center justify-between">
                        <label className="text-[10px] font-black text-rose-400 uppercase tracking-widest block">
                          {'Exclusão por Hashtag'}
                        </label>
                        <div className="flex bg-zinc-950 p-1 rounded-lg border border-zinc-800">
                          <button
                            onClick={() =>
                              setCleanupSettings({ ...cleanupSettings, tagMode: 'any' })
                            }
                            className={`px-3 py-1 rounded-md text-[9px] font-black uppercase tracking-tighter transition-all ${cleanupSettings.tagMode === 'any' ? 'bg-rose-500 text-white shadow-lg shadow-rose-500/20' : 'text-zinc-600 hover:text-zinc-400'}`}
                          >
                            {'Qualquer'}
                          </button>
                          <button
                            onClick={() =>
                              setCleanupSettings({ ...cleanupSettings, tagMode: 'only' })
                            }
                            className={`px-3 py-1 rounded-md text-[9px] font-black uppercase tracking-tighter transition-all ${cleanupSettings.tagMode === 'only' ? 'bg-amber-500 text-zinc-950 shadow-lg shadow-amber-500/20' : 'text-zinc-600 hover:text-zinc-400'}`}
                          >
                            {'Exata'}
                          </button>
                        </div>
                      </div>

                      <div className="relative group">
                        <div className="absolute left-3 top-3.5 text-rose-500/30 group-hover:text-rose-500/50 transition-colors">
                          <svg
                            className="w-4 h-4"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
                            <path
                              strokeLinecap="round"
                              strokeLinejoin="round"
                              strokeWidth="2.5"
                              d="M7 20l4-16m2 16l4-16M6 9h14M4 15h14"
                            />
                          </svg>
                        </div>
                        <input
                          type="text"
                          value={cleanupSettings.targetTags}
                          onChange={(e: any) =>
                            setCleanupSettings({ ...cleanupSettings, targetTags: e.target.value })
                          }
                          placeholder="#temp, #rascunho..."
                          className="w-full bg-zinc-950 border border-zinc-800 text-zinc-100 pl-9 pr-4 py-3 rounded-xl focus:ring-1 focus:ring-rose-500/50 outline-none font-mono text-sm shadow-inner group-hover:border-zinc-700 transition-all placeholder:text-zinc-800"
                        />
                      </div>
                    </div>
                  )}
                </div>

                {/* Coluna de Ações e Resultados (Direita) */}
                <div className="xl:col-span-7 flex flex-col gap-6">
                  <div className="grid grid-cols-2 gap-4">
                    <button
                      onClick={async () => {
                        setIsAnalyzing(true);
                        setStaleInfo(null);
                        try {
                          const qs = new URLSearchParams({
                            days: (cleanupSettings.years * 365).toString(),
                            notes: cleanupSettings.notes.toString(),
                            photos: cleanupSettings.photos.toString(),
                            pdfs: cleanupSettings.pdfs.toString(),
                            zombies: cleanupSettings.zombies.toString(),
                            abandoned: cleanupSettings.abandoned.toString(),
                            captures: cleanupSettings.captures.toString(),
                            inactivity: cleanupSettings.inactivity.toString(),
                            minSizeMb: cleanupSettings.minSizeMb.toString(),
                            targetTags: cleanupSettings.targetTags,
                            tagMode: cleanupSettings.tagMode,
                          }).toString();

                          const res = await fetchWithAuth(`/api/maintenance/stale?${qs}`);
                          if (res?.ok) {
                            const data = await res.json();
                            setStaleInfo(data);
                          }
                        } catch (err) {
                          console.error(err);
                        } finally {
                          setIsAnalyzing(false);
                        }
                      }}
                      disabled={isAnalyzing}
                      className="py-6 rounded-3xl bg-zinc-900 border border-zinc-800 hover:border-zinc-700 hover:bg-zinc-800/80 text-zinc-300 font-black text-[11px] uppercase tracking-[0.2em] transition-all active:scale-[0.98] flex flex-col items-center justify-center gap-3 shadow-xl"
                    >
                      <div
                        className={`w-10 h-10 rounded-2xl bg-zinc-800 flex items-center justify-center text-zinc-400 ${isAnalyzing ? 'animate-pulse' : ''}`}
                      >
                        {isAnalyzing ? (
                          <svg className="w-5 h-5 animate-spin" fill="none" viewBox="0 0 24 24">
                            <circle
                              className="opacity-25"
                              cx="12"
                              cy="12"
                              r="10"
                              stroke="currentColor"
                              strokeWidth="4"
                            ></circle>
                            <path
                              className="opacity-75"
                              fill="currentColor"
                              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                            ></path>
                          </svg>
                        ) : (
                          <svg
                            className="w-5 h-5"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
                            <path
                              strokeLinecap="round"
                              strokeLinejoin="round"
                              strokeWidth="2.5"
                              d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                            />
                          </svg>
                        )}
                      </div>
                      {'Analisar Espaço'}
                    </button>

                    <button
                      onClick={() => setIsCleanupConfirmOpen(true)}
                      disabled={!staleInfo || staleInfo.totalCount === 0}
                      className={`py-6 rounded-3xl font-black text-[11px] uppercase tracking-[0.2em] transition-all active:scale-[0.98] flex flex-col items-center justify-center gap-3 shadow-xl ${!staleInfo || staleInfo.totalCount === 0 ? 'bg-zinc-900/50 text-zinc-700 border border-zinc-800 cursor-not-allowed' : 'bg-rose-500/10 border border-rose-500/50 text-rose-500 hover:bg-rose-500 hover:text-white shadow-rose-500/20'}`}
                    >
                      <div
                        className={`w-10 h-10 rounded-2xl flex items-center justify-center ${!staleInfo || staleInfo.totalCount === 0 ? 'bg-zinc-800' : 'bg-rose-500/20'}`}
                      >
                        <svg
                          className="w-5 h-5"
                          fill="none"
                          stroke="currentColor"
                          viewBox="0 0 24 24"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth="2.5"
                            d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                          />
                        </svg>
                      </div>
                      {'Executar Limpeza'}
                    </button>
                  </div>

                  <div className="flex-1 bg-zinc-900/60 border border-zinc-800 rounded-[32px] p-8 flex flex-col min-h-[400px]">
                    <div className="flex items-center justify-between mb-8">
                      <div>
                        <h4 className="text-zinc-100 font-black text-xs uppercase tracking-[0.3em] mb-1">
                          {'Resultado da Análise'}
                        </h4>
                        <p className="text-zinc-600 text-[10px] font-bold uppercase tracking-widest">
                          {'Detalhamento por arquivo'}
                        </p>
                      </div>
                      {staleInfo && (
                        <div className="px-4 py-2 bg-rose-500/10 rounded-2xl border border-rose-500/20 text-rose-400 text-[11px] font-black tracking-tighter">
                          {staleInfo.totalCount} arquivos
                        </div>
                      )}
                    </div>

                    {!staleInfo ? (
                      <div className="flex-1 flex flex-col items-center justify-center text-center p-10 opacity-30">
                        <svg
                          className="w-16 h-16 text-zinc-700 mb-6"
                          fill="none"
                          stroke="currentColor"
                          viewBox="0 0 24 24"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth="1"
                            d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"
                          />
                        </svg>
                        <p className="text-xs font-bold text-zinc-500 uppercase tracking-widest leading-loose">
                          {
                            "Clique em 'Analisar Espaço' para iniciar a varredura inteligente do PKM."
                          }
                        </p>
                      </div>
                    ) : staleInfo.totalCount === 0 ? (
                      <div className="flex-1 flex flex-col items-center justify-center text-center p-10">
                        <div className="w-16 h-16 rounded-full bg-emerald-500/10 flex items-center justify-center mb-6">
                          <svg
                            className="w-8 h-8 text-emerald-500"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
                            <path
                              strokeLinecap="round"
                              strokeLinejoin="round"
                              strokeWidth="2.5"
                              d="M5 13l4 4L19 7"
                            />
                          </svg>
                        </div>
                        <p className="text-sm font-black text-zinc-300 uppercase tracking-tighter mb-2">
                          Ambiente Limpo!
                        </p>
                        <p className="text-[10px] font-bold text-zinc-500 uppercase tracking-widest">
                          Nenhum arquivo encontrado com os critérios atuais.
                        </p>
                      </div>
                    ) : (
                      <div className="flex flex-col flex-1">
                        <div className="flex-1 overflow-y-auto custom-scrollbar pr-2 -mx-2">
                          <table className="w-full text-left text-[11px] border-separate border-spacing-y-2">
                            <thead className="sticky top-0 bg-transparent text-zinc-500 uppercase tracking-widest font-black text-[9px] z-10 backdrop-blur-sm">
                              <tr>
                                <th className="pb-4 pl-4">Caminho</th>
                                <th className="pb-4">Motivo Principal</th>
                                <th className="pb-4 text-right pr-4">Tamanho</th>
                              </tr>
                            </thead>
                            <tbody>
                              {staleInfo.files.slice(0, 100).map((f: any, i: number) => (
                                <tr key={i} className="group">
                                  <td className="py-3 pl-4 bg-zinc-950/40 rounded-l-xl border-l border-y border-zinc-800/50 group-hover:border-zinc-700 group-hover:bg-zinc-800/20 transition-all">
                                    <div className="flex flex-col">
                                      <span
                                        className="text-zinc-300 font-bold max-w-[200px] truncate"
                                        title={f.name}
                                      >
                                        {f.name.split('/').pop()}
                                      </span>
                                      <span className="text-[9px] text-zinc-600 truncate max-w-[180px] font-mono italic">
                                        {f.name.substring(0, f.name.lastIndexOf('/'))}
                                      </span>
                                    </div>
                                  </td>
                                  <td className="py-3 bg-zinc-950/40 border-y border-zinc-800/50 group-hover:border-zinc-700 group-hover:bg-zinc-800/20 transition-all">
                                    <div className="flex items-center gap-2">
                                      <span className="px-2 py-0.5 rounded-lg bg-rose-500/10 text-rose-400 text-[9px] font-black uppercase tracking-tighter transition-all border border-rose-500/10">
                                        {f.reason || 'Inatividade'}
                                      </span>
                                      <span className="text-zinc-600 font-bold italic">
                                        ({f.ageDays} dias)
                                      </span>
                                    </div>
                                  </td>
                                  <td className="py-3 pr-4 text-right bg-zinc-950/40 rounded-r-xl border-r border-y border-zinc-800/50 group-hover:border-zinc-700 group-hover:bg-zinc-800/20 transition-all">
                                    <span className="text-zinc-400 font-black">
                                      {(f.size / 1024).toFixed(1)}{' '}
                                      <span className="text-[8px] text-zinc-600">KB</span>
                                    </span>
                                  </td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        </div>

                        <div className="mt-8 pt-6 border-t border-zinc-800 flex justify-between items-end">
                          <div className="flex flex-col gap-1">
                            <span className="text-[10px] text-zinc-600 font-black tracking-widest uppercase">
                              {'Otimização Possível'}
                            </span>
                            <div className="flex items-baseline gap-2">
                              <span className="text-3xl font-black text-rose-500 tracking-tighter">
                                {(staleInfo.totalSize / 1024 / 1024).toFixed(2)}
                              </span>
                              <span className="text-xs font-black text-rose-500 opacity-50 uppercase">
                                {'MB de Espaço'}
                              </span>
                            </div>
                          </div>
                          <p className="text-[9px] text-zinc-600 italic">
                            {'Varredura completa em todos os setores habilitados.'}
                          </p>
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              </div>
            )}
          </div>
        </main>
      </div>

      {isResetConfirmOpen && (
        <div className="fixed inset-0 z-[160] flex items-center justify-center p-4">
          <div
            className="absolute inset-0 bg-zinc-950/60 backdrop-blur-md animate-in fade-in duration-200"
            onClick={() => setIsResetConfirmOpen(false)}
          />
          <div className="bg-zinc-900 border border-zinc-800 p-8 rounded-[32px] shadow-2xl relative z-10 w-full max-sm animate-in fade-in duration-200">
            <div className="w-16 h-16 bg-amber-500/10 rounded-2xl flex items-center justify-center border border-amber-500/20 mb-6 mx-auto">
              <svg
                className="w-8 h-8 text-amber-500"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2"
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
            </div>
            <h4 className="text-xl font-black text-white tracking-tighter mb-2 text-center">
              Restaurar Padrões?
            </h4>
            <p className="text-zinc-400 text-sm leading-relaxed mb-8 text-center px-2">
              Esta ação irá voltar todos os pesos para os valores de fábrica. Não pode ser desfeito.
            </p>
            <div className="flex gap-3">
              <button
                onClick={() => setIsResetConfirmOpen(false)}
                className="flex-1 px-6 py-3.5 rounded-2xl bg-zinc-800 text-zinc-300 text-[10px] font-black tracking-widest uppercase hover:bg-zinc-700 transition-all"
              >
                Cancelar
              </button>
              <button
                onClick={handleReset}
                className="flex-1 px-6 py-3.5 rounded-2xl bg-amber-500 text-zinc-950 text-[10px] font-black tracking-widest uppercase hover:bg-amber-400 transition-all shadow-lg shadow-amber-500/20"
              >
                Resetar
              </button>
            </div>
          </div>
        </div>
      )}

      {isCleanupConfirmOpen && (
        <div className="fixed inset-0 z-[200] flex items-center justify-center p-4">
          <div
            className="absolute inset-0 bg-black/80 backdrop-blur-md animate-in fade-in duration-300"
            onClick={() => !isCleaning && setIsCleanupConfirmOpen(false)}
          />

          <div className="bg-zinc-900 border border-rose-500/20 rounded-[32px] shadow-[0_0_80px_-20px_rgba(244,63,94,0.3)] relative z-10 w-full max-w-sm p-8 flex flex-col items-center text-center animate-in fade-in duration-200 overflow-hidden">
            <div className="absolute top-0 inset-x-0 h-px bg-gradient-to-r from-transparent via-rose-500/50 to-transparent"></div>

            <div className="relative group mb-8 mt-2">
              <div className="absolute inset-0 bg-rose-500/20 rounded-full blur-2xl group-hover:bg-rose-500/40 transition-all duration-700"></div>
              <div className="w-20 h-20 rounded-3xl bg-zinc-950 border border-rose-500/30 flex items-center justify-center relative rotate-3 group-hover:rotate-12 group-hover:scale-110 transition-all duration-500 shadow-2xl">
                <svg
                  className="w-10 h-10 text-rose-500"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth="2"
                    d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                  />
                </svg>
              </div>
            </div>

            <h4 className="text-2xl font-black text-transparent bg-clip-text bg-gradient-to-br from-rose-400 to-rose-600 tracking-tighter mb-3">
              Vaporizar Arquivos
            </h4>
            <p className="text-zinc-400 text-sm leading-relaxed mb-8">
              Você está prestes a excluir definitivamente{' '}
              <strong className="text-white px-1.5 py-0.5 rounded bg-zinc-800">
                {staleInfo?.totalCount} arquivo(s)
              </strong>
              . <br />
              <span className="text-[10px] text-zinc-500 uppercase tracking-widest mt-2 block font-bold">
                Ação Irreversível
              </span>
            </p>

            <div className="flex flex-col w-full gap-3 relative z-20">
              <button
                disabled={isCleaning || isSuccess}
                onClick={async () => {
                  setIsCleaning(true);
                  try {
                    const payload: any = {
                      ...cleanupSettings,
                      days: cleanupSettings.years * 365,
                      inactivity: cleanupSettings.inactivity,
                      captures: cleanupSettings.captures,
                    };
                    delete payload.years;

                    const res = await fetchWithAuth('/api/maintenance/cleanup', {
                      method: 'POST',
                      headers: { 'Content-Type': 'application/json' },
                      body: JSON.stringify(payload),
                    });
                    if (res?.ok) {
                      setIsSuccess(true);
                      setTimeout(() => {
                        setStaleInfo(null);
                        setIsCleanupConfirmOpen(false);
                        setIsSuccess(false);
                        if (onUpdate) onUpdate();
                      }, 1500);
                    } else {
                      console.error('Erro ao limpar arquivos.');
                    }
                  } catch (err) {
                    console.error(err);
                  } finally {
                    setIsCleaning(false);
                  }
                }}
                className={`w-full relative group overflow-hidden py-4 rounded-2xl text-white text-[10px] font-black tracking-widest uppercase active:scale-[0.98] transition-all disabled:opacity-100 disabled:cursor-wait ${
                  isSuccess
                    ? 'bg-emerald-500 shadow-[0_0_30px_-5px_rgba(16,185,129,0.6)]'
                    : 'bg-rose-500 hover:bg-rose-600 shadow-[0_0_20px_-5px_rgba(244,63,94,0.5)]'
                }`}
              >
                {!isSuccess && (
                  <div className="absolute inset-0 w-full h-full bg-gradient-to-r from-transparent via-white/20 to-transparent -translate-x-full group-hover:animate-[shimmer_1.5s_infinite]"></div>
                )}
                <span className="relative z-10 flex items-center justify-center gap-2">
                  {isSuccess ? (
                    <>
                      <svg
                        className="w-4 h-4 text-white"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth="3"
                          d="M5 13l4 4L19 7"
                        ></path>
                      </svg>
                      Excluídos com Sucesso!
                    </>
                  ) : isCleaning ? (
                    <>
                      <span className="w-3 h-3 border-2 border-white/30 border-t-white rounded-full animate-spin"></span>
                      Desintegrando...
                    </>
                  ) : (
                    'Sim, Confirmar'
                  )}
                </span>
              </button>
              <button
                onClick={() => !isSuccess && setIsCleanupConfirmOpen(false)}
                disabled={isCleaning || isSuccess}
                className="w-full py-4 rounded-2xl bg-zinc-950 border border-zinc-800 text-zinc-400 text-[10px] font-black tracking-widest uppercase hover:bg-zinc-800 hover:text-white transition-all disabled:opacity-50"
              >
                Cancelar
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default WeightsSettings;
