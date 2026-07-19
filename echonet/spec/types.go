package spec

import "fmt"

// DeviceClass represents an ECHONET Lite device object class.
type DeviceClass struct {
	GroupCode   byte
	ClassCode   byte
	NameJP      string
	NameEN      string
	Description string
	EPCs        []EPCDef
}

// EOJHex returns the EOJ class code as a 4-character hex string (e.g. "0130").
func (d DeviceClass) EOJHex() string {
	return fmt.Sprintf("%02X%02X", d.GroupCode, d.ClassCode)
}

// EPCDef represents an ECHONET Property Code definition.
type EPCDef struct {
	Code        byte
	NameJP      string
	NameEN      string
	DataType    string
	Unit        string
	AccessRules string // e.g. "Get", "Set/Get", "Anno/Get"
	Description string
}
