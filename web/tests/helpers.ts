const credentials = btoa('admin:ton618_secret');

export const BASE_URL = 'http://localhost:5173';
export const API_URL = 'http://localhost:6180';

// Caminhos de autenticação para URL (Basic Auth na query para page.goto)
export function authUrl(path: string): string {
  return `http://admin:ton618_secret@localhost:5173${path}`;
}

// Headers de autenticação para fetch()
function authHeaders(): Record<string, string> {
  return {
    Authorization: `Basic ${credentials}`,
    'Content-Type': 'application/json',
  };
}

// Cria uma nota via API para ser usada nos testes
export async function createNoteViaApi(title: string, content: string): Promise<string> {
  const filename = `test-${Date.now()}-${Math.random().toString(36).slice(2)}.md`;
  const fullContent = `# ${title}\n\n${content}`;

  const response = await fetch(`${API_URL}/api/file?name=${filename}`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ content: fullContent }),
  });

  if (!response.ok) throw new Error(`Failed to create note: ${response.status}`);
  return filename;
}

// Deleta uma nota via API
export async function deleteNoteViaApi(filename: string): Promise<void> {
  await fetch(`${API_URL}/api/file?name=${filename}`, {
    method: 'DELETE',
    headers: authHeaders(),
  });
}
