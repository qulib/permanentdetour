// Copyright 2019 Carleton University Library All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
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
