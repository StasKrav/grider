package app

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sheet/internal/calc"
	"sheet/internal/grid"
	"sheet/internal/storage"

	"github.com/gdamore/tcell/v2"
)

type App struct {
	// layout
	LeftGutter    int
	StatusLines   int
	DefaultWidth  int
	DefaultHeight int

	CellPadding int

	// grid data
	ColWidths  []int
	RowHeights []int
	Grid       map[[2]int]grid.Cell

	// cursor / view
	CurRow  int
	CurCol  int
	ViewRow int
	ViewCol int

	// UI state
	Mode       string // normal | insert | command | confirm
	InputBuf   string
	CommandBuf string
	ConfirmMsg string
	Quit       bool

	// editing behavior options
	EnterStartsEdit     bool
	PrintableStartsEdit bool
	MoveAfterEnter      bool
	SelectAllOnEdit     bool
	ReplaceOnNextRune   bool

	// UI: help popup visibility
	HelpVisible bool
}

func NewApp() *App {
	a := &App{
		LeftGutter:          4,
		StatusLines:         2,
		DefaultWidth:        16,
		DefaultHeight:       1,
		CellPadding:         1,
		ColWidths:           []int{},
		RowHeights:          []int{},
		Grid:                map[[2]int]grid.Cell{},
		CurRow:              0,
		CurCol:              0,
		ViewRow:             0,
		ViewCol:             0,
		Mode:                "normal",
		InputBuf:            "",
		CommandBuf:          "",
		ConfirmMsg:          "",
		Quit:                false,
		EnterStartsEdit:     true,
		PrintableStartsEdit: false,
		MoveAfterEnter:      true,
		SelectAllOnEdit:     true,
		ReplaceOnNextRune:   false,
		HelpVisible:         false,
	}
	// initial sizes (like original)
	for i := 0; i < 8; i++ {
		a.ColWidths = append(a.ColWidths, a.DefaultWidth)
		a.RowHeights = append(a.RowHeights, a.DefaultHeight)
	}
	return a
}

// ----------------------------- Events / Input -----------------------------

