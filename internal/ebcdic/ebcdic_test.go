package ebcdic

import (
	"testing"
)

func TestRoundTrip(t *testing.T) {
	ascii := "Hello, World!\nLine 2\n"
	ebcdicBytes := ToEBCDIC([]byte(ascii))
	back := ToASCII(ebcdicBytes)
	if string(back) != ascii {
		t.Errorf("round trip failed: got %q, want %q", string(back), ascii)
	}
}

func TestIsEBCDIC(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{
			name: "ASCII text",
			data: []byte("Hello World\nThis is ASCII\n"),
			want: false,
		},
		{
			name: "EBCDIC text",
			data: ToEBCDIC([]byte("Hello World\nThis is EBCDIC\n")),
			want: true,
		},
		{
			name: "empty",
			data: []byte{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsEBCDIC(tt.data); got != tt.want {
				t.Errorf("IsEBCDIC() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKnownMappings(t *testing.T) {
	// Verify key character mappings
	mappings := map[byte]byte{
		0x40: 0x20, // EBCDIC space → ASCII space
		0x4B: 0x2E, // EBCDIC . → ASCII .
		0xC1: 0x41, // EBCDIC A → ASCII A
		0xD1: 0x4A, // EBCDIC J → ASCII J
		0xE2: 0x53, // EBCDIC S → ASCII S
		0x81: 0x61, // EBCDIC a → ASCII a
		0x91: 0x6A, // EBCDIC j → ASCII j
		0xA2: 0x73, // EBCDIC s → ASCII s
		0xF0: 0x30, // EBCDIC 0 → ASCII 0
		0xF9: 0x39, // EBCDIC 9 → ASCII 9
		0x15: 0x0A, // EBCDIC NL → ASCII LF
		0x25: 0x0A, // EBCDIC LF → ASCII LF
	}

	for eb, want := range mappings {
		got := toASCIITable[eb]
		if got != want {
			t.Errorf("toASCII[0x%02X] = 0x%02X, want 0x%02X", eb, got, want)
		}
	}
}
