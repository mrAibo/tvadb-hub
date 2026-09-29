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
	portValue, portErr := strconv.Atoi(trimmedPort)
	if portErr != nil || portValue < 1 || portValue > 65535 {
		return "", core.NewOperationError("enable_wireless_tcpip", "TCP/IP port is invalid", "port must be between 1 and 65535", false)
	}

	result, err := core.RunCommand(ctx, core.ExecRequest{
		Command: s.getBinPath().Adb,
		Args:    []string{"-s", trimmedSerial, "tcpip", trimmedPort},
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
	trimmedCode := strings.TrimSpace(code)
	if !isSixDigitPairingCode(trimmedCode) {
		return "", core.NewOperationError("pair_wireless", "pairing code is invalid", "pairing code must contain exactly six digits", false)
	}

	result, err := core.RunCommand(ctx, core.ExecRequest{
		Command: s.getBinPath().Adb,
		Args:    []string{"pair", trimmedAddress, trimmedCode},
		Timeout: 15e9,
	})
	if err != nil {
		return "", core.NewOperationError("pair_wireless", "failed to pair wireless device", err.Error(), true)
	}
	if result.ExitCode != 0 {
		return "", core.NewOperationError("pair_wireless", "wireless pairing failed", strings.TrimSpace(result.Stderr), true)
	}

	message := extractFirstOutputLine(result.Stdout)
	if message == "" {
		message = fmt.Sprintf("Paired with %s", trimmedAddress)
	}
	return message, nil
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
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if host == "" || strings.ContainsAny(host, "/\\") {
		return "", fmt.Errorf("host is invalid")
	}
	portValue, err := strconv.Atoi(port)
	if err != nil || portValue < 1 || portValue > 65535 {
		return "", fmt.Errorf("port must be between 1 and 65535")
	}

	return net.JoinHostPort(host, strconv.Itoa(portValue)), nil
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