func (a *App) HandleKeyEvent(s tcell.Screen, ev *tcell.EventKey) {
	if a.Mode == "insert" {
		mod := ev.Modifiers()
		switch ev.Key() {
		case tcell.KeyEsc:
			// cancel edit
			a.Mode = "normal"
			a.InputBuf = ""
			a.ReplaceOnNextRune = false
		case tcell.KeyEnter:
			// Shift+Enter or Alt+Enter -> insert newline into cell (if supported)
			if mod&tcell.ModShift != 0 || mod&tcell.ModAlt != 0 {
				a.InputBuf += "\n"
			} else {
				// commit
				if a.InputBuf != "" {
					a.EnsureColExists(a.CurCol)
					a.EnsureRowExists(a.CurRow)
					a.Grid[[2]int{a.CurRow, a.CurCol}] = grid.Cell{Text: a.InputBuf}
				} else {
					delete(a.Grid, [2]int{a.CurRow, a.CurCol})
				}
				a.Mode = "normal"
				a.InputBuf = ""
				a.ReplaceOnNextRune = false
				// move after enter unless Ctrl held
				if mod&tcell.ModCtrl == 0 && a.MoveAfterEnter {
					a.CurRow++
					a.EnsureRowExists(a.CurRow)
				}
			}
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if len(a.InputBuf) > 0 {
				a.InputBuf = a.InputBuf[:len(a.InputBuf)-1]
			}
			a.ReplaceOnNextRune = false
		default:
			// regular rune input
			r := ev.Rune()
			if r != 0 {
				if a.ReplaceOnNextRune {
					// replace entire buffer with this rune
					a.InputBuf = string(r)
					a.ReplaceOnNextRune = false
				} else {
					a.InputBuf += string(r)
				}
			}
		}
		return
	}

	// Удаляем блок "command" mode, так как PopupInput берет на себя всю его функциональность
	// if a.Mode == "command" {
	//     switch ev.Key() {
	//     case tcell.KeyEsc:
	//         a.Mode = "normal"
	//         a.CommandBuf = ""
	//     case tcell.KeyEnter:
	//         a.ExecuteCommand(a.CommandBuf)
	//         a.Mode = "normal"
	//         a.CommandBuf = ""
	//     case tcell.KeyBackspace, tcell.KeyBackspace2:
	//         if len(a.CommandBuf) > 0 {
	//             a.CommandBuf = a.CommandBuf[:len(a.CommandBuf)-1]
	//         }
	//     default:
	//         r := ev.Rune()
	//         if r != 0 {
	//             a.CommandBuf += string(r)
	//         }
	//     }
	//     return
	// }

	// If help popup is visible, consume most keys and only allow closing with Esc or "?"
	if a.HelpVisible {
		if ev.Key() == tcell.KeyEsc {
			a.HelpVisible = false
			return
		}
		r := ev.Rune()
		if r == '?' {
			a.HelpVisible = false
			return
		}
		return
	}

	// normal mode
	mod := ev.Modifiers()
	switch ev.Key() {
	case tcell.KeyEsc:
		// noop
	case tcell.KeyCtrlC:
		a.Quit = true
	case tcell.KeyUp:
		if mod&tcell.ModCtrl != 0 {
			// ctrl+up -> decrease row height
			if a.CurRow >= 0 && a.CurRow < len(a.RowHeights) {
				if a.RowHeights[a.CurRow] > 1 {
					a.RowHeights[a.CurRow]--
				}
			}
		} else {
			if a.CurRow > 0 {
				a.CurRow--
			}
		}
	case tcell.KeyDown:
		if mod&tcell.ModCtrl != 0 {
			if a.CurRow >= 0 && a.CurRow < len(a.RowHeights) {
				a.RowHeights[a.CurRow]++
			}
		} else {
			a.CurRow++
			a.EnsureRowExists(a.CurRow)
		}
	case tcell.KeyLeft:
		if mod&tcell.ModCtrl != 0 {
			if a.CurCol >= 0 && a.CurCol < len(a.ColWidths) {
				if a.ColWidths[a.CurCol] > 4 {
					a.ColWidths[a.CurCol]--
				}
			}
		} else {
			if a.CurCol > 0 {
				a.CurCol--
			}
		}
	case tcell.KeyRight:
		if mod&tcell.ModCtrl != 0 {
			if a.CurCol >= 0 && a.CurCol < len(a.ColWidths) {
				a.ColWidths[a.CurCol]++
			}
		} else {
			a.CurCol++
			a.EnsureColExists(a.CurCol)
		}
	case tcell.KeyPgUp:
		vr, _ := a.ComputeVisible(s)
		a.ViewRow -= vr
		if a.ViewRow < 0 {
			a.ViewRow = 0
		}
	case tcell.KeyPgDn:
		vr, _ := a.ComputeVisible(s)
		a.ViewRow += vr
		if a.ViewRow >= len(a.RowHeights) {
			a.ViewRow = maxInt(0, len(a.RowHeights)-1)
		}
	case tcell.KeyHome:
		a.ViewCol = 0
		a.ViewRow = 0
	case tcell.KeyEnd:
		a.ViewCol = maxInt(0, len(a.ColWidths)-1)
		a.ViewRow = maxInt(0, len(a.RowHeights)-1)
	case tcell.KeyF2:
		// add row after current
		idx := a.CurRow + 1
		if idx < 0 {
			idx = 0
		}
		if idx > len(a.RowHeights) {
			idx = len(a.RowHeights)
		}
		a.RowHeights = append(a.RowHeights[:idx], append([]int{a.DefaultHeight}, a.RowHeights[idx:]...)...)
	case tcell.KeyF3:
		// add column after current
		idx := a.CurCol + 1
		if idx < 0 {
			idx = 0
		}
		if idx > len(a.ColWidths) {
			idx = len(a.ColWidths)
		}
		a.ColWidths = append(a.ColWidths[:idx], append([]int{a.DefaultWidth}, a.ColWidths[idx:]...)...)
		// shift existing cells to the right for columns >= idx
		newGrid := map[[2]int]grid.Cell{}
		for k, v := range a.Grid {
			r, c := k[0], k[1]
			if c >= idx {
				newGrid[[2]int{r, c + 1}] = v
			} else {
				newGrid[[2]int{r, c}] = v
			}
		}
		a.Grid = newGrid
	case tcell.KeyF4:
		// delete current row
		if len(a.RowHeights) > 0 && a.CurRow >= 0 && a.CurRow < len(a.RowHeights) {
			a.RowHeights = append(a.RowHeights[:a.CurRow], a.RowHeights[a.CurRow+1:]...)
			newGrid := map[[2]int]grid.Cell{}
			for k, v := range a.Grid {
				r, c := k[0], k[1]
				if r == a.CurRow {
					continue
				}
				if r > a.CurRow {
					newGrid[[2]int{r - 1, c}] = v
				} else {
					newGrid[[2]int{r, c}] = v
				}
			}
			a.Grid = newGrid
			if a.CurRow >= len(a.RowHeights) {
				a.CurRow = maxInt(0, len(a.RowHeights)-1)
			}
		}
	case tcell.KeyF5:
		// delete current column
		if len(a.ColWidths) > 0 && a.CurCol >= 0 && a.CurCol < len(a.ColWidths) {
			colIdx := a.CurCol
			a.ColWidths = append(a.ColWidths[:colIdx], a.ColWidths[colIdx+1:]...)
			newGrid := map[[2]int]grid.Cell{}
			for k, v := range a.Grid {
				r, c := k[0], k[1]
				if c == colIdx {
					continue
				}
				if c > colIdx {
					newGrid[[2]int{r, c - 1}] = v
				} else {
					newGrid[[2]int{r, c}] = v
				}
			}
			a.Grid = newGrid
			if a.CurCol >= len(a.ColWidths) {
				a.CurCol = maxInt(0, len(a.ColWidths)-1)
			}
		}
	default:
		// printable keys and special handling for Enter -> start edit
		r := ev.Rune()

		// handle Enter starting edit (if configured)
		if ev.Key() == tcell.KeyEnter && a.EnterStartsEdit {
			// start edit mode
			a.Mode = "insert"
			if cell, ok := a.Grid[[2]int{a.CurRow, a.CurCol}]; ok {
				a.InputBuf = cell.Text
			} else {
				a.InputBuf = ""
			}
			if a.SelectAllOnEdit {
				a.ReplaceOnNextRune = true
			} else {
				a.ReplaceOnNextRune = false
			}
			return
		}

		if r != 0 {
			switch r {
			case 'q':
				a.Quit = true
			case 'i':
				// vim-like insert
				a.Mode = "insert"
				if cell, ok := a.Grid[[2]int{a.CurRow, a.CurCol}]; ok {
					a.InputBuf = cell.Text
				} else {
					a.InputBuf = ""
				}
				if a.SelectAllOnEdit {
					a.ReplaceOnNextRune = true
				} else {
					a.ReplaceOnNextRune = false
				}

			// =========================================================================
			// НОВЫЙ КОД для обработки ':' и '=' с помощью PopupInput
			case ':':
				// Вместо установки a.Mode = "command", вызываем PopupInput
				command, ok := a.PopupInput(s, ":", "")
				if ok {
					a.ExecuteCommand(command)
				}
				// PopupInput уже вернул режим normal и перерисовал основной UI,
				// поэтому a.Mode = "normal" и a.CommandBuf = "" не нужны.
				// Возвращаемся из обработчика события.
				return
			case '=':
				// Вызываем PopupInput для ввода значения
				value, ok := a.PopupInput(s, "", "=")
				if ok {
					// Здесь ваша логика обработки введенного значения
					// Например, обновить ячейку, или вызвать другой метод App
					a.SetCellValue(value) // Предполагаем, что у вас есть такой метод
				}
				return
			// =========================================================================

			case '?':
				// show help popup centered with border
				a.HelpVisible = true
			default:
				// other printable character
				if a.PrintableStartsEdit {
					a.Mode = "insert"
					// start editing with this rune (replace)
					a.InputBuf = string(r)
					a.ReplaceOnNextRune = false
				}
			}
		}
	}
}

