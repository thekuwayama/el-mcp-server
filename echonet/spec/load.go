package spec

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

//go:embed mra/*.json
var mraFS embed.FS

// Classes are the device classes loaded from the embedded MRA
// (Machine Readable Appendix) JSON data.
var Classes []DeviceClass

// SuperClassEPCs are the common EPCs of the Device Object Super Class,
// loaded from the embedded MRA JSON data.
var SuperClassEPCs []EPCDef

type mraText struct {
	Ja string `json:"ja"`
	En string `json:"en"`
}

type mraDevice struct {
	EOJ          string    `json:"eoj"`
	ClassName    mraText   `json:"className"`
	ElProperties []mraProp `json:"elProperties"`
}

type mraProp struct {
	EPC          string `json:"epc"`
	ValidRelease struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"validRelease"`
	PropertyName mraText `json:"propertyName"`
	AccessRule   struct {
		Get string `json:"get"`
		Set string `json:"set"`
		Inf string `json:"inf"`
	} `json:"accessRule"`
	Descriptions mraText         `json:"descriptions"`
	Data         json.RawMessage `json:"data"`
}

type mraData struct {
	Ref      string          `json:"$ref"`
	OneOf    []mraData       `json:"oneOf"`
	Type     string          `json:"type"`
	Format   string          `json:"format"`
	Unit     string          `json:"unit"`
	Multiple float64         `json:"multiple"`
	Size     int             `json:"size"`
	MinSize  int             `json:"minSize"`
	MaxSize  int             `json:"maxSize"`
	Enum     json.RawMessage `json:"enum"`
}

type mraEnumItem struct {
	EDT          string  `json:"edt"`
	Descriptions mraText `json:"descriptions"`
}

var mraDefinitions map[string]json.RawMessage

func init() {
	var defs struct {
		Definitions map[string]json.RawMessage `json:"definitions"`
	}
	mustUnmarshalFile("mra/definitions.json", &defs)
	mraDefinitions = defs.Definitions

	entries, err := mraFS.ReadDir("mra")
	if err != nil {
		panic(fmt.Sprintf("spec: read mra dir: %v", err))
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "0x") || !strings.HasSuffix(name, ".json") {
			continue
		}
		var dev mraDevice
		mustUnmarshalFile("mra/"+name, &dev)
		if dev.EOJ == "0x0000" {
			SuperClassEPCs = convertEPCs(dev.ElProperties)
			continue
		}
		Classes = append(Classes, convertDevice(dev))
	}
	sort.Slice(Classes, func(i, j int) bool { return Classes[i].EOJHex() < Classes[j].EOJHex() })
}

func mustUnmarshalFile(path string, v any) {
	b, err := mraFS.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("spec: read %s: %v", path, err))
	}
	if err := json.Unmarshal(b, v); err != nil {
		panic(fmt.Sprintf("spec: parse %s: %v", path, err))
	}
}

func convertDevice(dev mraDevice) DeviceClass {
	code, err := strconv.ParseUint(strings.TrimPrefix(dev.EOJ, "0x"), 16, 16)
	if err != nil {
		panic(fmt.Sprintf("spec: invalid eoj %q: %v", dev.EOJ, err))
	}
	return DeviceClass{
		GroupCode:   byte(code >> 8),
		ClassCode:   byte(code),
		NameJP:      dev.ClassName.Ja,
		NameEN:      dev.ClassName.En,
		Description: dev.ClassName.En,
		EPCs:        convertEPCs(dev.ElProperties),
	}
}

func convertEPCs(props []mraProp) []EPCDef {
	var epcs []EPCDef
	for _, p := range props {
		// The same EPC appears once per release range when its definition
		// changed across Appendix releases; keep only the current one.
		if p.ValidRelease.To != "latest" {
			continue
		}
		code, err := strconv.ParseUint(strings.TrimPrefix(p.EPC, "0x"), 16, 8)
		if err != nil {
			panic(fmt.Sprintf("spec: invalid epc %q: %v", p.EPC, err))
		}
		data := resolveData(p.Data)
		desc := p.Descriptions.Ja
		if s := enumSummary(data); s != "" {
			desc += "（" + s + "）"
		}
		epcs = append(epcs, EPCDef{
			Code:        byte(code),
			NameJP:      p.PropertyName.Ja,
			NameEN:      p.PropertyName.En,
			DataType:    dataTypeString(data),
			Unit:        unitString(data),
			AccessRules: accessRuleString(p.AccessRule.Get, p.AccessRule.Set, p.AccessRule.Inf),
			Description: desc,
		})
	}
	sort.Slice(epcs, func(i, j int) bool { return epcs[i].Code < epcs[j].Code })
	return epcs
}

func resolveData(raw json.RawMessage) mraData {
	var d mraData
	if err := json.Unmarshal(raw, &d); err != nil {
		panic(fmt.Sprintf("spec: parse data: %v", err))
	}
	if d.Ref != "" {
		name := d.Ref[strings.LastIndex(d.Ref, "/")+1:]
		def, ok := mraDefinitions[name]
		if !ok {
			panic(fmt.Sprintf("spec: unknown definition %q", name))
		}
		return resolveData(def)
	}
	return d
}

func dataTypeString(d mraData) string {
	if len(d.OneOf) > 0 {
		parts := make([]string, 0, len(d.OneOf))
		for _, sub := range d.OneOf {
			if sub.Ref != "" {
				sub = resolveData(json.RawMessage(mustMarshal(sub)))
			}
			parts = append(parts, dataTypeString(sub))
		}
		return strings.Join(parts, " | ")
	}
	switch d.Type {
	case "number":
		if d.Format != "" {
			return "number (" + d.Format + ")"
		}
		return "number"
	case "state":
		return fmt.Sprintf("state (%d byte)", d.Size)
	case "raw":
		if d.MinSize == d.MaxSize && d.MinSize > 0 {
			return fmt.Sprintf("raw (%d bytes)", d.MinSize)
		}
		return fmt.Sprintf("raw (%d-%d bytes)", d.MinSize, d.MaxSize)
	case "":
		return "unknown"
	default:
		return d.Type
	}
}

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("spec: marshal: %v", err))
	}
	return b
}

func unitString(d mraData) string {
	if d.Unit == "" {
		for _, sub := range d.OneOf {
			if sub.Ref != "" {
				sub = resolveData(json.RawMessage(mustMarshal(sub)))
			}
			if u := unitString(sub); u != "" {
				return u
			}
		}
		return ""
	}
	unit := d.Unit
	if unit == "Celsius" {
		unit = "℃"
	}
	if d.Multiple != 0 && d.Multiple != 1 {
		return strconv.FormatFloat(d.Multiple, 'f', -1, 64) + " " + unit
	}
	return unit
}

func accessRuleString(get, set, inf string) string {
	var parts []string
	if inf == "required" {
		parts = append(parts, "Anno")
	}
	if set != "notApplicable" {
		parts = append(parts, "Set")
	}
	if get != "notApplicable" {
		parts = append(parts, "Get")
	}
	return strings.Join(parts, "/")
}

func enumSummary(d mraData) string {
	if d.Type != "state" || len(d.Enum) == 0 {
		return ""
	}
	var items []mraEnumItem
	if err := json.Unmarshal(d.Enum, &items); err != nil {
		return ""
	}
	const maxItems = 8
	var parts []string
	for i, it := range items {
		if i >= maxItems {
			parts = append(parts, "…")
			break
		}
		parts = append(parts, it.Descriptions.Ja+"="+it.EDT)
	}
	return strings.Join(parts, ", ")
}
