package tuning

import (
	"ADBKit/internal/device"
	"sort"
	"strings"
)

const (
	sourceTVTweakURL    = "https://github.com/26zl/tv-tweak"
	sourceRegistryURL   = "https://github.com/PixelCode01/UIBloatwareRegistry"
	sourceADBDebloatURL = "https://github.com/farag2/ADB-Debloating"
	sourceTCLURL        = "https://github.com/seun-novodev/android-tv-debloat-toolkit"
)

func safe(pkg, label, category, reason string) PackageRule {
	return PackageRule{PackageName: pkg, Label: label, Category: category, Risk: RiskSafe, Reason: reason, DefaultSelected: true}
}

func caution(pkg, label, category, reason string) PackageRule {
	return PackageRule{PackageName: pkg, Label: label, Category: category, Risk: RiskCaution, Reason: reason}
}

func dangerous(pkg, label, category, reason string) PackageRule {
	return PackageRule{PackageName: pkg, Label: label, Category: category, Risk: RiskDangerous, Reason: reason}
}

func blocked(pkg, label, category, reason string) PackageRule {
	return PackageRule{PackageName: pkg, Label: label, Category: category, Risk: RiskBlocked, Reason: reason}
}

var builtinProfiles = []Profile{
	{
		ID:            "fire-tv-karat",
		Name:          "Fire TV Stick 4K Max (karat)",
		DeviceFamily:  "Amazon Fire TV",
		Description:   "Device-specific Fire OS 8 profile. Safe defaults target advertising, telemetry, promotional surfaces and disposable tutorials; voice, smart-home and Appstore-related packages remain opt-in.",
		SourceName:    "26zl/tv-tweak",
		SourceURL:     sourceTVTweakURL + "/tree/main/devices/firetv-stick-hd",
		SourceLicense: "MIT",
		Criteria:      MatchCriteria{Manufacturers: []string{"amazon"}, Models: []string{"aftkrt"}, Codenames: []string{"karat"}, TVOnly: true},
		Keep: []string{
			"android", "com.amazon.ale", "com.amazon.dcp", "com.amazon.device.controllermanager",
			"com.amazon.firebat", "com.amazon.fireinputdevices", "com.amazon.franktvinput",
			"com.amazon.identity.auth.device.authorization", "com.amazon.ssm", "com.amazon.ssmsys",
			"com.amazon.tcomm", "com.amazon.tcomm.client", "com.amazon.tv.ime", "com.amazon.tv.intentsupport",
			"com.amazon.tv.keypolicymanager", "com.amazon.tv.launcher", "com.amazon.tv.routing",
			"com.amazon.tv.settings.core", "com.amazon.tv.settings.v2", "com.amazon.webview.chromium",
			"com.android.systemui", "com.esaba.downloader", "com.mediatek.tvinput",
		},
		Rules: []PackageRule{
			safe("com.amazon.tv.acr", "Automatic content recognition", "Ads & telemetry", "Content recognition / ACR service."),
			safe("com.amazon.aca", "ACR companion", "Ads & telemetry", "HDMI/activity recognition companion."),
			safe("com.amazon.hybridadidservice", "Advertising ID service", "Ads & telemetry", "Amazon advertising identifier service."),
			safe("com.amazon.device.telemetry.emitter", "Telemetry emitter", "Ads & telemetry", "Telemetry emission."),
			safe("com.amazon.wirelessmetrics.service", "Wireless metrics", "Ads & telemetry", "Wireless metrics collection."),
			safe("com.amazon.perfc", "Performance collector", "Ads & telemetry", "Performance telemetry."),
			safe("com.amazon.perfcollection", "Performance collection", "Ads & telemetry", "Performance telemetry."),
			safe("com.amazon.dp.logger", "Device logger", "Ads & telemetry", "Diagnostic logging/telemetry."),
			safe("com.amazon.tv.ftvambient", "Fire TV Ambient", "Promotional", "Ad-bearing ambient screensaver."),
			safe("com.amazon.ftv.screensaver", "Fire TV Screensaver", "Promotional", "Stock screensaver surface."),
			safe("com.amazon.sneakpeek", "Sneak Peek", "Promotional", "Autoplaying promotional trailers."),
			safe("com.amazon.media.recommendations", "Media recommendations", "Promotional", "Recommendation surface."),
			safe("com.amazon.shoptv.client", "Shop TV", "Promotional", "Amazon shopping surface."),
			safe("com.amazon.shoptv.firetv.client", "Shop TV Fire TV", "Promotional", "Amazon shopping surface."),
			safe("com.amazon.minitv.android.app", "MiniTV", "Promotional", "Promotional media app."),
			safe("com.amazon.gamehub", "Game Hub", "Promotional", "Gaming promotional surface."),
			safe("com.amazon.tv.livetv", "Live TV", "Promotional", "Optional Live TV surface."),
			safe("com.amazon.tv.website_launcher", "Website launcher", "Promotional", "Promotional website launcher."),
			safe("com.amazon.bueller.music", "Amazon Music", "Promotional", "Optional preinstalled Amazon Music app."),
			safe("com.amazon.bueller.photos", "Amazon Photos", "Promotional", "Optional preinstalled Amazon Photos app."),
			safe("com.amazon.storm.lightning.tutorial", "Lightning tutorial", "Cruft", "Tutorial package."),
			safe("com.amazon.tmm.tutorial", "TMM tutorial", "Cruft", "Tutorial package."),
			safe("com.amazon.tv.releasenotes", "Release notes", "Cruft", "Release notes UI."),
			safe("com.amazon.firetv.troubleshooting", "Troubleshooting", "Cruft", "Optional troubleshooting UI."),
			safe("com.amazon.tv.support", "TV Support", "Cruft", "Optional support UI."),
			safe("com.amazon.tahoe", "Kids mode", "Cruft", "Optional kids mode."),
			safe("com.amazon.tv.legal.notices", "Legal notices", "Cruft", "Optional notices viewer."),
			safe("com.amazon.ods.kindleconnect", "Kindle Connect", "Cruft", "Unused Kindle integration on TV."),
			safe("com.amazon.tv.notificationcenter", "Notification Center", "Cruft", "Optional Fire TV notification surface."),
			safe("com.amazon.uxnotification", "UX notifications", "Cruft", "Optional notification component."),
			safe("com.amazon.systemnotices", "System notices", "Cruft", "Optional notices component."),
			safe("com.amazon.audiohome", "Audio Home", "Cruft", "Optional audio surface."),
			safe("com.amazon.dummy.alarmclock", "Dummy alarm clock", "Cruft", "Stub package."),
			safe("com.amazon.dummy.calendar", "Dummy calendar", "Cruft", "Stub package."),
			safe("com.amazon.dummy.contacts", "Dummy contacts", "Cruft", "Stub package."),
			safe("com.amazon.dummy.gallery", "Dummy gallery", "Cruft", "Stub package."),
			safe("com.amazon.dummy.music", "Dummy music", "Cruft", "Stub package."),
			safe("com.amazon.dummy.settings", "Dummy settings", "Cruft", "Stub package."),
			safe("com.amazon.dummy.soundpicker", "Dummy sound picker", "Cruft", "Stub package."),
			caution("com.amazon.tv.forcedotaupdater.v2", "Forced OTA updater", "Updates", "Disabling changes firmware update behaviour."),
			caution("com.amazon.tv.easyupgrade", "Easy Upgrade", "Updates", "Disabling changes firmware update behaviour."),
			caution("com.amazon.vizzini", "Alexa / Vizzini", "Voice", "Voice remote and Alexa functionality can stop; firmware may re-enable it."),
			caution("com.amazon.alexa.datastore.app", "Alexa datastore", "Voice", "Alexa dependency."),
			caution("com.amazon.alexamediaplayer.runtime.ftv", "Alexa media player", "Voice", "Alexa media dependency."),
			caution("com.amazon.alexadirectivebrokerservice", "Alexa directive broker", "Voice", "Alexa dependency."),
			caution("com.amazon.alexa.externalmediaplayer.fireos", "Alexa external media", "Voice", "Alexa dependency."),
			caution("com.amazon.tv.alexaalerts", "Alexa alerts", "Voice", "Alexa alerts."),
			caution("com.amazon.tv.alexanotifications", "Alexa notifications", "Voice", "Alexa notifications."),
			caution("com.amazon.tv.matter", "Matter", "Smart Home", "Matter smart-home support."),
			caution("com.amazon.tv.mattercohost", "Matter co-host", "Smart Home", "Matter smart-home support."),
			caution("com.amazon.smarthomemapviewapp", "Smart Home Map", "Smart Home", "Smart-home UI."),
			caution("com.amazon.whad", "Whisper discovery", "Smart Home", "Amazon smart-home discovery."),
			caution("com.amazon.whasettings", "Whisper settings", "Smart Home", "Amazon smart-home settings."),
			caution("com.amazon.whisperjoin.middleware.v2.np", "Whisper Join", "Smart Home", "Device onboarding/discovery."),
			caution("com.amazon.tv.ffsprovisioneeclient", "Frustration Free Setup", "Smart Home", "Amazon device provisioning."),
			caution("com.amazon.cloud9", "Silk Browser", "Aggressive", "Only disable if another browser is installed."),
			caution("com.amazon.imp", "Amazon install manager", "Aggressive", "Disabling can stop Amazon Appstore installs."),
			caution("com.amazon.ceviche", "Remote config / metrics", "Aggressive", "Remote configuration and metrics delivery."),
			blocked("com.amazon.device.software.ota", "Protected OTA service", "Protected", "Fire OS protects this package from shell disable/uninstall."),
			blocked("com.amazon.device.software.ota.override", "Protected OTA override", "Protected", "Fire OS protects this package."),
			blocked("com.amazon.client.metrics", "Protected metrics client", "Protected", "Fire OS protects this package."),
			blocked("com.amazon.device.metrics", "Protected device metrics", "Protected", "Fire OS protects this package."),
			blocked("com.amazon.device.crashmanager", "Protected crash manager", "Protected", "Fire OS protects this package."),
			blocked("com.amazon.device.logmanager", "Protected log manager", "Protected", "Fire OS protects this package."),
		},
	},
	{
		ID:            "sharp-google-tv-maniatika",
		Name:          "Sharp 4K Google TV (maniatika)",
		DeviceFamily:  "Google TV",
		Description:   "Device-specific Android 14 Google TV profile with TV input, DRM, remote and streaming keep-list protection.",
		SourceName:    "26zl/tv-tweak",
		SourceURL:     sourceTVTweakURL + "/tree/main/devices/sharp-4k-googletv",
		SourceLicense: "MIT",
		Criteria:      MatchCriteria{Manufacturers: []string{"sharp"}, Models: []string{"sharp 4k uhdtv"}, Codenames: []string{"maniatika"}, TVOnly: true},
		Keep: []string{
			"android", "com.android.providers.tv", "com.android.systemui", "com.android.tv.settings",
			"com.android.vending", "com.google.android.apps.tv.launcherx", "com.google.android.gms",
			"com.google.android.gsf", "com.google.android.inputmethod.latin", "com.google.android.marvin.talkback",
			"com.google.android.packageinstaller", "com.google.android.permissioncontroller", "com.google.android.tts",
			"com.google.android.tv.inputplayer", "com.google.android.tv.mtc.remotepairer",
			"com.google.android.tv.remote.service", "com.google.android.webview", "com.google.android.youtube.tv",
			"com.mediatek.autopair", "com.mediatek.dtv.tvinput.dvbtuner", "com.mediatek.tis",
			"com.mediatek.tv.agent", "com.mediatek.tv.oneworld.tvcenter", "com.mediatek.tv.settings",
			"com.mediatek.wwtv.mediaplayer", "com.netflix.ninja", "com.netflix.tokenmanager",
		},
		Rules: []PackageRule{
			safe("tv.anoki.acr.anokiacroptin", "Anoki ACR", "Ads & telemetry", "Automatic content recognition."),
			safe("com.mtc.uploadcsr", "Vendor report uploader", "Ads & telemetry", "Crash/usage report uploader."),
			safe("com.google.android.feedback", "Google Feedback", "Ads & telemetry", "Crash/feedback uploader."),
			safe("com.android.adservices.api", "Privacy Sandbox AdServices", "Ads & telemetry", "Advertising APIs."),
			safe("com.android.ondevicepersonalization.services", "On-device personalization", "Ads & telemetry", "Personalization service."),
			safe("com.android.federatedcompute.services", "Federated Compute", "Ads & telemetry", "Federated compute service."),
			safe("com.android.tv.feedbackconsent", "TV feedback consent", "Ads & telemetry", "Feedback consent component."),
			safe("com.google.android.partnersetup", "Google Partner Setup", "Ads & telemetry", "OEM partner hooks mainly used during setup."),
			safe("com.amazon.amazonvideo.livingroom", "Prime Video", "Promotional", "Optional preinstalled streaming app."),
			safe("com.google.android.play.games", "Google Play Games", "Promotional", "Optional games service."),
			safe("com.google.android.youtube.tvmusic", "YouTube Music TV", "Promotional", "Optional music app."),
			safe("com.mediatek.tv.retaildemo", "Retail demo", "Promotional", "Shop-floor demonstration loop."),
			safe("com.mtc.emanual", "Electronic manual", "Cruft", "On-screen manual."),
			safe("org.obs.mhegservice", "MHEG service", "Cruft", "Regional interactive TV service; verify region before applying."),
			caution("com.mediatek.tv.systemupdater", "MediaTek system updater", "Updates", "Disabling stops vendor firmware updates."),
			caution("com.android.dynsystem", "Dynamic System Update", "Updates", "Android DSU installer."),
			caution("com.google.android.katniss", "Google Assistant TV", "Voice", "Voice search and Assistant stop working."),
			caution("com.google.android.apps.mediashell", "Chromecast built-in", "Casting", "Casting from phones stops."),
			caution("com.mediatek.dialservice", "DIAL service", "Casting", "YouTube/Netflix DIAL discovery stops."),
			caution("com.mediatek.miracast", "Miracast", "Casting", "Miracast stops."),
		},
	},
	{
		ID:            "tcl-android-tv",
		Name:          "TCL Android / Google TV",
		DeviceFamily:  "TCL TV",
		Description:   "Conservative TCL TV profile. Package availability varies heavily by firmware, so only installed matches are shown.",
		SourceName:    "seun-novodev/android-tv-debloat-toolkit",
		SourceURL:     sourceTCLURL,
		SourceLicense: "MIT",
		Criteria:      MatchCriteria{Manufacturers: []string{"tcl"}, Brands: []string{"tcl"}, TVOnly: true},
		Rules: []PackageRule{
			safe("com.tcl.browser", "TCL Browser", "TCL apps", "Optional OEM browser."),
			safe("com.tcl.tv.appstore", "TCL App Store", "TCL apps", "Optional OEM app store."),
			caution("com.tcl.tv.cast", "TCL Cast", "Casting", "Disabling removes TCL casting."),
			safe("com.tcl.usercenter", "TCL User Center", "TCL services", "Optional user/account UI."),
			safe("com.tcl.tclaccount", "TCL Account", "TCL services", "Optional OEM account service."),
			safe("com.tcl.tv.tcloudaccount", "TCL Cloud Account", "TCL services", "Optional OEM cloud account."),
			safe("com.tcl.gallery", "TCL Gallery", "TCL apps", "Optional gallery."),
			safe("com.tcl.mediacenter", "TCL Media Center", "TCL apps", "Optional media center."),
			safe("com.tcl.screenadservice", "TCL Screen Ads", "Ads & telemetry", "OEM screen advertising service."),
			safe("com.tcl.screensaver", "TCL Screensaver", "Promotional", "OEM screensaver."),
			safe("com.tcl.eula", "TCL EULA", "Cruft", "EULA viewer after setup."),
			caution("com.google.android.tvrecommendations", "Android TV Recommendations", "Home screen", "Removing/disabling changes launcher recommendation rows."),
		},
	},
	{
		ID:            "samsung-android",
		Name:          "Samsung Galaxy / One UI",
		DeviceFamily:  "Samsung Android",
		Description:   "Cross-device Samsung profile with explicit safe/caution/dangerous classification. Only installed packages are shown.",
		SourceName:    "PixelCode01/UIBloatwareRegistry + farag2/ADB-Debloating",
		SourceURL:     sourceRegistryURL,
		SourceLicense: "MIT",
		Criteria:      MatchCriteria{Manufacturers: []string{"samsung"}, Brands: []string{"samsung"}},
		Rules: []PackageRule{
			safe("com.samsung.android.bixby.wakeup", "Bixby Wakeup", "Bixby", "Bixby wake-word service."),
			safe("com.samsung.android.app.spage", "Samsung Daily / Bixby Home", "Bixby", "Bixby/Samsung Daily home surface."),
			safe("com.samsung.android.app.routines", "Bixby Routines", "Bixby", "Optional routines feature."),
			safe("com.samsung.android.bixby.service", "Bixby Service", "Bixby", "Bixby feature service."),
			safe("com.samsung.android.visionintelligence", "Bixby Vision", "Bixby", "Bixby visual intelligence."),
			safe("com.samsung.android.bixby.agent", "Bixby Voice", "Bixby", "Bixby voice agent."),
			safe("com.samsung.android.email.provider", "Samsung Email", "Samsung apps", "Optional Samsung email client."),
			safe("com.sec.android.app.voicenote", "Voice Recorder", "Samsung apps", "Optional voice recorder."),
			safe("com.samsung.android.scloud", "Samsung Cloud", "Samsung services", "Optional Samsung Cloud."),
			safe("com.samsung.android.oneconnect", "SmartThings", "Samsung services", "Optional SmartThings integration."),
			safe("com.samsung.android.voc", "Samsung Members", "Samsung services", "Optional support/community app."),
			safe("com.samsung.ecomm.global", "Samsung Shop", "Samsung services", "Optional store app."),
			safe("com.vzw.hss.myverizon", "My Verizon", "Carrier", "Carrier application."),
			safe("com.att.myWireless", "myAT&T", "Carrier", "Carrier application."),
			caution("com.samsung.android.messaging", "Samsung Messages", "Samsung apps", "Messaging functionality."),
			caution("com.sec.android.app.sbrowser", "Samsung Internet", "Samsung apps", "Browser and components that may be referenced by other apps."),
			caution("com.samsung.android.calendar", "Samsung Calendar", "Samsung apps", "Calendar functionality."),
			caution("com.sec.android.app.popupcalculator", "Samsung Calculator", "Samsung apps", "Calculator functionality."),
			caution("com.samsung.android.samsungpass", "Samsung Pass", "Samsung services", "Credential/autofill ecosystem."),
			caution("com.samsung.vvm", "Visual Voicemail", "Carrier", "Carrier voicemail feature."),
			dangerous("com.samsung.android.spay", "Samsung Pay", "Security / payments", "Payment and secure-element integration; kept informational only."),
		},
	},
	{
		ID:            "xiaomi-android",
		Name:          "Xiaomi / Redmi / POCO",
		DeviceFamily:  "Xiaomi Android",
		Description:   "Conservative Xiaomi profile based on MIT registries; launchers and core ecosystem components are never selected by default.",
		SourceName:    "PixelCode01/UIBloatwareRegistry + farag2/ADB-Debloating",
		SourceURL:     sourceRegistryURL,
		SourceLicense: "MIT",
		Criteria:      MatchCriteria{Manufacturers: []string{"xiaomi", "redmi", "poco"}, Brands: []string{"xiaomi", "redmi", "poco"}},
		Rules: []PackageRule{
			safe("com.mi.android.globalpersonalassistant", "Mi Assistant", "MIUI apps", "Optional assistant/feed surface."),
			safe("com.mi.globalTrendNews", "Mi News", "MIUI apps", "Optional news surface."),
			safe("com.mi.health", "Mi Health", "MIUI apps", "Optional health app."),
			safe("com.miui.analytics", "MIUI Analytics", "Ads & telemetry", "MIUI analytics service; cross-checked against ADB-Debloating."),
			safe("com.miui.msa.global", "MIUI System Ads", "Ads & telemetry", "MIUI advertising service; cross-checked against ADB-Debloating."),
			safe("com.mi.globalbrowser", "Mi Browser", "MIUI apps", "Optional OEM browser."),
			safe("com.mi.global.shop", "Mi Community / Shop", "MIUI apps", "Optional OEM community/store surface."),
			safe("com.xiaomi.mipicks", "Mi Picks", "MIUI apps", "Optional recommendations/store surface."),
			safe("com.miui.android.fashiongallery", "Wallpaper Carousel", "MIUI apps", "Optional wallpaper recommendations."),
			caution("com.mi.android.globalFileexplorer", "Mi File Manager", "MIUI apps", "File manager; remove only with an alternative available."),
			dangerous("com.mi.android.globallauncher", "Mi Launcher", "Launcher", "Core home launcher; informational only."),
		},
	},
	{
		ID:            "google-pixel",
		Name:          "Google Pixel / AOSP-like",
		DeviceFamily:  "Google Android",
		Description:   "Optional Google packages with conservative risk classification.",
		SourceName:    "PixelCode01/UIBloatwareRegistry",
		SourceURL:     sourceRegistryURL + "/blob/main/Google/google-bloatware-list.md",
		SourceLicense: "MIT",
		Criteria:      MatchCriteria{Manufacturers: []string{"google"}, Brands: []string{"google"}},
		Rules: []PackageRule{
			safe("com.google.android.apps.videos", "Google TV", "Google apps", "Optional media store/player app."),
			safe("com.google.android.apps.podcasts", "Google Podcasts", "Google apps", "Optional/legacy podcasts app."),
			safe("com.google.android.apps.tachyon", "Google Meet", "Google apps", "Optional video calling app."),
			safe("com.google.ar.lens", "Google Lens", "Google apps", "Optional Lens integration."),
			caution("com.google.android.apps.wellbeing", "Digital Wellbeing", "Google services", "Usage controls and wellbeing features."),
			caution("com.google.android.apps.pixelmigrate", "Pixel Migrate", "Setup & migration", "Device migration helper; useful during setup/migration."),
			caution("com.google.android.apps.youtube.music", "YouTube Music", "Google apps", "Optional media app."),
			dangerous("com.google.android.apps.turbo", "Device Health Services", "Core optimization", "Adaptive battery/device health integration."),
			dangerous("com.google.android.apps.dialer", "Google Phone", "Telephony", "Core phone functionality."),
		},
	},
	{
		ID:            "oneplus-android",
		Name:          "OnePlus / OxygenOS",
		DeviceFamily:  "OnePlus Android",
		Description:   "OnePlus optional-app profile with launchers, telephony, security and framework packages protected by risk classification.",
		SourceName:    "PixelCode01/UIBloatwareRegistry",
		SourceURL:     sourceRegistryURL + "/blob/main/OnePlus/oneplus-bloatware-list.md",
		SourceLicense: "MIT",
		Criteria:      MatchCriteria{Manufacturers: []string{"oneplus"}, Brands: []string{"oneplus"}},
		Rules: []PackageRule{
			safe("com.oneplus.account", "OnePlus Account", "OnePlus services", "Optional OnePlus account service."),
			safe("com.oneplus.backuprestore", "Clone Phone", "OnePlus apps", "Optional migration tool."),
			safe("com.oneplus.calculator", "OnePlus Calculator", "OnePlus apps", "Optional calculator."),
			safe("com.oneplus.cloud", "OnePlus Cloud", "OnePlus services", "Optional cloud service."),
			safe("com.oneplus.compass", "OnePlus Compass", "OnePlus apps", "Optional compass."),
			safe("com.oneplus.filemanager", "OnePlus File Manager", "OnePlus apps", "Optional file manager; keep if no alternative."),
			safe("com.oneplus.gallery", "OnePlus Gallery", "OnePlus apps", "Optional gallery."),
			safe("com.oneplus.gamespace", "Game Space", "OnePlus apps", "Optional gaming surface."),
			safe("com.oneplus.market", "OnePlus Store", "OnePlus apps", "Optional store."),
			safe("com.oneplus.note", "OnePlus Notes", "OnePlus apps", "Optional notes app."),
			safe("com.oneplus.opsocialnetwork", "OnePlus Community", "OnePlus apps", "Optional community app."),
			safe("com.oneplus.soundrecorder", "Sound Recorder", "OnePlus apps", "Optional recorder."),
			safe("com.oneplus.weather", "OnePlus Weather", "OnePlus apps", "Optional weather app."),
			safe("net.oneplus.commonlogtool", "OnePlus Log Tool", "Diagnostics", "OEM log collection tool."),
			safe("net.oneplus.push", "OnePlus Push", "OnePlus services", "OEM push service."),
			caution("com.oneplus.contacts", "OnePlus Contacts", "Core apps", "Contacts functionality."),
			caution("com.oneplus.dialer", "OnePlus Phone", "Telephony", "Phone functionality."),
			caution("com.oneplus.launcher", "OnePlus Launcher", "Launcher", "Home launcher."),
			caution("com.oneplus.mms", "OnePlus Messages", "Telephony", "SMS/MMS functionality."),
			dangerous("com.oneplus.framework", "OnePlus Framework", "Core framework", "OEM framework."),
			dangerous("com.oneplus.security", "OnePlus Security", "Security", "OEM security framework."),
			dangerous("net.oneplus.opdiagnose", "OnePlus Diagnostics", "Core diagnostics", "System diagnostics component."),
		},
	},
	{
		ID:            "generic-google-optional",
		Name:          "Generic Android optional Google apps",
		DeviceFamily:  "Android",
		Description:   "Fallback profile for commonly preinstalled optional Google applications. Core Play Services, framework, telephony, launcher and accessibility packages are intentionally absent.",
		SourceName:    "PixelCode01/UIBloatwareRegistry + farag2/ADB-Debloating",
		SourceURL:     sourceADBDebloatURL,
		SourceLicense: "MIT",
		Criteria:      MatchCriteria{Generic: true},
		Rules: []PackageRule{
			safe("com.google.android.apps.docs", "Google Drive", "Google apps", "Optional Drive app."),
			safe("com.google.android.apps.photos", "Google Photos", "Google apps", "Optional Photos app."),
			safe("com.google.android.apps.tachyon", "Google Meet", "Google apps", "Optional video calling app."),
			safe("com.google.android.apps.podcasts", "Google Podcasts", "Google apps", "Optional/legacy podcasts app."),
			safe("com.google.ar.lens", "Google Lens", "Google apps", "Optional Lens integration."),
			caution("com.google.android.apps.maps", "Google Maps", "Google apps", "Mapping/location workflows may depend on it."),
			caution("com.google.android.gm", "Gmail", "Google apps", "Mail functionality."),
			caution("com.google.android.youtube", "YouTube", "Google apps", "YouTube app."),
			caution("com.google.android.apps.youtube.music", "YouTube Music", "Google apps", "Music app."),
			caution("com.google.android.apps.wellbeing", "Digital Wellbeing", "Google services", "Wellbeing/usage controls."),
		},
	},
}

