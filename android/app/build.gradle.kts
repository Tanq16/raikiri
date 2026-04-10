plugins {
    id("com.android.application")
}

android {
    namespace = "dev.tanq16.raikiri"
    compileSdk = 36

    defaultConfig {
        applicationId = "dev.tanq16.raikiri"
        minSdk = 26
        targetSdk = 35
        val ver = (project.findProperty("appVersion") as? String)?.removePrefix("v") ?: "0.0.0"
        val parts = ver.split(".")
        versionCode = (parts.getOrElse(0) { "0" }.toIntOrNull() ?: 0) * 10000 +
                       (parts.getOrElse(1) { "0" }.toIntOrNull() ?: 0) * 100 +
                       (parts.getOrElse(2) { "0" }.toIntOrNull() ?: 0)
        versionName = ver
    }

    signingConfigs {
        create("release") {
            val ksPath = System.getenv("KEYSTORE_PATH")
            if (ksPath != null) {
                storeFile = file(ksPath)
                storePassword = System.getenv("KEYSTORE_PASSWORD") ?: ""
                keyAlias = "raikiri"
                keyPassword = System.getenv("KEYSTORE_PASSWORD") ?: ""
                storeType = "PKCS12"
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            signingConfig = signingConfigs.getByName("release")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlin {
        jvmToolchain(17)
    }
}

dependencies {
    implementation("androidx.activity:activity-ktx:1.10.1")
    implementation("androidx.core:core-ktx:1.18.0")
    implementation("androidx.media:media:1.7.1")
}
