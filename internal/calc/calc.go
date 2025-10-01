package calc

import (
//    "fmt"
    "math"
    "strconv"
    "strings"

    "sheet/internal/grid"
)

// EvalExprForCell evaluates expr with a resolver callback.
// resolve(name) should return (value, errCode) where errCode is one of "", "#CYCLE", "#DIV/0", "#REF", "#ERR"
func EvalExprForCell(expr string, baseR, baseC int, resolve func(name string) (float64, string), visited map[[2]int]bool) (float64, string) {
    // The parser here is nearly identical to original, but uses the provided resolver
    p := parser{
        input:   expr,
        pos:     0,
        resolve: resolve,
    }

    val, err := p.parseExpr()
    if err != "" {
        return 0, err
    }
    p.skipSpaces()
    if p.pos < len(p.input) {
        return 0, "#ERR"
    }
    // Some additional sanity checks (avoid NaN/Inf leaking)
    if math.IsNaN(val) || math.IsInf(val, 0) {
        return 0, "#ERR"
    }
    return val, ""
}

type parser struct {
    input   string
    pos     int
    resolve func(name string) (float64, string)
}

func (p *parser) skipSpaces() {
    for p.pos < len(p.input) && (p.input[p.pos] == ' ' || p.input[p.pos] == '\t') {
        p.pos++
    }
}

func (p *parser) parseExpr() (float64, string) {
    return p.parseAddSub()
}

func (p *parser) parseAddSub() (float64, string) {
    val, err := p.parseMulDiv()
    if err != "" {
        return 0, err
    }
    for {
        p.skipSpaces()
        if p.pos >= len(p.input) {
            break
        }
        op := p.input[p.pos]
        if op != '+' && op != '-' {
            break
        }
        p.pos++
        right, rerr := p.parseMulDiv()
        if rerr != "" {
            return 0, rerr
        }
        if op == '+' {
            val = val + right
        } else {
            val = val - right
        }
    }
    return val, ""
}

func (p *parser) parseMulDiv() (float64, string) {
    val, err := p.parseFactor()
    if err != "" {
        return 0, err
    }
    for {
        p.skipSpaces()
        if p.pos >= len(p.input) {
            break
        }
        op := p.input[p.pos]
        if op != '*' && op != '/' {
            break
        }
        p.pos++
        right, rerr := p.parseFactor()
        if rerr != "" {
            return 0, rerr
        }
        if op == '*' {
            val = val * right
        } else {
            if math.Abs(right) < 1e-12 {
                return 0, "#DIV/0"
            }
            val = val / right
        }
    }
    return val, ""
}

func (p *parser) parseFactor() (float64, string) {
    p.skipSpaces()
    if p.pos < len(p.input) {
        ch := p.input[p.pos]
        if ch == '+' {
            p.pos++
            return p.parseFactor()
        }
        if ch == '-' {
            p.pos++
            v, err := p.parseFactor()
            if err != "" {
                return 0, err
            }
            return -v, ""
        }
    }
    return p.parsePrimary()
}

