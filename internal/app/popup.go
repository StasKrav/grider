package app


import (
    "unicode/utf8"


"github.com/gdamore/tcell/v2"

)


// PopupInput показывает модальное окно ввода с prompt и initial текстом.
// Возвращает введённую строку и true если пользователь нажал Enter,
// или пустую строку и false если пользователь отменил ввод (Esc).
//
// Теперь метод принимает экран s tcell.Screen и вызывает a.Draw(s).
// Требование: a.Draw(s tcell.Screen) должен существовать в вашем App.
func (a *App) PopupInput(s tcell.Screen, prompt, initial string) (string, bool) {
    style := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorReset)


promptRunes := []rune(prompt)
buf := []rune(initial)
pos := len(buf)

// локальный max, чтобы не конфликтовать с существующими символами
max := func(a, b int) int {
    if a > b {
        return a
    }
    return b
}

// вычисление размеров окна
w, h := s.Size()
minContentW := 20
contentW := max(minContentW, len(promptRunes)+len(buf)+2)
if contentW > w-4 {
    contentW = w - 4
}
boxW := contentW + 4
boxH := 3
left := (w - boxW) / 2
top := (h - boxH) / 2

drawBox := func() {
    // очистка области окна
    for y := top; y < top+boxH; y++ {
        for x := left; x < left+boxW; x++ {
            s.SetContent(x, y, ' ', nil, style)
        }
    }
    // рамка
    for x := left; x < left+boxW; x++ {
        s.SetContent(x, top, tcell.RuneHLine, nil, style)
        s.SetContent(x, top+boxH-1, tcell.RuneHLine, nil, style)
    }
    for y := top; y < top+boxH; y++ {
        s.SetContent(left, y, tcell.RuneVLine, nil, style)
        s.SetContent(left+boxW-1, y, tcell.RuneVLine, nil, style)
    }
    s.SetContent(left, top, tcell.RuneULCorner, nil, style)
    s.SetContent(left+boxW-1, top, tcell.RuneURCorner, nil, style)
    s.SetContent(left, top+boxH-1, tcell.RuneLLCorner, nil, style)
    s.SetContent(left+boxW-1, top+boxH-1, tcell.RuneLRCorner, nil, style)

    // prompt и поле ввода
    x := left + 2
    y := top + 1
    for i, r := range promptRunes {
        s.SetContent(x+i, y, r, nil, style)
    }
    x += len(promptRunes) + 1

    maxField := boxW - 4 - len(promptRunes)
    displayRunes := buf
    start := 0
    if len(displayRunes) > maxField {
        if pos > maxField {
            start = pos - maxField
        }
        displayRunes = displayRunes[start : start+maxField]
    }
    for i, r := range displayRunes {
        s.SetContent(x+i, y, r, nil, style)
    }
    for i := len(displayRunes); i < maxField; i++ {
        s.SetContent(x+i, y, ' ', nil, style)
    }

    cursorX := x + (pos - start)
    if cursorX < left+1 {
        cursorX = left + 1
    }
    s.ShowCursor(cursorX, y)
}

// первоначальная отрисовка: основное UI + оверлей
a.Draw(s)
drawBox()
s.Show()

for {
    ev := s.PollEvent()
    switch ev := ev.(type) {
    case *tcell.EventKey:
        switch ev.Key() {
        case tcell.KeyEsc:
            s.HideCursor()
            a.Draw(s)
            s.Show()
            return "", false
        case tcell.KeyEnter:
            s.HideCursor()
            a.Draw(s)
            s.Show()
            return string(buf), true
        case tcell.KeyBackspace, tcell.KeyBackspace2:
            if pos > 0 {
                buf = append(buf[:pos-1], buf[pos:]...)
                pos--
            }
        case tcell.KeyDelete:
            if pos < len(buf) {
                buf = append(buf[:pos], buf[pos+1:]...)
            }
        case tcell.KeyLeft:
            if pos > 0 {
                pos--
            }
        case tcell.KeyRight:
            if pos < len(buf) {
                pos++
            }
        case tcell.KeyHome:
            pos = 0
        case tcell.KeyEnd:
            pos = len(buf)
        default:
            if r := ev.Rune(); r != 0 {
                if utf8.RuneCountInString(string(buf)) < 4096 {
                    buf = append(buf[:pos], append([]rune{r}, buf[pos:]...)...)
                    pos++
                }
            }
        }
        // перерисовать
        a.Draw(s)
        drawBox()
        s.Show()
    case *tcell.EventResize:
        s.Sync()
        w, h = s.Size()
        if boxW > w-4 {
            boxW = w - 4
        }
        left = (w - boxW) / 2
        top = (h - boxH) / 2
        a.Draw(s)
        drawBox()
        s.Show()
    }
}

}
