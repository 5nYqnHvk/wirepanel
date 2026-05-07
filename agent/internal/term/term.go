package term

import (
	"context"
	"errors"
	"os"
	osexec "os/exec"
	"sync"

	"github.com/creack/pty"
)

type Session struct {
	ID   string
	pty  *os.File
	cmd  *osexec.Cmd
	mu   sync.Mutex
	done chan struct{}
}

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewManager() *Manager {
	return &Manager{sessions: make(map[string]*Session)}
}

func (m *Manager) Open(ctx context.Context, id, shell string, cols, rows int) (*Session, error) {
	if shell == "" {
		shell = "/bin/bash"
		if _, err := os.Stat(shell); err != nil {
			shell = "/bin/sh"
		}
	}
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	cmd := osexec.Command(shell, "-l")
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, err
	}
	s := &Session{ID: id, pty: f, cmd: cmd, done: make(chan struct{})}
	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()
	go func() {
		_ = cmd.Wait()
		close(s.done)
	}()
	return s, nil
}

func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

func (m *Manager) Close(id string) {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	if ok {
		s.kill()
	}
}

func (s *Session) Read(buf []byte) (int, error) {
	if s.pty == nil {
		return 0, errors.New("pty closed")
	}
	return s.pty.Read(buf)
}

func (s *Session) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pty == nil {
		return 0, errors.New("pty closed")
	}
	return s.pty.Write(data)
}

func (s *Session) Resize(cols, rows int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pty == nil {
		return errors.New("pty closed")
	}
	return pty.Setsize(s.pty, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

func (s *Session) Done() <-chan struct{} {
	return s.done
}

func (s *Session) kill() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pty != nil {
		_ = s.pty.Close()
		s.pty = nil
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
}
