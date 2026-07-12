package compiler

import (
	"fmt"
	"strconv"
)

type Parser struct {
	lexer *Lexer
	curr  Token
	peek  Token
}

func NewParser(input string) *Parser {
	p := &Parser{lexer: NewLexer(input)}
	p.nextToken()
	p.nextToken()
	return p
}

func (p *Parser) nextToken() {
	p.curr = p.peek
	p.peek = p.lexer.NextToken()
}

func (p *Parser) Parse() (*Manifest, error) {
	manifest := &Manifest{}

	if p.curr.Type == TokenVersion {
		p.nextToken()
		if p.curr.Type != TokenInt {
			return nil, fmt.Errorf("line %d: expected version number, got %v", p.curr.Line, p.curr.Value)
		}
		v, _ := strconv.Atoi(p.curr.Value)
		manifest.Version = v
		p.nextToken()
	}

	for p.curr.Type != TokenEOF {
		entry, err := p.parseEntry()
		if err != nil {
			return nil, err
		}
		manifest.Entries = append(manifest.Entries, entry)
	}

	return manifest, nil
}

func (p *Parser) parseEntry() (Entry, error) {
	entry := Entry{Line: p.curr.Line}

	if p.curr.Type == TokenLBracket {
		p.nextToken()
		if p.curr.Type != TokenIdentifier {
			return entry, fmt.Errorf("line %d: expected variable name after [", p.curr.Line)
		}
		entry.Name = p.curr.Value
		entry.IsVariable = true
		p.nextToken()
		if p.curr.Type != TokenRBracket {
			return entry, fmt.Errorf("line %d: expected ] after variable name", p.curr.Line)
		}
		p.nextToken()
	} else if p.curr.Type == TokenIdentifier {
		entry.Name = p.curr.Value
		p.nextToken()
	} else {
		return entry, fmt.Errorf("line %d: expected identifier or [variable], got %v", p.curr.Line, p.curr.Value)
	}

	if p.curr.Type != TokenLBrace {
		return entry, fmt.Errorf("line %d: expected { after %s, got %v", p.curr.Line, entry.Name, p.curr.Value)
	}
	p.nextToken()

	for p.curr.Type != TokenRBrace && p.curr.Type != TokenEOF {
		if p.curr.Type == TokenAt {
			intent, err := p.parseIntent()
			if err != nil {
				return entry, err
			}
			entry.Intents = append(entry.Intents, intent)
		} else {
			child, err := p.parseEntry()
			if err != nil {
				return entry, err
			}
			entry.Children = append(entry.Children, child)
		}
	}

	if p.curr.Type != TokenRBrace {
		return entry, fmt.Errorf("line %d: expected } at end of %s block", p.curr.Line, entry.Name)
	}
	p.nextToken()

	return entry, nil
}

func (p *Parser) parseIntent() (Intent, error) {
	intent := Intent{Line: p.curr.Line, Params: make(map[string]interface{})}
	p.nextToken() // skip @

	if p.curr.Type != TokenIdentifier {
		return intent, fmt.Errorf("line %d: expected intent name after @", p.curr.Line)
	}
	intent.Name = p.curr.Value
	p.nextToken()

	if p.curr.Type == TokenLParen {
		p.nextToken()
		for p.curr.Type != TokenRParen && p.curr.Type != TokenEOF {
			if p.curr.Type != TokenIdentifier {
				return intent, fmt.Errorf("line %d: expected parameter name", p.curr.Line)
			}
			paramName := p.curr.Value
			p.nextToken()

			if p.curr.Type != TokenColon {
				return intent, fmt.Errorf("line %d: expected : after parameter name", p.curr.Line)
			}
			p.nextToken()

			var paramValue interface{}
			if p.curr.Type == TokenInt {
				v, _ := strconv.ParseInt(p.curr.Value, 10, 64)
				paramValue = v
			} else if p.curr.Type == TokenString {
				paramValue = p.curr.Value
			} else {
				return intent, fmt.Errorf("line %d: expected int or string value", p.curr.Line)
			}
			intent.Params[paramName] = paramValue
			p.nextToken()

			if p.curr.Type == TokenComma {
				p.nextToken()
			}
		}
		if p.curr.Type != TokenRParen {
			return intent, fmt.Errorf("line %d: expected ) after parameter list", p.curr.Line)
		}
		p.nextToken()
	}

	return intent, nil
}
