package com.tanq16.raikiri.ui.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable

private val MochaDarkColorScheme = darkColorScheme(
    primary = CatMauve,
    onPrimary = CatCrust,
    primaryContainer = CatSurface1,
    onPrimaryContainer = CatLavender,

    secondary = CatBlue,
    onSecondary = CatCrust,
    secondaryContainer = CatSurface0,
    onSecondaryContainer = CatText,

    tertiary = CatPink,
    onTertiary = CatCrust,
    tertiaryContainer = CatSurface1,
    onTertiaryContainer = CatText,

    error = CatRed,
    onError = CatCrust,
    errorContainer = CatSurface0,
    onErrorContainer = CatRed,

    background = CatBase,
    onBackground = CatText,
    surface = CatMantle,
    onSurface = CatText,
    surfaceVariant = CatSurface0,
    onSurfaceVariant = CatSubtext1,
    surfaceTint = CatMauve,

    outline = CatOverlay1,
    outlineVariant = CatOverlay0,

    inverseSurface = CatText,
    inverseOnSurface = CatBase,
    inversePrimary = CatMauve,

    scrim = CatCrust,
)

@Composable
fun RaikiriTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = MochaDarkColorScheme,
        typography = RaikiriTypography,
        content = content
    )
}
