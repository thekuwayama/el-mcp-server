package echonet

import (
	"bytes"
	"context"
	"errors"
	"net"
	"reflect"
	"testing"
	"time"
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

func TestIsMulticastCandidate(t *testing.T) {
	cases := []struct {
		name  string
		flags net.Flags
		want  bool
	}{
		{"ethernet / wifi", net.FlagUp | net.FlagBroadcast | net.FlagMulticast | net.FlagRunning, true},
		{"down", net.FlagBroadcast | net.FlagMulticast, false},
		{"loopback", net.FlagUp | net.FlagLoopback | net.FlagMulticast, false},
		{"no multicast", net.FlagUp | net.FlagBroadcast, false},
		// macOS VPN tunnels (utun) are point-to-point and also have MULTICAST.
		{"point-to-point (utun)", net.FlagUp | net.FlagPointToPoint | net.FlagMulticast | net.FlagRunning, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isMulticastCandidate(c.flags); got != c.want {
				t.Errorf("isMulticastCandidate(%v) = %v, want %v", c.flags, got, c.want)
			}
		})
	}
}

func TestNewGetRequestMultipleEPCs(t *testing.T) {
	frame := NewGetRequest(0x1234, 0x027E01, 0x80, 0xDA, 0xE4)
	want := []byte{
		0x10, 0x81, // EHD
		0x12, 0x34, // TID
		0x05, 0xFF, 0x01, // SEOJ (controller)
		0x02, 0x7E, 0x01, // DEOJ
		0x62,       // ESV Get
		0x03,       // OPC
		0x80, 0x00, // EPC, PDC
		0xDA, 0x00,
		0xE4, 0x00,
	}
	got, err := frame.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Encode() = % X, want % X", got, want)
	}
}

