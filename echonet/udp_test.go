package echonet

import (
	"reflect"
	"testing"
)

func TestParseDiscoverResponse(t *testing.T) {
	const tid = uint16(0x1234)
	// Instance list with two EOJs: home air conditioner 013001 and storage battery 027D01.
	instanceList := []byte{0x02, 0x01, 0x30, 0x01, 0x02, 0x7D, 0x01}

	frame := func(tid uint16, esv byte, props ...Property) []byte {
		data, err := (&Frame{
			TID:   tid,
			SEOJ:  NodeProfileEOJ,
			DEOJ:  ControllerEOJ,
			ESV:   esv,
			Props: props,
		}).Encode()
		if err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
		return data
	}

	tests := []struct {
		name     string
		data     []byte
		wantEOJs []uint32
		wantOK   bool
	}{
		{
			name:     "Get_Res with matching TID",
			data:     frame(tid, ESVGetRes, Property{EPC: 0xD6, EDT: instanceList}),
			wantEOJs: []uint32{0x013001, 0x027D01},
			wantOK:   true,
		},
		{
			name:     "Get_Res with empty instance list",
			data:     frame(tid, ESVGetRes, Property{EPC: 0xD6, EDT: []byte{0x00}}),
			wantEOJs: []uint32{},
			wantOK:   true,
		},
		{
			name: "Get_Res with different TID (late response to another request)",
			data: frame(tid+1, ESVGetRes, Property{EPC: 0xD6, EDT: instanceList}),
		},
		{
			name: "Get_SNA with matching TID",
			data: frame(tid, ESVGetSNA, Property{EPC: 0xD6}),
		},
		{
			name: "INF instance list notification",
			data: frame(tid, ESVInf, Property{EPC: 0xD5, EDT: instanceList}),
		},
		{
			name: "Get_Res without EPC 0xD6",
			data: frame(tid, ESVGetRes, Property{EPC: 0x80, EDT: []byte{0x30}}),
		},
		{
			name: "undecodable packet",
			data: []byte{0x10, 0x81, 0x12},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eojs, ok := parseDiscoverResponse(tt.data, tid)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if !reflect.DeepEqual(eojs, tt.wantEOJs) {
				t.Errorf("eojs = %06X, want %06X", eojs, tt.wantEOJs)
			}
		})
	}
}
