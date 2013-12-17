# tokenizer
    import "github.com/fnl/tokenizer"

A concurrent, deterministic finite state tokenizer
for Latin scripts that separates strings into words, numbers, and symbols.
Words may be connected by dashes, underscores, and dots
(but cannot start or end with those characters).
Numbers may contain commas between digits,
but only before the comma-separator (a dot).
Anything else not a linebreak, control character,
or space is considered a symbol.
Symbol tokens are always single ("one-character") runes.

Options:

1. Normalization of quotes and dashes;
2. Tokenization of spaces and/or linebreaks;
3. Lowering the case of all words;
4. Unescaping of HTML entities;
5. Expansion of Greek letters.

In addition, a command-line tokenizer is provided as `fnltok`:

	go install github.com/fnl/tokenizer/fnltok

Usage:

	fnltok [options] [TEXTFILE ...]

`fnltok` is a high-throughput, line-based command-line interface
for the tokenizer that writes the tokens to `STDOUT`.
This script is about 100 times faster than an equivalent Perl tokenizer
using regular expressions for the same task.
It can tokenize input based on lines and/or tab-separated values
(while preserving the tabs).
The latter is useful to tokenize text in tabulated data files.
Because file I/O soon becomes the main bottleneck,
having more than two or three parallel tokenizer processes (`$GOMAXPROCS`)
does not improve its speed any further.

## Synopsis

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

## func Lex
<pre>func Lex(input chan string, outputBufferSize int, options Option) chan Token</pre>
Lex starts a scanner process to lex string input,
returning a Token output channel.
The outputBufferSize is the buffer size
that should be used to create the output channel.

The scanner waits for strings to lex on the input channel.
After lexing, it sends the found tokens back via the output channel.
After all tokens have been output for a given input, an EndToken is sent.
If the input channel is closed,
the output channel closes after the last (End) token has been emitted.

Possible options; combine Option values by or-ing ("|"):

	Spaces:
	  emit space tokens.
	Linebreak:
	  emit tokens containing EOLMarkers.
	Entities:
	  resolve and replace HTML entities (/&\w+;/).
	Quotes:
	  replace two single with one double quote and
	  U+02BC (modifier apostrophe) with U+0027 ("'" - apostrophe).
	Lowercase:
	  lower-case all words.
	Greek:
	  expand Greek letters to their upper-/lower-case Latin names.
	NoOptions:
	  use none of the options (the zero value default).
	AllOptions:
	  use all of the options.

