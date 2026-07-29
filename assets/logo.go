package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed logo.png
var logoPNG []byte

// Logo is the Robsong app mark (PNG with alpha).
var Logo = fyne.NewStaticResource("logo.png", logoPNG)
