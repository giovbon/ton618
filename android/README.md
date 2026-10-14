# TON-618 Mobile (Android APK)

Esta pasta contém o aplicativo Android nativo do **TON-618**.

Ele empacota o backend Go compilado nativamente para processadores **ARM64** e o executa como um serviço em segundo plano (`ForegroundService`) no Android, servindo a interface via **WebView** no próprio celular e simultaneamente disponibilizando o acesso na rede local Wi-Fi.

---

## 🏗️ Arquitetura

* **Backend:** Binário `ton618-arm64` (Go + SQLite FTS5) embutido em `app/src/main/assets/`.
* **Serviço de Segundo Plano (`TonServerService.kt`):**
  * Extrai o binário para a memória interna do aplicativo no primeiro boot (`chmod 755`).
  * Inicia o processo Go ouvindo em `0.0.0.0:6180`.
  * Cria uma notificação persistente para que o sistema operacional não suspenda o servidor ao desligar a tela.
* **Interface do Usuário (`MainActivity.kt`):**
  * Barra de status superior com:
    * Indicador visual de conexão (Verde = Ativo, Âmbar = Iniciando, Vermelho = Erro).
    * Endereço IP local (ex: `http://192.168.1.15:6180`).
    * Botão para **Copiar a URL** (para abrir no PC).
    * Botão de recarregar.
  * WebView acelerada por hardware carregando `http://127.0.0.1:6180` (latência zero no celular).

---

## 🚀 Como Compilar o Projeto

### Passo 1: Gerar o binário do Core em ARM64

Execute o script de compilação a partir da raiz ou da pasta `android`:

```bash
cd android
chmod +x build-core-arm64.sh
./build-core-arm64.sh
```

> **Dica:** O script procura o **Android NDK** (`ANDROID_NDK_HOME`) ou o compilador cruzado `aarch64-linux-gnu-gcc` para compilar o SQLite com suporte nativo a FTS5. O arquivo compilado será salvo automaticamente em `app/src/main/assets/ton618-arm64`.

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
