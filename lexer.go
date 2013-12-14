/*
A concurrent, deterministic finite state tokenizer
for Latin scripts that separates strings into words, numbers, and symbols.
Words may be connected by dashes, underscores, and dots
(but cannot start or end with those characters).
Numbers may contain commas between digits,
but only before the comma-separator (a dot).
Anything else not a linebreak, control character,
or space is considered a symbol.
Symbols are always single runes.

Options:
Normalization of quotes and dashes;
Tokenization of spaces and/or linebreaks;
Lowering the case of all words;
Unescaping of HTML entities.

SYNOPSIS

  // create an input channel for tokenization:
  in := make(chan string)

  // start the tokenizer;
  // returns an output channel of tokens:
  out := Lex(in, 100, AllOptions)

  // a semaphore to synchronize downstream results
  semaphore := make(chan int)

  // somehow concurrently process the tokens...
  go processTokens(out, semaphore)

  // send data to the tokenizer
  for data := range myInput {
    in <- data
  }
  // and close the input stream once done
  close(in)

  // wait for the output processing to complete
  <-semaphore

*/
package tokenizer

import (
	"fmt"
	"github.com/golang/glog"
	"html"
	"math/rand"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The lexer structure holds the lexer options
// and the state of the scanner.
type lexer struct {
	name   string     // an ID for this lexer (for logging)
	buffer string     // the current buffer being scanned
	start  int        // start position of the current token
	pos    int        // position of the scanner on the buffer
	width  int        // width of last rune scanned on the buffer before the current position
	output chan Token // Token output channel
	// user settings:
	input   chan string // string input channel
	options Option      // lexer options (Spaces, Entities, etc.)
}

// The scanner's states are encoded as state functions
// that return the scanner's next state.
type stateFn func(*lexer) stateFn

// a lexer configuration option
type Option int

const (
	Spaces     Option = 1 << iota   // emit space tokens
	Linebreaks Option = 1 << iota   // emit EOL tokens
	Entities   Option = 1 << iota   // unescape HTML entities
	Quotes     Option = 1 << iota   // normalize single quotes
	Lowercase  Option = 1 << iota   // normalize the case of words
	AllOptions Option = 1<<iota - 1 // use all options
	NoOptions         = Option(0)   // use no options
)

// all end-of-line runes that give rise to linebreak tokens
const EOLMarkers string = "\n\v\f\r\u0085\u2028\u2029"

// a regular expression to check if
// a string might start with an escaped HTML entity;
// does not guarantee its a valid entity -
// only signals that the string could encode an entity
var entity = regexp.MustCompile("^&\\w+;")

// mapping of single to double quote runes
var normalQuote = map[rune]string{
	'’':  "”",  // right single quote to right double quote
	'‘':  "“",  // left single quote to left double quote
	'‚':  "„",  // lower single quote to lower double quote
	'\'': "\"", // single quote/apostrophe to double quote
}

// Lex starts a scanner process to lex string input,
// returning a Token output channel.
// The outputBufferSize is the buffer size
// that should be used to create the output channel.
//
// The scanner waits for strings to lex on the input channel.
// After scanning, it send the found tokens back via the output channel.
// After all tokens have been output for a given input, an EndToken is sent.
// If the input channel is closed,
// the output channel closes after the last (End) token has been emitted.
//
// Possible options; combine Option values by or-ing ("|"):
//
//   Spaces:
//     emit space tokens.
//   Linebreak:
//     emit tokens containing EOLMarkers.
//   Entities:
//     resolve and replace HTML entities (/&\w+;/).
//   Quotes:
//     replace two single with one double quote and
//     U+02BC (modifier apostrophe) with U+0027 ("'" - apostrophe).
//   Lowercase:
//     lower-case all words.
//   NoOptions:
//     use none of the options (the zero value default).
//   AllOptions:
//     use all of the options.
func Lex(input chan string, outputBufferSize int, options Option) chan Token {
	l := &lexer{
		name:    fmt.Sprintf("lexer-%04d", rand.Intn(1e4)),
		options: options,
		input:   input,
		output:  make(chan Token, outputBufferSize),
	}
	go l.run() // concurrently runs the scanner
	return l.output
}

// create a lexer with no options
func LexNoOptions(input chan string, outputBufferSize int) chan Token {
	return Lex(input, outputBufferSize, NoOptions)
}

// create a lexer using all options
func LexAllOptions(input chan string, outputBufferSize int) chan Token {
	return Lex(input, outputBufferSize, AllOptions)
}

// create a lexer that unescapes entities, maps quotes and lowercases words,
// with an output buffer of 100 tokens
//
// This lexer is particularly useful to parse generic, line-based input.
func LexLines(input chan string) chan Token {
	return Lex(input, 100, Entities|Quotes|Lowercase)
}

// true if the scanner emits spaces
func (l *lexer) emitsSpaces() bool {
	return l.options&Spaces != 0
}

// true if the scanner emits EOL markers
func (l *lexer) emitsLinebreaks() bool {
	return l.options&Linebreaks != 0
}

// true if this lexer unescapes valid HTML entities
func (l *lexer) unescapesEntities() bool {
	return l.options&Entities != 0
}

// true if this lexer normalizes two single quotes to double quotes
func (l *lexer) normalizesQuotes() bool {
	return l.options&Quotes != 0
}

// true if this lexer lowercases words
func (l *lexer) lowersWords() bool {
	return l.options&Lowercase != 0
}

// run receives strings from the input channel;
// then, scan the string, storing the emitted tokens;
// finally, send the tokens back through the output channel;
// break the loop and send back `nil` if the input is closed
func (l *lexer) run() {
	options := make([]string, 5)
	if l.emitsSpaces() {
		options[0] = "Spaces "
	}
	if l.emitsLinebreaks() {
		options[1] = "Linebreaks "
	}
	if l.unescapesEntities() {
		options[2] = "Entities "
	}
	if l.normalizesQuotes() {
		options[3] = "Quotes "
	}
	if l.lowersWords() {
		options[4] = "Lowercase "
	}
	glog.Infof("%s starting up; options: %s\n", l.name, strings.Join(options, ""))

	for data := range l.input {
		l.width = 0
		l.pos = 0
		l.start = 0
		l.buffer = data
		for state := lexText; state != nil; {
			state = state(l)
		}
	}

	close(l.output)
	glog.Infof("%s shutting down\n", l.name)
}

// lastRune decodes the last rune once more from the buffer
func (l *lexer) lastRune() (r rune) {
	r, _ = utf8.DecodeRuneInString(l.buffer[l.pos-l.width:])
	return
}

// emit outputs the scanned token,
// assigning it the given class;
// lowercase words as requested;
// moves the scanner start offset
func (l *lexer) emit(class TokenClass) {
	value := l.buffer[l.start:l.pos]

	if l.lowersWords() && class == WordToken {
		value = strings.ToLower(value)
	}

	l.output <- Token{Class: class, Value: value}
	l.start = l.pos
}

// scan returns the next rune in the buffer;
// return zero if there are no more runes to decode;
// moves the scanner's position on the buffer
func (l *lexer) scan() (r rune) {
	if l.pos >= len(l.buffer) {
		l.width = 0
		return 0
	}

	r, l.width = utf8.DecodeRuneInString(l.buffer[l.pos:])
	l.pos += l.width
	return r
}

// ignore skips over the scanned runes (instead of emitting them);
// moves the scanner's start offset
func (l *lexer) ignore() {
	l.start = l.pos
}

// undo moves the scanner back one rune;
// can only undo the last scan();
// moves the scanner's position on the buffer
func (l *lexer) undo() {
	l.pos -= l.width
	l.width = 0
}

// peek previews the next rune without consuming it
func (l *lexer) peek() rune {
	w := l.width
	r := l.scan()
	l.undo()
	l.width = w
	return r
}

// accept consumes the next rune if it's from the valid set
// using undo() after this call has no effect
// if the rune was not accepted
// until a new scan() is made
func (l *lexer) accept(valid string) bool {
	if strings.ContainsRune(valid, l.scan()) {
		l.undo()
		return false
	} else {
		return true
	}
}

// acceptAll consumes runes while they are in a set of valid runes;
// using undo() after this call has no effect
// until a new scan() is made
func (l *lexer) acceptAll(valid string) {
	for strings.ContainsRune(valid, l.scan()) {
	}
	l.undo()
}

// acceptOn consume runes while they test positively;
// using undo() after this call has no effect
// until a new scan() is made
func (l *lexer) acceptOn(test func(rune) bool) {
	for tok := l.scan(); ; tok = l.scan() {
		if tok == '&' && l.probeEntity() {
			tok = l.scan()
		}
		if !test(tok) {
			break
		}
	}
	l.undo()
}

// probeEntity replaces text representing a valid HTML entity
//
// This method assumes the lexer has just consumed the required ampersand.
// If a valid HTML entity string is detected, the lexer will
// replace the entity string with the unescaped rune in the buffer
// and move the scanner back just before that rune,
// returning true.
// Otherwise, this method does nothing and returns false.
func (l *lexer) probeEntity() bool {
	if l.unescapesEntities() {
		candidate := l.buffer[l.pos-l.width:]
		idx := entity.FindStringIndex(candidate)

		if idx != nil {
			orig := candidate[:idx[1]]
			alt := html.UnescapeString(orig)

			if alt != orig {
				before := l.buffer[:l.pos-l.width]
				after := l.buffer[l.pos-l.width+idx[1]:]
				l.buffer = before + alt + after
				l.pos -= l.width
				return true
			}
		}
	}
	return false
}

// these lexer functions return the next
// state for the scanner as a function

// lexText tokenizes any kind of text
//
// Given the lexer options, this function might also
// emit space and EOL tokens.
func lexText(l *lexer) stateFn {
	for {
		// emit a stateFn by switching on the rune's category
		switch r := l.scan(); {
		case r == 0:
			return lexEnd // end
		case unicode.IsLetter(r):
			return lexWord // word
		case unicode.IsDigit(r):
			return lexNumber // number
		case isSpace(r):
			l.acceptOn(isSpace)
			if l.emitsSpaces() {
				l.emit(SpaceToken) // space
			} else {
				l.ignore()
			}
		case isEOL(r):
			l.acceptAll(EOLMarkers)
			if l.emitsLinebreaks() {
				l.emit(LinebreakToken) // linebreak
			} else {
				l.ignore()
			}
		case isSymbol(r):
			return lexSymbol // symbol
		default:
			l.ignore()
		}
	}
}

// lexEnd stops scanning, panicking if the lexer isn't in a valid terminal state
func lexEnd(l *lexer) stateFn {
	if l.pos != len(l.buffer) {
		glog.Errorf("unseen content: %q\n", l.buffer[l.pos:])
	}
	if l.pos != l.start {
		glog.Errorf("unhandled tokens: %q\n", l.buffer[l.start:l.pos])
	}
	l.emit(EndToken)
	return nil // stops the state loop
}

// lexWord consumes and produces a word
//
// Given the lexer options, this function might
// also replace HTML entities.
func lexWord(l *lexer) stateFn {
	for {
		switch r := l.scan(); {
		case r == '-' || r == '.' || r == '_':
			p := l.peek()
			if isLetterOrDigit(p) {
				l.scan() // also consume the alphanumeric rune
				continue // and continue
			} else {
				l.undo() // drop the rune from the word
			}
		case r == '&':
			if l.probeEntity() {
				continue // continue by rescanning the now-escaped entity
			} else {
				l.undo() // drop the ampersand from the word
			}
		case !isLetterOrDigit(r):
			l.undo() // drop the rune from the word
		default:
			continue // consume the alphanumeric rune and continue
		}
		l.emit(WordToken)
		return lexText // scan next token
	}
}

// lexNumber consumes and produces a number
func lexNumber(l *lexer) stateFn {
	l.acceptOn(unicode.IsDigit)
	switch r := l.scan(); r {
	case ',':
		if unicode.IsDigit(l.peek()) {
			return lexNumber // continue (recursion-safe)
		} else {
			l.undo()
		}
	case '.':
		if unicode.IsDigit(l.peek()) {
			l.acceptOn(unicode.IsDigit)
			if l.peek() == '.' {
				return lexWord // treat as word instead (123.123.123)
			}
		} else {
			l.undo()
		}
	default:
		l.undo()
	}
	l.emit(NumberToken)
	return lexText // scan next token
}

// lexSymbol consumes and produces a symbol
//
// If the symbol is '-' and followed by a digit,
// produce a number instead.
//
// Given the lexer options, this function also
// might change the actual content:
// If the symbol is '&' and followed by a valid
// HTML entity name, lex that entity instead.
// If there are two singe quotes, normalize
// it.
func lexSymbol(l *lexer) stateFn {
	r := l.lastRune()

	if r == '&' && l.probeEntity() {
		return lexText // retry scan...
	} else if l.normalizesQuotes() && normalQuote[r] != "" && l.peek() == r {
		before := l.buffer[:l.pos-l.width]
		after := l.buffer[l.pos+l.width:]
		l.buffer = before + normalQuote[r] + after
	} else if l.normalizesQuotes() && r == '\u02bc' {
		before := l.buffer[:l.pos-l.width]
		after := l.buffer[l.pos:]
		l.buffer = before + "'" + after
	}

	l.emit(SymbolToken)
	return lexText // scan next token
}

// checks to test if a rune belongs to some Unicode categories

// true if the rune is any of the EOLMarkers
func isEOL(r rune) bool {
	return strings.ContainsRune(EOLMarkers, r)
}

// true if the rune is a Unicode letter or digit
func isLetterOrDigit(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// true if the rune is from the Unicode S, P, Nl, or No categories
func isSymbol(r rune) bool {
	return unicode.Is(unicode.S, r) ||
		unicode.Is(unicode.P, r) ||
		unicode.Is(unicode.Nl, r) ||
		unicode.Is(unicode.No, r)
}

// true if the rune is a Unicode space and tab
func isSpace(r rune) bool {
	return unicode.Is(unicode.Zs, r) || r == '\t'
}
