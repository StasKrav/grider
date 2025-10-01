package main

import (
	"fmt"
	"os"

	"sheet/internal/app"

	"github.com/gdamore/tcell/v2"
)

func main() {
	// initialize app state
	a := app.NewApp()

	// start tcell
	s, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot create screen: %v\n", err)
		os.Exit(1)
	}
	if err := s.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "cannot init screen: %v\n", err)
		os.Exit(1)
	}
	defer s.Fini()

	// enable mouse (optional)
	s.EnableMouse()
	s.Clear()

	// main loop
	for !a.Quit {
		a.EnsureCursorVisible(s)
		a.Draw(s)
		ev := s.PollEvent()
		switch tev := ev.(type) {
		case *tcell.EventKey:
			a.HandleKeyEvent(s, tev)
		case *tcell.EventResize:
			s.Sync()
		}
	}
}
