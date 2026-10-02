package device

import (
	"ADBKit/internal/core"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
)

type WirelessService struct {
	dataDir    string
	getBinPath func() core.BinaryPaths
}

func NewWirelessService(dataDir string, getBinPath func() core.BinaryPaths) *WirelessService {
	return &WirelessService{dataDir: dataDir, getBinPath: getBinPath}
}

func (s *WirelessService) Connect(ctx context.Context, address string) (string, error) {
	var err error
	address, err = validateWirelessEndpoint(address)
	if err != nil {
		return "", core.NewOperationError("connect_wireless", "wireless address is invalid", err.Error(), false)
	}

	result, err := core.RunCommand(ctx, core.ExecRequest{
		Command: s.getBinPath().Adb,
		Args:    []string{"connect", address},
		Timeout: 10e9,
	})
	if err != nil {
		return "", core.NewOperationError("connect_wireless", "failed to connect wireless device", err.Error(), true)
	}
	if result.ExitCode != 0 {
		return "", core.NewOperationError("connect_wireless", "wireless connect failed", strings.TrimSpace(result.Stderr), true)
	}

	message := extractFirstOutputLine(result.Stdout)
	if message == "" {
		message = fmt.Sprintf("Connected to %s", address)
	}
	return message, nil
}

func (s *WirelessService) EnableTCPIP(ctx context.Context, serial string, port string) (string, error) {
	trimmedSerial := strings.TrimSpace(serial)
	trimmedPort := strings.TrimSpace(port)
	if trimmedSerial == "" {
		return "", core.NewOperationError("enable_wireless_tcpip", "device serial is required", "serial must not be empty", false)
	}
	if trimmedPort == "" {
		trimmedPort = "5555"
	}
	if _, portErr := parseWirelessPort(trimmedPort); portErr != nil {
		return "", core.NewOperationError("enable_wireless_tcpip", "TCP/IP port is invalid", "port must be between 1 and 65535", false)
	}

	result, err := core.RunCommand(ctx, core.ExecRequest{
		Command: s.getBinPath().Adb,
		Args:    enableTCPIPArgs(trimmedSerial, trimmedPort),
		Timeout: 10e9,
	})
	if err != nil {
		return "", core.NewOperationError("enable_wireless_tcpip", "failed to enable wireless TCP/IP mode", err.Error(), true)
	}
	if result.ExitCode != 0 {
		return "", core.NewOperationError("enable_wireless_tcpip", "tcpip command failed", strings.TrimSpace(result.Stderr), true)
	}

	message := extractFirstOutputLine(result.Stdout)
	if message == "" {
		message = fmt.Sprintf("ADB restarted in TCP/IP mode on port %s", trimmedPort)
	}
	return message, nil
}

// enableTCPIPArgs pins the confirmed serial with -s so switching a device into
// wireless TCP/IP mode can never retarget whichever device adb selects first.
func enableTCPIPArgs(serial string, port string) []string {
	return []string{"-s", serial, "tcpip", port}
}

func (s *WirelessService) Disconnect(ctx context.Context, address string) (string, error) {
	args := []string{"disconnect"}
	trimmedAddress := strings.TrimSpace(address)
	if trimmedAddress != "" {
		args = append(args, trimmedAddress)
	}

	result, err := core.RunCommand(ctx, core.ExecRequest{
		Command: s.getBinPath().Adb,
		Args:    args,
		Timeout: 10e9,
	})
	if err != nil {
		return "", core.NewOperationError("disconnect_wireless", "failed to disconnect wireless device", err.Error(), true)
	}
	if result.ExitCode != 0 {
		return "", core.NewOperationError("disconnect_wireless", "disconnect failed", strings.TrimSpace(result.Stderr), true)
	}

	message := extractFirstOutputLine(result.Stdout)
	if message == "" {
		if trimmedAddress == "" {
			message = "Disconnected wireless devices"
		} else {
			message = fmt.Sprintf("Disconnected %s", trimmedAddress)
		}
	}
	return message, nil
}

