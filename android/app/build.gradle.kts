// The app module: a WebView shell plus the arthik engine (jniLibs/<abi>/libarthik.so,
// built from ../cmd/arthik by scripts/build-in-container.sh).
plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

// App version = engine version (cmd/arthik/main.go), so phone and server always match.
val arthikVersion: String = (findProperty("arthikVersion") as String?)
    ?: Regex("var version = \"([^\"]+)\"").find(rootProject.file("../cmd/arthik/main.go").readText())?.groupValues?.get(1)
    ?: "0.0.0"

// 1.1.0 → 10100
fun versionCodeOf(v: String): Int {
    val p = v.split(".").map { it.takeWhile(Char::isDigit).toIntOrNull() ?: 0 } + listOf(0, 0, 0)
    return p[0] * 10000 + p[1] * 100 + p[2]
}

// Release signing comes from the environment (android/.env), never from the repo.
val keystorePath: String? = System.getenv("ARTHIK_KEYSTORE")?.takeIf { it.isNotBlank() }

android {
    namespace = "com.nihars.arthik"
    compileSdk = 34
    buildToolsVersion = "34.0.0"

    defaultConfig {
        applicationId = "com.nihars.arthik"
        minSdk = 26 // Android 8
        targetSdk = 34
        versionCode = versionCodeOf(arthikVersion)
        versionName = arthikVersion
    }

    signingConfigs {
        if (keystorePath != null) {
            create("release") {
                storeFile = file(keystorePath)
                storePassword = System.getenv("ARTHIK_KEYSTORE_PASSWORD")
                keyAlias = System.getenv("ARTHIK_KEY_ALIAS") ?: "arthik"
                keyPassword = System.getenv("ARTHIK_KEY_PASSWORD") ?: System.getenv("ARTHIK_KEYSTORE_PASSWORD")
            }
        }
    }

    buildTypes {
        getByName("release") {
            isMinifyEnabled = false // tiny app; keeps the JavaScript bridge intact
            if (keystorePath != null) signingConfig = signingConfigs.getByName("release")
        }
        getByName("debug") {
            applicationIdSuffix = ".debug" // installs next to the release app
            versionNameSuffix = "-debug"
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions {
        jvmTarget = "17"
    }

    packaging {
        jniLibs {
            useLegacyPackaging = true // extract libarthik.so to disk so it can be executed
        }
    }

    lint {
        checkReleaseBuilds = false
        abortOnError = false
    }
}

dependencies {
    testImplementation("junit:junit:4.13.2")
}
