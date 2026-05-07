package featgate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

type JSONLAudit struct {
	mu      sync.Mutex
	dir     string
	file    *os.File
	entries []*AuditEntry
	idx     map[string]*AuditEntry
	cap     int
}

func NewJSONLAudit(dir string) (*JSONLAudit, error) {
	if dir == "" {
		dir = "./data/audit"
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	logPath := filepath.Join(dir, "audit.jsonl")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, err
	}
	a := &JSONLAudit{
		dir:     dir,
		file:    f,
		entries: make([]*AuditEntry, 0, 256),
		idx:     make(map[string]*AuditEntry),
		cap:     1000,
	}
	_ = a.loadRecent(logPath)
	return a, nil
}

func (a *JSONLAudit) loadRecent(path string) error {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return err
	}
	dec := json.NewDecoder(newTailReader(data, 64*1024))
	for {
		var e AuditEntry
		if err := dec.Decode(&e); err != nil {
			break
		}
		ec := e
		a.entries = append(a.entries, &ec)
		a.idx[ec.ID] = &ec
		if len(a.entries) > a.cap {
			a.entries = a.entries[len(a.entries)-a.cap:]
		}
	}
	return nil
}

func (a *JSONLAudit) Enabled() bool { return true }

func (a *JSONLAudit) TrashDir() string { return filepath.Join(a.dir, "trash") }

func (a *JSONLAudit) Begin(s AuditStartArgs) *AuditEntry {
	e := &AuditEntry{
		ID:         uuid.NewString(),
		Timestamp:  time.Now().UTC(),
		User:       s.User,
		Role:       s.Role,
		IP:         s.IP,
		AgentID:    s.AgentID,
		Action:     s.Action,
		Target:     s.Target,
		Reversible: s.Reversible,
		Status:     AuditStatusOK,
	}
	if s.Args != nil {
		if b, err := json.Marshal(s.Args); err == nil {
			e.Args = b
		}
	}
	if s.PreImage != nil {
		if b, err := json.Marshal(s.PreImage); err == nil {
			e.PreImage = b
		}
	}
	return e
}

func (a *JSONLAudit) Commit(e *AuditEntry, err error) *AuditEntry {
	if err != nil {
		e.Status = AuditStatusFailed
		e.Error = err.Error()
		e.Reversible = false
	}
	a.append(e)
	return e
}

func (a *JSONLAudit) MarkRolledBack(id, by string, err error) (*AuditEntry, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	e, ok := a.idx[id]
	if !ok {
		return nil, false
	}
	info := &AuditRollbackInfo{At: time.Now().UTC(), By: by}
	if err != nil {
		info.Error = err.Error()
	} else {
		e.Status = AuditStatusRolledBack
	}
	e.RolledBack = info
	rb := *e
	a.appendLocked(&rb)
	return e, true
}

func (a *JSONLAudit) Get(id string) (*AuditEntry, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	e, ok := a.idx[id]
	if !ok {
		return nil, false
	}
	cp := *e
	return &cp, true
}

func (a *JSONLAudit) Recent(limit int) []*AuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	if limit <= 0 || limit > len(a.entries) {
		limit = len(a.entries)
	}
	out := make([]*AuditEntry, 0, limit)
	for i := len(a.entries) - 1; i >= 0 && len(out) < limit; i-- {
		cp := *a.entries[i]
		out = append(out, &cp)
	}
	return out
}

func (a *JSONLAudit) append(e *AuditEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.appendLocked(e)
}

func (a *JSONLAudit) appendLocked(e *AuditEntry) {
	b, _ := json.Marshal(e)
	b = append(b, '\n')
	_, _ = a.file.Write(b)
	if existing, ok := a.idx[e.ID]; ok {
		*existing = *e
		return
	}
	a.entries = append(a.entries, e)
	a.idx[e.ID] = e
	if len(a.entries) > a.cap {
		drop := a.entries[0]
		delete(a.idx, drop.ID)
		a.entries = a.entries[1:]
	}
}

type tailReader struct {
	data []byte
	pos  int
}

func newTailReader(data []byte, max int) *tailReader {
	if len(data) > max {
		i := len(data) - max
		for i < len(data) && data[i] != '\n' {
			i++
		}
		if i < len(data) {
			data = data[i+1:]
		}
	}
	return &tailReader{data: data}
}

func (t *tailReader) Read(p []byte) (int, error) {
	if t.pos >= len(t.data) {
		return 0, fmt.Errorf("eof")
	}
	n := copy(p, t.data[t.pos:])
	t.pos += n
	return n, nil
}

var _ Audit = (*JSONLAudit)(nil)
var _ = errors.New
