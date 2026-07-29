package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// PlayerTheme is a clean, compact light theme with a soft blue accent.
type PlayerTheme struct{}

var _ fyne.Theme = (*PlayerTheme)(nil)

var (
	colorBg       = color.NRGBA{R: 0xfa, G: 0xfb, B: 0xfc, A: 0xff}
	colorSurface  = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	colorPanel    = color.NRGBA{R: 0xf0, G: 0xf2, B: 0xf5, A: 0xff}
	colorHover    = color.NRGBA{R: 0xe6, G: 0xea, B: 0xef, A: 0xff}
	colorAccent   = color.NRGBA{R: 0x5b, G: 0x9b, B: 0xe6, A: 0xff}
	colorAccentDk = color.NRGBA{R: 0x4a, G: 0x86, B: 0xcc, A: 0xff}
	colorText     = color.NRGBA{R: 0x1c, G: 0x22, B: 0x2c, A: 0xff}
	colorMuted    = color.NRGBA{R: 0x7a, G: 0x84, B: 0x93, A: 0xff}
	colorSep      = color.NRGBA{R: 0xea, G: 0xed, B: 0xf1, A: 0xff}
	colorInput    = color.NRGBA{R: 0xf4, G: 0xf6, B: 0xf8, A: 0xff} // light fill for entries + slider track
	colorScroll   = color.NRGBA{R: 0xd0, G: 0xd5, B: 0xdc, A: 0xff}
	colorDisabled = color.NRGBA{R: 0xaa, G: 0xb1, B: 0xbb, A: 0xff}
	colorError    = color.NRGBA{R: 0xd9, G: 0x45, B: 0x4f, A: 0xff}
	colorSuccess  = color.NRGBA{R: 0x2f, G: 0xa3, B: 0x56, A: 0xff}
	colorWarning  = color.NRGBA{R: 0xc9, G: 0x8a, B: 0x1a, A: 0xff}
)

func (t *PlayerTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return colorBg
	case theme.ColorNameButton:
		return colorPanel
	case theme.ColorNameDisabledButton:
		return colorHover
	case theme.ColorNameDisabled:
		return colorDisabled
	case theme.ColorNameError:
		return colorError
	case theme.ColorNameFocus:
		return colorAccent
	case theme.ColorNameForeground:
		return colorText
	case theme.ColorNameForegroundOnPrimary:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	case theme.ColorNameHover:
		return colorHover
	case theme.ColorNameHeaderBackground:
		return colorSurface
	case theme.ColorNameInputBackground:
		return colorInput
	case theme.ColorNameInputBorder:
		return colorSep
	case theme.ColorNameMenuBackground:
		return colorSurface
	case theme.ColorNameOverlayBackground:
		return colorSurface // dialogs / popups — must match light UI
	case theme.ColorNamePlaceHolder:
		return colorMuted
	case theme.ColorNamePressed:
		return colorAccentDk
	case theme.ColorNamePrimary:
		return colorAccent
	case theme.ColorNameScrollBar:
		return colorScroll
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0x5b, G: 0x9b, B: 0xe6, A: 0x28}
	case theme.ColorNameSeparator:
		return colorSep
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0x1a, G: 0x1f, B: 0x2a, A: 0x10}
	case theme.ColorNameSuccess:
		return colorSuccess
	case theme.ColorNameWarning:
		return colorWarning
	default:
		return theme.DefaultTheme().Color(name, theme.VariantLight)
	}
}

func (t *PlayerTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t *PlayerTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *PlayerTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 12
	case theme.SizeNameInnerPadding:
		return 12
	case theme.SizeNameText:
		return 13
	case theme.SizeNameHeadingText:
		return 16
	case theme.SizeNameSubHeadingText:
		return 14
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameInputRadius:
		return 6
	case theme.SizeNameScrollBar:
		return 6
	case theme.SizeNameScrollBarSmall:
		return 3
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameInlineIcon:
		return 14
	case theme.SizeNamePopupRadius, theme.SizeNameDialogRadius:
		return 10
	default:
		return theme.DefaultTheme().Size(name)
	}
}

// UnderlineInputTheme is a flat white entry (no box border/shadow) for underline-style fields.
type UnderlineInputTheme struct{}

var _ fyne.Theme = (*UnderlineInputTheme)(nil)

func (t *UnderlineInputTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameInputBackground, theme.ColorNameInputBorder, theme.ColorNameShadow,
		theme.ColorNameScrollBar:
		// Transparent field chrome; caret uses Primary (must stay opaque).
		return color.NRGBA{A: 0}
	default:
		return (&PlayerTheme{}).Color(name, variant)
	}
}

func (t *UnderlineInputTheme) Font(style fyne.TextStyle) fyne.Resource {
	return (&PlayerTheme{}).Font(style)
}

func (t *UnderlineInputTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return (&PlayerTheme{}).Icon(name)
}

func (t *UnderlineInputTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameInputRadius, theme.SizeNameInputBorder:
		// Zero border: no focus ring. Caret is drawn by ui.searchEntry instead.
		return 0
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameText:
		return 14
	default:
		return (&PlayerTheme{}).Size(name)
	}
}
