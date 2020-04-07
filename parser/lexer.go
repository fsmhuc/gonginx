package parser

import (
	"bufio"
	"bytes"
	"io"
	"unicode"

	"github.com/tufanbarisyildirim/gonginx/token"
)

//Lexer is the main tokenizer
type Lexer struct {
	reader *bufio.Reader
	file   string
	line   int
	column int
	Latest token.Token
}

//Parse initializes a Lexer from string conetnt
func Parse(content string) *Lexer {
	return NewLexer(bytes.NewBuffer([]byte(content)))
}

//NewLexer initilizes a Lexer from a reader
func NewLexer(r io.Reader) *Lexer {
	return &Lexer{
		reader: bufio.NewReader(r),
	}
}

//Scan gives you next token
func (s *Lexer) Scan() token.Token {
	s.Latest = s.getNextToken()
	return s.Latest
}

//All scans all token and returns them as a slice
func (s *Lexer) All() token.Tokens {
	tokens := make([]token.Token, 0)
	for {
		v := s.Scan()
		if v.Type == token.Eof || v.Type == -1 {
			break
		}
		tokens = append(tokens, v)
	}
	return tokens
}

func (s *Lexer) getNextToken() token.Token {
reToken:
	ch := s.Peek()
	switch {
	case isSpace(ch):
		s.skipWhitespace()
		goto reToken
	case isEOF(ch):
		return s.NewToken(token.Eof).Lit(string(s.read()))
	case ch == ';':
		return s.NewToken(token.Semicolon).Lit(string(s.read()))
	case ch == '{':
		return s.NewToken(token.BlockStart).Lit(string(s.read()))
	case ch == '}':
		return s.NewToken(token.BlockEnd).Lit(string(s.read()))
	case ch == '#':
		return s.scanComment()
	case ch == '$':
		return s.scanVariable()
	case isQuote(ch):
		return s.scanQuotedString(ch)
	case isNotSpace(ch):
		return s.scanKeyword()
	}

	return s.NewToken(token.Illegal).Lit(string(s.read())) //that should never happen :)
}

//Peek returns nexr rune without consuming it
func (s *Lexer) Peek() rune {
	r := s.read()
	s.unread()
	return r
}

//PeekPrev returns review rune withouy actually seeking index to back
func (s *Lexer) PeekPrev() rune {
	s.unread()
	r := s.read()
	return r
}

type runeCheck func(rune) bool

func (s *Lexer) readUntil(until runeCheck) string {
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	for {
		if ch := s.read(); isEOF(ch) {
			break
		} else if until(ch) {
			s.unread()
			break
		} else {
			buf.WriteRune(ch)
		}
	}

	return buf.String()
}

//NewToken creates a new Token with its line and column
func (s *Lexer) NewToken(tokenType token.Type) token.Token {
	return token.Token{
		Type:   tokenType,
		Line:   s.line,
		Column: s.column,
	}
}

func (s *Lexer) readUntilWith(until runeCheck) string {
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	for {
		if ch := s.read(); isEOF(ch) {
			break
		} else if until(ch) {
			buf.WriteRune(ch)
			break
		} else {
			buf.WriteRune(ch)
		}
	}
	return buf.String()
}

func (s *Lexer) readWhile(while runeCheck) string {
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	for {
		ch := s.read()
		if while(ch) {
			buf.WriteRune(ch)
		} else {
			s.unread()
			break
		}
	}
	// unread the latest char we consume.
	return buf.String()
}

func (s *Lexer) skipWhitespace() {
	s.readWhile(isSpace)
}

func (s *Lexer) skipEndOfLine() {
	s.readUntilWith(isEndOfLine)
}

func (s *Lexer) scanComment() token.Token {
	return s.NewToken(token.Comment).Lit(s.readUntil(isEndOfLine))
}

func (s *Lexer) scanRegex() token.Token {
	return s.NewToken(token.Regex).Lit(s.readUntil(isSpace))
}

/**
\” – To escape “ within double quoted string.
\\ – To escape the backslash.
\n – To add line breaks between string.
\t – To add tab space.
\r – For carriage return.
*/
func (s *Lexer) scanQuotedString(delimiter rune) token.Token {
	var buf bytes.Buffer
	tok := s.NewToken(token.QuotedString)
	s.read() //consume delimiter
	for {
		ch := s.read()

		if ch == rune(token.Eof) {
			panic("unexpected end of file while scanning a string, maybe an unclosed quote?")
		}

		if ch == '\\' {
			if needsEscape(s.Peek(), delimiter) {
				switch s.read() {
				case 'n':
					buf.WriteRune('\n')
				case 'r':
					buf.WriteRune('\r')
				case 't':
					buf.WriteRune('\t')
				case '\\':
					buf.WriteRune('\\')
				case delimiter:
					buf.WriteRune(delimiter)
				}
				continue
			}
		}
		if ch == delimiter {
			break
		}
		buf.WriteRune(ch)
	}

	return tok.Lit(buf.String())
}

func (s *Lexer) scanKeyword() token.Token {
	return s.NewToken(token.Keyword).Lit(s.readUntil(isKeywordTerminator))
}

func (s *Lexer) scanVariable() token.Token {
	return s.NewToken(token.Variable).Lit(s.readUntil(isKeywordTerminator))
}

func (s *Lexer) unread() {
	_ = s.reader.UnreadRune()
	s.column--
}

func (s *Lexer) read() rune {
	ch, _, err := s.reader.ReadRune()
	if err != nil {
		return rune(token.Eof)
	}

	if ch == '\n' {
		s.column = 1
		s.line++
	} else {
		s.column++
	}
	return ch
}

func isQuote(ch rune) bool {
	return ch == '"' || ch == '\'' || ch == '`'
}

func isRegexDelimiter(ch rune) bool {
	return ch == '/'
}

func isNotSpace(ch rune) bool {
	return !isSpace(ch)
}

func isKeywordTerminator(ch rune) bool {
	return isSpace(ch) || isEndOfLine(ch) || ch == '{' || ch == ';'
}

func needsEscape(ch, delimiter rune) bool {
	return ch == delimiter || ch == 'n' || ch == 't' || ch == '\\' || ch == 'r'
}

func isSpace(ch rune) bool {
	return ch == ' ' || ch == '\t' || isEndOfLine(ch)
}

func isEOF(ch rune) bool {
	return ch == rune(token.Eof)
}

func isEndOfLine(ch rune) bool {
	return ch == '\r' || ch == '\n'
}

func isLetter(ch rune) bool {
	return ch == '_' || unicode.IsLetter(ch)
}

func isWordStart(ch rune) bool {
	return isLetter(ch) || unicode.IsDigit(ch)
}
