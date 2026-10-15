# TON-618 Mobile (Android APK)

Esta pasta contém o aplicativo Android nativo do **TON-618**.

Ele empacota o backend Go compilado nativamente para processadores **ARM64** e o executa como um serviço em segundo plano (`ForegroundService`) no Android, servindo a interface via **WebView** no próprio celular e simultaneamente disponibilizando o acesso na rede local Wi-Fi.

---

## 🏗️ Arquitetura

* **Backend:** binário `libton618.so` (Go + SQLite FTS5, sem CGO) publicado em `app/src/main/jniLibs/arm64-v8a/`.
  Fica no `nativeLibraryDir` porque o Android 10+ bloqueia execução de arquivos em `filesDir`.
* **Frontend:** bundle web (`web/static/**`) embutido em `app/src/main/assets/web/static/` e extraído
  para o armazenamento interno no primeiro boot (subpastas incluídas — ver "Solução de problemas").
* **Serviço de Segundo Plano (`TonServerService.kt`):**
  * Localiza o binário, extrai os assets web e inicia o processo Go ouvindo em `0.0.0.0:6180`.
  * Cria uma notificação persistente para que o sistema operacional não suspenda o servidor ao desligar a tela.
* **Painel de Controle (`MainActivity.kt`):**
  * Campo de **senha de acesso** (persistido em `SharedPreferences` privado do app) — usada no app
    e também no acesso pelo PC via Wi-Fi.
  * Indicador visual de conexão (Verde = Ativo, Âmbar = Iniciando, Vermelho = Erro).
  * Endereço IP local (ex: `http://192.168.1.15:6180`) + botão **Copiar Link** (para abrir no PC).
  * Botões de iniciar/parar/reiniciar e painel de **logs ao vivo** do processo Go.
* **Interface Web (`WebActivity.kt`):** `WebView` acelerada por hardware carregando
  `http://127.0.0.1:6180` (latência zero). Responde ao BasicAuth do core automaticamente.

---

## 🚀 Como Compilar o Projeto

### Passo 1: Gerar o binário do Core + os assets web

Execute o script de compilação a partir da pasta `android`:

```bash
cd android
chmod +x build-core-arm64.sh
./build-core-arm64.sh
```

O script:
1. gera os templates `templ` (versão fixada em `core/go.mod`);
2. compila o `core` para Android com o **Android NDK** (`CGO_ENABLED=1`) e `-buildmode=pie`,
   gravando `app/src/main/jniLibs/arm64-v8a/libton618.so`;
3. copia `core/web/static/**` para `app/src/main/assets/web/static/` **recursivamente**, sem
   `.gz`/`.br` e sem o `models/ort/`;
4. falha se algum asset essencial (`js/app.js`, `js/vendor/htmx.min.js`, `js/vendor/alpine.min.js`,
   `fonts/fonts.css`) estiver faltando — é exatamente isso que faz a interface abrir "morta".

