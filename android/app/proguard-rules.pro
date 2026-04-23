# kotlinx.serialization
-keepattributes *Annotation*, InnerClasses
-dontnote kotlinx.serialization.AnnotationsKt
-keepclassmembers class kotlinx.serialization.json.** { *** Companion; }
-keepclasseswithmembers class kotlinx.serialization.json.** { kotlinx.serialization.KSerializer serializer(...); }
-keep,includedescriptorclasses class com.tanq16.raikiri.**$$serializer { *; }
-keepclassmembers class com.tanq16.raikiri.** { *** Companion; }
-keepclasseswithmembers class com.tanq16.raikiri.** { kotlinx.serialization.KSerializer serializer(...); }

# Retrofit
-keep,allowobfuscation,allowshrinking interface retrofit2.Call
-keep,allowobfuscation,allowshrinking class retrofit2.Response
-keep,allowobfuscation,allowshrinking class kotlin.coroutines.Continuation

# Media3
-keep class androidx.media3.session.MediaSessionService { *; }