// Добавьте этот вспомогательный метод в вашу структуру App,
// если его еще нет, или используйте существующую логику.
// Этот метод будет вызываться при вводе "="
func (a *App) SetCellValue(value string) {
	// Пример: установить значение в текущую ячейку
	a.EnsureColExists(a.CurCol)
	a.EnsureRowExists(a.CurRow)
	a.Grid[[2]int{a.CurRow, a.CurCol}] = grid.Cell{Text: value}
}

// УДАЛИТЕ ЭТУ СТРОКУ, т.к.PopupInput взял на себя эту ответственность
// func maxInt(a, b int) int {
//     if a > b {
//         return a
//     }
//     return b
// }

// ----------------------------- Drawing -----------------------------

func (a *App) Draw(s tcell.Screen) {
	s.Clear()
	w, h := s.Size()

	// header row: column names
	x := a.LeftGutter
	for c := a.ViewCol; c < len(a.ColWidths); c++ {
		wc := a.ColWidths[c]
		if x+wc > w {
			// truncated column: we'll still attempt to draw header fragment and then break
		}
		name := grid.ColToName(c)

		// header style; invert if this is the active column
		hdrStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow)
		if c == a.CurCol {
			hdrStyle = tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorYellow)
			for dx := 0; dx < wc; dx++ {
				if x+dx >= 0 && x+dx < w {
					s.SetContent(x+dx, 0, ' ', nil, hdrStyle)
				}
			}
		}

		innerX := x + a.CellPadding
		innerW := wc - 2*a.CellPadding
		if innerW < 0 {
			innerW = 0
		}
		if innerW > 0 {
			a.printTextFixedWidth(s, innerX, 0, name, hdrStyle, innerW)
		} else {
			a.printTextFixedWidth(s, x, 0, name, hdrStyle, wc)
		}

		x += wc
		if x >= w {
			break
		}
	}

	// draw rows
	y := 1
	for r := a.ViewRow; r < len(a.RowHeights); r++ {
		if y >= h-a.StatusLines {
			break
		}
		// row number in the gutter
		rowNum := fmt.Sprintf("%d", r+1)
		gutterStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow)
		if r == a.CurRow {
			gutterStyle = tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorYellow)
			for gx := 0; gx < a.LeftGutter-1; gx++ {
				if gx >= 0 && gx < w {
					s.SetContent(gx, y, ' ', nil, gutterStyle)
				}
			}
		}
		a.printTextFixedWidth(s, 0, y, rowNum, gutterStyle, a.LeftGutter-1)

		x = a.LeftGutter
		hh := a.RowHeights[r]
		for c := a.ViewCol; c < len(a.ColWidths); c++ {
			if y >= h-a.StatusLines {
				break
			}
			wc := a.ColWidths[c]
			dispText := a.GetDisplayText(r, c)
			if a.Mode == "insert" && r == a.CurRow && c == a.CurCol {
				dispText = a.InputBuf
			}
			lines := a.splitLines(dispText, hh)

			isSelected := (r == a.CurRow && c == a.CurCol)

			var baseStyle tcell.Style
			if isSelected {
				baseStyle = tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorLightGray)
			} else {
				baseStyle = tcell.StyleDefault
			}

			// clear cell rectangle
			for dy := 0; dy < hh; dy++ {
				for dx := 0; dx < wc; dx++ {
					if x+dx >= 0 && y+dy >= 0 && x+dx < w && y+dy < h {
						s.SetContent(x+dx, y+dy, ' ', nil, baseStyle)
					}
				}
			}

			// print lines with left/right padding
			innerX := x + a.CellPadding
			innerW := wc - 2*a.CellPadding
			if innerW < 0 {
				innerW = 0
			}

			for dy := 0; dy < hh; dy++ {
				txt := ""
				if dy < len(lines) {
					txt = lines[dy]
				}
				if innerW > 0 {
					a.printTextFixedWidth(s, innerX, y+dy, txt, baseStyle, innerW)
				} else {
					a.printTextFixedWidth(s, x, y+dy, txt, baseStyle, wc)
				}
			}

			x += wc
			if x >= w {
				break
			}
		}
		y += hh
	}

	// Status area
	statusY := h - a.StatusLines
	if statusY < 0 {
		statusY = 0
	}
	statusStyle := tcell.StyleDefault.Background(tcell.ColorGray).Foreground(tcell.ColorWhite)

	curColW := a.DefaultWidth
	curRowH := a.DefaultHeight
	if a.CurCol >= 0 && a.CurCol < len(a.ColWidths) {
		curColW = a.ColWidths[a.CurCol]
	}
	if a.CurRow >= 0 && a.CurRow < len(a.RowHeights) {
		curRowH = a.RowHeights[a.CurRow]
	}

	statusLeft := fmt.Sprintf("Mode:%s  Cell:%d,%d  cw(cur)=%d rh(cur)=%d  View:%d,%d", a.Mode, a.CurRow+1, a.CurCol+1, curColW, curRowH, a.ViewRow+1, a.ViewCol+1)
	wTotal, _ := s.Size()
	a.printTextFixedWidth(s, 0, statusY, statusLeft, statusStyle, wTotal)

	if a.Mode == "insert" {
		prompt := "EDIT: " + a.InputBuf
		a.printTextFixedWidth(s, 0, statusY+1, prompt, statusStyle, wTotal)
	} else if a.Mode == "command" {
		prompt := ":" + a.CommandBuf
		a.printTextFixedWidth(s, 0, statusY+1, prompt, statusStyle, wTotal)
	} else if a.Mode == "confirm" {
		a.printTextFixedWidth(s, 0, statusY+1, a.ConfirmMsg, statusStyle, wTotal)
	}

	// If help popup requested, draw it on top
	if a.HelpVisible {
		help := "\n i / Enter - edit \n Ctrl+Enter - save&stay \n Shift/Alt+Enter - newline \n : - command \n = - formula \n Ctrl←/Ctrl→ - col width \n Ctrl↑/Ctrl↓ - row height \n F2/F3 - add row/col \n F4 - delete row \n F5 - delete col \n PgUp/PgDn/Home/End - scroll \n :w file [csv] | :o file [csv] \n "
		a.drawHelpPopup(s, help)
	}

	// Show cursor while in insert mode
	if a.Mode == "insert" {
		w, h := s.Size()
		// compute cell top-left
		cellX := a.LeftGutter
		if a.CurCol >= a.ViewCol {
			for cc := a.ViewCol; cc < a.CurCol && cc < len(a.ColWidths); cc++ {
				cellX += a.ColWidths[cc]
			}
		} else {
			for cc := a.ViewCol - 1; cc >= a.CurCol && cc >= 0 && cc < len(a.ColWidths); cc-- {
				cellX -= a.ColWidths[cc]
			}
		}
		cellY := 1
		if a.CurRow >= a.ViewRow {
			for rr := a.ViewRow; rr < a.CurRow && rr < len(a.RowHeights); rr++ {
				cellY += a.RowHeights[rr]
			}
		} else {
			for rr := a.ViewRow - 1; rr >= a.CurRow && rr >= 0 && rr < len(a.RowHeights); rr-- {
				cellY -= a.RowHeights[rr]
			}
		}

		// basic bounds check
		if cellX >= 0 && cellY >= 0 && cellX < w && cellY < h-a.StatusLines {
			lines := strings.Split(a.InputBuf, "\n")
			lastIdx := len(lines) - 1
			if lastIdx < 0 {
				lastIdx = 0
			}
			lastLine := ""
			if len(lines) > 0 {
				lastLine = lines[lastIdx]
			}
			colW := a.DefaultWidth
			rowH := a.DefaultHeight
			if a.CurCol >= 0 && a.CurCol < len(a.ColWidths) {
				colW = a.ColWidths[a.CurCol]
			}
			if a.CurRow >= 0 && a.CurRow < len(a.RowHeights) {
				rowH = a.RowHeights[a.CurRow]
			}

			innerW := colW - 2*a.CellPadding
			if innerW < 1 {
				cx := cellX + minInt(runeLen(lastLine), maxInt(0, colW-1))
				cy := cellY + minInt(lastIdx, maxInt(0, rowH-1))
				if cx >= 0 && cx < w && cy >= 0 && cy < h {
					s.SetContent(cx, cy, '▏', nil,
						tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorLightGray))
				} else {
					s.HideCursor()
				}
			} else {
				xOffset := minInt(runeLen(lastLine), maxInt(0, innerW-1))
				cx := cellX + a.CellPadding + xOffset
				cy := cellY + minInt(lastIdx, maxInt(0, rowH-1))
				if cx >= 0 && cx < w && cy >= 0 && cy < h {
					s.SetContent(cx, cy, '▏', nil,
						tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorLightGray))
				} else {
					s.HideCursor()
				}
			}
		} else {
			s.HideCursor()
		}
	} else {
		s.HideCursor()
	}

	s.Show()
}

