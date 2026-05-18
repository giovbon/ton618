import type { FullConfig } from '@playwright/test';

async function globalSetup(_config: FullConfig) {
  // Verificar que o servidor está respondendo antes de começar os testes
  const credentials = btoa('admin:ton618_secret');

  try {
    const response = await fetch('http://localhost:6180/api/status', {
      headers: {
        Authorization: `Basic ${credentials}`,
      },
    });
    if (!response.ok) {
      console.error(`Backend respondeu com ${response.status}. Execute o servidor Go primeiro.`);
      process.exit(1);
    }
    console.log('✅ Backend OK');
  } catch (_e) {
    console.error('Backend não está rodando em :6180. Execute o servidor Go primeiro.');
    process.exit(1);
  }
}

export default globalSetup;
