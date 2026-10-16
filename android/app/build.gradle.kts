plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.wcomsat.vpnclient"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.wcomsat.vpnclient"
        minSdk = 24
        targetSdk = 34
        versionCode = 1
        versionName = "0.1.0"
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            // Подпись — через keystore.properties (не коммитится), см. docs/build.md
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions {
        jvmTarget = "17"
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.13.1")
    implementation("androidx.appcompat:appcompat:1.7.0")
    implementation("com.google.android.material:material:1.12.0")

    // libXray (MIT) — ядро Xray-core для Android.
    // Положите собранный AAR в android/app/libs/libXray.aar и раскомментируйте:
    // implementation(files("libs/libXray.aar"))
    //
    // tun2socks (маршрутизация tun → SOCKS) добавляется на след. шаге
    // (нативный модуль / AAR).
}
