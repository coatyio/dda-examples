// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package wf provides a Partition-Compute-Accumulate distribution pattern that
// computes the frequency of occurrence of words in a set of UTF-8 encoded text
// documents.
package wf

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/bmatcuk/doublestar/v4"
	cmpt "github.com/coatyio/dda-examples/compute/computation"
	"github.com/rivo/uniseg"
)

// WordFrequency represents the number of occurrences of words in a corpus. The
// map is transmitted in gob encoding, a go only binary encoding format.
type WordFrequency = map[string]int

// WordFrequencyComputation implements the Computation interface to compute the
// number of occurrences of all words in a given set of UTF-8 encoded text
// documents.
//
// Input data for a partial computation, i.e. a string representing a paragraph
// of text, is transmitted in UTF-8 encoded binary serialization format. Output
// data of a partial computation, i.e. a map of word frequencies, is transmitted
// in gob encoding, a go only binary encoding format.
type WordFrequencyComputation struct {
	request   cmpt.ComputeRequest // only available in Partition, Accumulate, Finalize
	result    WordFrequency       // only available in Partition, Accumulate, Finalize
	fileCount int                 // only available in Partition, Accumulate, Finalize
}

func (c *WordFrequencyComputation) Name() string {
	return "wf"
}

func (c *WordFrequencyComputation) Description() string {
	return "computes frequency of occurrence of words in a set of UTF-8 text documents"
}

func (c *WordFrequencyComputation) Partition(request cmpt.ComputeRequest) (input <-chan cmpt.BinaryData, err error) {
	if len(request.Args) == 0 {
		return nil, fmt.Errorf("specify file globs (with ?, *, **, [], {}), e.g. f?o/**/bar-*.txt")
	}

	c.request = request
	c.result = make(WordFrequency)
	c.fileCount = 0

	in := make(chan cmpt.BinaryData, 100) // buffered channel with limited concurrent send/receive

	go func() {
		defer close(in)
		for _, glob := range c.request.Args {
			matches, err := doublestar.FilepathGlob(glob)
			if err != nil {
				fmt.Fprintf(c.request.OutputWriter, "Skipping bad file glob pattern: %s\n", glob)
				continue
			}
			if len(matches) == 0 {
				fmt.Fprintf(c.request.OutputWriter, "No matches for file glob pattern: %s\n", glob)
				continue
			}
			for _, path := range matches {
				c.partitionFile(path, in)
			}
		}
	}()

	return in, nil
}

func (c *WordFrequencyComputation) PartialCompute(input cmpt.BinaryData) (output cmpt.BinaryData) {
	f := c.computeParagraphFrequency(input)
	if bytes, err := c.encodeOutput(f); err != nil {
		return nil
	} else {
		return bytes
	}
}

func (c *WordFrequencyComputation) PartialComputeTimeout() time.Duration {
	return 60 * time.Second
}

func (c *WordFrequencyComputation) Accumulate(output cmpt.BinaryData) {
	if data, err := c.decodeOutput(output); err != nil {
		fmt.Fprintf(c.request.OutputWriter, "Skipping undecodable output: %v\n", err)
	} else {
		for word, count := range data {
			c.result[word] += count
		}
	}
}

func (c *WordFrequencyComputation) Finalize(start time.Time) {
	fmt.Fprintf(c.request.OutputWriter, "Computation time: %v\n", time.Since(start))

	var wc []struct {
		w string
		c int
		l int
	}
	total := 0
	maxwlen := 0
	for word, count := range c.result {
		total += count
		wlen := uniseg.StringWidth(word) // user-perceived characters (Unicode grapheme clusters)
		maxwlen = max(maxwlen, wlen)
		wc = append(wc, struct {
			w string
			c int
			l int
		}{word, count, wlen})
	}
	sort.Slice(wc, func(i, j int) bool { // sort descendingly by frequency, then ascendingly by words
		ci := wc[i].c
		cj := wc[j].c
		if ci > cj {
			return true
		}
		if ci == cj {
			return wc[i].w < wc[j].w // lexicographical order
		}
		return false
	})

	fmt.Fprintf(c.request.OutputWriter, "Computation %s%v counts %d different words out of %d words in total in %d files:\n",
		c.request.Name,
		c.request.Args,
		len(c.result),
		total, c.fileCount)

	maxclen := 0
	if len(wc) != 0 {
		maxclen = 1 + int(math.Log10(float64(wc[0].c)))
	}
	for _, v := range wc {
		fmt.Fprintf(c.request.OutputWriter, "%s%*s: %*d\n", v.w, maxwlen-v.l+1, " ", maxclen, v.c)
	}
}

func (c *WordFrequencyComputation) partitionFile(path string, input chan<- cmpt.BinaryData) {
	c.fileCount++
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		fmt.Fprintf(c.request.OutputWriter, "Skipping unopenable file %s: %v\n", path, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file) // split by lines
	paragraph := []byte{}             // UTF-8 encoded group of sentences
	eop := false                      // end of paragraph encountered
	for scanner.Scan() {              // read lines until EOF
		line := scanner.Bytes()
		if len(line) == 0 {
			if eop {
				continue
			}
			eop = true
			input <- paragraph
			paragraph = []byte{}
		} else {
			eop = false
			paragraph = append(paragraph, line...)
			paragraph = append(paragraph, '\n') // insert newline as word separator
		}
	}

	if len(paragraph) != 0 { // final paragraph without trailing end-of-line marker
		input <- paragraph
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(c.request.OutputWriter, "Error encountered reading file %s: %v\n", path, err)
		return
	}
}

func (c *WordFrequencyComputation) computeParagraphFrequency(p []byte) WordFrequency {
	ignoreWord := func(w []byte) bool {
		for len(w) > 0 {
			r, size := utf8.DecodeRune(w)
			if unicode.IsPunct(r) || unicode.IsSpace(r) || unicode.IsControl(r) {
				w = w[size:]
				continue
			}
			return false
		}
		return true
	}
	f := make(WordFrequency)

	state := -1
	var wd []byte
	for len(p) > 0 {
		wd, p, state = uniseg.FirstWord(p, state)
		if ignoreWord(wd) {
			continue // skip word consisting of only punctuation, space, or control characters
		}
		f[strings.ToLower(string(wd))]++ // normalize word
	}
	return f
}

func (c *WordFrequencyComputation) encodeOutput(data WordFrequency) (cmpt.BinaryData, error) {
	// Note that a gob encoder cannot be reused by a worker. On the first
	// encoding, it encodes type information of WordFrequency, i.e
	// map[string]int, and assigns a type id. In subsequent encodings only the
	// type id is encoded. A gob decoder running in a late joining coordinator
	// cannot decode the unknown type id of such encodings as the initial type
	// information has been missed.
	encodeBuf := &bytes.Buffer{}
	encoder := gob.NewEncoder(encodeBuf)
	if err := encoder.Encode(data); err != nil {
		return nil, err
	}
	bytes := encodeBuf.Bytes()
	return bytes, nil
}

func (c *WordFrequencyComputation) decodeOutput(output cmpt.BinaryData) (WordFrequency, error) {
	// Note that a gob decoder cannot be reused by a coordinator, as a worker
	// uses a new gob encoder for each encoding (see encodeOutput). If a gob
	// decoder receives an already known type information (and not its type id
	// only), it will error with "duplicate type received".
	decodeBuf := &bytes.Buffer{}
	decodeBuf.Write(output)
	decoder := gob.NewDecoder(decodeBuf)

	var data WordFrequency
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}
