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

> `ANDROID_SEMANTIC=1` embarca os ~22 MB do onnxruntime-web (runtime WebAssembly usado
> pela busca semântica), permitindo carregá-lo do próprio aparelho. Sem a flag o APK fica
> menor e o worker carrega o runtime do CDN do jsdelivr no 1º uso — a busca semântica
> continua funcionando (o modelo de embeddings também vem do CDN nesse caso).
> Ver `core/web/src/ort-wasm.js`.

> ⚠️ Os assets vivem fora do Git (`.gitignore` da pasta). Sempre rode este passo antes de gerar o APK.

### Passo 2: Gerar o APK

Abra esta pasta `android/` no **Android Studio**, ou compile pela linha de comando (não há
`gradlew`/wrapper versionado neste repositório — use o Gradle instalado):

```bash
# APK de release ASSINADO — é o que instala por cima sem perder dados
gradle assembleRelease

# APK de desenvolvimento (debug)
gradle assembleDebug
```

Saída:

```
android/app/build/outputs/apk/release/app-release.apk   # release
android/app/build/outputs/apk/debug/app-debug.apk       # debug
```

### Passo 3: Instalar no Smartphone

Conecte seu celular com a depuração USB ativada ou transfira o APK:

```bash
adb install -r app/build/outputs/apk/release/app-release.apk
```

O `-r` reinstala **mantendo os dados**. Ele só funciona quando o APK novo está assinado com a
**mesma** chave do instalado — ver a seção seguinte.

---

## 🔐 Assinatura do APK — por que a atualização não pode exigir desinstalar

**Regra do Android:** um APK só é instalado por cima de outro se a **assinatura for a mesma**.
Quando difere, o instalador aborta com `INSTALL_FAILED_UPDATE_INCOMPATIBLE` e a única saída é
desinstalar — e **desinstalar apaga todo o dado do app** (`filesDir` com `ton618.db` + `docs/`,
`shared_prefs` com a senha e o cache do WebView em `app_webview`).

**A causa:** o CI gerava o APK com `assembleDebug` e **nenhuma** configuração de assinatura; nesse
caso o AGP assina com o *debug keystore* que ele mesmo cria em `~/.android/debug.keystore`, e essa
chave é **gerada aleatoriamente** quando o arquivo não existe. Cada runner do GitHub Actions começa
limpo ⇒ **cada build saía com uma chave diferente** ⇒ nenhum APK novo conseguia atualizar o anterior.

**A correção:** um keystore **fixo** de release, mantido fora do repositório.

### Configuração (uma única vez)

1. Gere o keystore — **guarde bem**: sem ele não há como publicar atualizações do app já instalado
   (o `keytool` vem no JDK do Android Studio; os comandos abaixo valem no WSL):

```bash
keytool -genkeypair -v \
  -keystore ton618-release.jks -storetype JKS \
  -keyalg RSA -keysize 2048 -validity 10000 \
  -alias ton618 -storepass 'SENHA_FORTE' -keypass 'SENHA_FORTE' \
  -dname "CN=TON-618, O=TON-618, C=BR"
```

2. Gere a versão base64 (no Linux/WSL):

```bash
base64 -w0 ton618-release.jks
```

3. Crie os 4 secrets em **GitHub → Settings → Secrets and variables → Actions**:

| Secret | Valor |
| :--- | :--- |
| `ANDROID_KEYSTORE_BASE64` | saída do `base64` acima |
| `ANDROID_KEYSTORE_PASSWORD` | a `storepass` usada |
| `ANDROID_KEY_ALIAS` | `ton618` |
| `ANDROID_KEY_PASSWORD` | a `keypass` usada |

Sem esses secrets o CI continua entregando APK (variant *debug*) e escreve um **aviso** no run — mas
aí a atualização por cima volta a exigir desinstalar.

> ⚠️ **Nunca commite o `.jks`.** Quem tiver a chave consegue assinar um APK que o seu celular aceita
> instalar por cima do TON-618 — e daí ler a senha e as notas. O `android/.gitignore` já ignora
> `*.jks` / `*.keystore`.

### Compilar localmente com a mesma chave

```bash
cd android
TON618_KEYSTORE_PATH=/caminho/ton618-release.jks \
TON618_KEYSTORE_PASSWORD='SENHA_FORTE' \
TON618_KEY_ALIAS=ton618 \
TON618_KEY_PASSWORD='SENHA_FORTE' \
gradle assembleRelease
```

Sem essas variáveis o `assembleRelease` sai **sem assinatura** e o AGP gera
`app-release-unsigned.apk`, que **não instala** (por isso o CI escolhe `assembleDebug` quando o
secret não existe, em vez de publicar um APK inútil). O `assembleDebug` do dia a dia usa a chave de
debug da sua máquina: estável ali, mas **diferente** da do CI.

### Migrando do APK antigo (uma última desinstalação, de propósito)

O APK instalado hoje foi assinado por uma chave aleatória que **não existe mais**, então a primeira
instalação do APK de release vai exigir desinstalar. Para não perder nada:

1. **Configurações → Backup → Baixar (Backup Total)** — o ZIP vai para a pasta *Downloads*;
2. confira o arquivo (copie para o PC, de preferência);
3. desinstale o TON-618;
4. instale o `ton618-<versão>.apk` novo;
5. **Configurações → Backup → Restaurar** e escolha o ZIP (o app avisa que substitui tudo).

Daí em diante toda atualização é instalar por cima, **sem desinstalar e sem perder dado**. O outro
requisito do Android é o `versionCode` não diminuir: ele agora vem da tag
(`vX.Y.Z` → `MAJOR*10000 + MINOR*100 + PATCH`), que cresce a cada release.

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
> por isso o `onnxruntime-web` fica de fora por padrão. Sem ele, o worker **testa** o caminho local
> e carrega o runtime do CDN do jsdelivr (`core/web/src/ort-wasm.js` + CSP com `connect-src`
> liberado) — fixar o caminho local sem essa sonda quebrava a busca semântica com
> `no available backend found`. A busca textual (FTS5) continua completa e offline.

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

**Preciso desinstalar o app para instalar a versão nova (e perco todos os dados)**

A assinatura do APK novo é diferente da do instalado, então o Android recusa a atualização
(`INSTALL_FAILED_UPDATE_INCOMPATIBLE`) e a única saída é desinstalar — o que apaga o banco e os
arquivos. Veja **“Assinatura do APK”** acima: configure o keystore fixo e faça a última migração com
o Backup Total (exportar → desinstalar → instalar → restaurar). Depois disso a atualização passa a
ser por cima, com os dados preservados.

**Na busca semântica aparece `Servidor 0% · erro: lookup huggingface.co on [::1]:53: ... connection refused`**

O processo Go não consegue resolver DNS. Esse formato de erro (`on [::1]:53`) é exclusivo do
resolver **Go puro**, ou seja: o binário foi compilado **sem cgo**. Rode `./build-core-arm64.sh`
(ele exige o NDK e valida o binário no final) e gere o APK de novo. A busca textual e a interface
não são afetadas — o navegador baixa o modelo do CDN do HuggingFace como fallback (o painel
"Servidor X%" é só informativo).
