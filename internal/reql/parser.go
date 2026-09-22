package reql

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ReQL term type IDs (matching RethinkDB protocol)
const (
	TermDATUM            = 1
	TermMAKE_ARRAY       = 2
	TermMAKE_OBJ         = 3
	TermVAR              = 10
	TermJAVASCRIPT       = 11
	TermUUID             = 169
	TermHTTP             = 153
	TermERROR            = 12
	TermIMPLICIT_VAR     = 110 // r.row
	TermDB               = 14
	TermTABLE            = 10
	TermGET              = 70
	TermGET_ALL          = 78
	TermFILTER           = 39
	TermORDER_BY         = 41
	TermLIMIT            = 42
	TermSKIP             = 43
	TermCOUNT            = 86
	TermSUM              = 87
	TermAVG              = 88
	TermMIN              = 89
	TermMAX              = 90
	TermGROUP            = 91
	TermUNGROUP          = 92
	TermREDUCE           = 93
	TermMAP              = 40
	TermINSERT           = 17
	TermUPDATE           = 18
	TermDELETE           = 19
	TermREPLACE          = 20
	TermHAS_FIELDS       = 33
	TermWITHOUT          = 34
	TermMERGE            = 36
	TermBETWEEN          = 172
	TermCHANGES          = 152
	TermINDEX_CREATE     = 75
	TermINDEX_DROP       = 76
	TermINDEX_LIST       = 77
	TermDB_CREATE        = 57
	TermDB_DROP          = 58
	TermDB_LIST          = 59
	TermTABLE_CREATE     = 60
	TermTABLE_DROP       = 61
	TermTABLE_LIST       = 62
	TermBRANCH           = 15
	TermOR               = 7
	TermAND              = 8
	TermNOT              = 9
	TermEQ               = 4
	TermNE               = 5
	TermLT               = 6
	TermLE               = 73
	TermGT               = 74
	TermGE               = 71
	TermADD              = 2
	TermSUB              = 3
	TermMUL              = 4
	TermDIV              = 5
	TermMOD              = 6
	TermAPPEND           = 94
	TermPREPEND          = 95
	TermDIFFERENCE       = 96
	TermSET_INSERT       = 127
	TermSET_INTERSECTION = 128
	TermSET_UNION        = 129
	TermSET_DIFFERENCE   = 130
	TermTYPE_OF          = 97
	TermTO_JSON_STRING   = 173
	TermDATE             = 180
	TermTIME             = 181
	TermEPOCH_TIME       = 182
	TermISO8601          = 183
	TermIN_DISTINCT      = 184
	TermDISTINCT         = 44
	TermPLUCK            = 45
	TermLITERAL          = 132
)

