package dev.tanq16.raikiri

import android.content.Context
import android.util.AttributeSet
import android.view.View
import android.webkit.WebView

class MediaWebView @JvmOverloads constructor(
    context: Context,
    attrs: AttributeSet? = null,
    defStyleAttr: Int = 0
) : WebView(context, attrs, defStyleAttr) {

    override fun onWindowVisibilityChanged(visibility: Int) {
        if (visibility != View.GONE) {
            super.onWindowVisibilityChanged(visibility)
        }
        // When GONE (screen off), do NOT propagate — keeps Chromium
        // thinking the page is visible so JS event loop stays active
    }
}
