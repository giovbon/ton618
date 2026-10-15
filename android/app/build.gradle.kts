plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "org.ton618.mobile"
    compileSdk = 34

    // ── Assinatura ESTÁVEL (ver android/README.md › "Assinatura do APK") ─────
    //
    // O Android só instala um APK por cima de outro se a assinatura for a MESMA
    // (INSTALL_FAILED_UPDATE_INCOMPATIBLE). A saída quando ela difere é
    // desinstalar — e desinstalar APAGA todo o dado do app (filesDir,
    // shared_prefs, app_webview).
    //
    // O keystore de debug que o AGP gera sozinho é NOVO a cada máquina/runner,
    // então dois builds do CI nunca se atualizam por cima. O keystore fixo vem
    // do ambiente porque é segredo: no CI os GitHub Secrets exportam
    // TON618_KEYSTORE_PATH/_PASSWORD, TON618_KEY_ALIAS e TON618_KEY_PASSWORD.
    val keystorePath = System.getenv("TON618_KEYSTORE_PATH").orEmpty()
    val keystoreFile = file(keystorePath)
    val hasReleaseKeystore = keystorePath.isNotEmpty() && keystoreFile.exists()

    defaultConfig {
        applicationId = "org.ton618.mobile"
        minSdk = 26
        targetSdk = 34
        // Vêm da tag calculada pelo CI (-PappVersionCode=…). Os defaults mantêm
        // o build local (Android Studio) funcionando sem propriedade nenhuma.
        versionCode = project.findProperty("appVersionCode")?.toString()?.toIntOrNull() ?: 1
        versionName = project.findProperty("appVersionName")?.toString()?.removePrefix("v") ?: "2.0.0"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"

        // O binário do core só existe para as ABIs geradas por
        // android/build-core-arm64.sh (padrão: arm64-v8a).
        //   ANDROID_ABI=x86_64 ./build-core-arm64.sh   → habilita emulador
        ndk {
            abiFilters.addAll(listOf("arm64-v8a", "x86_64"))
        }
    }

    // Keystore fixo, quando disponível. Sem ele o variant `release` fica SEM
    // assinatura (o AGP geraria `app-release-unsigned.apk`, que não instala), por
    // isso o CI escolhe `assembleDebug` nesse caso em vez de arriscar um APK
    // inútil — ver o passo "Preparar assinatura estável" em deploy_core.yml.
    signingConfigs {
        if (hasReleaseKeystore) {
            create("release") {
                storeFile = keystoreFile
                storePassword = System.getenv("TON618_KEYSTORE_PASSWORD")
                keyAlias = System.getenv("TON618_KEY_ALIAS")
                keyPassword = System.getenv("TON618_KEY_PASSWORD")
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
            if (hasReleaseKeystore) {
                signingConfig = signingConfigs.getByName("release")
            }
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }

    buildFeatures {
        viewBinding = true
    }

    packaging {
        jniLibs {
            // Executar o binário Go como arquivo exige extração no install
            // (equivale a android:extractNativeLibs="true" no manifesto).
            useLegacyPackaging = true
            // libton618.so é um executável Go, não uma lib com símbolos que o AGP
            // reconheça: sem isso o build tenta o strip, falha e polui o log.
            keepDebugSymbols += "**/libton618.so"
        }
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.13.1")
    implementation("androidx.appcompat:appcompat:1.7.0")
    implementation("com.google.android.material:material:1.12.0")
    implementation("androidx.constraintlayout:constraintlayout:2.1.4")
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.8.6")
    implementation("androidx.webkit:webkit:1.12.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.8.1")
}
