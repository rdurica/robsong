package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"

	"github.com/rdurica/robsong/assets"
	"github.com/rdurica/robsong/internal/audio"
	"github.com/rdurica/robsong/internal/instance"
	"github.com/rdurica/robsong/internal/playlist"
	apptheme "github.com/rdurica/robsong/internal/theme"
	"github.com/rdurica/robsong/internal/ui"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	app.SetMetadata(fyne.AppMetadata{
		ID:      "com.rdurica.robsong",
		Name:    "Robsong",
		Version: "0.1.0",
		Icon:    assets.Logo,
		Migrations: map[string]bool{
			"fyneDo": true,
		},
	})

	tracks := collectTracks(args)
	mode := chooseLaunch(len(args) > 0, len(tracks))

	// Hand the request to a running instance before touching audio, DB or UI.
	abs := make([]string, 0, len(args))
	for _, a := range args {
		if p, err := filepath.Abs(a); err == nil {
			a = p
		}
		abs = append(abs, a)
	}
	if sent, err := instance.Send(abs); err == nil && sent {
		return 0
	}

	fa := app.NewWithID("com.rdurica.robsong")
	fa.SetIcon(assets.Logo)
	fa.Settings().SetTheme(&apptheme.PlayerTheme{})

	if mode == launchNoTracks {
		showNoTracks(fa)
		return 0
	}

	player, err := audio.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "audio: %v\n", err)
		return 1
	}

	shell := ui.NewShell(fa, player, playlist.Open)
	if l, err := instance.Listen(shell.Open); err != nil {
		fmt.Fprintf(os.Stderr, "single instance: %v\n", err)
	} else {
		defer l.Close()
	}

	if mode == launchMini {
		shell.RunMini(tracks[0])
	} else {
		shell.RunFull(tracks)
	}
	return 0
}

// showNoTracks tells the user that none of the opened files is playable.
func showNoTracks(fa fyne.App) {
	win := fa.NewWindow("Robsong")
	win.SetIcon(assets.Logo)
	win.Resize(fyne.NewSize(360, 160))
	d := dialog.NewError(errors.New("No supported audio file"), win)
	d.SetOnClosed(fa.Quit)
	win.SetCloseIntercept(fa.Quit)
	win.Show()
	d.Show()
	fa.Run()
}