// methodToTerm maps ReQL method names to their term type IDs
var methodToTerm = map[string]int{
	"table":           TermTABLE,
	"db":              TermDB,
	"get":             TermGET,
	"getAll":          TermGET_ALL,
	"filter":          TermFILTER,
	"orderBy":         TermORDER_BY,
	"limit":           TermLIMIT,
	"skip":            TermSKIP,
	"count":           TermCOUNT,
	"sum":             TermSUM,
	"avg":             TermAVG,
	"min":             TermMIN,
	"max":             TermMAX,
	"group":           TermGROUP,
	"ungroup":         TermUNGROUP,
	"reduce":          TermREDUCE,
	"map":             TermMAP,
	"insert":          TermINSERT,
	"update":          TermUPDATE,
	"delete":          TermDELETE,
	"replace":         TermREPLACE,
	"hasFields":       TermHAS_FIELDS,
	"without":         TermWITHOUT,
	"merge":           TermMERGE,
	"between":         TermBETWEEN,
	"changes":         TermCHANGES,
	"indexCreate":     TermINDEX_CREATE,
	"indexDrop":       TermINDEX_DROP,
	"indexList":       TermINDEX_LIST,
	"dbCreate":        TermDB_CREATE,
	"dbDrop":          TermDB_DROP,
	"dbList":          TermDB_LIST,
	"tableCreate":     TermTABLE_CREATE,
	"tableDrop":       TermTABLE_DROP,
	"tableList":       TermTABLE_LIST,
	"branch":          TermBRANCH,
	"or":              TermOR,
	"and":             TermAND,
	"not":             TermNOT,
	"eq":              TermEQ,
	"ne":              TermNE,
	"lt":              TermLT,
	"le":              TermLE,
	"gt":              TermGT,
	"ge":              TermGE,
	"add":             TermADD,
	"sub":             TermSUB,
	"mul":             TermMUL,
	"div":             TermDIV,
	"mod":             TermMOD,
	"append":          TermAPPEND,
	"prepend":         TermPREPEND,
	"difference":      TermDIFFERENCE,
	"setInsert":       TermSET_INSERT,
	"setIntersection": TermSET_INTERSECTION,
	"setUnion":        TermSET_UNION,
	"setDifference":   TermSET_DIFFERENCE,
	"typeOf":          TermTYPE_OF,
	"toJsonString":    TermTO_JSON_STRING,
	"date":            TermDATE,
	"time":            TermTIME,
	"epochTime":       TermEPOCH_TIME,
	"iso8601":         TermISO8601,
	"in":              TermIN_DISTINCT,
	"distinct":        TermDISTINCT,
	"pluck":           TermPLUCK,
	"literal":         TermLITERAL,
	"contains":        98, // CONTAINS
	"isEmpty":         130,
	"keys":            185,
	"values":          186,
	"object":          1,
	"coerceTo":        129,
	"do":              11,
	"default":         13,
	"nth":             72,
	"offsetsOf":       187,
	"isEmpty2":        130,
	"union":           46,
	"sample":          47,
	"innerJoin":       48,
	"outerJoin":       49,
	"eqJoin":          50,
	"zip":             51,
	"range":           173,
	"args":            133,
	"json":            174,
	"http":            TermHTTP,
	"error":           TermERROR,
	"random":          175,
	"now":             176,
	"round":           177,
	"floor":           178,
	"ceil":            179,
	"toEpochTime":     188,
	"toISO8601":       189,
	"during":          190,
	"monday":          200,
	"tuesday":         201,
	"wednesday":       202,
	"thursday":        203,
	"friday":          204,
	"saturday":        205,
	"sunday":          206,
	"january":         207,
	"february":        208,
	"march":           209,
	"april":           210,
	"may":             211,
	"june":            212,
	"july":            213,
	"august":          214,
	"september":       215,
	"october":         216,
	"november":        217,
	"december":        218,
	"minval":          219,
	"maxval":          220,
	"geometry":        221,
	"point":           222,
	"line":            223,
	"polygon":         224,
	"distance":        225,
	"intersects":      226,
	"includes":        227,
	"fill":            228,
	"getIntersecting": 229,
	"polygonSub":      230,
	"toGeojson":       231,
	"info":            232,
	"match":           233,
	"split":           234,
	"upcase":          235,
	"downcase":        236,
	"sample2":         47,
	"unfold":          237,
	"groupMap":        238,
	"forEach":         239,
	"wait":            240,
	"reconfigure":     241,
	"rebalance":       242,
	"sync":            243,
	"grant":           244,
	"status":          137,
	"config":          245,
	"cache":           246,
}

// ParseReQL parses a ReQL string expression into a term array.
// Supports: r.table("name"), r.db("name").table("name"), method chains, etc.
func ParseReQL(input string) (interface{}, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty query")
	}

	p := &parser{input: input, pos: 0}
	result, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.input) {
		remaining := strings.TrimSpace(p.input[p.pos:])
		if remaining != "" {
			return nil, fmt.Errorf("unexpected trailing content: %s", remaining)
		}
	}
	return result, nil
}

type parser struct {
	input string
	pos   int
}

func (p *parser) skipWhitespace() {
	for p.pos < len(p.input) && unicode.IsSpace(rune(p.input[p.pos])) {
		p.pos++
	}
}

func (p *parser) peek() byte {
	p.skipWhitespace()
	if p.pos >= len(p.input) {
		return 0
	}
	return p.input[p.pos]
}

func (p *parser) advance() byte {
	ch := p.input[p.pos]
	p.pos++
	return ch
}

func (p *parser) expect(ch byte) error {
	p.skipWhitespace()
	if p.pos >= len(p.input) {
		return fmt.Errorf("expected '%c' at end of input", ch)
	}
	if p.input[p.pos] != ch {
		return fmt.Errorf("expected '%c' at position %d, got '%c'", ch, p.pos, p.input[p.pos])
	}
	p.pos++
	return nil
}

func (p *parser) parseExpression() (interface{}, error) {
	p.skipWhitespace()

	// Handle r.xxx(...) chains
	if p.pos+1 < len(p.input) && p.input[p.pos] == 'r' && p.input[p.pos+1] == '.' {
		return p.parseRChain()
	}

	// Handle parenthesized expression
	if p.peek() == '(' {
		p.advance()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		p.expect(')')
		return expr, nil
	}

	// Handle literal values
	return p.parseLiteral()
}

