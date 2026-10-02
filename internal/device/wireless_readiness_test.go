package device

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func readyConnectService() MDNSService {
	return MDNSService{
		InstanceName: "adb-tv",
		Kind:         MDNSServiceConnect,
		Address:      "192.168.1.20:37121",
		Host:         "192.168.1.20",
		Port:         "37121",
		Secure:       true,
	}
}

// TestDiscoverWithReadinessRetryWaitsForInitiallyEmptyResult covers the Android 12
// pairing-screen case: the first query legitimately returns nothing, and the
// bounded retry must pick up the service as soon as adb publishes it.
func TestDiscoverWithReadinessRetryWaitsForInitiallyEmptyResult(t *testing.T) {
	calls := 0
	services, err := discoverWithReadinessRetry(context.Background(), func(context.Context) ([]MDNSService, error) {
		calls++
		if calls < 2 {
			return nil, nil
		}
		return []MDNSService{readyConnectService()}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected exactly one retry, got %d attempts", calls)
	}
	if len(services) != 1 || services[0].Host != "192.168.1.20" {
		t.Fatalf("unexpected retry result: %#v", services)
	}
}

func TestDiscoverWithReadinessRetryIsBoundedAndStaysEmpty(t *testing.T) {
	calls := 0
	services, err := discoverWithReadinessRetry(context.Background(), func(context.Context) ([]MDNSService, error) {
		calls++
		return nil, nil
	})
	if err != nil {
		t.Fatalf("an empty discovery is not an error: %v", err)
	}
	if len(services) != 0 {
		t.Fatalf("expected no services, got %#v", services)
	}
	if calls != mdnsReadyAttempts {
		t.Fatalf("expected %d bounded attempts, got %d", mdnsReadyAttempts, calls)
	}
}

func TestDiscoverWithReadinessRetryReportsRealErrorsImmediately(t *testing.T) {
	boom := errors.New("adb exploded")
	calls := 0
	_, err := discoverWithReadinessRetry(context.Background(), func(context.Context) ([]MDNSService, error) {
		calls++
		return nil, boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected the real discovery error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("a real error must not be retried, got %d attempts", calls)
	}
}

func TestDiscoverWithReadinessRetryHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	_, err := discoverWithReadinessRetry(ctx, func(context.Context) ([]MDNSService, error) {
		calls++
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected cancellation instead of an empty result")
	}
	if !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("expected the cancellation to be reported, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("cancellation must stop polling immediately, got %d attempts", calls)
	}
}

func TestDiscoverWithReadinessRetryReturnsServicesWithoutWaiting(t *testing.T) {
	calls := 0
	services, err := discoverWithReadinessRetry(context.Background(), func(context.Context) ([]MDNSService, error) {
		calls++
		return []MDNSService{readyConnectService()}, nil
	})
	if err != nil || calls != 1 || len(services) != 1 {
		t.Fatalf("expected a single immediate successful query, calls=%d services=%#v err=%v", calls, services, err)
	}
}