> ⚠️ **O NDK é obrigatório mesmo não havendo código cgo.** O SQLite é Go puro
> (`modernc.org/sqlite`), mas o Android resolve DNS via `netd` (socket Unix), não por
> `/etc/resolv.conf` — por isso o Go só tem resolver funcional no Android com cgo
> ([golang/go#10714](https://github.com/golang/go/issues/10714)). Com `CGO_ENABLED=0` o binário
> falha com `lookup huggingface.co on [::1]:53: connection refused`, e junto vão embora o
> download do modelo de embeddings, as legendas do YouTube, o clip de artigos e o ntfy.
> O script procura o NDK (`ANDROID_NDK_HOME`, `ANDROID_SDK_ROOT/ndk/*`, …) e **valida** o
> binário pelo buildinfo, abortando com mensagem clara se não encontrar.

Variáveis opcionais:

```bash
ANDROID_ABI=x86_64 ./build-core-arm64.sh     # emulador
ANDROID_SEMANTIC=1 ./build-core-arm64.sh     # inclui onnxruntime-web (+22 MB)
```

> ⚠️ Os assets vivem fora do Git (`.gitignore` da pasta). Sempre rode este passo antes de gerar o APK.

### Passo 2: Gerar o APK

Você pode abrir esta pasta `android/` diretamente no **Android Studio**, ou compilar via linha de comando:

```bash
# Gerar APK de desenvolvimento (Debug)
./gradlew assembleDebug
```

O arquivo `.apk` gerado estará em:
```
android/app/build/outputs/apk/debug/app-debug.apk
```

### Passo 3: Instalar no Smartphone

Conecte seu celular com a depuração USB ativada ou transfira o APK:

```bash
adb install -r app/build/outputs/apk/debug/app-debug.apk
```

---

## 🔒 Dados e Armazenamento

* O banco de dados (`ton618.db`) e os arquivos das notas (`docs/`) são mantidos no armazenamento interno e seguro do aplicativo (`/data/user/0/org.ton618.mobile/files/`).
* Não há envio de dados para servidores externos ou nuvem.
* O modelo de embeddings (~120 MB) **não** é baixado no aparelho: o app sobe o core com
  `MODEL_DOWNLOAD=false` e o WebView busca o modelo no CDN do HuggingFace — o mesmo caminho que o
  desktop usa enquanto o download não termina. Sem isso, +120 MB iriam para o armazenamento do app.

---

## 🔑 Senha de acesso

O TON-618 é protegido por **senha** (o usuário é fixo e ignorado pelo core — só a senha importa,
como na tela `/login`). A senha sai de fábrica como `ton618`.

Trocar no app:

1. abra o TON-618 e edite o campo **Senha do TON-618**;
2. toque em **Salvar senha** — o app grava em `SharedPreferences` e **reinicia o servidor**;
3. o painel de logs confirma: `[App] Senha de acesso: personalizada`.

A mesma senha protege:

* a interface dentro do app (`WebView` em `127.0.0.1`);
* o acesso de outros dispositivos na rede Wi-Fi (`http://<ip-do-celular>:6180`), que cai na tela
  `/login`.

> ⚠️ A senha fica em texto claro no `SharedPreferences` privado do app (mesmo nível de proteção
> dos dados das notas). Não use uma senha que você reutiliza em outros serviços.

---
## 💾 Backup e restauração pelo app

A aba **Configurações → Backup** tem três opções; apenas o **Backup Total** pode ser importado.

- **Baixar** (Rápido, Completo ou Total): o `WebActivity` tem um `DownloadListener` que entrega o
  arquivo ao `DownloadManager`, repassando o cookie de autenticação (`ton_auth`) — sem ele o
  servidor responderia 302 para `/login` e o "backup" seria a página de login. O ZIP vai para a
  pasta **Downloads** do aparelho (em Android ≤ 28 é pedida a permissão de armazenamento).
- **Restaurar**: o seletor de arquivo é atendido por `onShowFileChooser` (sem ele o
  `<input type="file">` não abre nada). A restauração **substitui tudo**, então o app responde com
  um aviso de confirmação antes de enviar.
- Ao terminar, o core encerra de propósito (o banco aberto é o arquivo antigo) e o
  `TonServerService` **sobe o servidor de novo automaticamente** — o mesmo mecanismo cobre quedas
  inesperadas do processo (até 3 tentativas seguidas).
- `android:allowBackup="false"`: o Auto Backup do Android não copia mais o banco para o Google
  Drive; o caminho oficial de cópia é o Backup Total.

---
## 📦 Tamanho do APK

O que pesava no APK e o que foi feito:

| Item | Antes | Agora |
|------|-------|-------|
| Binário Go | 2 cópias (jniLibs **e** `assets/ton618-arm64`) | 1 cópia (só `jniLibs`) |
| `*.gz` / `*.br` do bundle web | duplicavam o conteúdo (o core já comprime em runtime) | fora |
| `models/ort/` (onnxruntime-web) | 22 MB | fora por padrão (`ANDROID_SEMANTIC=1` para incluir) |

> A busca semântica no celular exige baixar um modelo de ~120 MB e rodar inferência ONNX em CPU;
> por isso o `onnxruntime-web` fica de fora por padrão. A busca textual (FTS5) continua completa.

---

## 🩺 Solução de Problemas

**A página abre, mas nenhum botão/recurso funciona (não cria nem pesquisa notas)**

Quase sempre significa que os assets web não chegaram ao aparelho — o HTML é renderizado pelo
servidor Go, mas `/static/js/vendor/htmx.min.js`, `js/vendor/alpine.min.js` e `js/app.js` retornam
404 e o frontend inteiro fica inerte.

1. Abra o app e olhe o painel **Logs e Diagnóstico**: deve aparecer
   `[App] Assets web extraídos: N arquivo(s).` (na primeira execução de cada versão do APK).
   Se aparecer `❌ Assets essenciais ausentes no APK`, rode `./build-core-arm64.sh` e gere o APK de novo.
2. Teste no navegador do celular: `http://127.0.0.1:6180/static/js/app.js`.
   Um JSON/JS de verdade = assets ok; `404` = rode o build novamente.
3. Os assets só são re-extraídos quando a versão do APK muda. Ao trocar apenas arquivos web sem
   reinstalar, reexporte o APK (o stamp usa `versionCode` + `lastUpdateTime`) ou desinstale/instale.

**`❌ Timeout: a porta 6180 não respondeu`** — veja as linhas `[Go]` / `[Go Err]` nos logs; a causa
mais comum é o banco não conseguir abrir em `filesDir/data/ton618.db`.

**Na busca semântica aparece `Servidor 0% · erro: lookup huggingface.co on [::1]:53: ... connection refused`**

O processo Go não consegue resolver DNS. Esse formato de erro (`on [::1]:53`) é exclusivo do
resolver **Go puro**, ou seja: o binário foi compilado **sem cgo**. Rode `./build-core-arm64.sh`
(ele exige o NDK e valida o binário no final) e gere o APK de novo. A busca textual e a interface
não são afetadas — o navegador baixa o modelo do CDN do HuggingFace como fallback (o painel
"Servidor X%" é só informativo).
