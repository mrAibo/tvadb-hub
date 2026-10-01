package shell

import (
	"ADBKit/internal/core"
	"context"
	"strings"
	"sync"
	"time"
)

const (
	logcatProcessRefreshInterval = 5 * time.Second
	logcatProcessQueryTimeout    = 3 * time.Second
)

func queryLogcatProcessNames(ctx context.Context, adbPath, serial string) (map[string]string, error) {
	commands := [][]string{
		{"-s", serial, "shell", "ps", "-A", "-o", "PID,NAME"},
		{"-s", serial, "shell", "ps", "-A"},
	}

	var lastErr error
	for _, args := range commands {
		result, err := core.RunCommand(ctx, core.ExecRequest{
			Command: adbPath,
			Args:    args,
			Timeout: logcatProcessQueryTimeout,
		})
		if err != nil {
			lastErr = err
			continue
		}
		processes := parseLogcatProcessNames(result.Stdout)
		if len(processes) > 0 {
			return processes, nil
		}
	}

	return map[string]string{}, lastErr
}

func parseLogcatProcessNames(output string) map[string]string {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	headerIndex := -1
	var header []string
	for i, line := range lines {
		fields := strings.Fields(line)
		for _, field := range fields {
			if strings.EqualFold(field, "PID") {
				headerIndex = i
				header = fields
				break
			}
		}
		if headerIndex >= 0 {
			break
		}
	}
	if headerIndex < 0 {
		return map[string]string{}
	}

	pidIndex := -1
	nameIndex := -1
	for i, field := range header {
		switch strings.ToUpper(field) {
		case "PID":
			pidIndex = i
		case "NAME":
			nameIndex = i
		}
	}
	if nameIndex < 0 {
		for i, field := range header {
			switch strings.ToUpper(field) {
			case "CMD", "COMMAND", "ARGS":
				nameIndex = i
			}
		}
	}
	if pidIndex < 0 || nameIndex < 0 {
		return map[string]string{}
	}

	processes := make(map[string]string)
	for _, line := range lines[headerIndex+1:] {
		fields := strings.Fields(line)
		if len(fields) <= pidIndex || len(fields) <= nameIndex {
			continue
		}
		pid := fields[pidIndex]
		if !isDecimalPID(pid) {
			continue
		}
		name := strings.TrimSpace(fields[nameIndex])
		if name != "" {
			processes[pid] = name
		}
	}
	return processes
}

func isDecimalPID(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

type processNameCache struct {
	mu    sync.RWMutex
	names map[string]string
}

func newProcessNameCache(initial map[string]string) *processNameCache {
	cache := &processNameCache{names: make(map[string]string)}
	cache.replace(initial)
	return cache
}

func (c *processNameCache) replace(processes map[string]string) {
	if c == nil || len(processes) == 0 {
		return
	}
	copyMap := make(map[string]string, len(processes))
	for pid, name := range processes {
		copyMap[pid] = name
	}
	c.mu.Lock()
	c.names = copyMap
	c.mu.Unlock()
}

func (c *processNameCache) get(pid string) string {
	if c == nil || pid == "" {
		return ""
	}
	c.mu.RLock()
	name := c.names[pid]
	c.mu.RUnlock()
	return name
}

func refreshLogcatProcessNames(ctx context.Context, cache *processNameCache, adbPath, serial string) {
	ticker := time.NewTicker(logcatProcessRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processes, err := queryLogcatProcessNames(ctx, adbPath, serial)
			if err == nil {
				cache.replace(processes)
			}
		}
	}
}
