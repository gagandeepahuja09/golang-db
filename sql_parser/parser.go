package sqlparser

import (
	"fmt"
)

// not handling \n for now.
const (
	KeywordCreate                = "CREATE"
	KeywordTable                 = "TABLE"
	KeywordPrimary               = "PRIMARY"
	KeywordKey                   = "KEY"
	SymbolOpenRoundBracket       = "("
	SymbolClosedRoundBracket     = ")"
	ExpectedSyntaxCmdCreateTable = "expected more arguments in CREATE TABLE command. Expected syntax: CREATE TABLE name_of_table ( c1 int, c2 bool, c3 string )"
)

type Parser struct {
	tokeniser    *Tokeniser
	currentToken Token
}

func NewParser(input string) *Parser {
	t := NewTokeniser(input)
	currentToken := t.NextToken()
	return &Parser{
		tokeniser:    t,
		currentToken: currentToken,
	}
}

func (p *Parser) consume(tt TokenType, expectedVal string) error {
	if p.currentToken.Type != tt || (expectedVal != "" && p.currentToken.Value != expectedVal) {
		return fmt.Errorf(
			"syntax error: expected %s %q, got %s %q",
			tt, expectedVal, p.currentToken.Type, p.currentToken.Value)
	}
	p.currentToken = p.tokeniser.NextToken()
	return nil
}

func getDataTypeFromString(columnType string) (DataType, error) {
	switch columnType {
	case "INT":
		return Int, nil
	case "STRING":
		return String, nil
	case "BOOL":
		return Bool, nil
	}
	return DataType(25), fmt.Errorf("data type '%s' not found. expected one of INT, STRING, BOOL",
		columnType)
}

// todo: add support for storing primary key index also.
func (p *Parser) ParseCreateTable() (*CreateTable, error) {
	if err := p.consume(KEYWORD, KeywordCreate); err != nil {
		return nil, err
	}
	if err := p.consume(KEYWORD, KeywordTable); err != nil {
		return nil, err
	}
	tableName := p.currentToken.Value
	if err := p.consume(IDENTIFIER, ""); err != nil {
		return nil, err
	}

	if err := p.consume(SYMBOL, SymbolOpenRoundBracket); err != nil {
		return nil, err
	}

	columnDetails := []Column{}
	pkColumn := ""
	for p.currentToken.Value != SymbolClosedRoundBracket {
		if p.currentToken.Value == "," {
			p.consume(SYMBOL, ",")
		}

		// todo: add an error for maximum columns limit

		if p.currentToken.Value == KeywordPrimary {
			if err := p.consume(KEYWORD, KeywordPrimary); err != nil {
				return nil, err
			}
			if err := p.consume(KEYWORD, KeywordKey); err != nil {
				return nil, err
			}
			if err := p.consume(SYMBOL, SymbolOpenRoundBracket); err != nil {
				return nil, err
			}
			// only primary key with one column supported as of now
			pkColumn = p.currentToken.Value
			if err := p.consume(IDENTIFIER, ""); err != nil {
				return nil, err
			}
			if err := p.consume(SYMBOL, SymbolClosedRoundBracket); err != nil {
				return nil, err
			}
			continue
		}

		columnName := p.currentToken.Value
		if err := p.consume(IDENTIFIER, ""); err != nil {
			return nil, err
		}
		columnType := p.currentToken.Value
		if err := p.consume(IDENTIFIER, ""); err != nil {
			return nil, err
		}
		dataType, err := getDataTypeFromString(columnType)
		if err != nil {
			return nil, err
		}
		columnDetails = append(columnDetails, Column{
			ColumnName: columnName,
			DataType:   dataType,
		})
	}

	if len(columnDetails) == 0 {
		return nil, fmt.Errorf("expected atleast one column detail, found none")
	}

	pkColumnPosition := 0
	if pkColumn != "" {
		for i, col := range columnDetails {
			if col.ColumnName == pkColumn {
				pkColumnPosition = i
			}
		}
		if pkColumnPosition == 0 {
			return nil, fmt.Errorf("primary key column '%s' not found", pkColumn)
		}
	}

	if err := p.consume(SYMBOL, SymbolClosedRoundBracket); err != nil {
		return nil, err
	}

	return &CreateTable{
		TableName:                tableName,
		ColumnDetails:            columnDetails,
		PrimaryKeyColumnPosition: pkColumnPosition,
	}, nil
}
