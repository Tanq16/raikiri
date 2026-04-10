-keepattributes JavascriptInterface

-keepclassmembers class * {
    @android.webkit.JavascriptInterface <methods>;
}

-keep class dev.tanq16.raikiri.PlaybackService { *; }
-keep class dev.tanq16.raikiri.WebViewPlayer { *; }
