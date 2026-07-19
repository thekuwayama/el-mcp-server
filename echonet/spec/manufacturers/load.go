package manufacturers

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed codes.json
var codesJSON []byte

var codes map[string]string

func init() {
	if err := json.Unmarshal(codesJSON, &codes); err != nil {
		panic(fmt.Sprintf("manufacturers: failed to load codes.json: %v", err))
	}
}

// Lookup returns the manufacturer name for a 3-byte code (e.g. [0x00, 0x00, 0x01]).
// Returns ("", false) if not found.
func Lookup(edt []byte) (string, bool) {
	if len(edt) != 3 {
		return "", false
	}
	key := strings.ToUpper(fmt.Sprintf("%02X%02X%02X", edt[0], edt[1], edt[2]))
	name, ok := codes[key]
	return name, ok
}
