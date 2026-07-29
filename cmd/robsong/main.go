package main

import (
	"fmt"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/rdurica/robsong/assets"
	"github.com/rdurica/robsong/internal/audio"
	"github.com/rdurica/robsong/internal/playlist"
	apptheme "github.com/rdurica/robsong/internal/theme"
	"github.com/rdurica/robsong/internal/ui"
)

func main() {
	app.SetMetadata(fyne.AppMetadata{
		ID:      "com.rdurica.robsong",
		Name:    "Robsong",
		Version: "0.1.0",
		Icon:    assets.Logo,
		Migrations: map[string]bool{
			"fyneDo": true,
		},
	})

	store, err := playlist.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "open library: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	player, err := audio.New()
	if err != nil {
		_ = store.Close()
		fmt.Fprintf(os.Stderr, "audio: %v\n", err)
		os.Exit(1)
	}

	fa := app.NewWithID("com.rdurica.robsong")
	fa.SetIcon(assets.Logo)
	fa.Settings().SetTheme(&apptheme.PlayerTheme{})

	ui.NewApp(fa, store, player).ShowAndRun()
}
