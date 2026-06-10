package daemon

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type state struct {
	Offsets   map[string]int64       `json:"offsets"`
	Hits      map[string][]time.Time `json:"hits"`
	Bans      map[string]time.Time   `json:"bans"`
	LastAudit time.Time              `json:"last_audit"`
}

func loadState(path string) (state, error) {
	st := state{Offsets: map[string]int64{}, Hits: map[string][]time.Time{}, Bans: map[string]time.Time{}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return st, err
	}
	if st.Offsets == nil {
		st.Offsets = map[string]int64{}
	}
	if st.Hits == nil {
		st.Hits = map[string][]time.Time{}
	}
	if st.Bans == nil {
		st.Bans = map[string]time.Time{}
	}
	return st, nil
}

func saveState(path string, st state) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
