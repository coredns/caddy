// Copyright 2015 Light Code Labs, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package caddyfile

import (
	"encoding/json"
	"log"
	"strings"
	"testing"
	"unicode/utf8"
)

type lexerTestCase struct {
	input    string
	expected []Token
}

func TestLexer(t *testing.T) {
	testCases := []lexerTestCase{
		{
			input: `host:123`,
			expected: []Token{
				{Line: 1, Text: "host:123"},
			},
		},
		{
			input: `host:123

					directive`,
			expected: []Token{
				{Line: 1, Text: "host:123"},
				{Line: 3, Text: "directive"},
			},
		},
		{
			input: `host:123 {
						directive
					}`,
			expected: []Token{
				{Line: 1, Text: "host:123"},
				{Line: 1, Text: "{"},
				{Line: 2, Text: "directive"},
				{Line: 3, Text: "}"},
			},
		},
		{
			input: `host:123 { directive }`,
			expected: []Token{
				{Line: 1, Text: "host:123"},
				{Line: 1, Text: "{"},
				{Line: 1, Text: "directive"},
				{Line: 1, Text: "}"},
			},
		},
		{
			input: `host:123 {
						#comment
						directive
						# comment
						foobar # another comment
					}`,
			expected: []Token{
				{Line: 1, Text: "host:123"},
				{Line: 1, Text: "{"},
				{Line: 3, Text: "directive"},
				{Line: 5, Text: "foobar"},
				{Line: 6, Text: "}"},
			},
		},
		{
			input: `a "quoted value" b
					foobar`,
			expected: []Token{
				{Line: 1, Text: "a"},
				{Line: 1, Text: "quoted value"},
				{Line: 1, Text: "b"},
				{Line: 2, Text: "foobar"},
			},
		},
		{
			input: `A "quoted \"value\" inside" B`,
			expected: []Token{
				{Line: 1, Text: "A"},
				{Line: 1, Text: `quoted "value" inside`},
				{Line: 1, Text: "B"},
			},
		},
		{
			input: `"don't\escape"`,
			expected: []Token{
				{Line: 1, Text: `don't\escape`},
			},
		},
		{
			input: `"don't\\escape"`,
			expected: []Token{
				{Line: 1, Text: `don't\\escape`},
			},
		},
		{
			input: `A "quoted value with line
					break inside" {
						foobar
					}`,
			expected: []Token{
				{Line: 1, Text: "A"},
				{Line: 1, Text: "quoted value with line\n\t\t\t\t\tbreak inside"},
				{Line: 2, Text: "{"},
				{Line: 3, Text: "foobar"},
				{Line: 4, Text: "}"},
			},
		},
		{
			input: `"C:\php\php-cgi.exe"`,
			expected: []Token{
				{Line: 1, Text: `C:\php\php-cgi.exe`},
			},
		},
		{
			input: `empty "" string`,
			expected: []Token{
				{Line: 1, Text: `empty`},
				{Line: 1, Text: ``},
				{Line: 1, Text: `string`},
			},
		},
		{
			input: "skip those\r\nCR characters",
			expected: []Token{
				{Line: 1, Text: "skip"},
				{Line: 1, Text: "those"},
				{Line: 2, Text: "CR"},
				{Line: 2, Text: "characters"},
			},
		},
		{
			input: "\xEF\xBB\xBF:8080", // test with leading byte order mark
			expected: []Token{
				{Line: 1, Text: ":8080"},
			},
		},
	}

	for i, testCase := range testCases {
		actual := tokenize(testCase.input)
		lexerCompare(t, i, testCase.expected, actual)
		checkOffsets(t, i, testCase.input, actual)
	}
}