func (p *parser) parsePrimary() (float64, string) {
    p.skipSpaces()
    if p.pos >= len(p.input) {
        return 0, "#ERR"
    }
    ch := p.input[p.pos]
    if ch == '(' {
        p.pos++
        v, err := p.parseExpr()
        if err != "" {
            return 0, err
        }
        p.skipSpaces()
        if p.pos >= len(p.input) || p.input[p.pos] != ')' {
            return 0, "#ERR"
        }
        p.pos++
        return v, ""
    }
    // number
    if isDigit(ch) || ch == '.' {
        start := p.pos
        j := p.pos
        seenDot := false
        seenE := false
        for j < len(p.input) {
            c := p.input[j]
            if c >= '0' && c <= '9' {
                j++
                continue
            }
            if c == '.' {
                if seenDot || seenE {
                    break
                }
                seenDot = true
                j++
                continue
            }
            if c == 'e' || c == 'E' {
                if seenE {
                    break
                }
                seenE = true
                j++
                if j < len(p.input) && (p.input[j] == '+' || p.input[j] == '-') {
                    j++
                }
                continue
            }
            break
        }
        if j == start {
            return 0, "#ERR"
        }
        numStr := p.input[start:j]
        p.pos = j
        v, err := strconv.ParseFloat(numStr, 64)
        if err != nil {
            return 0, "#ERR"
        }
        return v, ""
    }

    // identifier: letters then either '(' (function) or digits (cell ref)
    if isLetter(ch) {
        startLetters := p.pos
        j := p.pos
        for j < len(p.input) && isLetter(p.input[j]) {
            j++
        }
        letters := p.input[startLetters:j]
        p.pos = j
        p.skipSpaces()

        // function call: LETTERS(...)
        if p.pos < len(p.input) && p.input[p.pos] == '(' {
            p.pos++
            funcName := strings.ToUpper(letters)
            switch funcName {
            case "SUM":
                sum := 0.0
                p.skipSpaces()
                // empty args -> SUM() == 0
                if p.pos < len(p.input) && p.input[p.pos] == ')' {
                    p.pos++
                    return 0, ""
                }
                for {
                    // extract argument substring up to comma or closing paren at top nesting
                    start := p.pos
                    nest := 0
                    i := p.pos
                    for i < len(p.input) {
                        c := p.input[i]
                        if c == '(' {
                            nest++
                        } else if c == ')' {
                            if nest == 0 {
                                break
                            }
                            nest--
                        } else if c == ',' && nest == 0 {
                            break
                        }
                        i++
                    }
                    if i > len(p.input) {
                        return 0, "#ERR"
                    }
                    argStr := strings.TrimSpace(p.input[start:i])
                    if argStr == "" {
                        return 0, "#ERR"
                    }

                    // try range syntax LEFT:RIGHT (only top-level colon)
                    isRange := false
                    if strings.Contains(argStr, ":") {
                        parts := strings.SplitN(argStr, ":", 2)
                        left := strings.TrimSpace(parts[0])
                        right := strings.TrimSpace(parts[1])
                        r1, c1, ok1 := grid.ParseCellRef(left)
                        r2, c2, ok2 := grid.ParseCellRef(right)
                        if ok1 && ok2 {
                            rmin := minInt(r1, r2)
                            rmax := maxInt(r1, r2)
                            cmin := minInt(c1, c2)
                            cmax := maxInt(c1, c2)
                            for rr := rmin; rr <= rmax; rr++ {
                                for cc := cmin; cc <= cmax; cc++ {
                                    name := grid.ColRowToName(cc, rr)
                                    v, err := p.resolve(name)
                                    if err != "" {
                                        return 0, err
                                    }
                                    sum += v
                                }
                            }
                            isRange = true
                        }
                    }

                    if !isRange {
                        sub := parser{
                            input:   argStr,
                            pos:     0,
                            resolve: p.resolve,
                        }
                        v, err := sub.parseExpr()
                        if err != "" {
                            return 0, err
                        }
                        sub.skipSpaces()
                        if sub.pos < len(sub.input) {
                            return 0, "#ERR"
                        }
                        sum += v
                    }

                    if i >= len(p.input) {
                        return 0, "#ERR"
                    }
                    if p.input[i] == ',' {
                        p.pos = i + 1
                        p.skipSpaces()
                        continue
                    }
                    if p.input[i] == ')' {
                        p.pos = i + 1
                        break
                    }
                    return 0, "#ERR"
                }
                return sum, ""
            default:
                return 0, "#ERR"
            }
        }

        // otherwise treat as cell reference: need digits after letters
        k := j
        for k < len(p.input) && isDigit(p.input[k]) {
            k++
        }
        if k == j {
            p.pos = k
            return 0, "#REF"
        }
        name := p.input[startLetters:k]
        p.pos = k
        if p.resolve == nil {
            return 0, "#ERR"
        }
        val, err := p.resolve(name)
        return val, err
    }

    return 0, "#ERR"
}

func isLetter(b byte) bool {
    return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}
func isDigit(b byte) bool {
    return (b >= '0' && b <= '9')
}

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