func ProfilesForDevice(info device.Info) []ProfileSummary {
	return profilesForDevice(builtinProfiles, info)
}

func profilesForDevice(profiles []Profile, info device.Info) []ProfileSummary {
	summaries := make([]ProfileSummary, 0, len(profiles))
	maxScore := -1
	for _, profile := range profiles {
		score := profileMatchScore(profile, info)
		if score <= 0 {
			continue
		}
		if score > maxScore {
			maxScore = score
		}
		summaries = append(summaries, profileSummary(profile, score, false))
	}
	for i := range summaries {
		summaries[i].Recommended = summaries[i].MatchScore == maxScore
	}
	sort.SliceStable(summaries, func(i, j int) bool {
		if summaries[i].MatchScore != summaries[j].MatchScore {
			return summaries[i].MatchScore > summaries[j].MatchScore
		}
		return summaries[i].Name < summaries[j].Name
	})
	return summaries
}

func FindProfile(id string) (Profile, bool) {
	return findProfile(builtinProfiles, id)
}

func findProfile(profiles []Profile, id string) (Profile, bool) {
	for _, profile := range profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return Profile{}, false
}

func RecommendedProfile(info device.Info) (Profile, bool) {
	return recommendedProfile(builtinProfiles, info)
}

func recommendedProfile(profiles []Profile, info device.Info) (Profile, bool) {
	summaries := profilesForDevice(profiles, info)
	if len(summaries) == 0 {
		return Profile{}, false
	}
	return findProfile(profiles, summaries[0].ID)
}