func TestLexerOffsets(t *testing.T) {
	testCases := []lexerTestCase{
		{
			input: "a bb  ccc",
			expected: []Token{
				{Line: 1, Offset: 0, Text: "a"},
				{Line: 1, Offset: 2, Text: "bb"},
				{Line: 1, Offset: 6, Text: "ccc"},
			},
		},
		{
			input: "host:123 {\n\tdir arg\n}",
			expected: []Token{
				{Line: 1, Offset: 0, Text: "host:123"},
				{Line: 1, Offset: 9, Text: "{"},
				{Line: 2, Offset: 12, Text: "dir"},
				{Line: 2, Offset: 16, Text: "arg"},
				{Line: 3, Offset: 20, Text: "}"},
			},
		},
		{
			input: `a "b c" d`,
			expected: []Token{
				{Line: 1, Offset: 0, Text: "a"},
				{Line: 1, Offset: 2, Text: "b c"},
				{Line: 1, Offset: 8, Text: "d"},
			},
		},
		{
			input: `x "a\"b" y`,
			expected: []Token{
				{Line: 1, Offset: 0, Text: "x"},
				{Line: 1, Offset: 2, Text: `a"b`},
				{Line: 1, Offset: 9, Text: "y"},
			},
		},
		{
			input: `"don't\escape" z`,
			expected: []Token{
				{Line: 1, Offset: 0, Text: `don't\escape`},
				{Line: 1, Offset: 15, Text: "z"},
			},
		},
		{
			input: `a "" b`,
			expected: []Token{
				{Line: 1, Offset: 0, Text: "a"},
				{Line: 1, Offset: 2, Text: ""},
				{Line: 1, Offset: 5, Text: "b"},
			},
		},
		{
			input: "a \"b\nc\" d",
			expected: []Token{
				{Line: 1, Offset: 0, Text: "a"},
				{Line: 1, Offset: 2, Text: "b\nc"},
				{Line: 2, Offset: 8, Text: "d"},
			},
		},
		{
			input: "\"a\r\nb\" c",
			expected: []Token{
				{Line: 1, Offset: 0, Text: "a\r\nb"},
				{Line: 2, Offset: 7, Text: "c"},
			},
		},
		{
			input: `a "bc`,
			expected: []Token{
				{Line: 1, Offset: 0, Text: "a"},
				{Line: 1, Offset: 2, Text: "bc"},
			},
		},
		{
			input: "# c1\nfoo # c2\n bar",
			expected: []Token{
				{Line: 2, Offset: 5, Text: "foo"},
				{Line: 3, Offset: 15, Text: "bar"},
			},
		},
		{
			input: "a#b c",
			expected: []Token{
				{Line: 1, Offset: 0, Text: "a"},
				{Line: 1, Offset: 4, Text: "c"},
			},
		},
		{
			input: "a\r\nb\r\n  c",
			expected: []Token{
				{Line: 1, Offset: 0, Text: "a"},
				{Line: 2, Offset: 3, Text: "b"},
				{Line: 3, Offset: 8, Text: "c"},
			},
		},
		{
			input: "a\rb c",
			expected: []Token{
				{Line: 1, Offset: 0, Text: "ab"},
				{Line: 1, Offset: 4, Text: "c"},
			},
		},
		{
			input: "é ü x",
			expected: []Token{
				{Line: 1, Offset: 0, Text: "é"},
				{Line: 1, Offset: 3, Text: "ü"},
				{Line: 1, Offset: 6, Text: "x"},
			},
		},
		{
			input: "a\u3000b",
			expected: []Token{
				{Line: 1, Offset: 0, Text: "a"},
				{Line: 1, Offset: 4, Text: "b"},
			},
		},
		{
			input: "\xff a",
			expected: []Token{
				{Line: 1, Offset: 0, Text: "\uFFFD"},
				{Line: 1, Offset: 2, Text: "a"},
			},
		},
		{
			input: "\xEF\xBB\xBF:8080 x",
			expected: []Token{
				{Line: 1, Offset: 3, Text: ":8080"},
				{Line: 1, Offset: 9, Text: "x"},
			},
		},
	}

	for i, testCase := range testCases {
		actual := tokenize(testCase.input)
		if len(actual) != len(testCase.expected) {
			t.Errorf("Test case %d: expected %d token(s) but got %d", i, len(testCase.expected), len(actual))
		}
		for j := 0; j < len(actual) && j < len(testCase.expected); j++ {
			if actual[j] != testCase.expected[j] {
				t.Errorf("Test case %d token %d: expected %+v but got %+v", i, j, testCase.expected[j], actual[j])
			}
		}
		checkOffsets(t, i, testCase.input, actual)
	}
}

func TestTokenJSONOmitsOffset(t *testing.T) {
	b, err := json.Marshal(Token{File: "f", Line: 2, Text: "t", Offset: 5})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(b), `{"File":"f","Line":2,"Text":"t"}`; got != want {
		t.Errorf("expected %s but got %s", want, got)
	}
}

func checkOffsets(t *testing.T, n int, input string, tokens []Token) {
	for i, tok := range tokens {
		if i > 0 && tok.Offset <= tokens[i-1].Offset {
			t.Errorf("Test case %d token %d: offset %d does not follow previous offset %d",
				n, i, tok.Offset, tokens[i-1].Offset)
		}
		if tok.Offset < 0 || tok.Offset >= len(input) {
			t.Errorf("Test case %d token %d: offset %d out of range", n, i, tok.Offset)
			continue
		}
		src, _ := utf8.DecodeRuneInString(input[tok.Offset:])
		first, _ := utf8.DecodeRuneInString(tok.Text)
		if src != '"' && src != first {
			t.Errorf("Test case %d token %d ('%s'): offset %d points at %q",
				n, i, tok.Text, tok.Offset, src)
		}
	}
}

func tokenize(input string) (tokens []Token) {
	l := lexer{}
	if err := l.load(strings.NewReader(input)); err != nil {
		log.Printf("[ERROR] load failed: %v", err)
	}
	for l.next() {
		tokens = append(tokens, l.token)
	}
	return
}

func lexerCompare(t *testing.T, n int, expected, actual []Token) {
	if len(expected) != len(actual) {
		t.Errorf("Test case %d: expected %d token(s) but got %d", n, len(expected), len(actual))
	}

	for i := 0; i < len(actual) && i < len(expected); i++ {
		if actual[i].Line != expected[i].Line {
			t.Errorf("Test case %d token %d ('%s'): expected line %d but was line %d",
				n, i, expected[i].Text, expected[i].Line, actual[i].Line)
			break
		}
		if actual[i].Text != expected[i].Text {
			t.Errorf("Test case %d token %d: expected text '%s' but was '%s'",
				n, i, expected[i].Text, actual[i].Text)
			break
		}
	}
}
