import { useEffect, useRef } from 'preact/compat';

/**
 * useSSE - Conecta ao endpoint de Server-Sent Events do backend
 * e despacha handlers para eventos específicos.
 */
export function useSSE(
  authHeader: string | null,
  handlers: Record<string, (data: any) => void> = {},
) {
  const eventSourceRef = useRef<EventSource | null>(null);
  const handlersRef = useRef(handlers);

  // Sempre atualiza o ref com os handlers mais recentes sem disparar re-render
  useEffect(() => {
    handlersRef.current = handlers;
  }, [handlers]);

  useEffect(() => {
    if (!authHeader) return;

    let reconnectTimeout: number;

    const connect = () => {
      console.log('[SSE] Conectando...');
      const es = new EventSource(`/api/events?token=${encodeURIComponent(authHeader)}`);

      es.onmessage = (e) => {
        try {
          const data = JSON.parse(e.data);
          console.log('[SSE] Mensagem genérica:', data);
        } catch (_err) {
          console.log('[SSE] Mensagem não-JSON:', e.data);
        }
      };

      es.onerror = (err) => {
        console.error('[SSE] Erro na conexão. Tentando reconectar em 5s...', err);
        es.close();
        reconnectTimeout = window.setTimeout(connect, 5000);
      };

      // Registrar handlers dinâmicos usando o ref
      // Dessa forma, se um handler mudar, o EventSource continua o mesmo,
      // mas a função chamada será a mais atual.
      const eventTypes = [
        'sync:started',
        'sync:finished',
        'ocr:started',
        'ocr:finished',
        'file:vectorizing',
        'file:ready',
      ];

      eventTypes.forEach((eventType) => {
        es.addEventListener(eventType, (e: any) => {
          const currentHandler = handlersRef.current[eventType];
          if (currentHandler) {
            try {
              const data = JSON.parse(e.data);
              currentHandler(data);
            } catch (_err) {
              currentHandler(e.data);
            }
          }
        });
      });

      eventSourceRef.current = es;
    };

    connect();

    return () => {
      if (eventSourceRef.current) {
        console.log('[SSE] Fechando conexão.');
        eventSourceRef.current.close();
      }
      if (reconnectTimeout) clearTimeout(reconnectTimeout);
    };
  }, [authHeader]); // RE-CONECTA APENAS SE O AUTH MUDAR
}