// ----------------------------- Helpers -----------------------------

func (a *App) EnsureColExists(idx int) {
	for len(a.ColWidths) <= idx {
		a.ColWidths = append(a.ColWidths, a.DefaultWidth)
	}
}

func (a *App) EnsureRowExists(idx int) {
	for len(a.RowHeights) <= idx {
		a.RowHeights = append(a.RowHeights, a.DefaultHeight)
	}
}

func (a *App) printTextFixedWidth(s tcell.Screen, x, y int, str string, style tcell.Style, width int) {
	runes := []rune(str)
	for i := 0; i < width; i++ {
		var ch rune = ' '
		if i < len(runes) {
			ch = runes[i]
		}
		if x+i >= 0 && y >= 0 {
			s.SetContent(x+i, y, ch, nil, style)
		}
	}
}

func (a *App) splitLines(text string, maxLines int) []string {
	if maxLines <= 0 {
		return []string{}
	}
	out := make([]string, maxLines)
	if text == "" {
		for i := range out {
			out[i] = ""
		}
		return out
	}
	parts := strings.Split(text, "\n")
	for i := 0; i < maxLines; i++ {
		if i < len(parts) {
			out[i] = parts[i]
		} else {
			out[i] = ""
		}
	}
	return out
}

func (a *App) drawHelpPopup(s tcell.Screen, help string) {
	w, h := s.Size()
	if w < 10 || h < 5 {
		return
	}

	padding := 4 // отступ внутри рамки
	maxPW := w - 6
	maxPH := h - 6

	innerW := minInt(maxPW-padding*2, 50)
	if innerW < 30 {
		innerW = maxInt(30, maxPW-padding*2)
	}
	innerW = minInt(innerW, maxPW-padding*2)

	lines := wrapText(help, innerW)
	if len(lines) > maxPH-padding*2 {
		lines = lines[:maxPH-padding*2]
	}

	innerH := len(lines)
	if innerH < 3 {
		innerH = 3
	}

	pw := innerW + padding*2
	ph := innerH + padding*2

	left := (w - pw) / 2
	top := (h - ph) / 2

	borderStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorDefault)
	bgStyle := tcell.StyleDefault.Background(tcell.ColorDefault).Foreground(tcell.ColorWhite)

	// рисуем фон
	for yy := 0; yy < ph; yy++ {
		for xx := 0; xx < pw; xx++ {
			s.SetContent(left+xx, top+yy, ' ', nil, bgStyle)
		}
	}

	// рисуем рамку
	s.SetContent(left, top, '┌', nil, borderStyle)
	s.SetContent(left+pw-1, top, '┐', nil, borderStyle)
	s.SetContent(left, top+ph-1, '└', nil, borderStyle)
	s.SetContent(left+pw-1, top+ph-1, '┘', nil, borderStyle)
	for xx := 1; xx < pw-1; xx++ {
		s.SetContent(left+xx, top, '─', nil, borderStyle)
		s.SetContent(left+xx, top+ph-1, '─', nil, borderStyle)
	}
	for yy := 1; yy < ph-1; yy++ {
		s.SetContent(left, top+yy, '│', nil, borderStyle)
		s.SetContent(left+pw-1, top+yy, '│', nil, borderStyle)
	}

	// вертикальное центрирование блока текста
	vOffset := (ph - padding*2 - innerH) / 2

	// печатаем строки, выравненные по левому краю
	for i, ln := range lines {
		a.printTextFixedWidth(s, left+padding, top+padding+vOffset+i, ln, bgStyle, innerW)
	}
}

