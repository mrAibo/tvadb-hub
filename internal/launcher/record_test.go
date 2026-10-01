package launcher

import (
	"ADBKit/internal/core"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validRecord(id string, state State) Record {
	return Record{
		SchemaVersion:      SchemaVersion,
		ID:                 id,
		Serial:             testSerial,
		UserID:             0,
		Action:             ActionSetHome,
		State:              state,
		CreatedAt:          "2026-10-01T00:00:00Z",
		UpdatedAt:          "2026-10-01T00:00:01Z",
		OriginalComponent:  testOriginal,
		CandidateComponent: mustNormalize(testCandidate),
	}
}

func TestRecordRoundTripAndStrictValidation(t *testing.T) {
	service := NewService(t.TempDir(), func() core.BinaryPaths { return core.BinaryPaths{Adb: "adb"} })
	store := service.store()

	if err := store.saveNew(validRecord("op-roundtrip", StatePending)); err != nil {
		t.Fatalf("saveNew: %v", err)
	}
	loaded, err := store.load("op-roundtrip")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded != validRecord("op-roundtrip", StatePending) {
		t.Fatalf("round trip changed the record: %+v", loaded)
	}

	path := store.path("op-roundtrip")
	base, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		mutate  func(map[string]any)
		payload string
	}{
		{name: "unknown field", mutate: func(document map[string]any) { document["extra"] = true }},
		{name: "wrong schema version", mutate: func(document map[string]any) { document["schemaVersion"] = SchemaVersion + 1 }},
		{name: "unknown state", mutate: func(document map[string]any) { document["state"] = "bulldozed" }},
		{name: "unsupported action", mutate: func(document map[string]any) { document["action"] = "uninstall-stock" }},
		{name: "non normalized component", mutate: func(document map[string]any) { document["originalComponent"] = "com.stock.launcher/.HomeActivity" }},
		{name: "package-only component", mutate: func(document map[string]any) { document["candidateComponent"] = "com.custom.launcher" }},
		{name: "invalid serial", mutate: func(document map[string]any) { document["serial"] = "bad serial; rm -rf" }},
		{name: "non zero user", mutate: func(document map[string]any) { document["userId"] = 10 }},
		{name: "short operation id", mutate: func(document map[string]any) { document["id"] = "short" }},
		{name: "trailing data", payload: string(base) + "{}"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := []byte(tc.payload)
			if payload == nil {
				var document map[string]any
				if err := json.Unmarshal(base, &document); err != nil {
					t.Fatal(err)
				}
				tc.mutate(document)
				encoded, err := json.Marshal(document)
				if err != nil {
					t.Fatal(err)
				}
				payload = encoded
			}
			if err := os.WriteFile(path, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.load("op-roundtrip"); err == nil {
				t.Fatalf("%s was accepted", tc.name)
			}
		})
	}

	if err := os.WriteFile(path, base, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.load("op-roundtrip"); err != nil {
		t.Fatalf("restored record must load: %v", err)
	}
}

func TestSaveNewRejectsDuplicateOperationID(t *testing.T) {
	service := NewService(t.TempDir(), nil)
	if err := service.store().saveNew(validRecord("op-duplicate1", StatePlanned)); err != nil {
		t.Fatal(err)
	}
	err := service.store().saveNew(validRecord("op-duplicate1", StatePending))
	if err == nil || !strings.Contains(err.Error(), "already has a durable record") {
		t.Fatalf("a duplicate operation ID must be refused: %v", err)
	}
	loaded, loadErr := service.store().load("op-duplicate1")
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if loaded.State != StatePlanned {
		t.Fatalf("the existing record was overwritten: %+v", loaded)
	}
}

func TestStoreRejectsUnsafeRecordIDs(t *testing.T) {
	service := NewService(t.TempDir(), nil)
	for _, id := range []string{"../escape", "a/b", `a\b`, "short", "", "with space"} {
		if _, err := service.store().load(id); err == nil {
			t.Fatalf("record ID %q was accepted", id)
		}
	}
}

func TestManualCommandQuotesHostAndRemoteShells(t *testing.T) {
	innerClass := "com.example.launcher/com.example.launcher.Main$Inner"
	record := validRecord("op-manual001", StateUnknown)
	record.OriginalComponent = innerClass

	posix := record.manualCommand("adb", "linux")
	quoteEscape := `'"'"'`
	wantPOSIX := "adb -s 'SERIAL-1' shell 'cmd package set-home-activity --user 0 " + quoteEscape + innerClass + quoteEscape + "'"
	if posix != wantPOSIX {
		t.Fatalf("posix manual command = %q, want %q", posix, wantPOSIX)
	}
	if !strings.Contains(posix, "Main$Inner") {
		t.Fatalf("the inner-class name was mangled: %q", posix)
	}

	windows := record.manualCommand("C:\\tools\\adb.exe", "windows")
	wantWindows := "C:\\tools\\adb.exe -s '" + testSerial + "' shell 'cmd package set-home-activity --user 0 ''" + innerClass + "'''"
	if windows != wantWindows {
		t.Fatalf("windows manual command = %q, want %q", windows, wantWindows)
	}
	if !strings.Contains(windows, "$Inner") {
		t.Fatalf("the inner-class name was expanded: %q", windows)
	}
}

func TestNormalizeComponentContract(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "short class", input: "com.example.launcher/.MainActivity", want: "com.example.launcher/com.example.launcher.MainActivity"},
		{name: "full class", input: "com.example.launcher/com.example.launcher.MainActivity", want: "com.example.launcher/com.example.launcher.MainActivity"},
		{name: "inner class", input: "com.example.launcher/.Main$Inner", want: "com.example.launcher/com.example.launcher.Main$Inner"},
		{name: "package only", input: "com.example.launcher", wantErr: true},
		{name: "empty class", input: "com.example.launcher/", wantErr: true},
		{name: "shell text", input: "com.example.launcher/.Main; rm -rf /sdcard", wantErr: true},
		{name: "substring", input: "example", wantErr: true},
		{name: "empty", input: "   ", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeComponent(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("normalizeComponent(%q) accepted an invalid value as %q", tc.input, got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("normalizeComponent(%q) = %q, %v; want %q", tc.input, got, err, tc.want)
			}
		})
	}
}

