package sqlparser

import (
	"fmt"
)

const (
	KeywordCreate            = "CREATE"
	KeywordInsert            = "INSERT"
	KeywordInto              = "INTO"
	KeywordTable             = "TABLE"
	KeywordValues            = "VALUES"
	KeywordPrimary           = "PRIMARY"
	KeywordKey               = "KEY"
	SymbolOpenRoundBracket   = "("
	SymbolClosedRoundBracket = ")"
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

func (p *Parser) parsePrimaryKeyColumn() (string, error) {
	if err := p.consume(KEYWORD, KeywordPrimary); err != nil {
		return "", err
	}
	if err := p.consume(KEYWORD, KeywordKey); err != nil {
		return "", err
	}
	if err := p.consume(SYMBOL, SymbolOpenRoundBracket); err != nil {
		return "", err
	}
	// only primary key with one column supported as of now
	pkColumn := p.currentToken.Value
	if err := p.consume(IDENTIFIER, ""); err != nil {
		return "", err
	}
	err := p.consume(SYMBOL, SymbolClosedRoundBracket)
	return pkColumn, err
}

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
			var err error
			pkColumn, err = p.parsePrimaryKeyColumn()
			if err != nil {
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

// INSERT INTO has 2 syntaxes:
// 1. All column values provided
// INSERT INTO table_name VALUES (all values ...)
// 2. Specific column values provided
// INSERT INTO table_name (col1, col2, col3) VALUES (only the provided column values ...)
// We are supporting only 2 for now.
func (p *Parser) ParseInsertIntoTable() (*InsertIntoTable, error) {
	if err := p.consume(KEYWORD, KeywordInsert); err != nil {
		return nil, err
	}
	if err := p.consume(KEYWORD, KeywordInto); err != nil {
		return nil, err
	}
	tableName := p.currentToken.Value
	if err := p.consume(IDENTIFIER, ""); err != nil {
		return nil, err
	}

	if err := p.consume(KEYWORD, KeywordValues); err != nil {
		return nil, err
	}
	if err := p.consume(SYMBOL, SymbolOpenRoundBracket); err != nil {
		return nil, err
	}

	columnValues := []string{}
	for p.currentToken.Value != SymbolClosedRoundBracket {
		if p.currentToken.Value == "," {
			p.consume(SYMBOL, ",")
		}

		columnValue := p.currentToken.Value
		if err := p.consume(IDENTIFIER, ""); err != nil {
			return nil, err
		}
		columnValues = append(columnValues, columnValue)
	}

	if err := p.consume(SYMBOL, SymbolClosedRoundBracket); err != nil {
		return nil, err
	}

	return &InsertIntoTable{
		TableName:    tableName,
		ColumnValues: columnValues,
	}, nil
}