func wrapText(s string, max int) []string {
	if max <= 2 {
		return []string{s}
	}

	var result []string
	paragraphs := strings.Split(s, "\n") // разбиваем по \n

	for pi, para := range paragraphs {
		words := strings.Fields(para)
		if len(words) == 0 {
			result = append(result, "") // пустая строка для переноса
			continue
		}

		cur := " " // левый отступ

		for _, w := range words {
			if runeLen(w) > max-1 {
				chunks := chunkString(w, max-1)
				for i, c := range chunks {
					if i == 0 {
						if runeLen(cur)+1+runeLen(c) <= max {
							cur += " " + c
						} else {
							result = append(result, cur)
							cur = " " + c
						}
					} else {
						result = append(result, " "+c)
						cur = " "
					}
				}
				continue
			}

			if runeLen(cur)+1+runeLen(w) <= max {
				if runeLen(cur) > 1 {
					cur += " " + w
				} else {
					cur += w
				}
			} else {
				result = append(result, cur)
				cur = " " + w
			}
		}

		if cur != "" {
			result = append(result, cur)
		}

		if pi < len(paragraphs)-1 {
			result = append(result, "") // сохраняем пустую строку между абзацами
		}
	}

	return result
}

func runeLen(s string) int {
	return len([]rune(s))
}

