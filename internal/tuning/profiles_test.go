package tuning

import (
	"ADBKit/internal/device"
	"testing"
)

func TestProfileMatchingPrefersExactTVProfile(t *testing.T) {
	info := device.Info{
		Model: "SHARP 4K UHDTV", Codename: "maniatika",
		Manufacturer: "Sharp", IsTV: true,
	}
	profiles := ProfilesForDevice(info)
	if len(profiles) == 0 {
		t.Fatal("expected profiles")
	}
	if profiles[0].ID != "sharp-google-tv-maniatika" || !profiles[0].Recommended {
		t.Fatalf("unexpected recommendation: %+v", profiles[0])
	}
}

func TestPhoneDoesNotReceiveTVOnlyProfile(t *testing.T) {
	info := device.Info{Model: "AFTKRT", Manufacturer: "Amazon", IsTV: false}
	for _, profile := range ProfilesForDevice(info) {
		if profile.ID == "fire-tv-karat" {
			t.Fatal("TV-only profile matched a non-TV device")
		}
	}
}

func TestSamsungGetsBrandProfileAndGenericFallback(t *testing.T) {
	info := device.Info{Manufacturer: "Samsung", Brand: "samsung", Model: "SM-S928B"}
	profiles := ProfilesForDevice(info)
	var samsung, generic bool
	for _, profile := range profiles {
		samsung = samsung || profile.ID == "samsung-android"
		generic = generic || profile.ID == "generic-google-optional"
	}
	if !samsung || !generic {
		t.Fatalf("expected Samsung and generic profiles, got %+v", profiles)
	}
}

func TestDangerousRulesAreNeverDefaultSelected(t *testing.T) {
	for _, profile := range builtinProfiles {
		for _, rule := range profile.Rules {
			if (rule.Risk == RiskDangerous || rule.Risk == RiskBlocked) && rule.DefaultSelected {
				t.Fatalf("%s/%s is high risk but default-selected", profile.ID, rule.PackageName)
			}
		}
	}
}

func TestKeepListDoesNotOverlapActionableRules(t *testing.T) {
	for _, profile := range builtinProfiles {
		keep := map[string]struct{}{}
		for _, pkg := range profile.Keep {
			keep[pkg] = struct{}{}
		}
		for _, rule := range profile.Rules {
			if _, ok := keep[rule.PackageName]; ok && rule.Risk != RiskBlocked {
				t.Fatalf("%s has package in both keep and actionable rule list: %s", profile.ID, rule.PackageName)
			}
		}
	}
}
