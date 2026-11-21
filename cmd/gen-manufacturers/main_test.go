package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

// TestWriteCodesMatchesCommittedJSON checks that writeCodes reproduces the
// committed codes.json byte for byte from its own contents, i.e. that the
// output does not depend on map iteration order.
func TestWriteCodesMatchesCommittedJSON(t *testing.T) {
	want, err := os.ReadFile("../../echonet/spec/manufacturers/codes.json")
	if err != nil {
		t.Fatalf("read codes.json: %v", err)
	}

	var codes map[string]string
	if err := json.Unmarshal(want, &codes); err != nil {
		t.Fatalf("unmarshal codes.json: %v", err)
	}

	for i := 0; i < 3; i++ {
		var got bytes.Buffer
		if err := writeCodes(&got, codes); err != nil {
			t.Fatalf("writeCodes() error = %v", err)
		}
		if !bytes.Equal(got.Bytes(), want) {
			t.Fatalf("writeCodes() output differs from committed codes.json")
		}
	}
}
