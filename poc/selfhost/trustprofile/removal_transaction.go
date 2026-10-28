package trustprofile

import (
	"bytes"
	"encoding/hex"
	"sync"
	"time"
)

// RemovalTransaction never returns a usable profile or clears maintenance.
// Its trusted caller must retain the exclusive journal lease for its lifetime.
type RemovalTransaction struct {
	mu      sync.Mutex
	journal TrustJournal
	record  installationRecord
	raw     []byte
	broken  bool
}

func OpenRemovalTransaction(a InstallationAnchor, j TrustJournal, clock func() time.Time) (*RemovalTransaction, error) {
	r, raw, err := readInstallation(a, j, clock)
	if err != nil {
		return nil, err
	}
	if r.Maintenance == nil {
		return nil, ErrTrust
	}
	return &RemovalTransaction{journal: j, record: r, raw: append([]byte(nil), raw...)}, nil
}

func (m *RemovalTransaction) unchanged() bool {
	raw, err := m.journal.Load()
	if err != nil || !bytes.Equal(raw, m.raw) {
		m.broken = true
		return false
	}
	return true
}

func (m *RemovalTransaction) State() (MaintenanceState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.broken || !m.unchanged() {
		return MaintenanceState{}, ErrTrust
	}
	r := m.record
	return MaintenanceState{State: r.Maintenance.State, Epoch: r.Epoch, Transaction: r.Maintenance.ID, PlanSHA256: r.Maintenance.PlanSHA256}, nil
}

// BindRemovalPlan must precede every program-file deletion. The digest binds a
// separately persisted, signed-inventory/object-identity plan; it is not a path.
func (m *RemovalTransaction) BindRemovalPlan(digest string) error { return m.transition(digest, false) }

// MarkFilesRemoved requires the trusted caller to verify all planned files are
// absent. It preserves pending authority/floor; no product can restart here.
func (m *RemovalTransaction) MarkFilesRemoved(digest string) error { return m.transition(digest, true) }

func (m *RemovalTransaction) transition(digest string, complete bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, err := hex.DecodeString(digest)
	if err != nil || len(b) != 32 || hex.EncodeToString(b) != digest || m.broken || !m.unchanged() {
		return ErrTrust
	}
	old := m.record.Maintenance
	if old.State == "files_removed" && old.PlanSHA256 == digest {
		return nil
	}
	if !complete && old.State == "deleting" && old.PlanSHA256 == digest {
		return nil
	}
	if (!complete && old.State != "pending") || (complete && (old.State != "deleting" || old.PlanSHA256 != digest)) {
		return ErrTrust
	}
	r := m.record
	next := *old
	next.State = "deleting"
	next.PlanSHA256 = digest
	if complete {
		next.State = "files_removed"
	}
	r.Maintenance = &next
	raw, err := encodeInstallation(r)
	if err != nil || m.journal.Commit(m.raw, raw) != nil {
		m.broken = true
		return ErrTrust
	}
	m.record = r
	m.raw = raw
	if !m.unchanged() {
		return ErrTrust
	}
	return nil
}
