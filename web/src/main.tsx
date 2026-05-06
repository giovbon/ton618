import { render } from 'preact/compat';
import './index.css';
import 'highlight.js/styles/github-dark.css'; // Tema para o realce de sintaxe
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { App } from './App';

console.log('TON-618: Script main.tsx carregado.');

const queryClient = new QueryClient();
const root = document.getElementById('app');

if (!root) {
  throw new Error('TON-618: Elemento raiz #app não encontrado!');
}

try {
  console.log('TON-618: Tentando renderizar App...');
  render(
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>,
    root,
  );
  console.log('TON-618: Renderização concluída.');
} catch (error: any) {
  console.error('TON-618: Falha fatal na renderização:', error);
  root.innerHTML = `
    <div style="padding: 20px; background: #450a0a; color: #fecaca; font-family: sans-serif; border: 1px solid #ef4444; border-radius: 8px; margin: 20px;">
      <h1 style="font-size: 18px; margin-bottom: 10px;">Erro Fatal de Carregamento</h1>
      <p style="font-size: 14px; margin-bottom: 10px;">A aplicação falhou ao iniciar.</p>
      <pre style="background: #000; padding: 10px; border-radius: 4px; overflow: auto; font-size: 12px;">${error.stack || error.message}</pre>
    </div>
  `;
}
