package tuning

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

type feedTransport func(*http.Request) (*http.Response, error)

func (f feedTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func feedTestRevisions(t *testing.T) (FeedConfig, func(uint64) []byte) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	config := FeedConfig{URL: "https://example.com/feed", PublicKey: base64.StdEncoding.EncodeToString(public)}
	return config, func(revision uint64) []byte {
		p := validFeedPayloadForTest()
		p.Revision = revision
		data, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		signed, err := SignFeedPayload(data, private)
		if err != nil {
			t.Fatal(err)
		}
		return signed
	}
}

func TestRefreshDiscardsDownloadAfterTrustIsDisabled(t *testing.T) {
	config, sign := feedTestRevisions(t)
	data := sign(1)
	var current atomic.Value
	current.Store(config)
	started, release := make(chan struct{}), make(chan struct{})
	manager := newFeedManager(t.TempDir())
	manager.setResolver(func() FeedConfig { return current.Load().(FeedConfig) })
	manager.client = &http.Client{Transport: feedTransport(func(*http.Request) (*http.Response, error) {
		close(started)
		<-release
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data))}, nil
	})}
	done := make(chan error, 1)
	go func() { _, err := manager.refresh(context.Background()); done <- err }()
	<-started
	current.Store(FeedConfig{})
	disabled := manager.statusSnapshot()
	if disabled.Active || disabled.Configured {
		t.Fatalf("disable did not take effect: %+v", disabled)
	}
	close(release)
	if err := <-done; err == nil {
		t.Fatal("old trusted download was accepted after disable")
	}
	if status := manager.statusSnapshot(); status.Active || status.Configured {
		t.Fatalf("download reactivated feed: %+v", status)
	}
	if _, err := os.Stat(manager.currentPathFor(config)); !os.IsNotExist(err) {
		t.Fatalf("stale download touched cache: %v", err)
	}
}

func TestConcurrentRefreshesCommitInOrderWithoutStaleDowngrade(t *testing.T) {
	config, sign := feedTestRevisions(t)
	data1, data2 := sign(1), sign(2)
	manager := newFeedManager(t.TempDir())
	manager.setResolver(func() FeedConfig { return config })
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	manager.client = &http.Client{Transport: feedTransport(func(*http.Request) (*http.Response, error) {
		data := data2
		if calls.Add(1) == 1 {
			close(started)
			<-release
			data = data1
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data))}, nil
	})}
	done := make(chan error, 2)
	go func() { _, err := manager.refresh(context.Background()); done <- err }()
	<-started
	go func() { _, err := manager.refresh(context.Background()); done <- err }()
	select {
	case err := <-done:
		t.Fatalf("second refresh bypassed transaction lock: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if status := manager.statusSnapshot(); status.Revision != 2 {
		t.Fatalf("stale revision active: %+v", status)
	}
	data, err := os.ReadFile(manager.currentPathFor(config))
	if err != nil {
		t.Fatal(err)
	}
	verified, err := verifySignedFeed(data, config.PublicKey)
	if err != nil || verified.payload.Revision != 2 {
		t.Fatalf("cache not coherent: %+v %v", verified, err)
	}
	manager.client = &http.Client{Transport: feedTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data1))}, nil
	})}
	if _, err := manager.refresh(context.Background()); err == nil {
		t.Fatal("remote downgrade accepted")
	}
}