func chunkString(s string, size int) []string {
	r := []rune(s)
	var out []string
	for i := 0; i < len(r); i += size {
		j := i + size
		if j > len(r) {
			j = len(r)
		}
		out = append(out, string(r[i:j]))
	}
	return out
}

// ----------------------------- Commands / Storage -----------------------------

func (a *App) ExecuteCommand(cmd string) {
	if cmd == "" {
		return
	}
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}
	switch parts[0] {
	case "q", "quit":
		a.Quit = true
	case "cw":
		if len(parts) >= 2 {
			if v, err := strconv.Atoi(parts[1]); err == nil && v >= 4 {
				for i := range a.ColWidths {
					a.ColWidths[i] = v
				}
			}
		}
	case "rh":
		if len(parts) >= 2 {
			if v, err := strconv.Atoi(parts[1]); err == nil && v >= 1 {
				for i := range a.RowHeights {
					a.RowHeights[i] = v
				}
			}
		}
	case "w":
		if len(parts) >= 2 {
			// Проверяем, хотим ли сохранить в формате CSV или grider
			// Файл сохраняется как CSV, если:
			// 1. Указан третий аргумент "csv", или
			// 2. Имя файла заканчивается на ".csv"
			filename := parts[1]
			if (len(parts) >= 3 && parts[2] == "csv") || filepath.Ext(filename) == ".csv" {
				// Для CSV не добавляем расширение .grider
				// Если имя файла не заканчивается на .csv, добавляем это расширение
				if filepath.Ext(filename) != ".csv" {
					filename += ".csv"
				}
				if err := storage.SaveCSV(a.Grid, filename); err != nil {
					fmt.Fprintf(os.Stderr, "error saving CSV: %v\n", err)
				}
			} else {
				// Сохраняем в формате grider
				if err := storage.SaveDocument(a.Grid, a.ColWidths, a.RowHeights, filename); err != nil {
					fmt.Fprintf(os.Stderr, "error saving document: %v\n", err)
				}
			}
		}
	case "o":
		if len(parts) >= 2 {
			// Проверяем, хотим ли загрузить из формата CSV или grider
			// Файл загружается как CSV, если:
			// 1. Указан третий аргумент "csv", или
			// 2. Имя файла заканчивается на ".csv"
			filename := parts[1]
			if (len(parts) >= 3 && parts[2] == "csv") || filepath.Ext(filename) == ".csv" {
				// Для CSV не добавляем расширение .grider
				// Если имя файла не заканчивается на .csv, добавляем это расширение
				if filepath.Ext(filename) != ".csv" {
					filename += ".csv"
				}
				gridMap, maxR, maxC, err := storage.LoadCSV(filename)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error loading CSV: %v\n", err)
					return
				}
				a.Grid = gridMap
				for i := 0; i <= maxC; i++ {
					a.EnsureColExists(i)
				}
				for i := 0; i <= maxR; i++ {
					a.EnsureRowExists(i)
				}
			} else {
				// Загружаем из формата grider
				grid, colWidths, rowHeights, err := storage.LoadDocument(filename)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error loading document: %v\n", err)
					return
				}
				a.Grid = grid
				a.ColWidths = colWidths
				a.RowHeights = rowHeights
			}
			a.CurRow = 0
			a.CurCol = 0
			a.ViewRow = 0
			a.ViewCol = 0
		}
	default:
		// unknown command
	}
}