// Pair performs `adb pair <host:port> <code>` to authorize a wireless debugging
// session on a device. The device must already be in pairing mode (it shows
// the host:port and a one-time pairing code in Developer Options).
func (s *WirelessService) Pair(ctx context.Context, address string, code string) (string, error) {
	trimmedAddress, endpointErr := validateWirelessEndpoint(address)
	if endpointErr != nil {
		return "", core.NewOperationError("pair_wireless", "wireless address is invalid", endpointErr.Error(), false)
	}
	// The pairing code is an exact six-character ASCII PIN: leading zeros are
	// significant and surrounding whitespace is not silently accepted.
	if !isSixDigitPairingCode(code) {
		return "", core.NewOperationError("pair_wireless", "pairing code is invalid", "pairing code must contain exactly six digits", false)
	}

	result, err := core.RunCommand(ctx, core.ExecRequest{
		Command: s.getBinPath().Adb,
		Args:    pairArgs(trimmedAddress, code),
		Timeout: 15e9,
	})
	if err != nil {
		return "", core.NewOperationError("pair_wireless", "failed to pair wireless device", redactPairingCode(err.Error(), code), true)
	}
	if result.ExitCode != 0 {
		return "", core.NewOperationError("pair_wireless", "wireless pairing failed", redactPairingCode(strings.TrimSpace(result.Stderr), code), true)
	}

	message := redactPairingCode(extractFirstOutputLine(result.Stdout), code)
	if message == "" {
		message = fmt.Sprintf("Paired with %s", trimmedAddress)
	}
	return message, nil
}

// redactPairingCode keeps a one-time pairing code out of returned errors, audit
// details and messages: adb output and process errors can echo the original argv.
func redactPairingCode(detail string, code string) string {
	if detail == "" || code == "" {
		return detail
	}
	return strings.ReplaceAll(detail, code, "******")
}

// pairArgs keeps the six-character code byte-exact in argv, so leading zeros are
// preserved and the code is never reformatted or parsed as a number.
func pairArgs(address string, code string) []string {
	return []string{"pair", address, code}
}

func extractFirstOutputLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}


func validateWirelessEndpoint(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.ContainsAny(trimmed, "\x00\r\n\t ") {
		return "", fmt.Errorf("address must be a single host:port value")
	}

	host, port, err := net.SplitHostPort(trimmed)
	if err != nil {
		return "", fmt.Errorf("address must use host:port format: %w", err)
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if !validWirelessHost(host) {
		return "", fmt.Errorf("host must be an IP address or an ASCII DNS name")
	}
	portValue, err := parseWirelessPort(port)
	if err != nil {
		return "", err
	}

	return net.JoinHostPort(strings.TrimSuffix(host, "."), strconv.Itoa(portValue)), nil
}

// validWirelessHost admits an IPv4/IPv6 literal (an optional interface zone is
// allowed) or an ASCII DNS hostname. mDNS instance names, TXT values and other
// free text are not endpoints and must not reach adb's argv.
func validWirelessHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" || strings.ContainsAny(host, "/\\ \t\x00\r\n") {
		return false
	}

	if zone := strings.IndexByte(host, '%'); zone >= 0 {
		if !validInterfaceZone(host[zone+1:]) {
			return false
		}
		host = host[:zone]
	}

	if net.ParseIP(host) != nil {
		return true
	}
	return validASCIIDNSName(host)
}

func validInterfaceZone(zone string) bool {
	if zone == "" {
		return false
	}
	for i := 0; i < len(zone); i++ {
		c := zone[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-' {
			continue
		}
		return false
	}
	return true
}

// validASCIIDNSName accepts a conventional hostname: ASCII only, dot-separated
// labels of 1-63 characters using letters, digits and inner hyphens.
func validASCIIDNSName(name string) bool {
	name = strings.TrimSuffix(name, ".")
	if name == "" || len(name) > 253 {
		return false
	}
	for i := 0; i < len(name); i++ {
		if name[i] > 0x7f {
			return false
		}
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' {
				continue
			}
			return false
		}
	}
	return true
}

// parseWirelessPort accepts only decimal digits, so sign-prefixed or padded values
// cannot slip past as a valid TCP port.
func parseWirelessPort(port string) (int, error) {
	invalid := fmt.Errorf("port must be between 1 and 65535")
	if port == "" || len(port) > 5 {
		return 0, invalid
	}
	for i := 0; i < len(port); i++ {
		if port[i] < '0' || port[i] > '9' {
			return 0, invalid
		}
	}
	value, err := strconv.Atoi(port)
	if err != nil || value < 1 || value > 65535 {
		return 0, invalid
	}
	return value, nil
}

func isSixDigitPairingCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
