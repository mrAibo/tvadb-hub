package adbproto

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultServerAddress = "127.0.0.1:5037"
	defaultDialTimeout   = 2 * time.Second
	defaultIOTimeout     = 5 * time.Second
	maxFramePayload      = 0xffff
)

// Client is a deliberately small client for the local ADB server smart socket.
// It is not a replacement for the official adb CLI. The current prototype only
// exposes read-only host:devices-l for benchmarking high-frequency discovery.
type Client struct {
	Address     string
	DialTimeout time.Duration
	IOTimeout   time.Duration
}

func NewClient(address string) *Client {
	address = strings.TrimSpace(address)
	if address == "" {
		address = DefaultServerAddress
	}
	return &Client{
		Address:     address,
		DialTimeout: defaultDialTimeout,
		IOTimeout:   defaultIOTimeout,
	}
}

// ListDevicesLong queries host:devices-l and returns the raw extended device
// list produced by the running ADB server. Starting or upgrading the server is
// intentionally left to the official adb CLI.
func (c *Client) ListDevicesLong(ctx context.Context) (string, error) {
	payload, err := c.hostQuery(ctx, "host:devices-l")
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func (c *Client) hostQuery(ctx context.Context, service string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !strings.HasPrefix(service, "host:") {
		return nil, fmt.Errorf("adb smart socket: host service required")
	}
	if len(service) > maxFramePayload {
		return nil, fmt.Errorf("adb smart socket: service name is too long")
	}

	address := strings.TrimSpace(c.Address)
	if address == "" {
		address = DefaultServerAddress
	}

	dialer := net.Dialer{Timeout: positiveOrDefault(c.DialTimeout, defaultDialTimeout)}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, wrapContextError(ctx, fmt.Errorf("adb smart socket: connect %s: %w", address, err))
	}
	defer conn.Close()

	if err := setConnectionDeadline(conn, ctx, positiveOrDefault(c.IOTimeout, defaultIOTimeout)); err != nil {
		return nil, fmt.Errorf("adb smart socket: set deadline: %w", err)
	}

	cancelWatchDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-cancelWatchDone:
		}
	}()
	defer close(cancelWatchDone)

	request := fmt.Sprintf("%04x%s", len(service), service)
	if err := writeAll(conn, []byte(request)); err != nil {
		return nil, wrapContextError(ctx, fmt.Errorf("adb smart socket: write request: %w", err))
	}

	status := make([]byte, 4)
	if _, err := io.ReadFull(conn, status); err != nil {
		return nil, wrapContextError(ctx, fmt.Errorf("adb smart socket: read status: %w", err))
	}

	switch string(status) {
	case "OKAY":
		payload, err := readFrame(conn)
		if err != nil {
			return nil, wrapContextError(ctx, fmt.Errorf("adb smart socket: read response: %w", err))
		}
		return payload, nil
	case "FAIL":
		payload, err := readFrame(conn)
		if err != nil {
			return nil, wrapContextError(ctx, fmt.Errorf("adb smart socket: read failure response: %w", err))
		}
		message := strings.TrimSpace(string(payload))
		if message == "" {
			message = "ADB server rejected request"
		}
		return nil, fmt.Errorf("adb smart socket: %s", message)
	default:
		return nil, fmt.Errorf("adb smart socket: unexpected status %q", string(status))
	}
}

func readFrame(r io.Reader) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	size, err := strconv.ParseUint(string(header), 16, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid frame length %q: %w", string(header), err)
	}
	if size == 0 {
		return []byte{}, nil
	}

	payload := make([]byte, int(size))
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func writeAll(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func setConnectionDeadline(conn net.Conn, ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	return conn.SetDeadline(deadline)
}

func positiveOrDefault(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func wrapContextError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return err
}
