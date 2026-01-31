package sqlparser

import (
	"strings"
	"unicode"
)

type TokenType string

const (
	IDENTIFIER TokenType = "IDENTIFIER"
	KEYWORD    TokenType = "KEYWORD"
	SYMBOL     TokenType = "SYMBOL"
	EOF        TokenType = "EOF"
)

const (
	symbols = "(),;"
)

var keywords = map[string]bool{
	"CREATE": true,
	"TABLE":  true,
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
