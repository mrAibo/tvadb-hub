package device

import "testing"

func TestValidateWirelessEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"ipv4", "192.168.1.20:37121", "192.168.1.20:37121", false},
		{"hostname", "android.local:5555", "android.local:5555", false},
		{"ipv6", "[fe80::1234]:42629", "[fe80::1234]:42629", false},
		{"missing port", "192.168.1.20", "", true},
		{"port too high", "192.168.1.20:70000", "", true},
		{"newline", "192.168.1.20:5555\nmalicious", "", true},
		{"space", "192.168.1.20:5555 extra", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateWirelessEndpoint(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateWirelessEndpoint(%q) err=%v wantErr=%v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("validateWirelessEndpoint(%q)=%q want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSixDigitPairingCode(t *testing.T) {
	for _, value := range []string{"123456", "000001", "999999"} {
		if !isSixDigitPairingCode(value) {
			t.Fatalf("expected %q to be accepted", value)
		}
	}
	for _, value := range []string{"", "12345", "1234567", "12a456", "123 456", "123456\n"} {
		if isSixDigitPairingCode(value) {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}
