// gen-manufacturers parses the ECHONET manufacturer code XLSX and writes
// echonet/spec/manufacturers/codes.json. Run from the repository root:
//
//	go run ./cmd/gen-manufacturers <path/to/list_code.xlsx>
package main

import (
	"archive/zip"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: gen-manufacturers <list_code.xlsx>")
		os.Exit(1)
	}

	codes, err := parseXLSX(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// sort keys for deterministic output
	keys := make([]string, 0, len(codes))
	for k := range codes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ordered := make(map[string]string, len(codes))
	for _, k := range keys {
		ordered[k] = codes[k]
	}

	out, err := os.Create("echonet/spec/manufacturers/codes.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer out.Close()

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(ordered); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%d entries written to echonet/spec/manufacturers/codes.json\n", len(codes))
}

// --- XLSX parsing (archive/zip + encoding/xml, no external deps) ---

type sst struct {
	Items []si `xml:"si"`
}

type si struct {
	T  string `xml:"t"`
	Rs []r    `xml:"r"`
}

func (s si) text() string {
	if s.T != "" {
		return s.T
	}
	var sb strings.Builder
	for _, run := range s.Rs {
		sb.WriteString(run.T)
	}
	return sb.String()
}

type r struct {
	T string `xml:"t"`
}

type worksheet struct {
	Rows []row `xml:"sheetData>row"`
}

type row struct {
	Cells []cell `xml:"c"`
}

type cell struct {
	Ref   string `xml:"r,attr"`
	Type  string `xml:"t,attr"`
	Value string `xml:"v"`
}

func parseXLSX(path string) (map[string]string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer zr.Close()

	var sharedStrings []string
	var sheetXML []byte

	for _, f := range zr.File {
		switch f.Name {
		case "xl/sharedStrings.xml":
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
			var s sst
			if err := xml.Unmarshal(data, &s); err != nil {
				return nil, fmt.Errorf("parse sharedStrings: %w", err)
			}
			sharedStrings = make([]string, len(s.Items))
			for i, item := range s.Items {
				sharedStrings[i] = item.text()
			}
		case "xl/worksheets/sheet1.xml":
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			sheetXML, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
		}
	}

	if sheetXML == nil {
		return nil, fmt.Errorf("sheet1.xml not found")
	}

	var ws worksheet
	if err := xml.Unmarshal(sheetXML, &ws); err != nil {
		return nil, fmt.Errorf("parse sheet: %w", err)
	}

	codes := make(map[string]string)
	for _, row := range ws.Rows {
		// use cell ref (e.g. "A5", "B5") to map to columns A=0, B=1
		colVals := map[string]string{}
		for _, c := range row.Cells {
			if c.Value == "" || len(c.Ref) == 0 {
				continue
			}
			col := strings.TrimRight(c.Ref, "0123456789")
			var v string
			if c.Type == "s" {
				idx := 0
				fmt.Sscanf(c.Value, "%d", &idx)
				if idx < len(sharedStrings) {
					v = sharedStrings[idx]
				}
			} else {
				v = c.Value
			}
			colVals[col] = v
		}
		codeVal := strings.ToUpper(colVals["A"])
		nameVal := colVals["B"]
		if len(codeVal) != 6 || nameVal == "" {
			continue
		}
		var dummy int
		if _, err := fmt.Sscanf(codeVal, "%X", &dummy); err != nil {
			continue
		}
		codes[codeVal] = nameVal
	}

	return codes, nil
}