// ----------------------------- Display / Formulas -----------------------------

func (a *App) GetDisplayText(r, c int) string {
	key := [2]int{r, c}
	cell, ok := a.Grid[key]
	if !ok {
		return ""
	}
	text := cell.Text
	if text == "" {
		return ""
	}
	if !strings.HasPrefix(text, "=") {
		return text
	}
	expr := text[1:]
	visited := map[[2]int]bool{}
	// mark base to detect self reference
	baseKey := [2]int{r, c}
	if visited[baseKey] {
		return "#CYCLE"
	}
	visited[baseKey] = true
	defer delete(visited, baseKey)

	// resolver closure to give to calc package
	resolve := func(name string) (float64, string) {
		ridx, cidx, ok := grid.ParseCellRef(name)
		if !ok {
			return 0, "#REF"
		}
		if ridx < 0 || cidx < 0 {
			return 0, "#REF"
		}
		if ridx >= len(a.RowHeights) || cidx >= len(a.ColWidths) {
			return 0, "#REF"
		}
		k := [2]int{ridx, cidx}
		if visited[k] {
			return 0, "#CYCLE"
		}
		cell, ok := a.Grid[k]
		if !ok || cell.Text == "" {
			return 0, ""
		}
		if strings.HasPrefix(cell.Text, "=") {
			visited[k] = true
			val, err := a.evalFormulaForCell(cell.Text[1:], ridx, cidx, visited)
			delete(visited, k)
			return val, err
		}
		v, err := strconv.ParseFloat(cell.Text, 64)
		if err != nil {
			return 0, "#ERR"
		}
		return v, ""
	}

	val, errCode := calc.EvalExprForCell(expr, r, c, resolve, visited)
	if errCode != "" {
		return errCode
	}
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return "#ERR"
	}
	if math.Abs(val-math.Round(val)) < 1e-9 {
		return fmt.Sprintf("%.0f", math.Round(val))
	}
	s := strconv.FormatFloat(val, 'f', 6, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

// helper to evaluate formula string given visited; delegates to calc (keeps same semantics)
func (a *App) evalFormulaForCell(expr string, baseR, baseC int, visited map[[2]int]bool) (float64, string) {
	// create a resolver similar to above (used by GetDisplayText recursion)
	resolve := func(name string) (float64, string) {
		ridx, cidx, ok := grid.ParseCellRef(name)
		if !ok {
			return 0, "#REF"
		}
		if ridx < 0 || cidx < 0 {
			return 0, "#REF"
		}
		if ridx >= len(a.RowHeights) || cidx >= len(a.ColWidths) {
			return 0, "#REF"
		}
		k := [2]int{ridx, cidx}
		if visited[k] {
			return 0, "#CYCLE"
		}
		cell, ok := a.Grid[k]
		if !ok || cell.Text == "" {
			return 0, ""
		}
		if strings.HasPrefix(cell.Text, "=") {
			visited[k] = true
			val, err := a.evalFormulaForCell(cell.Text[1:], ridx, cidx, visited)
			delete(visited, k)
			return val, err
		}
		v, err := strconv.ParseFloat(cell.Text, 64)
		if err != nil {
			return 0, "#ERR"
		}
		return v, ""
	}
	return calc.EvalExprForCell(expr, baseR, baseC, resolve, visited)
}

// ----------------------------- Viewport / Geometry -----------------------------

func (a *App) ComputeVisible(s tcell.Screen) (visibleRows, visibleCols int) {
	w, h := s.Size()
	usableW := w - a.LeftGutter
	if usableW < 1 {
		usableW = 1
	}
	usableH := h - a.StatusLines - 1
	if usableH < 1 {
		usableH = 1
	}
	sumW := 0
	cols := 0
	for c := a.ViewCol; c < len(a.ColWidths); c++ {
		wc := a.ColWidths[c]
		if sumW+wc > usableW {
			break
		}
		sumW += wc
		cols++
	}
	if cols < 1 {
		cols = 1
	}
	sumH := 0
	rows := 0
	for r := a.ViewRow; r < len(a.RowHeights); r++ {
		hh := a.RowHeights[r]
		if sumH+hh > usableH {
			break
		}
		sumH += hh
		rows++
	}
	if rows < 1 {
		rows = 1
	}
	return rows, cols
}

func (a *App) EnsureCursorVisible(s tcell.Screen) {
	if s == nil {
		return
	}
	w, h := s.Size()
	usableW := w - a.LeftGutter
	if usableW < 1 {
		usableW = 1
	}
	usableH := h - a.StatusLines - 1
	if usableH < 1 {
		usableH = 1
	}

	visibleCols := 0
	sumW := 0
	for c := a.ViewCol; c < len(a.ColWidths); c++ {
		wc := a.ColWidths[c]
		if sumW+wc > usableW {
			break
		}
		sumW += wc
		visibleCols++
	}
	if visibleCols < 1 {
		visibleCols = 1
	}

	visibleRows := 0
	sumH := 0
	for r := a.ViewRow; r < len(a.RowHeights); r++ {
		hh := a.RowHeights[r]
		if sumH+hh > usableH {
			break
		}
		sumH += hh
		visibleRows++
	}
	if visibleRows < 1 {
		visibleRows = 1
	}

	if a.CurCol < a.ViewCol {
		a.ViewCol = a.CurCol
	} else if a.CurCol >= a.ViewCol+visibleCols {
		a.ViewCol = a.CurCol - visibleCols + 1
	}
	if a.ViewCol < 0 {
		a.ViewCol = 0
	}
	if a.ViewCol >= len(a.ColWidths) {
		if len(a.ColWidths) == 0 {
			a.ViewCol = 0
		} else {
			a.ViewCol = len(a.ColWidths) - 1
		}
	}

	if a.CurRow < a.ViewRow {
		a.ViewRow = a.CurRow
	} else if a.CurRow >= a.ViewRow+visibleRows {
		a.ViewRow = a.CurRow - visibleRows + 1
	}
	if a.ViewRow < 0 {
		a.ViewRow = 0
	}
	if a.ViewRow >= len(a.RowHeights) {
		if len(a.RowHeights) == 0 {
			a.ViewRow = 0
		} else {
			a.ViewRow = len(a.RowHeights) - 1
		}
	}
}

// ----------------------------- Misc -----------------------------

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
