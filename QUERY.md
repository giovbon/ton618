# 🌌 Guia de Consultas TON-618

O TON-618 oferece dois níveis de busca: a **Busca Global** (rápida e semântica) e a **Super Busca (Dataview)** (estruturada e técnica). 

---

## 🚀 1. Super Busca (Modo Dataview) DQL Light
Ativada quando a query começa com `TABLE` ou `LIST`. Ela transforma os resultados em tabelas dinâmicas baseadas no Frontmatter das suas notas.

### Sintaxe Geral
`COMANDO [campos] [FROM fonte] [WHERE condições] [SORT campo ORDEM]`

### Comandos
- **`TABLE`**: Exibe os resultados em uma tabela.
- **`LIST`**: Exibe uma lista simplificada apenas com os nomes dos arquivos.

### Campos (Apenas para TABLE)
Você pode pedir qualquer campo que exista no Frontmatter da sua nota.
- Ex: `TABLE status, prioridade, autor`

#### Metadados Automáticos (file.*)
Campos que o sistema gera sozinho:
- `file.name`: Nome do arquivo.
- `file.mtime`: Data da última modificação.
- `file.size`: Tamanho em KB.
- `file.path`: Caminho completo dentro do cofre.

### Funções de Agregação
- **`count()`**: Conta o número de itens encontrados.
- **Exemplo**: `TABLE count(file.name) FROM #projeto` (Exibe uma linha de resumo com o total).

### Fonte (FROM) - *Opcional*
- **Pasta**: `FROM notes/projetos`
- **Tag**: `FROM #trabalho`
- **Todos**: Se omitir o `FROM`, ele busca em todas as notas `.md`.

### Filtros (WHERE)
Suporta operadores lógicos e condições múltiplas com `AND`.
- **Operadores**: `==`, `=`, `!=`, `>`, `<`, `>=`, `<=`
- **Exemplo**: `WHERE status == "fazendo" AND prioridade == "alta"`

### Ordenação (SORT)
- **ASC**: Crescente.
- **DESC**: Decrescente.
- **Exemplo**: `SORT file.mtime DESC`

---

## 🔍 2. Busca Global (Vortex Search)
Busca rápida por texto ou significado.

| Tipo de Busca | Sintaxe | O que faz |
| :--- | :--- | :--- |
| **Simples** | `termo` | Busca inteligente por aproximação no texto e título. |
| **Exata** | `"frase exata"` | Busca o termo exatamente como escrito. |
| **Obrigatória** | `+importante` | A nota **tem que ter** esse termo. |
| **Exclusão** | `-rascunho` | **Esconde** notas que tenham esse termo. |
| **Tag** | `#idea` | Filtra rapidamente por uma hashtag. |
| **Semântica** | (Ícone do Cérebro) | Busca notas que tenham o **mesmo sentido**, mesmo com palavras diferentes. |

---

## 💡 Exemplos Práticos

1. **Painel de Projetos Ativos:**
   `TABLE prioridade, status FROM #projeto WHERE status != "concluído" SORT prioridade ASC`

2. **O que eu editei hoje?**
   `TABLE file.mtime SORT file.mtime DESC`

3. **Lista simples de documentos em uma pasta:**
---

## 🛠️ Próximos Passos (Roadmap DQL Light)
Recursos planejados para futuras atualizações:
- [ ] **TASK**: Filtro de tarefas pendentes (`TASK WHERE !completed`).
- [ ] **GROUP BY**: Agrupamento visual por campos (ex: por status).
- [ ] **Visual Progress**: Renderização de barras de progresso para campos numéricos.
- [ ] **Pills Coloridas**: Realce visual de status e tags nas tabelas.



TABLE status WHERE status == "fazendo"
TABLE file.mtime, status SORT file.mtime DESC
TABLE file.name, file.size SORT file.size DESC
TABLE count(file.name) FROM #projeto