func profileSummary(profile Profile, score int, recommended bool) ProfileSummary {
	return ProfileSummary{
		ID: profile.ID, Name: profile.Name, DeviceFamily: profile.DeviceFamily,
		Description: profile.Description, SourceName: profile.SourceName, SourceURL: profile.SourceURL,
		SourceLicense: profile.SourceLicense, MatchScore: score, Recommended: recommended,
	}
}

func profileMatchScore(profile Profile, info device.Info) int {
	if !profileEligible(profile, info) {
		return 0
	}
	c := profile.Criteria
	if c.TVOnly && !info.IsTV {
		return 0
	}
	if c.Generic {
		return 1
	}
	score := 0
	if containsAny(info.Manufacturer, c.Manufacturers) {
		score += 40
	}
	if containsAny(info.Brand, c.Brands) {
		score += 35
	}
	if containsAny(info.Model, c.Models) {
		score += 80
	}
	codename := info.Codename
	if codename == "" {
		codename = info.Device
	}
	if containsAny(codename, c.Codenames) {
		score += 100
	}
	return score
}

func containsAny(value string, needles []string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return false
	}
	for _, needle := range needles {
		needle = strings.ToLower(strings.TrimSpace(needle))
		if needle != "" && value == needle {
			return true
		}
	}
	return false
}

// Eligibility is a safety constraint; a score only ranks eligible profiles.
// Manufacturer/brand are alternative family identifiers; model/codename are
// alternative exact device identifiers. A device-specific profile needs both
// its family (when specified) and its exact identity to match.
func profileEligible(profile Profile, info device.Info) bool {
	c := profile.Criteria
	if c.TVOnly && !info.IsTV {
		return false
	}
	codename := info.Codename
	if codename == "" {
		codename = info.Device
	}
	familySpecified := len(c.Manufacturers) > 0 || len(c.Brands) > 0
	familyMatches := containsAny(info.Manufacturer, c.Manufacturers) || containsAny(info.Brand, c.Brands)
	identitySpecified := len(c.Models) > 0 || len(c.Codenames) > 0
	identityMatches := containsAny(info.Model, c.Models) || containsAny(codename, c.Codenames)
	return (!familySpecified || familyMatches) && (!identitySpecified || identityMatches) &&
		(c.Generic || familySpecified || identitySpecified)
}