func TestChooserDetection(t *testing.T) {
	choosers := []string{
		"android/com.android.internal.app.ResolverActivity",
		"com.android.intentresolver/.ChooserActivity",
		"com.example.home/.ResolverActivity",
	}
	for _, component := range choosers {
		if !isChooserOrResolver(component) {
			t.Fatalf("%s must be treated as a platform dispatcher", component)
		}
	}
	if isChooserOrResolver(testCandidate) || isChooserOrResolver(testOriginal) {
		t.Fatal("real launchers must not be treated as dispatchers")
	}
}

func TestRecoveryCatalogIsLocalAndReadOnly(t *testing.T) {
	device := newFakeDevice()
	service := newTestService(t, device)

	pending := validRecord("op-offline01", StatePending)
	if err := service.store().persist(pending); err != nil {
		t.Fatal(err)
	}
	other := validRecord("op-offline02", StateApplied)
	other.Serial = "SERIAL-2"
	if err := service.store().persist(other); err != nil {
		t.Fatal(err)
	}

	recovery, err := service.ReadRecovery(testSerial)
	if err != nil {
		t.Fatalf("ReadRecovery: %v", err)
	}
	if len(recovery.Records) != 1 {
		t.Fatalf("recovery listing = %+v", recovery.Records)
	}
	entry := recovery.Records[0]
	if entry.ID != "op-offline01" || entry.Serial != testSerial || !entry.BlocksApply || !entry.NeedsRecovery {
		t.Fatalf("unexpected recovery entry: %+v", entry)
	}
	if !strings.Contains(entry.ManualCommand, testSerial) || !strings.Contains(entry.ManualCommand, testOriginal) {
		t.Fatalf("manual command must be serial-pinned and name the original component: %q", entry.ManualCommand)
	}
	if len(device.recorded()) != 0 {
		t.Fatalf("recovery browsing must not touch the device: %v", device.commands())
	}

	all, err := service.ReadRecovery("")
	if err != nil || len(all.Records) != 2 {
		t.Fatalf("unfiltered recovery = %+v, %v", all, err)
	}
}

