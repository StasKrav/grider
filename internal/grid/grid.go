package grid

import (
    "fmt"
    "strconv"
    "strings"
)

// Cell represents a single cell content.
type Cell struct {
    Text string
}

// ColToName: 0 -> A, 25 -> Z, 26 -> AA and so on
func ColToName(col int) string {
    if col < 0 {
        return "?"
    }
    result := ""
    n := col + 1
    for n > 0 {
        n--
        result = string(rune('A'+(n%26))) + result
        n /= 26
    }
    return result
}

// ColRowToName builds cell name from 0-based col,row -> e.g., col 0,row0 -> "A1"
func ColRowToName(col, row int) string {
    n := col + 1
    letters := ""
    for n > 0 {
        rem := (n-1)%26
        letters = string('A'+rem) + letters
        n = (n-1) / 26
    }
    return fmt.Sprintf("%s%d", letters, row+1)
}

// ParseCellRef parses names like A1, AA10 returning 0-based (row, col)
// Accepts sheet prefixes like Sheet!A1 and removes $ signs.
func ParseCellRef(name string) (int, int, bool) {
    name = strings.TrimSpace(name)
    // remove sheet! prefix if present
    if idx := strings.LastIndex(name, "!"); idx != -1 {
        name = strings.TrimSpace(name[idx+1:])
    }
    // remove $ from absolute refs
    name = strings.ReplaceAll(name, "$", "")
    if name == "" {
        return 0, 0, false
    }

    i := 0
    for i < len(name) && isLetter(name[i]) {
        i++
    }
    if i == 0 || i >= len(name) {
        return 0, 0, false
    }
    colPart := strings.ToUpper(name[:i])
    rowPart := name[i:]
    col := 0
    for j := 0; j < len(colPart); j++ {
        col = col*26 + int(colPart[j]-'A') + 1
    }
    col = col - 1
    rowNum, err := strconv.Atoi(rowPart)
    if err != nil {
        return 0, 0, false
    }
    row := rowNum - 1
    if row < 0 || col < 0 {
        return 0, 0, false
    }
    return row, col, true
}

func isLetter(b byte) bool {
    return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}
func isDigit(b byte) bool {
    return (b >= '0' && b <= '9')
}
