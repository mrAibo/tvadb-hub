package app

import (
	"runtime"
	"runtime/debug"

	"ADBKit/internal/core"
)

// AppInfo contains local build/runtime metadata for the About page.
// It deliberately does not perform any network requests.
type AppInfo struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	Description   string `json:"description"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	GoVersion     string `json:"goVersion"`
	WailsVersion  string `json:"wailsVersion,omitempty"`
	Repository    string `json:"repository"`
	License       string `json:"license"`
	UpstreamName  string `json:"upstreamName"`
	UpstreamURL   string `json:"upstreamUrl"`
}

func (a *App) GetAppInfo() AppInfo {
	return AppInfo{
		Name:         "DroidSphere",
		Version:      core.Version,
		Description:  "Cross-platform Android device manager powered by ADB, Fastboot and scrcpy",
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		GoVersion:    runtime.Version(),
		WailsVersion: moduleVersion("github.com/wailsapp/wails/v3"),
		Repository:   "https://github.com/mrAibo/tvadb-hub",
		License:      "MIT",
		UpstreamName: "ADBKit v2.0.0",
		UpstreamURL:  "https://github.com/Drenzzz/ADBKit",
	}
}

func moduleVersion(path string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, dep := range info.Deps {
		if dep.Path == path {
			if dep.Replace != nil && dep.Replace.Version != "" {
				return dep.Replace.Version
			}
			return dep.Version
		}
	}
	return ""
}
