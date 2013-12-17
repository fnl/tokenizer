package tokenizer

import (
	"fmt"
	"testing"
)

func lexerTest(t *testing.T, input string) {
	in := make(chan string, 1)
	cnt := 0
	in <- input
	close(in)

	for token := range Lex(in, 1, NoOptions) {
		cnt++

		switch token.Class {
		case NumberToken:
			expected := fmt.Sprintf("%d", cnt)

			if token.Value != expected {
				t.Errorf("expected %q, got %s", expected, token.String())
			}
		case EndToken:
			if cnt != 4 {
				t.Errorf("expected end at position 3, found at %d", cnt)
			} else {
				cnt++
			}
		default:
			cnt++
			t.Errorf("expected a Number or End, got %s", token.String())
		}
	}

	if cnt == 0 {
		t.Errorf("nothing lexed from %q", input)
	}
}

func TestLexerWithNewline(t *testing.T) {
	lexerTest(t, "1 2 3\n")
}

func TestLexerWithoutNewline(t *testing.T) {
	lexerTest(t, "1 2 3")
}

type checkFn func(Token) bool

type lexerSimpleTestCase struct {
	description string
	expected    []string
	check       checkFn
}

func simpleLexerTest(t *testing.T, description string, check checkFn, testCases []string) {
	in := make(chan string)
	out := LexNoOptions(in, 1)

	for _, ex := range testCases {
		in <- ex
		cnt := 0

	Reader:
		for token := range out {
			switch token.Class {
			case EndToken:
				if cnt != 1 {
					t.Errorf("%s: cnt is %d at End", description, cnt)
				} else {
					cnt++
				}
				break Reader
			default:
				if token.Value != ex {
					t.Errorf("%s: expected %q, got %s", description, ex, token.String())
				} else {
					cnt++

					if !check(token) {
						t.Errorf("%s: check failed for %s", token.String())
					}
				}
			}
		}

		if 2 != cnt {
			t.Errorf("%s: expected 2 tokens, got %d", description, cnt)
		}
	}

	close(in)
	ensureClosing(t, description, in, out)
}

func ensureClosing(t *testing.T, description string, in chan string, out chan Token) {
	tok, ok := <-out

	if ok {
		t.Errorf("%s: the output channel did not close, got %s", description, tok)
	}
}

var lexerSimpleCases = []lexerSimpleTestCase{
	{"Numbers",
		[]string{"123", "123.456", "123,456", "1,2,3.4", "६೬𝟨", "200,000.00"},
		func(tok Token) bool { return tok.IsNumber() }},
	{"Numeral Symbols", []string{"¹", "①", "¾", "Ⅹ"},
		func(tok Token) bool { return tok.IsSymbol() }},
	{"Words", []string{"abc", "cc-cc", "cd_ef", "AA.BB", "1.2.3", "vóila",
		"X123", "六" /* Chinese numbers are words! */},
		func(tok Token) bool { return tok.IsWord() }},
	{"Symbols", []string{".", "_", "-", "!", "?", ":", ";", ",", "<", ">", "@", "€"},
		func(tok Token) bool { return tok.IsSymbol() }},
}

func TestSimpleCases(t *testing.T) {
	for _, test := range lexerSimpleCases {
		simpleLexerTest(t, test.description, test.check, test.expected)
	}
}

type lexerOptionsTestCase struct {
	description string
	options     Option
	expected    []string
}

func lexerOptionsTest(t *testing.T, description string, opts Option, expected []string) {
	in := make(chan string)
	out := Lex(in, 10, opts)
	in <- " \nA\u2014&alpha;''"
	i := -1
	close(in)

	for token := range out {
		i++

		if i < len(expected) {
			if token.Value != expected[i] {
				t.Errorf("%s: expected %q, got %s at %d", description, expected[i], token.String(), i)
			}
		} else if token.IsEnd() {
			break
		} else {
			t.Errorf("%s: more tokens than expected; got %s at %d", description, token.String(), i)
		}
	}

	if len(expected) != i {
		t.Errorf("%s: expected %d tokens, got %d", description, len(expected), i)
	}

	ensureClosing(t, description, in, out)
}