func TestRecoveryReportsUnreadableRecordsInsteadOfSkipping(t *testing.T) {
	service := NewService(t.TempDir(), nil)
	if err := os.MkdirAll(service.store().dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(service.store().dir, "op-broken01.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.store().persist(validRecord("op-good0001", StateApplied)); err != nil {
		t.Fatal(err)
	}

	recovery, err := service.ReadRecovery("")
	if err != nil {
		t.Fatalf("ReadRecovery: %v", err)
	}
	var broken *RecordSummary
	for i := range recovery.Records {
		if recovery.Records[i].Unreadable {
			broken = &recovery.Records[i]
		}
	}
	if broken == nil || broken.File != "op-broken01.json" {
		t.Fatalf("an unreadable record must be reported: %+v", recovery.Records)
	}
	if len(recovery.Records) != 2 {
		t.Fatalf("expected both records, got %+v", recovery.Records)
	}
}

func TestPendingAndUnknownRecordsBlockAnotherApply(t *testing.T) {
	for _, state := range []State{StatePending, StateUnknown, StateRestorePending} {
		t.Run(string(state), func(t *testing.T) {
			device := newFakeDevice()
			service := newTestService(t, device)
			if err := service.store().persist(validRecord("op-blocks001", state)); err != nil {
				t.Fatal(err)
			}
			_, err := service.Apply(context.Background(), ApplyRequest{
				OperationID: "op-newapply1", ExpectedSerial: testSerial,
				ExpectedComponent: testOriginal, CandidateComponent: testCandidate,
			})
			if err == nil || !strings.Contains(err.Error(), "unresolved launcher operation") {
				t.Fatalf("state %s must block a new apply: %v", state, err)
			}
			if device.sawCommand("set-home-activity") {
				t.Fatalf("a blocked apply must not reach the device: %v", device.commands())
			}
		})
	}
}

func TestUnreadableRecordAlsoBlocksApply(t *testing.T) {
	device := newFakeDevice()
	service := newTestService(t, device)
	if err := os.MkdirAll(service.store().dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(service.store().dir, "op-corrupt01.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := service.Apply(context.Background(), ApplyRequest{
		OperationID: "op-newapply2", ExpectedSerial: testSerial,
		ExpectedComponent: testOriginal, CandidateComponent: testCandidate,
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be read") {
		t.Fatalf("an unreadable record must block the apply: %v", err)
	}
	if device.sawCommand("set-home-activity") {
		t.Fatalf("no device change may happen while records are unreadable: %v", device.commands())
	}
}

func TestRestoreUsesLiveToolAndRecordedComponents(t *testing.T) {
	device := newFakeDevice()
	service := newTestService(t, device)

	record := validRecord("op-restore01", StateApplied)
	record.ToolPath = filepath.Join(t.TempDir(), "hostile-adb") // must never be executed
	if err := service.store().persist(record); err != nil {
		t.Fatal(err)
	}
	device.current = mustNormalize(testCandidate) // the candidate is the current HOME

	result, err := service.Restore(context.Background(), RestoreRequest{ExpectedSerial: testSerial, RecordID: "op-restore01"})
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if !result.Verified || result.State != StateRestored || result.CurrentComponent != testOriginal {
		t.Fatalf("unexpected restore result: %+v", result)
	}
	for _, req := range device.recorded() {
		if req.Command != "adb-pinned" {
			t.Fatalf("a recorded tool path was used for execution: %+v", req)
		}
		if req.Args[1] != testSerial {
			t.Fatalf("restore was not pinned to the expected serial: %+v", req)
		}
		if req.Timeout <= 0 {
			t.Fatalf("restore command without a finite timeout: %+v", req)
		}
	}
	loaded, err := service.store().load("op-restore01")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.State != StateRestored {
		t.Fatalf("restore did not persist the outcome: %+v", loaded)
	}
}

func TestRestoreRefusesForeignRecordAndUnrelatedHome(t *testing.T) {
	t.Run("foreign record", func(t *testing.T) {
		device := newFakeDevice()
		service := newTestService(t, device)
		record := validRecord("op-foreign01", StateApplied)
		record.Serial = "SERIAL-9"
		if err := service.store().persist(record); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Restore(context.Background(), RestoreRequest{ExpectedSerial: testSerial, RecordID: "op-foreign01"}); err == nil {
			t.Fatal("a record of another device must not be restored")
		}
		if device.sawCommand("set-home-activity") {
			t.Fatalf("a refused restore must not reach the device: %v", device.commands())
		}
	})

	t.Run("home moved elsewhere", func(t *testing.T) {
		device := newFakeDevice()
		service := newTestService(t, device)
		if err := service.store().persist(validRecord("op-moved001", StateApplied)); err != nil {
			t.Fatal(err)
		}
		device.current = testOther
		result, err := service.Restore(context.Background(), RestoreRequest{ExpectedSerial: testSerial, RecordID: "op-moved001"})
		if err == nil {
			t.Fatal("an unrelated HOME must not be overwritten")
		}
		if result.State != StateUnknown {
			t.Fatalf("unexpected restore result: %+v", result)
		}
		if !strings.Contains(result.ManualCommand, testSerial) || !strings.Contains(result.ManualCommand, testOriginal) {
			t.Fatalf("manual recovery must be serial-pinned and name the original: %q", result.ManualCommand)
		}
		if !strings.Contains(err.Error(), "manual recovery") {
			t.Fatalf("the failure must carry the manual recovery path: %v", err)
		}
		if device.sawCommand("set-home-activity") {
			t.Fatalf("the unrelated HOME must not be bulldozed: %v", device.commands())
		}
	})
}

func TestRestoreKeepsUnknownWhenRollbackCannotBeVerified(t *testing.T) {
	device := newFakeDevice()
	service := newTestService(t, device)
	if err := service.store().persist(validRecord("op-rbfail001", StateApplied)); err != nil {
		t.Fatal(err)
	}
	device.current = mustNormalize(testCandidate)
	device.failMutation = true // the restore command fails

	result, err := service.Restore(context.Background(), RestoreRequest{ExpectedSerial: testSerial, RecordID: "op-rbfail001"})
	if err == nil {
		t.Fatal("a failed restore must not be reported as success")
	}
	if result.State != StateUnknown || !strings.Contains(result.Detail, "manual recovery") {
		t.Fatalf("unexpected restore result: %+v", result)
	}
	loaded, loadErr := service.store().load("op-rbfail001")
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if loaded.State != StateUnknown {
		t.Fatalf("record state = %s, want unknown", loaded.State)
	}
}

func TestRepositoryStartupNeverTouchesADevice(t *testing.T) {
	device := newFakeDevice()
	service := newTestService(t, device)
	if err := service.store().persist(validRecord("op-startup01", StatePending)); err != nil {
		t.Fatal(err)
	}
	// Constructing the service and listing recovery is all a startup may do.
	constructed := NewService(service.dataDir, func() core.BinaryPaths { return core.BinaryPaths{Adb: "adb-pinned"} })
	constructed.runCommand = device.runCtx()
	if _, err := constructed.ReadRecovery(testSerial); err != nil {
		t.Fatal(err)
	}
	if len(device.recorded()) != 0 {
		t.Fatalf("startup/recovery issued device commands: %v", device.commands())
	}
}
