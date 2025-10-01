package storage

import (
    "bufio"
    "encoding/csv"
    "fmt"
    "os"

    "sheet/internal/grid"
)

// SaveCSV writes grid to CSV file
func SaveCSV(g map[[2]int]grid.Cell, filename string) error {
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
        f, err := os.Create(filename)
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
    f, err := os.Create(filename)
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

// LoadCSV loads CSV into grid (overwrites). Returns grid map, max row index, max col index, error.
func LoadCSV(filename string) (map[[2]int]grid.Cell, int, int, error) {
    f, err := os.Open(filename)
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
