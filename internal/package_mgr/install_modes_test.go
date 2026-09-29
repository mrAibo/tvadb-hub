package packagemgr

import (
	"reflect"
	"testing"
)

func TestParseInstallMode(t *testing.T) {
	tests := []struct {
		input string
		want  InstallMode
		ok    bool
	}{
		{"install", InstallModeInstall, true},
		{"replace", InstallModeReplace, true},
		{"downgrade", InstallModeDowngrade, true},
		{" RePlAcE ", InstallModeReplace, true},
		{"", InstallModeReplace, true},
		{"unknown", "", false},
	}

	for _, tc := range tests {
		got, err := parseInstallMode(tc.input)
		if tc.ok && err != nil {
			t.Fatalf("parseInstallMode(%q) returned error: %v", tc.input, err)
		}
		if !tc.ok {
			if err == nil {
				t.Fatalf("parseInstallMode(%q) unexpectedly succeeded", tc.input)
			}
			continue
		}
		if got != tc.want {
			t.Fatalf("parseInstallMode(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestBuildInstallArgs(t *testing.T) {
	tests := []struct {
		name string
		mode InstallMode
		want []string
	}{
		{
			name: "clean install",
			mode: InstallModeInstall,
			want: []string{"-s", "tv:1234", "install", "app.apk"},
		},
		{
			name: "replace update",
			mode: InstallModeReplace,
			want: []string{"-s", "tv:1234", "install", "-r", "app.apk"},
		},
		{
			name: "downgrade",
			mode: InstallModeDowngrade,
			want: []string{"-s", "tv:1234", "install", "-r", "-d", "app.apk"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildInstallArgs("tv:1234", "app.apk", tc.mode)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("buildInstallArgs() = %#v, want %#v", got, tc.want)
			}
		})
	}
}
