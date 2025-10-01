package main

import (
	"fmt"
	"os"

	"sheet/internal/app"

	"github.com/gdamore/tcell/v2"
)

func main() {
	a := app.NewApp()

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

	s.EnableMouse()
	s.Clear()

	// Запускаем экран входа
	app.LoginScreen(s)

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
