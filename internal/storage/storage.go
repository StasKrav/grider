package storage

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"sheet/internal/grid"
)

// Document представляет всю структуру документа
type Document struct {
	Grid       map[string]grid.Cell `json:"grid"`
	ColWidths  []int                `json:"col_widths"`
	RowHeights []int                `json:"row_heights"`
	// Добавим другие поля документа по мере необходимости
}

// Вспомогательные функции для преобразования ключей
func keyToString(key [2]int) string {
	return fmt.Sprintf("%d,%d", key[0], key[1])
}

func stringToKey(s string) ([2]int, error) {
	var key [2]int
	_, err := fmt.Sscanf(s, "%d,%d", &key[0], &key[1])
	return key, err
}

// SaveCSV writes grid to CSV file
func SaveCSV(g map[[2]int]grid.Cell, filename string) error {
	// Создаем путь к файлу в директории documents
	docPath := filepath.Join("documents", filename)

	maxR, maxC := -1, -1
	for k := range g {
		if k[0] > maxR {
			maxR = k[0]
		}
		if k[1] > maxC {
			maxC = k[1]
		}
	}
	if maxR < 0 || maxC < 0 {
		f, err := os.Create(docPath)
		if err != nil {
			return err
		}
		f.Close()
		return nil
	}
	out := make([][]string, maxR+1)
	for r := 0; r <= maxR; r++ {
		row := make([]string, maxC+1)
		for c := 0; c <= maxC; c++ {
			if cell, ok := g[[2]int{r, c}]; ok {
				row[c] = cell.Text
			} else {
				row[c] = ""
			}
		}
		out[r] = row
	}
	f, err := os.Create(docPath)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.WriteAll(out); err != nil {
		return fmt.Errorf("error writing CSV: %w", err)
	}
	w.Flush()
	return nil
}

// ConvertGridToDocumentGrid преобразует map[[2]int]grid.Cell в map[string]grid.Cell
func ConvertGridToDocumentGrid(g map[[2]int]grid.Cell) map[string]grid.Cell {
	result := make(map[string]grid.Cell)
	for k, v := range g {
		result[keyToString(k)] = v
	}
	return result
}

// ConvertDocumentGridToGrid преобразует map[string]grid.Cell в map[[2]int]grid.Cell
func ConvertDocumentGridToGrid(g map[string]grid.Cell) (map[[2]int]grid.Cell, error) {
	result := make(map[[2]int]grid.Cell)
	for k, v := range g {
		key, err := stringToKey(k)
		if err != nil {
			return nil, fmt.Errorf("error parsing key %s: %w", k, err)
		}
		result[key] = v
	}
	return result, nil
}

// LoadCSV loads CSV into grid (overwrites). Returns grid map, max row index, max col index, error.
func LoadCSV(filename string) (map[[2]int]grid.Cell, int, int, error) {
	// Создаем путь к файлу в директории documents
	docPath := filepath.Join("documents", filename)

	f, err := os.Open(docPath)
	if err != nil {
		return nil, -1, -1, err
	}
	defer f.Close()
	r := csv.NewReader(bufio.NewReader(f))
	records, err := r.ReadAll()
	if err != nil {
		return nil, -1, -1, err
	}
	g := map[[2]int]grid.Cell{}
	for rIdx, row := range records {
		for cIdx, val := range row {
			if val != "" {
				g[[2]int{rIdx, cIdx}] = grid.Cell{Text: val}
			}
		}
	}
	maxR, maxC := -1, -1
	for k := range g {
		if k[0] > maxR {
			maxR = k[0]
		}
		if k[1] > maxC {
			maxC = k[1]
		}
	}
	return g, maxR, maxC, nil
}

// SaveDocument сохраняет весь документ в JSON формате
func SaveDocument(grid map[[2]int]grid.Cell, colWidths []int, rowHeights []int, filename string) error {
	// Проверяем, есть ли у файла расширение .grider
	if filepath.Ext(filename) != ".grider" {
		filename += ".grider"
	}

	// Создаем путь к файлу в директории documents
	docPath := filepath.Join("documents", filename)

	// Преобразуем grid в формат, поддерживаемый JSON
	docGrid := ConvertGridToDocumentGrid(grid)

	doc := &Document{
		Grid:       docGrid,
		ColWidths:  colWidths,
		RowHeights: rowHeights,
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling document: %w", err)
	}

	err = os.WriteFile(docPath, data, 0644)
	if err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	return nil
}

// LoadDocument загружает документ из JSON формата
func LoadDocument(filename string) (map[[2]int]grid.Cell, []int, []int, error) {
	// Проверяем, есть ли у файла расширение .grider
	if filepath.Ext(filename) != ".grider" {
		filename += ".grider"
	}

	// Создаем путь к файлу в директории documents
	docPath := filepath.Join("documents", filename)

	data, err := os.ReadFile(docPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error reading file: %w", err)
	}

	var doc Document
	err = json.Unmarshal(data, &doc)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error unmarshaling document: %w", err)
	}

	// Преобразуем grid обратно в формат map[[2]int]grid.Cell
	grid, err := ConvertDocumentGridToGrid(doc.Grid)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error converting grid: %w", err)
	}

	return grid, doc.ColWidths, doc.RowHeights, nil
}
