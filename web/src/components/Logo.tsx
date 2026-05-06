import { useMemo } from 'preact/hooks';

interface LogoProps {
  size?: number;
  className?: string;
}

/**
 * Logo TON-618 — Conceito "Event Horizon Ultimate" (V8.1)
 * Refinamento: Removidos os jatos verticais e o brilho em formato de estrela
 * para evitar a "faixa vertical" indesejada. Foco em um brilho radial puro e vórtex intenso.
 */
export const Logo = ({ size = 32, className = '' }: LogoProps) => {
  const filterId = useMemo(() => `horizonGlow-${Math.random().toString(36).substring(2, 9)}`, []);

  return (
    <div
      className={`relative flex items-center justify-center shrink-0 group cursor-pointer transition-all duration-300 hover:scale-125 hover:z-50 ${className}`}
      style={{ width: size, height: size }}
    >
      <svg
        viewBox="0 0 100 100"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
        className="w-full h-full drop-shadow-[0_0_20px_rgba(255,255,255,0.6)] group-hover:animate-chaos transition-all"
      >
        <defs>
          <filter id={filterId} x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur stdDeviation="4" result="blur" />
            <feColorMatrix
              in="blur"
              type="matrix"
              values="1 0 0 0 1  0 1 0 0 1  0 0 1 0 1  0 0 0 22 -8"
              result="glow"
            />
            <feComposite in="SourceGraphic" in2="glow" operator="over" />
          </filter>

          <linearGradient id="vortexGradMain" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stopColor="#ffffff" />
            <stop offset="100%" stopColor="#94a3b8" />
          </linearGradient>

          <radialGradient id="singularityGrad" cx="50%" cy="50%" r="50%">
            <stop offset="65%" stopColor="#000000" />
            <stop offset="100%" stopColor="#0f172a" />
          </radialGradient>
        </defs>

        {/* 1. HALO DE RADIAÇÃO CIRCULAR (Fundo) - Substituído o formato estrela por círculos para evitar faixas */}
        <g className="animate-[pulse_4s_ease-in-out_infinite] opacity-30">
          <circle cx="50" cy="50" r="45" fill="white" filter={`url(#${filterId})`} />
          <circle cx="50" cy="50" r="30" fill="white" opacity="0.5" filter={`url(#${filterId})`} />
        </g>

        {/* 2. DISCO DE ACREÇÃO (Com Beaming Assimétrico) */}
        <g className="animate-[spin_18s_linear_infinite] group-hover:animate-fast-spin origin-center">
          <path
            d="M50 5 C80 5 100 25 100 50 C100 70 85 85 70 90 C85 75 90 55 85 40 C80 30 65 20 50 20 Z"
            fill="url(#vortexGradMain)"
            opacity="1"
          />
          <path
            d="M50 10 C75 10 95 30 95 50 C95 65 85 80 70 85 C80 70 85 55 80 45 C75 35 60 25 50 25 Z"
            fill="#e2e8f0"
            opacity="0.5"
            transform="rotate(90 50 50)"
          />
          <path
            d="M50 8 C75 8 95 25 95 50 C95 65 85 80 70 85 C80 70 80 55 75 45 C70 35 60 25 50 25 Z"
            fill="#ffffff"
            opacity="0.75"
            transform="rotate(180 50 50)"
          />
          <path
            d="M50 12 C70 12 90 32 90 50 C90 65 80 80 65 85 C75 70 75 55 70 45 C65 35 55 25 50 25 Z"
            fill="#cbd5e1"
            opacity="0.3"
            transform="rotate(270 50 50)"
          />
        </g>

        {/* 3. ESFERA DE FÓTONS (Photon Sphere) */}
        <circle
          cx="50"
          cy="50"
          r="26"
          stroke="white"
          strokeWidth="1.5"
          strokeOpacity="0.85"
          filter={`url(#${filterId})`}
          className="animate-pulse"
        />

        {/* 4. SINGULARIDADE (O Vácuo) */}
        <circle cx="50" cy="50" r="24" fill="url(#singularityGrad)" />
        <circle
          cx="50"
          cy="50"
          r="20"
          fill="#000000"
          className="group-hover:scale-110 transition-transform origin-center"
        />

        {/* 5. ENXAME DE PARTÍCULAS (Faíscas e Detritos) */}
        <g className="z-20">
          <circle cx="50" cy="12" r="1.5" fill="white" className="animate-ping" />
          <circle cx="88" cy="50" r="1" fill="white" className="animate-ping duration-1000" />
          <circle cx="50" cy="88" r="1.8" fill="white" className="animate-ping duration-700" />
          <circle cx="12" cy="50" r="1.3" fill="white" className="animate-ping duration-1500" />

          <circle cx="65" cy="35" r="0.8" fill="white" className="animate-pulse" />
          <circle cx="35" cy="65" r="0.8" fill="white" className="animate-pulse delay-500" />
          <circle cx="72" cy="60" r="0.7" fill="#cbd5e1" className="animate-pulse delay-200" />
          <circle cx="28" cy="40" r="0.7" fill="#cbd5e1" className="animate-pulse delay-700" />
        </g>
      </svg>
    </div>
  );
};
