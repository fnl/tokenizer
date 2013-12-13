/*
A high-throughput, line-based command-line interface for the tokenizer that writes the tokens to <STDOUT>.
This script is about 100 times faster than an equivalent Perl tokenizer using regular expressions for the same task.
It can tokenize input based on lines and/or tab-separated values (while preserving the tabs).
The latter is useful to tokenize text in tabulated data files.
Because file I/O soon becomes the main bottleneck, having more than two or three parallel tokenizer processes ($GOMAXPROCS) running does not improve its speed any further.
*/
package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/fnl/tokenizer"
	"github.com/golang/glog"
	"io"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
)

var all bool
var entities bool
var lowercase bool
var quotes bool
var spaces bool
var split bool
var tsv bool
var cpuProfileFile string
var heapProfileFile string

func init() {
	flag.BoolVar(&all, "all", false, "enable -entities, -lowercase, and -quotes")
	flag.BoolVar(&entities, "entities", false, "unescape HTML entities")
	flag.BoolVar(&lowercase, "lowercase", false, "lowercase words")
	flag.BoolVar(&quotes, "quotes", false, "normalize quotes")
	flag.BoolVar(&split, "split", false, "split tokens by newlines (default: spaces)")
	flag.BoolVar(&spaces, "spaces", false, "emit spaces (forces -split)")
	flag.BoolVar(&tsv, "tsv", false, "maintain tab-separation of input")
	flag.StringVar(&cpuProfileFile, "pprof", "", "write CPU profile to file")
	flag.StringVar(&heapProfileFile, "mprof", "", "write heap profile to file")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [Options] [FILE ...]\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
}

func main() {
	var options tokenizer.Option
	sep := " "

	flag.Parse()

	if split || spaces {
		sep = "\n"
	}

	if all {
		options = tokenizer.Entities | tokenizer.Quotes | tokenizer.Lowercase
	}
	if spaces || tsv {
		options = options | tokenizer.Spaces

		if spaces && tsv {
			glog.Fatalln("-spaces and -tsv are incompatible options")
		}
	}
	if entities {
		options |= tokenizer.Entities
	}
	if quotes {
		options |= tokenizer.Quotes
	}
	if lowercase {
		options |= tokenizer.Lowercase
	}

	if cpuProfileFile != "" {
		profile, err := os.Create(cpuProfileFile)

		if err != nil {
			glog.Fatalf("creating CPU profile file failed: %s\n", err)
		}

		err = pprof.StartCPUProfile(profile)

		if err != nil {
			glog.Fatalf("starting CPU profile failed: %s\n", err)
		}

		defer pprof.StopCPUProfile()
	}

	if flag.NArg() > 0 {
		for _, path := range flag.Args() {
			file, err := os.Open(path)

			if err != nil {
				glog.Fatalf("reading %q failed: %s\n", path, err)
			}

			defer file.Close()
			tokenize(file, options, sep)
		}
	} else {
		tokenize(os.Stdin, options, sep)
	}

	if heapProfileFile != "" {
		profile, err := os.Create(heapProfileFile)

		if err != nil {
			glog.Fatalf("creating heap profile file failed: %s\n", err)
		}

		err = pprof.WriteHeapProfile(profile)

		if err != nil {
			glog.Fatalf("writing heap profile failed: %s\n", err)
		}
	}

}

func min(a, b int) int {
	if a <= b {
		return a
	} else {
		return b
	}
}

func tokenize(file io.Reader, options tokenizer.Option, sep string) {
	n := min(runtime.GOMAXPROCS(0), runtime.NumCPU())
	input := make(chan string, n)
	output := make(chan string, n)
	semaphore := make(chan int)

	for i := 0; i < n; i++ {
		tokens := tokenizer.Lex(input, 50, options)
		go convertTokens(tokens, sep, output, semaphore)
	}

	go writeResults(output, semaphore)

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		input <- scanner.Text()
	}

	close(input)

	for n > 0 {
		<-semaphore
		n--
	}

	close(output)
	<-semaphore
}

func convertTokens(in chan tokenizer.Token, sep string, out chan string, done chan int) {
	var buffer []string
	tsvOffset := 0

	for token := range in {
		switch token.Class {
		case tokenizer.EndToken:
			if tsv {
				buffer, tsvOffset = tsvTokenizer(buffer, tsvOffset, sep)
				out <- strings.Join(buffer, "\t")
				tsvOffset = 0
			} else {
				out <- strings.Join(buffer, sep)
			}
			buffer = buffer[:0]
		default:
			if tsv && token.IsSpace() && strings.ContainsRune(token.Value, '\t') {
				buffer, tsvOffset = tsvTokenizer(buffer, tsvOffset, sep)

				// append trailing tabs:
				cnt := strings.Count(token.Value, "\t")
				for i := 1; i < cnt; i++ {
					buffer = append(buffer, "")
					tsvOffset++
				}
			} else if !tsv || !token.IsSpace() {
				buffer = append(buffer, token.Value)
			}
		}
	}
	done <- 1
}

func tsvTokenizer(buffer []string, tsvOffset int, sep string) ([]string, int) {
	if tsvOffset < len(buffer) {
		// sep-join all tokens between the the last tab (if any) and the current one
		buffer[tsvOffset] = strings.Join(buffer[tsvOffset:], sep)
		// truncate the buffer accordingly
		buffer = buffer[:tsvOffset+1]
	} else {
		// update an already sep-joined buffer to include the current tab
		buffer = append(buffer, "")
	}

	return buffer, tsvOffset + 1
}

func writeResults(lines chan string, done chan int) {
	for l := range lines {
		fmt.Println(l)
	}
	done <- 1
}
