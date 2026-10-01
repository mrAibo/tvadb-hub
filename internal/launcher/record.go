package launcher

import (
	"ADBKit/internal/core"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// store persists one launcher record per operation under <dataDir>/launcher/.
type store struct {
	dir string
}

func newStore(dataDir string) *store {
	return &store{dir: filepath.Join(dataDir, "launcher")}
}

func (st *store) path(id string) string {
	return filepath.Join(st.dir, id+".json")
}

// saveNew writes a brand new record and refuses to overwrite an existing one, so a
// client cannot stomp an unrelated operation by reusing its ID.
func (st *store) saveNew(record Record) error {
	path := st.path(record.ID)
	if _, err := os.Stat(path); err == nil {
		return core.NewOperationError("launcher_record", "Operation ID already has a durable record", record.ID, false)
	} else if !os.IsNotExist(err) {
		return core.NewOperationError("launcher_record", "Failed to inspect launcher records", err.Error(), true)
	}
	return st.persist(record)
}

// persist atomically replaces the record of one owned operation.
func (st *store) persist(record Record) error {
	if err := record.validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(st.dir, 0o700); err != nil {
		return core.NewOperationError("launcher_record", "Failed to prepare the launcher record directory", err.Error(), true)
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return core.NewOperationError("launcher_record", "Failed to encode the launcher record", err.Error(), true)
	}
	if err := core.WriteFileAtomicWithMode(st.path(record.ID), data, 0o600); err != nil {
		return err
	}
	return nil
}

func (st *store) load(id string) (Record, error) {
	if _, err := validateOperationID(id); err != nil {
		return Record{}, err
	}
	if filepath.Base(id) != id || strings.ContainsAny(id, `/\`) {
		return Record{}, core.NewOperationError("launcher_record", "Invalid record ID", id, false)
	}
	data, err := os.ReadFile(st.path(id))
	if err != nil {
		if os.IsNotExist(err) {
			return Record{}, core.NewOperationError("launcher_record", "Launcher record was not found", id, false)
		}
		return Record{}, core.NewOperationError("launcher_record", "Failed to read the launcher record", err.Error(), true)
	}
	return decodeRecord(data)
}

// list returns every decodable record plus the base names of files that could not
// be used. Unreadable files are reported, never silently skipped.
func (st *store) list() ([]Record, []string, error) {
	entries, err := os.ReadDir(st.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, core.NewOperationError("launcher_record", "Failed to read the launcher record directory", err.Error(), true)
	}
	records := make([]Record, 0, len(entries))
	unreadable := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		record, err := st.load(id)
		if err != nil {
			unreadable = append(unreadable, name)
			continue
		}
		records = append(records, record)
	}
	return records, unreadable, nil
}

// decodeRecord applies the strict record contract: known schema version, no
// unknown fields, no trailing data and validated identities.
func decodeRecord(data []byte) (Record, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record Record
	if err := decoder.Decode(&record); err != nil {
		return Record{}, core.NewOperationError("launcher_record", "Launcher record is not valid", err.Error(), false)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Record{}, core.NewOperationError("launcher_record", "Launcher record has trailing data", "", false)
	}
	if err := record.validate(); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (r Record) validate() error {
	if r.SchemaVersion != SchemaVersion {
		return core.NewOperationError("launcher_record", "Unsupported launcher record schema version", fmt.Sprintf("%d", r.SchemaVersion), false)
	}
	if _, err := validateOperationID(r.ID); err != nil {
		return core.NewOperationError("launcher_record", "Launcher record has an invalid operation ID", r.ID, false)
	}
	if _, err := validateSerial(r.Serial); err != nil {
		return core.NewOperationError("launcher_record", "Launcher record has an invalid device serial", r.Serial, false)
	}
	if r.UserID != 0 {
		return core.NewOperationError("launcher_record", "Launcher record must target Android user 0", fmt.Sprintf("%d", r.UserID), false)
	}
	if r.Action != ActionSetHome && r.Action != ActionRestoreHome {
		return core.NewOperationError("launcher_record", "Launcher record has an unsupported action", string(r.Action), false)
	}
	switch r.State {
	case StatePlanned, StatePending, StateApplied, StateUnknown, StateRestorePending, StateRestored:
	default:
		return core.NewOperationError("launcher_record", "Launcher record has an unknown state", string(r.State), false)
	}
	if err := requireNormalizedComponent("original", r.OriginalComponent); err != nil {
		return err
	}
	if err := requireNormalizedComponent("candidate", r.CandidateComponent); err != nil {
		return err
	}
	if r.CurrentComponent != "" {
		if err := requireNormalizedComponent("current", r.CurrentComponent); err != nil {
			return err
		}
	}
	return nil
}

func requireNormalizedComponent(field, component string) error {
	normalized, err := normalizeComponent(component)
	if err != nil {
		return core.NewOperationError("launcher_record", "Launcher record has an invalid "+field+" component", component, false)
	}
	if normalized != component {
		return core.NewOperationError("launcher_record", "Launcher record component is not normalized", component, false)
	}
	return nil
}

// manualCommand renders a copyable, explicitly serial-pinned command that restores
// the recorded original HOME component. Both the host shell and the remote shell
// are quoted, so `$`-bearing inner-class names survive verbatim.
//
// The tool argument is provided by the caller from the live configuration. A tool
// path stored in the record is display data only and is never executed.
func (r Record) manualCommand(tool, goos string) string {
	return ManualSetHomeCommand(tool, r.Serial, r.UserID, r.OriginalComponent, goos)
}

// ManualSetHomeCommand renders the supported HOME setter as a user-copyable
// command with host-shell quoting for the given platform.
func ManualSetHomeCommand(tool, serial string, user int, component, goos string) string {
	remote := fmt.Sprintf("cmd package set-home-activity --user %d %s", user, core.QuoteShellArg(component))
	tool = strings.TrimSpace(tool)
	if tool == "" {
		tool = core.BinaryNameAdb
	}
	if goos == "windows" {
		return fmt.Sprintf("%s -s %s shell %s", tool, powershellQuote(serial), powershellQuote(remote))
	}
	return fmt.Sprintf("%s -s %s shell %s", tool, core.QuoteShellArg(serial), core.QuoteShellArg(remote))
}

// powershellQuote single-quotes a PowerShell argument; inner single quotes are
// doubled and `$` stays literal.
func powershellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
