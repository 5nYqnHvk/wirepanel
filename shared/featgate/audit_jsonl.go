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
		if e.Status == AuditStatusRollingBack {
			e.Status = AuditStatusOK
		}
	} else {
		e.Status = AuditStatusRolledBack
	}
	e.RolledBack = info
	rb := *e
	a.appendLocked(&rb)
	return e, true
}

func (a *JSONLAudit) BeginRollback(id string) (*AuditEntry, error) {
	a.mu.Lock()
	e, ok := a.idx[id]
	if !ok {
		a.mu.Unlock()
		if disk, found := a.loadByID(id); found {
			a.mu.Lock()
			if _, exists := a.idx[id]; !exists {
				a.idx[id] = disk
			}
			e = a.idx[id]
		} else {
			return nil, ErrAuditNotFound
		}
	}
	defer a.mu.Unlock()
	if !e.Reversible {
		return nil, ErrAuditNotReversible
	}
	switch e.Status {
	case AuditStatusRolledBack:
		return nil, ErrAuditAlreadyRolledBack
	case AuditStatusRollingBack:
		return nil, ErrAuditRollbackInProgress
	}
	e.Status = AuditStatusRollingBack
	cp := *e
	return &cp, nil
}

func (a *JSONLAudit) Get(id string) (*AuditEntry, bool) {
	a.mu.Lock()
	if e, ok := a.idx[id]; ok {
		cp := *e
		a.mu.Unlock()
		return &cp, true
	}
	a.mu.Unlock()
	return a.loadByID(id)
}

func (a *JSONLAudit) loadByID(id string) (*AuditEntry, bool) {
	path := filepath.Join(a.dir, "audit.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	var match *AuditEntry
	for {
		var e AuditEntry
		if err := dec.Decode(&e); err != nil {
			break
		}
		if e.ID == id {
			ec := e
			match = &ec
		}
	}
	if match == nil {
		return nil, false
	}
	return match, true
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
	if _, werr := a.file.Write(b); werr == nil {
		_ = a.file.Sync()
	}
	if existing, ok := a.idx[e.ID]; ok {
		*existing = *e
		return
	}
	a.entries = append(a.entries, e)
	a.idx[e.ID] = e
	if len(a.entries) > a.cap {
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