func (p *parser) parseRChain() (interface{}, error) {
	// Consume 'r.'
	p.pos += 2 // skip 'r.'
	p.skipWhitespace()

	// Parse method name
	method, err := p.parseIdentifier()
	if err != nil {
		return nil, fmt.Errorf("expected method name after 'r.': %w", err)
	}

	// Parse arguments
	args, err := p.parseArgList()
	if err != nil {
		return nil, err
	}

	termID := methodToTerm[method]
	if termID == 0 {
		return nil, fmt.Errorf("unknown ReQL method: r.%s", method)
	}

	// Build term array: [termID, ...args]
	term := make([]interface{}, 0, len(args)+1)
	term = append(term, termID)
	term = append(term, args...)

	// Check for method chaining (.method(...))
	for p.peek() == '.' {
		p.advance() // consume '.'
		p.skipWhitespace()

		nextMethod, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}

		nextArgs, err := p.parseArgList()
		if err != nil {
			return nil, err
		}

		nextTermID := methodToTerm[nextMethod]
		if nextTermID == 0 {
			return nil, fmt.Errorf("unknown ReQL method: .%s", nextMethod)
		}

		// Build new term: [nextTermID, previousTerm, ...nextArgs]
		newTerm := make([]interface{}, 0, len(nextArgs)+2)
		newTerm = append(newTerm, nextTermID)
		newTerm = append(newTerm, term)
		newTerm = append(newTerm, nextArgs...)
		term = newTerm
	}

	return term, nil
}

func (p *parser) parseIdentifier() (string, error) {
	p.skipWhitespace()
	start := p.pos
	for p.pos < len(p.input) && (unicode.IsLetter(rune(p.input[p.pos])) || unicode.IsDigit(rune(p.input[p.pos])) || p.input[p.pos] == '_') {
		p.pos++
	}
	if p.pos == start {
		return "", fmt.Errorf("expected identifier at position %d", p.pos)
	}
	return p.input[start:p.pos], nil
}

func (p *parser) parseArgList() ([]interface{}, error) {
	p.skipWhitespace()
	if p.peek() != '(' {
		return nil, fmt.Errorf("expected '(' at position %d", p.pos)
	}
	p.advance() // consume '('

	var args []interface{}
	p.skipWhitespace()

	// Handle empty arg list
	if p.peek() == ')' {
		p.advance()
		return args, nil
	}

	for {
		p.skipWhitespace()
		arg, err := p.parseArgValue()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)

		p.skipWhitespace()
		if p.peek() == ',' {
			p.advance()
			continue
		}
		break
	}

	p.expect(')')
	return args, nil
}

func (p *parser) parseArgValue() (interface{}, error) {
	p.skipWhitespace()
	ch := p.peek()

	switch {
	case ch == '"':
		return p.parseString()
	case ch == '\'':
		return p.parseSingleQuotedString()
	case ch == '{':
		return p.parseObject()
	case ch == '[':
		return p.parseArray()
	case ch == '(':
		p.advance()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		p.expect(')')
		return expr, nil
	case ch == '-' || (ch >= '0' && ch <= '9'):
		return p.parseNumber()
	case ch == 't' || ch == 'f':
		return p.parseBoolean()
	case ch == 'n':
		return p.parseNull()
	case ch == 'r' && p.pos+1 < len(p.input) && p.input[p.pos+1] == '.':
		return p.parseRChain()
	case ch == 'r' && p.pos+2 < len(p.input) && p.input[p.pos+1] == 'o' && p.input[p.pos+2] == 'w':
		// r.row
		p.pos += 3
		return map[string]interface{}{"$reql_type$": "BINARY", "data": []interface{}{TermIMPLICIT_VAR}}, nil
	default:
		// Try to parse as identifier (could be r.row, true, false, null)
		ident, err := p.parseIdentifier()
		if err != nil {
			return nil, fmt.Errorf("unexpected character '%c' at position %d", ch, p.pos)
		}
		switch ident {
		case "true":
			return true, nil
		case "false":
			return false, nil
		case "null":
			return nil, nil
		case "r":
			// Could be start of r.xxx
			if p.peek() == '.' {
				return p.parseRChain()
			}
			return nil, fmt.Errorf("unexpected 'r' without dot")
		default:
			return nil, fmt.Errorf("unexpected identifier: %s", ident)
		}
	}
}

func (p *parser) parseString() (string, error) {
	p.advance() // consume opening quote
	var sb strings.Builder
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '\\' {
			p.pos++
			if p.pos >= len(p.input) {
				return "", fmt.Errorf("unterminated string escape")
			}
			escaped := p.input[p.pos]
			switch escaped {
			case '"':
				sb.WriteByte('"')
			case '\\':
				sb.WriteByte('\\')
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case 'r':
				sb.WriteByte('\r')
			default:
				sb.WriteByte('\\')
				sb.WriteByte(escaped)
			}
			p.pos++
		} else if ch == '"' {
			p.pos++
			return sb.String(), nil
		} else {
			sb.WriteByte(ch)
			p.pos++
		}
	}
	return "", fmt.Errorf("unterminated string")
}

