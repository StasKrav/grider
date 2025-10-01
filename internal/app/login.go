package app

import (
	"time"

	"github.com/gdamore/tcell/v2"
)

func LoginScreen(s tcell.Screen) {
	text := []struct {
		char  rune
		color tcell.Color
	}{
		{'G', tcell.ColorWhite},
		{'R', tcell.ColorWhite},
		{'I', tcell.ColorYellow},
		{':', tcell.ColorYellow},
		{'D', tcell.ColorYellow},
		{'E', tcell.ColorWhite},
		{'R', tcell.ColorWhite},
	}

	width, height := s.Size()

	// Анимация появления букв
	for reveal := 1; reveal <= len(text); reveal++ {
		s.Clear()

		startX := (width - len(text)) / 2
		y := height / 2

		for i := 0; i < reveal; i++ {
			style := tcell.StyleDefault.Foreground(text[i].color).Bold(true)
			s.SetContent(startX+i, y, text[i].char, nil, style)
		}

		// Подсказка
		hint := "Press any key to enter the application"
		startHintX := (width - len(hint)) / 2
		style := tcell.StyleDefault.Foreground(tcell.ColorYellow)
		for i, ch := range hint {
			s.SetContent(startHintX+i, y+2, ch, nil, style)
		}

		s.Show()
		time.Sleep(150 * time.Millisecond)
	}

	// Ждём нажатия любой клавиши
	for {
		switch s.PollEvent().(type) {
		case *tcell.EventKey:
			return
		case *tcell.EventResize:
			s.Sync()
		}
	}
	
	
}
