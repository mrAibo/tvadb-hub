package core

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func recoveryTestRunner(t *testing.T, outputs []string) CommandRunner {
	t.Helper()
	i := 0
	return func(_ context.Context, req ExecRequest) (*ExecResult, error) {
		if req.Command != "adb-A" || len(req.Args) < 3 || req.Args[0] != "-s" || req.Args[1] != "A" || req.Args[2] != "shell" || !strings.Contains(strings.Join(req.Args, " "), "--user 0") {
			t.Fatalf("unpinned target/user/path: %+v", req)
		}
		if i >= len(outputs) {
			t.Fatal("unexpected extra read")
		}
		out := outputs[i]
		i++
		return &ExecResult{Stdout: out}, nil
	}
}

func TestAndroidRecoveryPackagesProtectUnknownOEMsAndFallbacks(t *testing.T) {
	outputs := []string{"com.oem.home/.Home\r\ncom.backup.launcher/com.backup.launcher.Home\r\n", "com.oem.home/.Home\n", "com.oem.keyboard/.IME\n", "com.oem.keyboard/.IME;-1;123:com.backup.keyboard/.IME;456\n"}
	got, err := AndroidRecoveryPackages(context.Background(), "adb-A", "A", recoveryTestRunner(t, outputs))
	want := map[string]struct{}{"com.oem.home": {}, "com.backup.launcher": {}, "com.oem.keyboard": {}, "com.backup.keyboard": {}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("protection=%+v err=%v", got, err)
	}
}

func TestAndroidRecoveryPackagesFailClosedOnUnknownOutput(t *testing.T) {
	valid := []string{"com.oem.home/.Home", "com.oem.home/.Home", "com.oem.keyboard/.IME", "com.oem.keyboard/.IME;1"}
	for i := range valid {
		for _, invalid := range []string{"", "Unknown command", "No activities found", "com.oem.home/.Home\npermission denied", "com.oem/.Home;unexpected"} {
			outputs := append([]string{}, valid...)
			outputs[i] = invalid
			if _, err := AndroidRecoveryPackages(context.Background(), "adb-A", "A", recoveryTestRunner(t, outputs)); err == nil {
				t.Fatalf("accepted read %d output %q", i, invalid)
			}
		}
	}
	outputs := append([]string{}, valid...)
	outputs[1] = "com.oem.home/.Home\ncom.other.home/.Home"
	if _, err := AndroidRecoveryPackages(context.Background(), "adb-A", "A", recoveryTestRunner(t, outputs)); err == nil {
		t.Fatal("accepted ambiguous current HOME")
	}
}

func TestAndroidRecoveryPackagesAllowExplicitNullIME(t *testing.T) {
	got, err := AndroidRecoveryPackages(context.Background(), "adb-A", "A", recoveryTestRunner(t, []string{"com.oem.home/.Home", "android/com.android.internal.app.ResolverActivity", "null\n", "null\n"}))
	if err != nil || len(got) != 2 {
		t.Fatalf("explicit absence=%+v err=%v", got, err)
	}
}

func TestAndroidRecoveryPackagesRejectExecutionErrorsAndCancellation(t *testing.T) {
	for _, result := range []*ExecResult{nil, {ExitCode: 1}, {Stdout: "com.oem.home/.Home", Stderr: "permission denied"}, {Stdout: strings.Repeat("x", 64*1024+1)}} {
		if _, err := AndroidRecoveryPackages(context.Background(), "adb-A", "A", func(context.Context, ExecRequest) (*ExecResult, error) { return result, nil }); err == nil {
			t.Fatalf("accepted invalid result %+v", result)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, err := AndroidRecoveryPackages(ctx, "adb-A", "A", func(context.Context, ExecRequest) (*ExecResult, error) {
		cancel()
		return &ExecResult{Stdout: "com.oem.home/.Home"}, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation hidden: %v", err)
	}
}

func TestAndroidComponentPackageRejectsNonComponents(t *testing.T) {
	for _, invalid := range []string{"", "com.example", "com.example/", "com.example/.Home extra", "com.example/.Home\n", "com.example/.Home;command", "com.example//Home", "../.Home"} {
		if _, err := AndroidComponentPackage(invalid); err == nil {
			t.Fatalf("accepted %q", invalid)
		}
	}
	for _, valid := range []string{"android/com.android.Home", "com.oem.home/.Main", "com.oem.home/com.oem.Home$Alias"} {
		if _, err := AndroidComponentPackage(valid); err != nil {
			t.Fatalf("rejected %q: %v", valid, err)
		}
	}
}