func TestParseGetResponse(t *testing.T) {
	epcs := []byte{0x80, 0xDA, 0xE4}

	tests := []struct {
		name    string
		data    []byte
		want    map[byte][]byte
		wantErr bool
	}{
		{
			name: "Get_Res with all EPCs",
			data: []byte{
				0x10, 0x81, 0x12, 0x34, 0x02, 0x7E, 0x01, 0x05, 0xFF, 0x01,
				0x72, 0x03,
				0x80, 0x01, 0x30,
				0xDA, 0x01, 0x42,
				0xE4, 0x01, 0x50,
			},
			want: map[byte][]byte{0x80: {0x30}, 0xDA: {0x42}, 0xE4: {0x50}},
		},
		{
			name: "Get_SNA omits EPCs with PDC 0",
			data: []byte{
				0x10, 0x81, 0x12, 0x34, 0x02, 0x7E, 0x01, 0x05, 0xFF, 0x01,
				0x52, 0x03,
				0x80, 0x01, 0x30,
				0xDA, 0x00,
				0xE4, 0x01, 0x50,
			},
			want: map[byte][]byte{0x80: {0x30}, 0xE4: {0x50}},
		},
		{
			name: "Get_SNA with no readable EPC",
			data: []byte{
				0x10, 0x81, 0x12, 0x34, 0x02, 0x7E, 0x01, 0x05, 0xFF, 0x01,
				0x52, 0x03,
				0x80, 0x00,
				0xDA, 0x00,
				0xE4, 0x00,
			},
			want: map[byte][]byte{},
		},
		{
			name: "EPCs not requested are ignored",
			data: []byte{
				0x10, 0x81, 0x12, 0x34, 0x02, 0x7E, 0x01, 0x05, 0xFF, 0x01,
				0x72, 0x02,
				0x80, 0x01, 0x30,
				0x9F, 0x01, 0x00,
			},
			want: map[byte][]byte{0x80: {0x30}},
		},
		{
			name: "unexpected ESV",
			data: []byte{
				0x10, 0x81, 0x12, 0x34, 0x02, 0x7E, 0x01, 0x05, 0xFF, 0x01,
				0x71, 0x01,
				0x80, 0x00,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := Decode(tt.data)
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			got, err := parseGetResponse(resp, epcs)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseGetResponse() = %v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGetResponse() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseGetResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseGetPropertyResponse(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		want      []byte
		wantErr   bool
		wantNotAv bool
	}{
		{
			name: "Get_Res",
			data: []byte{
				0x10, 0x81, 0x12, 0x34, 0x02, 0x88, 0x01, 0x05, 0xFF, 0x01,
				0x72, 0x01,
				0xE4, 0x02, 0x00, 0x01,
			},
			want: []byte{0x00, 0x01},
		},
		{
			name: "Get_SNA",
			data: []byte{
				0x10, 0x81, 0x12, 0x34, 0x02, 0x88, 0x01, 0x05, 0xFF, 0x01,
				0x52, 0x01,
				0xE4, 0x00,
			},
			wantErr:   true,
			wantNotAv: true,
		},
		{
			name: "unexpected ESV",
			data: []byte{
				0x10, 0x81, 0x12, 0x34, 0x02, 0x88, 0x01, 0x05, 0xFF, 0x01,
				0x71, 0x01,
				0xE4, 0x00,
			},
			wantErr: true,
		},
		{
			name: "EPC missing from Get_Res",
			data: []byte{
				0x10, 0x81, 0x12, 0x34, 0x02, 0x88, 0x01, 0x05, 0xFF, 0x01,
				0x72, 0x01,
				0xE2, 0x01, 0x00,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := Decode(tt.data)
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			got, err := parseGetPropertyResponse(resp, 0xE4)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseGetPropertyResponse() = %v, want error", got)
				}
				if errors.Is(err, ErrPropertyNotAvailable) != tt.wantNotAv {
					t.Errorf("errors.Is(%v, ErrPropertyNotAvailable) = %v, want %v", err, !tt.wantNotAv, tt.wantNotAv)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGetPropertyResponse() error = %v", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("parseGetPropertyResponse() = %X, want %X", got, tt.want)
			}
		})
	}
}

func TestGetPropertyGetSNAMessage(t *testing.T) {
	resp := &Frame{ESV: ESVGetSNA, Props: []Property{{EPC: 0xE4}}}
	_, err := parseGetPropertyResponse(resp, 0xE4)
	if want := "device returned Get_SNA (EPC E4 not available)"; err == nil || err.Error() != want {
		t.Errorf("parseGetPropertyResponse(Get_SNA) error = %v, want %q", err, want)
	}
}

func TestGetPropertiesNoEPCs(t *testing.T) {
	got, err := GetProperties(context.Background(), "192.0.2.1", 0x027E01, 0)
	if err != nil {
		t.Fatalf("GetProperties() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("GetProperties() = %v, want empty", got)
	}
}

func TestReceiveDeadline(t *testing.T) {
	// Without a ctx deadline, the timeout decides.
	before := time.Now()
	got := receiveDeadline(context.Background(), 5*time.Second)
	if got.Before(before.Add(5*time.Second)) || got.After(time.Now().Add(5*time.Second)) {
		t.Errorf("receiveDeadline(no ctx deadline) = %v, want about now+5s", got)
	}

	// An earlier ctx deadline wins over the timeout.
	ctxDeadline := time.Now().Add(time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), ctxDeadline)
	defer cancel()
	if got := receiveDeadline(ctx, 5*time.Second); !got.Equal(ctxDeadline) {
		t.Errorf("receiveDeadline(earlier ctx deadline) = %v, want %v", got, ctxDeadline)
	}

	// A later ctx deadline leaves the timeout in place.
	ctx2, cancel2 := context.WithDeadline(context.Background(), time.Now().Add(time.Hour))
	defer cancel2()
	if got := receiveDeadline(ctx2, 5*time.Second); got.After(time.Now().Add(5 * time.Second)) {
		t.Errorf("receiveDeadline(later ctx deadline) = %v, want about now+5s", got)
	}
}

func TestCancelOnDoneUnblocksRead(t *testing.T) {
	// Loopback with an ephemeral port: nothing is sent on the LAN.
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatalf("SetDeadline: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	stop := cancelOnDone(ctx, conn)
	defer stop()

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, _, err = conn.ReadFromUDP(make([]byte, 16))
	if err == nil {
		t.Fatal("ReadFromUDP returned without error, want deadline error")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("ReadFromUDP took %v after cancel, want it to return promptly", elapsed)
	}
}

func TestSendAndDiscoverWithCanceledContext(t *testing.T) {
	// An already-canceled ctx must return before any socket is opened.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := Send(ctx, "192.0.2.1", NewGetRequest(1, 0x027E01, 0x80), time.Second); !errors.Is(err, context.Canceled) {
		t.Errorf("Send(canceled ctx) error = %v, want context.Canceled", err)
	}
	if _, err := Discover(ctx, time.Second); !errors.Is(err, context.Canceled) {
		t.Errorf("Discover(canceled ctx) error = %v, want context.Canceled", err)
	}
}
