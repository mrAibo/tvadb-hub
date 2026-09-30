package device

import "testing"

func TestResolveTVRemoteKey(t *testing.T) {
	cases := []struct {
		input   string
		wantKey string
		wantID  string
	}{
		{"home", "KEYCODE_HOME", "home"},
		{"UP", "KEYCODE_DPAD_UP", "up"},
		{"volume down", "KEYCODE_VOLUME_DOWN", "volume_down"},
		{"play-pause", "KEYCODE_MEDIA_PLAY_PAUSE", "play_pause"},
		{"ok", "KEYCODE_DPAD_CENTER", "ok"},
		{"enter", "KEYCODE_DPAD_CENTER", "enter"},
		{"wake", "KEYCODE_WAKEUP", "wake"},
		{"paste", "KEYCODE_PASTE", "paste"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			gotKey, gotID, ok := resolveTVRemoteKey(tc.input)
			if !ok {
				t.Fatalf("resolveTVRemoteKey(%q) returned !ok", tc.input)
			}
			if gotKey != tc.wantKey || gotID != tc.wantID {
				t.Fatalf(
					"resolveTVRemoteKey(%q) = (%q, %q), want (%q, %q)",
					tc.input,
					gotKey,
					gotID,
					tc.wantKey,
					tc.wantID,
				)
			}
		})
	}
}

func TestResolveTVRemoteKeyRejectsUnknownInput(t *testing.T) {
	if keycode, normalized, ok := resolveTVRemoteKey("rm -rf /"); ok {
		t.Fatalf("unexpected key accepted: %q %q", keycode, normalized)
	}
}
