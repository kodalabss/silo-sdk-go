package compiler

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

type TokenType int

const (
	TokenError TokenType = iota
	TokenEOF
	TokenVersion
	TokenIdentifier
	TokenInt
	TokenString
	TokenLBrace   // {
	TokenRBrace   // }
	TokenLBracket // [
	TokenRBracket // ]
	TokenLParen   // (
	TokenRParen   // )
	TokenColon    // :
	TokenComma    // ,
	TokenAt       // @
)

type Token struct {
	Type  TokenType
	Value string
	Line  int
	Pos   int
}

type Lexer struct {
	input string
	start int
	pos   int
	width int
	line  int
}

func NewLexer(input string) *Lexer {
	return &Lexer{input: input, line: 1}
}

func (l *Lexer) NextToken() Token {
	l.skipWhitespace()
	if l.isEOF() {
		return Token{Type: TokenEOF, Line: l.line}
	}

	r := l.next()
	switch r {
	case '{':
		return Token{Type: TokenLBrace, Value: "{", Line: l.line}
	case '}':
		return Token{Type: TokenRBrace, Value: "}", Line: l.line}
	case '[':
		return Token{Type: TokenLBracket, Value: "[", Line: l.line}
	case ']':
		return Token{Type: TokenRBracket, Value: "]", Line: l.line}
	case '(':
		return Token{Type: TokenLParen, Value: "(", Line: l.line}
	case ')':
		return Token{Type: TokenRParen, Value: ")", Line: l.line}
	case ':':
		return Token{Type: TokenColon, Value: ":", Line: l.line}
	case ',':
		return Token{Type: TokenComma, Value: ",", Line: l.line}
	case '@':
		return Token{Type: TokenAt, Value: "@", Line: l.line}
	case '"':
		return l.lexString()
	}

	if unicode.IsDigit(r) {
		l.backup()
		return l.lexNumber()
	}

	if unicode.IsLetter(r) || r == '_' {
		l.backup()
		return l.lexIdentifier()
	}

	return Token{Type: TokenError, Value: fmt.Sprintf("unexpected character: %c", r), Line: l.line}
}

func (l *Lexer) next() rune {
	if l.pos >= len(l.input) {
		l.width = 0
		return -1
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.width = w
	l.pos += l.width
	if r == '\n' {
		l.line++
	}
	return r
}

func (l *Lexer) backup() {
	l.pos -= l.width
	if l.width > 0 {
		r, _ := utf8.DecodeRuneInString(l.input[l.pos:])
		if r == '\n' {
			l.line--
		}
	}
}

func (l *Lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

func (l *Lexer) isEOF() bool {
	return l.pos >= len(l.input)
}

func (l *Lexer) skipWhitespace() {
	for {
		r := l.next()
		if !unicode.IsSpace(r) {
			l.backup()
			break
		}
	}
}

func (l *Lexer) lexIdentifier() Token {
	start := l.pos
	for {
		r := l.next()
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			// keep going
		} else {
			l.backup()
			break
		}
	}
	val := l.input[start:l.pos]
	if val == "version" {
		return Token{Type: TokenVersion, Value: val, Line: l.line}
	}
	return Token{Type: TokenIdentifier, Value: val, Line: l.line}
}

func (l *Lexer) lexNumber() Token {
	start := l.pos
	for {
		r := l.next()
		if !unicode.IsDigit(r) {
			l.backup()
			break
		}
	}
	return Token{Type: TokenInt, Value: l.input[start:l.pos], Line: l.line}
}

func (l *Lexer) lexString() Token {
	start := l.pos
	for {
		r := l.next()
		if r == '"' || r == -1 {
			break
		}
	}
	return Token{Type: TokenString, Value: l.input[start : l.pos-1], Line: l.line}
}
