package adbproto

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestListDevicesLongSuccess(t *testing.T) {
	payload := "SERIAL-1\tdevice product:foo model:TV_Box device:box transport_id:1\n"
	address := serveOnce(t, func(conn net.Conn) {
		service, err := readTestFrame(conn)
		if err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		if service != "host:devices-l" {
			t.Errorf("service = %q, want host:devices-l", service)
			return
		}
		if err := writeTestResponse(conn, "OKAY", payload); err != nil {
			t.Errorf("write response: %v", err)
		}
	})

	client := NewClient(address)
	got, err := client.ListDevicesLong(context.Background())
	if err != nil {
		t.Fatalf("ListDevicesLong() error = %v", err)
	}
	if got != payload {
		t.Fatalf("ListDevicesLong() = %q, want %q", got, payload)
	}
}

func TestListDevicesLongFailure(t *testing.T) {
	address := serveOnce(t, func(conn net.Conn) {
		if _, err := readTestFrame(conn); err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		if err := writeTestResponse(conn, "FAIL", "device tracker unavailable"); err != nil {
			t.Errorf("write response: %v", err)
		}
	})

	client := NewClient(address)
	_, err := client.ListDevicesLong(context.Background())
	if err == nil || !strings.Contains(err.Error(), "device tracker unavailable") {
		t.Fatalf("ListDevicesLong() error = %v, want server failure detail", err)
	}
}

func TestListDevicesLongRejectsUnexpectedStatus(t *testing.T) {
	address := serveOnce(t, func(conn net.Conn) {
		if _, err := readTestFrame(conn); err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		_, _ = io.WriteString(conn, "NOPE")
	})

	client := NewClient(address)
	_, err := client.ListDevicesLong(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unexpected status") {
		t.Fatalf("ListDevicesLong() error = %v, want unexpected status", err)
	}
}

func TestListDevicesLongRejectsMalformedLength(t *testing.T) {
	address := serveOnce(t, func(conn net.Conn) {
		if _, err := readTestFrame(conn); err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		_, _ = io.WriteString(conn, "OKAYzzzz")
	})

	client := NewClient(address)
	_, err := client.ListDevicesLong(context.Background())
	if err == nil || !strings.Contains(err.Error(), "invalid frame length") {
		t.Fatalf("ListDevicesLong() error = %v, want invalid frame length", err)
	}
}

func TestListDevicesLongCancellationClosesSocket(t *testing.T) {
	ready := make(chan struct{})
	release := make(chan struct{})
	address := serveOnce(t, func(conn net.Conn) {
		if _, err := readTestFrame(conn); err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		if _, err := io.WriteString(conn, "OKAY"); err != nil {
			t.Errorf("write status: %v", err)
			return
		}
		close(ready)
		<-release
	})

	client := NewClient(address)
	client.IOTimeout = 30 * time.Second
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := client.ListDevicesLong(ctx)
		result <- err
	}()

	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("fake server did not receive request")
	}

	cancel()
	select {
	case err := <-result:
		if err != context.Canceled {
			t.Fatalf("ListDevicesLong() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("ListDevicesLong() did not stop after cancellation")
	}
	close(release)
}

func TestReadFrameZeroLength(t *testing.T) {
	payload, err := readFrame(strings.NewReader("0000"))
	if err != nil {
		t.Fatalf("readFrame() error = %v", err)
	}
	if len(payload) != 0 {
		t.Fatalf("readFrame() payload length = %d, want 0", len(payload))
	}
}

func serveOnce(t *testing.T, handler func(net.Conn)) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		handler(conn)
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	})

	return listener.Addr().String()
}

func readTestFrame(r io.Reader) (string, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return "", err
	}
	var size int
	if _, err := fmt.Sscanf(string(header), "%04x", &size); err != nil {
		return "", err
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		return "", err
	}
	return string(payload), nil
}

func writeTestResponse(w io.Writer, status, payload string) error {
	if len(status) != 4 {
		return fmt.Errorf("status must be four bytes")
	}
	if len(payload) > maxFramePayload {
		return fmt.Errorf("payload too large")
	}
	_, err := fmt.Fprintf(w, "%s%04x%s", status, len(payload), payload)
	return err
}
