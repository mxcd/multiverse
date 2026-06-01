package ingest

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// TouchedNote records a note the ingester integrated during this session, so a later
// run in the same session can continue building on it instead of creating duplicates.
type TouchedNote struct {
	Path    string `json:"path"`
	Action  string `json:"action"` // created | updated | linked
	Summary string `json:"summary"`
	Run     int    `json:"run"`
}

// Ledger is the persisted per-session ingestion state: how far into the transcript we
// have integrated (Cursor) and which notes we have already touched (TouchedNotes).
type Ledger struct {
	SessionID    string        `json:"session_id"`
	Cursor       int           `json:"cursor"` // transcript messages already processed
	LastUUID     string        `json:"last_uuid,omitempty"`
	TouchedNotes []TouchedNote `json:"touched_notes"`
	Runs         int           `json:"runs"`
	UpdatedAt    string        `json:"updated_at"`

	path string
}

func ledgerPath(sessionID string) string {
	return filepath.Join(stateDir(), sanitize(sessionID)+".json")
}

// LoadLedger reads a session's ledger, or returns a fresh one if none exists.
func LoadLedger(sessionID string) (*Ledger, error) {
	p := ledgerPath(sessionID)
	l := &Ledger{SessionID: sessionID, path: p}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return l, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, l); err != nil {
		return nil, err
	}
	l.path = p
	return l, nil
}

// Save persists the ledger.
func (l *Ledger) Save() error {
	if l.path == "" {
		l.path = ledgerPath(l.SessionID)
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	l.UpdatedAt = nowStamp()
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(l.path, data, 0o644)
}

// AddTouched merges notes into the ledger, deduping by path (latest action wins).
func (l *Ledger) AddTouched(notes []TouchedNote) {
	for _, n := range notes {
		replaced := false
		for i := range l.TouchedNotes {
			if l.TouchedNotes[i].Path == n.Path {
				l.TouchedNotes[i] = n
				replaced = true
				break
			}
		}
		if !replaced {
			l.TouchedNotes = append(l.TouchedNotes, n)
		}
	}
}
