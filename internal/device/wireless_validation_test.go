package device

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"ADBKit/internal/core"
)

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
		{"ipv6 zone", "[fe80::1%eth0]:5555", "[fe80::1%eth0]:5555", false},
		{"dns label and digits", "MY-TV-1:5555", "MY-TV-1:5555", false},
		{"trailing root dot", "android.local.:5555", "android.local:5555", false},
		{"single label", "tv:1", "tv:1", false},
		{"surrounding whitespace is trimmed", " 192.168.1.20:5555 ", "192.168.1.20:5555", false},
		{"missing port", "192.168.1.20", "", true},
		{"empty host", ":5555", "", true},
		{"port too high", "192.168.1.20:70000", "", true},
		{"port zero", "192.168.1.20:0", "", true},
		{"signed port", "192.168.1.20:+5555", "", true},
		{"underscore host", "bad_host:5555", "", true},
		{"leading hyphen label", "-lead:5555", "", true},
		{"trailing hyphen label", "trail-:5555", "", true},
		{"empty dns label", "dou..ble:5555", "", true},
		{"non ascii host", "café:5555", "", true},
		{"path in host", "192.168.1.20/nope:5555", "", true},
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
	for _, value := range []string{"123456", "000001", "000000", "000999", "999999"} {
		if !isSixDigitPairingCode(value) {
			t.Fatalf("expected %q to be accepted", value)
		}
	}
	for _, value := range []string{"", "12345", "1234567", "12a456", "123 456", "123456\n", " 123456", "123456 ", "１２３４５６"} {
		if isSixDigitPairingCode(value) {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestPairArgsKeepExactPairingCodeIncludingLeadingZeros(t *testing.T) {
	got := pairArgs("192.168.1.20:37121", "000123")
	want := []string{"pair", "192.168.1.20:37121", "000123"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pairArgs() = %#v, want %#v", got, want)
	}
}

func TestPairRejectsNonExactCodeBeforeResolvingAdb(t *testing.T) {
	resolved := false
	service := NewWirelessService(t.TempDir(), func() core.BinaryPaths {
		resolved = true
		return core.BinaryPaths{Adb: "adb"}
	})

	for _, code := range []string{"", "12345", "1234567", " 123456", "123456 ", "00012a"} {
		if _, err := service.Pair(context.Background(), "192.168.1.20:37121", code); err == nil {
			t.Fatalf("Pair accepted pairing code %q", code)
		}
	}
	if resolved {
		t.Fatal("Pair resolved the adb tool before validating the pairing code")
	}
}

func TestRedactPairingCodeKeepsSecretsOutOfDetails(t *testing.T) {
	detail := "adb: failed to pair 192.168.1.20:37121 with code 000123"
	got := redactPairingCode(detail, "000123")
	if strings.Contains(got, "000123") {
		t.Fatalf("pairing code leaked into %q", got)
	}
	if !strings.Contains(got, "******") {
		t.Fatalf("expected a redaction marker in %q", got)
	}
	if clean := redactPairingCode("nothing secret", "000123"); clean != "nothing secret" {
		t.Fatalf("unrelated detail was altered: %q", clean)
	}
}

func TestEnableTCPIPArgsPinConfirmedSerial(t *testing.T) {
	got := enableTCPIPArgs("192.168.1.20:37121", "5555")
	want := []string{"-s", "192.168.1.20:37121", "tcpip", "5555"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("enableTCPIPArgs() = %#v, want %#v", got, want)
	}
}

func TestEnableTCPIPRejectsInvalidSerialOrPortBeforeResolvingAdb(t *testing.T) {
	resolved := false
	service := NewWirelessService(t.TempDir(), func() core.BinaryPaths {
		resolved = true
		return core.BinaryPaths{Adb: "adb"}
	})

	cases := []struct {
		serial string
		port   string
	}{
		{serial: "   ", port: "5555"},
		{serial: "SERIAL-1", port: "0"},
		{serial: "SERIAL-1", port: "65536"},
		{serial: "SERIAL-1", port: "+5555"},
		{serial: "SERIAL-1", port: "not-a-port"},
	}
	for _, tc := range cases {
		if _, err := service.EnableTCPIP(context.Background(), tc.serial, tc.port); err == nil {
			t.Fatalf("EnableTCPIP accepted serial=%q port=%q", tc.serial, tc.port)
		}
	}
	if resolved {
		t.Fatal("EnableTCPIP resolved the adb tool for invalid input")
	}
}