var lexerOptionsCases = []lexerOptionsTestCase{
	{"Spaces", Spaces,
		[]string{" ", "A", "\u2014", "&", "alpha", ";", "'", "'"}},
	{"Linebreaks", Linebreaks,
		[]string{"\n", "A", "\u2014", "&", "alpha", ";", "'", "'"}},
	{"Spaces|Linebreaks", Spaces | Linebreaks,
		[]string{" ", "\n", "A", "\u2014", "&", "alpha", ";", "'", "'"}},
	{"Entities", Entities,
		[]string{"A", "\u2014", "α", "'", "'"}},
	{"Quotes", Quotes,
		[]string{"A", "\u2014", "&", "alpha", ";", "\""}},
	{"Linebreaks", Greek,
		[]string{"A", "\u2014", "&", "alpha", ";", "'", "'"}},
	{"Linebreaks", Hyphens,
		[]string{"A", "-", "&", "alpha", ";", "'", "'"}},
	{"Entities|Quotes", Entities | Quotes,
		[]string{"A", "\u2014", "α", "\""}},
	{"Entities|Hyphens", Entities | Hyphens,
		[]string{"A-α", "'", "'"}},
	{"Entities|Greek", Entities | Greek,
		[]string{"A", "\u2014", "alpha", "'", "'"}},
	{"Entities|Greek|Hyphens", Entities | Greek | Hyphens,
		[]string{"A-alpha", "'", "'"}},
	{"Lowercase", Lowercase,
		[]string{"a", "\u2014", "&", "alpha", ";", "'", "'"}},
	{"Lowercase|Spaces", Lowercase | Spaces,
		[]string{" ", "a", "\u2014", "&", "alpha", ";", "'", "'"}},
}

func TestLexerOptions(t *testing.T) {
	for _, test := range lexerOptionsCases {
		lexerOptionsTest(t, test.description, test.options, test.expected)
	}
}

type lexerFullTestCase struct {
	description string
	line        string
	expected    []string
}

func fullLexerTest(t *testing.T, description, line string, expected []string) {
	in := make(chan string)
	out := LexAllOptions(in, 100)
	cnt := -1
	in <- line
	close(in)

	for token := range out {
		cnt++
		switch token.Class {
		case EndToken:
			if len(expected) != cnt {
				t.Errorf("%s: wrong number of tokens; got %d", description, cnt)
			}
		default:
			if cnt < len(expected) {
				if token.Value != expected[cnt] {
					t.Errorf("%s: expected %q, got %s", description, expected[cnt], token.String())
				}
			} else {
				t.Errorf("%s: too many tokens; got %s", description, token.String())
			}
		}
	}

	if cnt != len(expected) {
		t.Errorf("%s: expected %d, got %d tokens from %q", description, len(expected), cnt, line)
	}
}

var lexerFullCases = []lexerFullTestCase{
	{"regular sentence lexing", "Gene 23p, too.",
		[]string{"gene", " ", "23p", ",", " ", "too", "."}},
	{"abbreviation handling", "Mr. White.",
		[]string{"mr", ".", " ", "white", "."}},
	{"alt space/linebreak detection", "1 \t 2 \v 3 \u00A0 4 \n",
		[]string{"1", " \t ", "2", " ", "\v", " ", "3", " \u00A0 ", "4", " ", "\n"}},
	{"handling consecutive symbols", "http://fnl.es",
		[]string{"http", ":", "/", "/", "fnl.es"}},
	{"digits-dot-digits-word", "123.456abc",
		[]string{"123.456", "abc"}},
	{"digits-dot-word", "123.abc",
		[]string{"123", ".", "abc"}},
	{"digits-comma-digits-word", "123,456abc",
		[]string{"123,456abc"}},
	{"digits-comma-word", "123,abc",
		[]string{"123", ",", "abc"}},
	{"dash at end of word", "this- that",
		[]string{"this", "-", " ", "that"}},
	{"underscore at end of word", "this_ that",
		[]string{"this", "_", " ", "that"}},
	{"entity detection basics", "k&amp;k",
		[]string{"k", "&", "k"}},
	{"entity detection in word", "x&alpha;x",
		[]string{"xalphax"}},
	{"entity detection at start of word", "x &alpha;x",
		[]string{"x", " ", "alphax"}},
	{"entity detection at end of word", "x&alpha; x",
		[]string{"xalpha", " ", "x"}},
	{"entity detection in string starting with digits", "1&alpha;x",
		[]string{"1alphax"}},
	{"entity detection in word only with digit", "1&alpha;",
		[]string{"1alpha"}},
	{"entity detection in alnum starting with digits", "1&amp;x",
		[]string{"1", "&", "x"}},
	{"entity detection in word ending with digits", "x&alpha;1",
		[]string{"xalpha1"}},
	{"entity detection requires semicolon", "x&alphax",
		[]string{"x", "&", "alphax"}},
	{"entity detection requires valid entity name", "x&grblfx;x",
		[]string{"x", "&", "grblfx", ";", "x"}},
	{"entity in symbol", "k@&amp;@k",
		[]string{"k", "@", "&", "@", "k"}},
	{"symbol entity mess", "amp&&amp;&amp",
		[]string{"amp", "&", "&", "&", "amp"}},
	{"mapping single quotes", "''hi''",
		[]string{"\"", "hi", "\""}},
	{"mapping single quotes", "‘‘hi‚‚",
		[]string{"“", "hi", "„"}},
	{"normalize apostrophes", "Anselm’s",
		[]string{"anselm", "’", "s"}},
}

func TestFullLexing(t *testing.T) {
	for _, test := range lexerFullCases {
		fullLexerTest(t, test.description, test.line, test.expected)
	}
}
