package main

import (
	"fmt"
	"log"

	"github.com/gdamore/tcell/v2"

	"sheet/internal/app"
)

func main() {
	// Инициализация экрана tcell
	s, err := tcell.NewScreen()
	if err != nil {
		log.Fatalf("Ошибка создания экрана: %v", err)
	}
	if err := s.Init(); err != nil {
		log.Fatalf("Ошибка инициализации экрана: %v", err)
	}
	defer s.Fini()

	// Показ экрана приветствия
	app.LoginScreen(s)

	// Создание приложения
	a := app.NewApp()

	// Основной цикл приложения
	for !a.Quit {
		// Отрисовка
		a.Draw(s)
		s.Show()

		// Обработка событий
		ev := s.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			a.HandleKeyEvent(s, ev)
		case *tcell.EventResize:
			s.Sync()
		}
	}

	// Очистка перед выходом
	s.Fini()
	fmt.Println("Приложение завершено")
}
