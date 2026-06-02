package session

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "sync"
    "time"
)

var stateMu sync.Mutex

// ViewerState holds a minimal set of UI fields we persist per-transaction.
type ViewerState struct {
    CurrentStep   int       `json:"current_step"`
    SearchQuery   string    `json:"search_query,omitempty"`
    CurrentMatch  int       `json:"current_match,omitempty"` // 1-based
    EventFilter   string    `json:"event_filter,omitempty"`
    HideStdLib    bool      `json:"hide_stdlib,omitempty"`
    UpdatedAt     time.Time `json:"updated_at,omitempty"`
}

func stateFilePath() (string, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    dir := filepath.Join(home, ".Glassbox")
    if err := os.MkdirAll(dir, 0o755); err != nil {
        return "", err
    }
    return filepath.Join(dir, "viewer_state.json"), nil
}

// LoadViewerState returns the persisted state for txHash if present.
func LoadViewerState(txHash string) (ViewerState, bool, error) {
    stateMu.Lock()
    defer stateMu.Unlock()
    path, err := stateFilePath()
    if err != nil {
        return ViewerState{}, false, err
    }
    b, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return ViewerState{}, false, nil
        }
        return ViewerState{}, false, err
    }
    var m map[string]ViewerState
    if err := json.Unmarshal(b, &m); err != nil {
        return ViewerState{}, false, err
    }
    v, ok := m[txHash]
    return v, ok, nil
}

// SaveViewerState stores the viewer state for txHash.
func SaveViewerState(txHash string, st ViewerState) error {
    stateMu.Lock()
    defer stateMu.Unlock()
    path, err := stateFilePath()
    if err != nil {
        return err
    }
    m := make(map[string]ViewerState)
    if b, err := os.ReadFile(path); err == nil {
        _ = json.Unmarshal(b, &m) // best-effort load existing
    }
    st.UpdatedAt = time.Now().UTC()
    m[txHash] = st
    out, err := json.MarshalIndent(m, "", "  ")
    if err != nil {
        return fmt.Errorf("marshal state: %w", err)
    }
    return os.WriteFile(path, out, 0o600)
}