func (p *parser) parseSingleQuotedString() (string, error) {
	p.advance() // consume opening quote
	var sb strings.Builder
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '\\' {
			p.pos++
			if p.pos >= len(p.input) {
				return "", fmt.Errorf("unterminated string escape")
			}
			sb.WriteByte(p.input[p.pos])
			p.pos++
		} else if ch == '\'' {
			p.pos++
			return sb.String(), nil
		} else {
			sb.WriteByte(ch)
			p.pos++
		}
	}
	return "", fmt.Errorf("unterminated string")
}

func (p *parser) parseNumber() (interface{}, error) {
	start := p.pos
	if p.peek() == '-' {
		p.advance()
	}
	for p.pos < len(p.input) && ((p.input[p.pos] >= '0' && p.input[p.pos] <= '9') || p.input[p.pos] == '.') {
		p.pos++
	}
	numStr := p.input[start:p.pos]
	if strings.Contains(numStr, ".") {
		f, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number: %s", numStr)
		}
		return f, nil
	}
	i, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		f, err2 := strconv.ParseFloat(numStr, 64)
		if err2 != nil {
			return nil, fmt.Errorf("invalid number: %s", numStr)
		}
		return f, nil
	}
	return float64(i), nil
}

func (p *parser) parseBoolean() (interface{}, error) {
	if p.pos+4 <= len(p.input) && p.input[p.pos:p.pos+4] == "true" {
		p.pos += 4
		return true, nil
	}
	if p.pos+5 <= len(p.input) && p.input[p.pos:p.pos+5] == "false" {
		p.pos += 5
		return false, nil
	}
	return nil, fmt.Errorf("expected boolean at position %d", p.pos)
}

func (p *parser) parseNull() (interface{}, error) {
	if p.pos+4 <= len(p.input) && p.input[p.pos:p.pos+4] == "null" {
		p.pos += 4
		return nil, nil
	}
	return nil, fmt.Errorf("expected null at position %d", p.pos)
}

func (p *parser) parseObject() (map[string]interface{}, error) {
	p.advance() // consume '{'
	result := make(map[string]interface{})
	p.skipWhitespace()

	if p.peek() == '}' {
		p.advance()
		return result, nil
	}

	for {
		p.skipWhitespace()
		// Parse key
		var key string
		var err error
		ch := p.peek()
		if ch == '"' {
			key, err = p.parseString()
		} else if ch == '\'' {
			key, err = p.parseSingleQuotedString()
		} else {
			key, err = p.parseIdentifier()
		}
		if err != nil {
			return nil, fmt.Errorf("expected object key: %w", err)
		}

		p.skipWhitespace()
		p.expect(':')

		p.skipWhitespace()
		value, err := p.parseArgValue()
		if err != nil {
			return nil, err
		}
		result[key] = value

		p.skipWhitespace()
		if p.peek() == ',' {
			p.advance()
			continue
		}
		break
	}

	p.expect('}')
	return result, nil
}

func (p *parser) parseArray() ([]interface{}, error) {
	p.advance() // consume '['
	var result []interface{}
	p.skipWhitespace()

	if p.peek() == ']' {
		p.advance()
		return result, nil
	}

	for {
		p.skipWhitespace()
		value, err := p.parseArgValue()
		if err != nil {
			return nil, err
		}
		result = append(result, value)

		p.skipWhitespace()
		if p.peek() == ',' {
			p.advance()
			continue
		}
		break
	}

	p.expect(']')
	return result, nil
}

func (p *parser) parseLiteral() (interface{}, error) {
	p.skipWhitespace()
	ch := p.peek()

	switch {
	case ch == '"':
		return p.parseString()
	case ch == '\'':
		return p.parseSingleQuotedString()
	case ch == '{':
		return p.parseObject()
	case ch == '[':
		return p.parseArray()
	case ch == '-' || (ch >= '0' && ch <= '9'):
		return p.parseNumber()
	case ch == 't' || ch == 'f':
		return p.parseBoolean()
	case ch == 'n':
		return p.parseNull()
	default:
		ident, err := p.parseIdentifier()
		if err != nil {
			return nil, fmt.Errorf("unexpected character at position %d: '%c'", p.pos, ch)
		}
		switch ident {
		case "true":
			return true, nil
		case "false":
			return false, nil
		case "null":
			return nil, nil
		default:
			return nil, fmt.Errorf("unexpected identifier: %s", ident)
		}
	}
}
