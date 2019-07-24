// Copyright 2019 Carleton University Library All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"reflect"
	"testing"
)

func TestProcessLine(t *testing.T) {
	var tests = []struct {
		line  string
		bibID uint32
		exlID uint64
		error bool
	}{
		{"", 0, 0, true},
		{"0,b0", 0, 0, false},
		{"1,b1-", 1, 1, false},
		{"1,b-", 0, 0, true},
		{"1,-", 0, 0, true},
		{"invalid,a0-", 0, 0, true},
		{"0,invalid", 0, 0, true},
		{"900000000000000001,b1000001-01suffix,", 1000001, 900000000000000001, false},
		{"900000000000000001,b1000001-01suffix,,,,,", 1000001, 900000000000000001, false},
		{"900000000000000001,b1000001-01suffix", 1000001, 900000000000000001, false},
		{"18446744073709551615,b4294967295-01suffix,", 4294967295, 18446744073709551615, false},
		{"18446744073709551616,b4294967296-01suffix,", 0, 0, true},
		{"-1,a-1", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			bibID, exlID, err := processLine(tt.line)

			if tt.error && err == nil {
				t.Fatalf("processLine(\"%v\") should have returned an error, but it did not.\n", tt.line)
			}
			if !tt.error && err != nil {
				t.Fatalf("processLine(\"%v\") should not have returned an error, but it did: %v.\n", tt.line, err)
			}
			if (bibID != tt.bibID) || (exlID != tt.exlID) {
				t.Fatalf("processLine(\"%v\") returned %v, %v, not %v, %v", tt.line, bibID, exlID, tt.bibID, tt.exlID)
			}
		})
	}
}

func TestDecodeAdvancedSearch(t *testing.T) {
	var tests = []struct {
		search string
		terms  []string
	}{
		{"", []string{}},
		{"()", []string{}},
		{"(())", []string{}},
		{"))", []string{}},
		{"((", []string{}},
		{"spiders", []string{"any,contains,spiders,AND"}},
		{"(spiders)", []string{"any,contains,spiders,AND"}},
		{"t:(spiders)", []string{"title,contains,spiders,AND"}},
		{"(t:(spiders))", []string{"title,contains,spiders,AND"}},
		{"(spiders) and (snakes)", []string{"any,contains,spiders,AND", "any,contains,snakes,AND"}},
		{"(and) and (or)", []string{"any,contains,and,AND", "any,contains,or,AND"}},
		{"((spiders) and (snakes))", []string{"any,contains,spiders,AND", "any,contains,snakes,AND"}},
		{"(((spiders) and (snakes)))", []string{"any,contains,spiders,AND", "any,contains,snakes,AND"}},
		{"(((spiders) and snakes))", []string{"any,contains,spiders,AND", "any,contains,snakes,AND"}},
		{"(t:(spiders) and a:(lee))", []string{"title,contains,spiders,AND", "creator,contains,lee,AND"}},
		{"t:(spiders) and a:(lee)", []string{"title,contains,spiders,AND", "creator,contains,lee,AND"}},
		{"(t:(spiders) and not d:(biology))", []string{"title,contains,spiders,NOT", "sub,contains,biology,AND"}},
		{"(t:spiders) and not (d:biology)", []string{"any,contains,t:spiders,NOT", "any,contains,d:biology,AND"}},
		{"(t:spiders) or d:(biology)", []string{"any,contains,t:spiders,OR", "sub,contains,biology,AND"}},
		{"(t:(spiders) AND NOT d:(biology))", []string{"title,contains,spiders,NOT", "sub,contains,biology,AND"}},
		{"(t:(spiders) OR d:(biology))", []string{"title,contains,spiders,OR", "sub,contains,biology,AND"}},
		{"(t:(spiders) or a:(lee))", []string{"title,contains,spiders,OR", "creator,contains,lee,AND"}},
		{"((t:(spiders) or a:(lee)))", []string{"title,contains,spiders,OR", "creator,contains,lee,AND"}},
		{"(((t:(spiders) or a:(lee))))", []string{"title,contains,spiders,OR", "creator,contains,lee,AND"}},
		{"(t:(and) or a:(lee))", []string{"title,contains,and,OR", "creator,contains,lee,AND"}},
		{"(t:(spiders and snakes) and not d:(biology) or a:(lee))", []string{"title,contains,spiders and snakes,NOT", "sub,contains,biology,OR", "creator,contains,lee,AND"}},
	}

	for _, tt := range tests {
		t.Run(tt.search, func(t *testing.T) {
			terms := decodeAdvancedSearch(tt.search)
			if !reflect.DeepEqual(terms, tt.terms) {
				t.Fatalf("processLine(\"%v\") returned %v not %v", tt.search, terms, tt.terms)
			}
		})
	}
}
