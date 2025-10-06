package calc

import (
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
			case "AVERAGE":
				sum := 0.0
				count := 0.0
				p.skipSpaces()
				// empty args -> AVERAGE() == 0
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
									count += 1.0
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
						count += 1.0
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
				if count == 0 {
					return 0, ""
				}
				return sum / count, ""
			case "MIN":
				values := []float64{}
				p.skipSpaces()
				// empty args -> MIN() == 0
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
									values = append(values, v)
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
						values = append(values, v)
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
				if len(values) == 0 {
					return 0, ""
				}
				minVal := values[0]
				for _, v := range values {
					if v < minVal {
						minVal = v
					}
				}
				return minVal, ""
			case "MAX":
				values := []float64{}
				p.skipSpaces()
				// empty args -> MAX() == 0
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
									values = append(values, v)
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
						values = append(values, v)
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
				if len(values) == 0 {
					return 0, ""
				}
				maxVal := values[0]
				for _, v := range values {
					if v > maxVal {
						maxVal = v
					}
				}
				return maxVal, ""
			case "COUNT":
				count := 0.0
				p.skipSpaces()
				// empty args -> COUNT() == 0
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
									_, err := p.resolve(name)
									if err != "" && err != "#CYCLE" {
										// Пропускаем ячейки с ошибками
										continue
									}
									// Увеличиваем счетчик только для числовых значений
									count += 1.0
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
						_, err := sub.parseExpr()
						if err != "" {
							// Пропускаем аргументы с ошибками
							// Просто пропускаем этот аргумент и продолжаем
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
						sub.skipSpaces()
						if sub.pos < len(sub.input) {
							return 0, "#ERR"
						}
						count += 1.0
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
				return count, ""
			case "ROUND":
				p.skipSpaces()
				// ROUND requires at least one argument
				if p.pos >= len(p.input) || p.input[p.pos] == ')' {
					return 0, "#ERR"
				}

				// Parse the first argument (value to round)
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

				sub := parser{
					input:   argStr,
					pos:     0,
					resolve: p.resolve,
				}
				value, err := sub.parseExpr()
				if err != "" {
					return 0, err
				}
				sub.skipSpaces()
				if sub.pos < len(sub.input) {
					return 0, "#ERR"
				}

				// Check if there's a second argument (decimal places)
				decimalPlaces := 0.0
				if i < len(p.input) && p.input[i] == ',' {
					p.pos = i + 1
					p.skipSpaces()

					// Parse the second argument (decimal places)
					start = p.pos
					nest = 0
					i = p.pos
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
					argStr = strings.TrimSpace(p.input[start:i])
					if argStr == "" {
						return 0, "#ERR"
					}

					sub = parser{
						input:   argStr,
						pos:     0,
						resolve: p.resolve,
					}
					dp, err := sub.parseExpr()
					if err != "" {
						return 0, err
					}
					sub.skipSpaces()
					if sub.pos < len(sub.input) {
						return 0, "#ERR"
					}
					decimalPlaces = dp
				}

				// Move parser position to the closing parenthesis
				if i < len(p.input) && p.input[i] == ')' {
					p.pos = i + 1
				} else {
					return 0, "#ERR"
				}

				// Round the value
				multiplier := math.Pow(10, decimalPlaces)
				return math.Round(value*multiplier) / multiplier, ""
			case "IF":
				p.skipSpaces()
				// IF requires at least one argument
				if p.pos >= len(p.input) || p.input[p.pos] == ')' {
					return 0, "#ERR"
				}

				// Parse the first argument (condition)
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
				conditionStr := strings.TrimSpace(p.input[start:i])
				if conditionStr == "" {
					return 0, "#ERR"
				}

				sub := parser{
					input:   conditionStr,
					pos:     0,
					resolve: p.resolve,
				}
				condition, err := sub.parseExpr()
				if err != "" {
					return 0, err
				}
				sub.skipSpaces()
				if sub.pos < len(sub.input) {
					return 0, "#ERR"
				}

				// Check if there's a second argument (value if true)
				if i >= len(p.input) || p.input[i] != ',' {
					return 0, "#ERR"
				}

				p.pos = i + 1
				p.skipSpaces()

				// Parse the second argument (value if true)
				start = p.pos
				nest = 0
				i = p.pos
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
				trueValueStr := strings.TrimSpace(p.input[start:i])
				if trueValueStr == "" {
					return 0, "#ERR"
				}

				// Check if there's a third argument (value if false)
				var falseValueStr string
				if i < len(p.input) && p.input[i] == ',' {
					p.pos = i + 1
					p.skipSpaces()

					// Parse the third argument (value if false)
					start = p.pos
					nest = 0
					i = p.pos
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
					falseValueStr = strings.TrimSpace(p.input[start:i])
				} else {
					// Default false value is 0
					falseValueStr = "0"
				}

				// Move parser position to the closing parenthesis
				if i < len(p.input) && p.input[i] == ')' {
					p.pos = i + 1
				} else {
					return 0, "#ERR"
				}

				// Evaluate the appropriate branch
				var resultStr string
				if math.Abs(condition) > 1e-12 { // condition is true if not close to zero
					resultStr = trueValueStr
				} else {
					resultStr = falseValueStr
				}

				sub = parser{
					input:   resultStr,
					pos:     0,
					resolve: p.resolve,
				}
				result, err := sub.parseExpr()
				if err != "" {
					return 0, err
				}
				sub.skipSpaces()
				if sub.pos < len(sub.input) {
					return 0, "#ERR"
				}

				return result, ""
			case "AND":
				p.skipSpaces()
				// empty args -> AND() == 1 (true)
				if p.pos < len(p.input) && p.input[p.pos] == ')' {
					p.pos++
					return 1, ""
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

					sub := parser{
						input:   argStr,
						pos:     0,
						resolve: p.resolve,
					}
					value, err := sub.parseExpr()
					if err != "" {
						return 0, err
					}
					sub.skipSpaces()
					if sub.pos < len(sub.input) {
						return 0, "#ERR"
					}

					// If any argument is false (close to zero), return false
					if math.Abs(value) < 1e-12 {
						// Skip remaining arguments
						for i < len(p.input) && p.input[i] != ')' {
							i++
						}
						if i < len(p.input) && p.input[i] == ')' {
							p.pos = i + 1
						}
						return 0, ""
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
				return 1, "" // All arguments are true
			case "OR":
				p.skipSpaces()
				// empty args -> OR() == 0 (false)
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

					sub := parser{
						input:   argStr,
						pos:     0,
						resolve: p.resolve,
					}
					value, err := sub.parseExpr()
					if err != "" {
						return 0, err
					}
					sub.skipSpaces()
					if sub.pos < len(sub.input) {
						return 0, "#ERR"
					}

					// If any argument is true (not close to zero), return true
					if math.Abs(value) > 1e-12 {
						// Skip remaining arguments
						for i < len(p.input) && p.input[i] != ')' {
							i++
						}
						if i < len(p.input) && p.input[i] == ')' {
							p.pos = i + 1
						}
						return 1, ""
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
				return 0, "" // All arguments are false
			case "NOT":
				p.skipSpaces()
				// NOT requires exactly one argument
				if p.pos >= len(p.input) || p.input[p.pos] == ')' {
					return 0, "#ERR"
				}

				// Parse the argument
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

				sub := parser{
					input:   argStr,
					pos:     0,
					resolve: p.resolve,
				}
				value, err := sub.parseExpr()
				if err != "" {
					return 0, err
				}
				sub.skipSpaces()
				if sub.pos < len(sub.input) {
					return 0, "#ERR"
				}

				// Move parser position to the closing parenthesis
				if i < len(p.input) && p.input[i] == ')' {
					p.pos = i + 1
				} else {
					return 0, "#ERR"
				}

				// Return 1 if value is close to zero (false), 0 otherwise (true)
				if math.Abs(value) < 1e-12 {
					return 1, ""
				}
				return 0, ""
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
