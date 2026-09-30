package core

import "strings"

type ScrcpyPreset struct {
	Name    string        `json:"name"`
	Options ScrcpyOptions `json:"options"`
}

const (
	ScrcpyAudioSourceOutput   = "output"
	ScrcpyAudioSourcePlayback = "playback"
	ScrcpyAudioSourceMic      = "mic"
)

type ScrcpyOptions struct {
	MaxSize            int    `json:"max_size"`
	BitRate            int    `json:"bit_rate"`
	MaxFPS             int    `json:"max_fps"`
	AudioBitRate       int    `json:"audio_bit_rate"`
	AudioCodec         string `json:"audio_codec"`
	AudioSource        string `json:"audio_source"`
	AudioOnly          bool   `json:"audio_only"`
	VideoCodec         string `json:"video_codec"`
	ShowTouches        bool   `json:"show_touches"`
	NoAudio            bool   `json:"no_audio"`
	NoControl          bool   `json:"no_control"`
	StayAwake          bool   `json:"stay_awake"`
	TurnScreenOff      bool   `json:"turn_screen_off"`
	PowerOffOnClose    bool   `json:"power_off_on_close"`
	Fullscreen         bool   `json:"fullscreen"`
	AlwaysOnTop        bool   `json:"always_on_top"`
	DisableScreensaver bool   `json:"disable_screensaver"`
	Rotation           int    `json:"rotation"`
	DisplayID          int    `json:"display_id"`
	TimeLimit          int    `json:"time_limit"`
}

func DefaultScrcpyOptions() ScrcpyOptions {
	return ScrcpyOptions{
		BitRate:      8_000_000,
		AudioBitRate: 128_000,
		AudioCodec:   "opus",
		AudioSource:  ScrcpyAudioSourceOutput,
		VideoCodec:   "h264",
		StayAwake:    true,
	}
}

type PreferencesPayload struct {
	Theme                string            `json:"theme"`
	DeviceNicknames      map[string]string `json:"device_nicknames"`
	LogcatBufferLimit    int               `json:"logcat_buffer_limit"`
	ScrcpyPresets        []ScrcpyPreset    `json:"scrcpy_presets"`
	DefaultTerminalMode  string            `json:"default_terminal_mode"`
	AutoRefreshDevices   bool              `json:"auto_refresh_devices"`
	DeviceRefreshSeconds int               `json:"device_refresh_seconds"`
	AuditEnabled         *bool             `json:"audit_enabled,omitempty"`
	FileTransferCompression string          `json:"file_transfer_compression"`
	VerifyAfterTransfer      *bool           `json:"verify_after_transfer,omitempty"`
}

type AppConfigSnapshot struct {
	AdbPath              string            `json:"adb_path"`
	FastbootPath         string            `json:"fastboot_path"`
	ScrcpyPath           string            `json:"scrcpy_path"`
	SetupCompleted       bool              `json:"setup_completed"`
	Theme                string            `json:"theme"`
	BinaryVersions       map[string]string `json:"binary_versions"`
	DeviceNicknames      map[string]string `json:"device_nicknames"`
	LogcatBufferLimit    int               `json:"logcat_buffer_limit"`
	ScrcpyOptions        ScrcpyOptions     `json:"scrcpy_options"`
	ScrcpyPresets        []ScrcpyPreset    `json:"scrcpy_presets"`
	DefaultTerminalMode  string            `json:"default_terminal_mode"`
	AutoRefreshDevices   bool              `json:"auto_refresh_devices"`
	DeviceRefreshSeconds int               `json:"device_refresh_seconds"`
	AuditEnabled         bool              `json:"audit_enabled"`
	FileTransferCompression string          `json:"file_transfer_compression"`
	VerifyAfterTransfer      bool            `json:"verify_after_transfer"`
}

type RuntimeDiagnostics struct {
	OS               string            `json:"os"`
	Arch             string            `json:"arch"`
	DataDir          string            `json:"data_dir"`
	ConfigPath       string            `json:"config_path"`
	ManagedBinaryDir string            `json:"managed_binary_dir"`
	SetupCompleted   bool              `json:"setup_completed"`
	Theme            string            `json:"theme"`
	BinaryVersions   map[string]string `json:"binary_versions"`
	Capabilities     map[string]bool   `json:"capabilities"`
}


func IsValidScrcpyAudioSource(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ScrcpyAudioSourceOutput, ScrcpyAudioSourcePlayback, ScrcpyAudioSourceMic:
		return true
	default:
		return false
	}
}

// NormalizeScrcpyOptions keeps old config files and TVADB settings backups
// compatible while ensuring newly-added audio fields have safe defaults.
func NormalizeScrcpyOptions(options ScrcpyOptions) ScrcpyOptions {
	source := strings.ToLower(strings.TrimSpace(options.AudioSource))
	if !IsValidScrcpyAudioSource(source) {
		source = ScrcpyAudioSourceOutput
	}
	options.AudioSource = source
	if options.NoAudio {
		options.AudioOnly = false
	}
	return options
}

func NormalizeScrcpyPresets(presets []ScrcpyPreset) []ScrcpyPreset {
	for i := range presets {
		presets[i].Options = NormalizeScrcpyOptions(presets[i].Options)
	}
	return presets
}
