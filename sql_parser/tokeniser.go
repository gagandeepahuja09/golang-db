package sqlparser

import (
	"fmt"
	"strings"
	"unicode"
)

type TokenType string

const (
	IDENTIFIER           TokenType = "IDENTIFIER"
	KEYWORD              TokenType = "KEYWORD"
	SYMBOL               TokenType = "SYMBOL"
	CONDITIONAL_OPERATOR TokenType = "CONDITIONAL_OPERATOR"
	EOF                  TokenType = "EOF"
)

var (
	symbols = fmt.Sprintf("%s%s%s%s%s",
		SymbolOpenRoundBracket,
		SymbolClosedRoundBracket,
		SymbolComma,
		SymbolSemiColon,
		SymbolStar)
	conditionalOperators = "<>="
)

var keywords = map[string]bool{
	KeywordCreate:  true,
	KeywordTable:   true,
	KeywordPrimary: true,
	KeywordKey:     true,
	KeywordInsert:  true,
	KeywordInto:    true,
	KeywordValues:  true,
	KeywordSelect:  true,
	KeywordFrom:    true,
	KeywordWhere:   true,
	KeywordAnd:     true,
}

type Token struct {
	Type  TokenType
	Value string
}

type Tokeniser struct {
	input []rune
	pos   int
}

func NewTokeniser(input string) *Tokeniser {
	return &Tokeniser{input: []rune(input)}
}

// todo: code might be stuck in loop if unsupported character is provided: non alpha-numeric
// OR operator. eg. even _ as of now
// start throwing an error as well for token
func (t *Tokeniser) NextToken() Token {
	for t.pos < len(t.input) && unicode.IsSpace(t.input[t.pos]) {
		t.pos++
	}

	if t.pos >= len(t.input) {
		return Token{Type: EOF}
	}

	ch := t.input[t.pos]

	if strings.ContainsRune(symbols, ch) {
		t.pos++
		return Token{Type: SYMBOL, Value: string(ch)}
	}

	if strings.ContainsRune(conditionalOperators, t.input[t.pos]) {
		start := t.pos
		for t.pos < len(t.input) && strings.ContainsRune(conditionalOperators, t.input[t.pos]) {
			t.pos++
		}
		return Token{Type: CONDITIONAL_OPERATOR, Value: string(t.input[start:t.pos])}
	} else {
		start := t.pos
		for t.pos < len(t.input) && (unicode.IsDigit(t.input[t.pos]) ||
			unicode.IsLetter(t.input[t.pos])) {
			t.pos++
		}

		val := string(t.input[start:t.pos])
		if keywords[strings.ToUpper(val)] {
			return Token{Type: KEYWORD, Value: val}
		}

		return Token{Type: IDENTIFIER, Value: val}
	}
}
