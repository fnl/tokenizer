package tokenizer

import "fmt"

// the class ("type") of a token
type TokenClass int

// all possible classes of tokens
const (
	EndToken       TokenClass = iota // end-of-input token
	LinebreakToken                   // linebreak token
	WordToken                        // alphanumeric token
	NumberToken                      // numeric (digits) token
	SpaceToken                       // whitespaces, tabs, etc. (category Z)
	SymbolToken                      // anything else; non-whitespace, single rune
)

var className = []string{
	"End",
	"Linebreak",
	"Word",
	"Number",
	"Space",
	"Symbol",
}

// a token, as produced by the lexer
type Token struct {
	Class TokenClass // the class of the token
	Value string     // the value of the token
	//PoS   string     // the token's part-of-speech (not set by the lexer)
}

// the token's class name
func (t *Token) ClassName() string {
	return className[t.Class]
}

// the token's value
func (t *Token) String() string {
	//if t.PoS != "" {
	//	return fmt.Sprintf("%s:%q:$s", t.ClassName(), t.Value, t.PoS)
	//} else {
	return fmt.Sprintf("%s:%q", t.ClassName(), t.Value)
	//}
}

// true if the token marks the end of an input
func (t Token) IsEnd() bool {
	return t.Class == EndToken
}

// true if the token breaks lines (EOL)
func (t Token) IsLinebreak() bool {
	return t.Class == LinebreakToken
}

// true if the token is a word
func (t Token) IsWord() bool {
	return t.Class == WordToken
}

// true if the token is a number
func (t Token) IsNumber() bool {
	return t.Class == NumberToken
}

// true if the token is a space
func (t Token) IsSpace() bool {
	return t.Class == SpaceToken
}

// true if the token is a symbol
func (t Token) IsSymbol() bool {
	return t.Class == SymbolToken
}
