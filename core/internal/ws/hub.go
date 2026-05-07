package ws

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/wirepanel/wirepanel/core/internal/agents"
	"github.com/wirepanel/wirepanel/shared/proto"
)

type Conn struct {
	AgentID string
	c       *websocket.Conn
	send    chan proto.Envelope
	hub     *Hub
	ctx     context.Context
	cancel  context.CancelFunc
}

type TaskOwner struct {
	UserID   string
	Username string
	AgentID  string
}

type Hub struct {
	mu             sync.RWMutex
	conns          map[string]*Conn
	registry       *agents.Registry
	masterToken    string
	origins        []string
	strictOrigins  bool

	taskMu     sync.RWMutex
	taskSubs   map[string]chan proto.Envelope
	taskTimers map[string]*time.Timer
	taskOwners map[string]TaskOwner

	termMu   sync.RWMutex
	termSubs map[string]chan proto.Envelope
}

func NewHub(reg *agents.Registry, masterToken string, allowedOrigins []string, strictOrigins bool) *Hub {
	return &Hub{
		conns:         make(map[string]*Conn),
		registry:      reg,
		masterToken:   masterToken,
		origins:       allowedOrigins,
		strictOrigins: strictOrigins,
		taskSubs:      make(map[string]chan proto.Envelope),
		taskTimers:    make(map[string]*time.Timer),
		taskOwners:    make(map[string]TaskOwner),
		termSubs:      make(map[string]chan proto.Envelope),
	}
}

// DeriveAgentToken returns the per-agent token bound to agentID using the
// shared master token via HMAC-SHA256. Agents must send this derived value.
func DeriveAgentToken(master, agentID string) string {
	mac := hmac.New(sha256.New, []byte(master))
	mac.Write([]byte(agentID))
	return hex.EncodeToString(mac.Sum(nil))
}

func (h *Hub) AcceptOptions() *websocket.AcceptOptions {
	if len(h.origins) > 0 {
		return &websocket.AcceptOptions{OriginPatterns: h.origins}
	}
	if h.strictOrigins {
		return &websocket.AcceptOptions{}
	}
	return &websocket.AcceptOptions{InsecureSkipVerify: true}
}

func (h *Hub) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, h.AcceptOptions())
		if err != nil {
			log.Printf("ws accept: %v", err)
			return
		}
		c.SetReadLimit(8 << 20)

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		ag, err := h.handshake(ctx, c)
		if err != nil {
			log.Printf("handshake fail: %v", err)
			c.Close(websocket.StatusPolicyViolation, "handshake failed")
			return
		}

		conn := &Conn{
			AgentID: ag.ID,
			c:       c,
			send:    make(chan proto.Envelope, 256),
			hub:     h,
			ctx:     ctx,
			cancel:  cancel,
		}
		h.add(conn)
		defer h.remove(conn)

		log.Printf("agent connected: %s (%s)", ag.ID, ag.Hostname)

		go conn.writer(ctx)
		conn.reader(ctx)
	}
}

func (h *Hub) handshake(ctx context.Context, c *websocket.Conn) (*agents.Agent, error) {
	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, data, err := c.Read(rctx)
	if err != nil {
		return nil, err
	}
	var env proto.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	if env.Type != proto.MsgRegister {
		return nil, errors.New("expected register")
	}
	var p proto.RegisterPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return nil, err
	}
	if p.AgentID == "" {
		writeEnvelope(rctx, c, proto.Envelope{
			ID: env.ID, Type: proto.MsgRegisterAck,
			Payload: mustJSON(proto.RegisterAckPayload{OK: false, Message: "agent_id required"}),
		})
		return nil, errors.New("missing agent_id")
	}
	expected := DeriveAgentToken(h.masterToken, p.AgentID)
	if !hmac.Equal([]byte(p.Token), []byte(expected)) {
		writeEnvelope(rctx, c, proto.Envelope{
			ID: env.ID, Type: proto.MsgRegisterAck,
			Payload: mustJSON(proto.RegisterAckPayload{OK: false, Message: "bad token"}),
		})
		return nil, errors.New("bad token for agent " + p.AgentID)
	}
	ag := &agents.Agent{
		ID: p.AgentID, Hostname: p.Hostname, OS: p.OS, Arch: p.Arch, Version: p.Version,
	}
	h.registry.Add(ag)

	if err := writeEnvelope(rctx, c, proto.Envelope{
		ID: env.ID, Type: proto.MsgRegisterAck,
		Payload: mustJSON(proto.RegisterAckPayload{OK: true}),
	}); err != nil {
		return nil, err
	}
	return ag, nil
}

func (h *Hub) add(c *Conn) {
	h.mu.Lock()
	prev, dup := h.conns[c.AgentID]
	h.conns[c.AgentID] = c
	h.mu.Unlock()
	if dup && prev != nil {
		log.Printf("agent %s reconnected; closing prior connection", c.AgentID)
		prev.cancel()
		_ = prev.c.Close(websocket.StatusPolicyViolation, "superseded by new connection")
	}
}

func (h *Hub) remove(c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if cur, ok := h.conns[c.AgentID]; ok && cur == c {
		delete(h.conns, c.AgentID)
		h.registry.Remove(c.AgentID)
		close(c.send)
	}
}

func (h *Hub) Get(agentID string) (*Conn, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.conns[agentID]
	return c, ok
}

func (c *Conn) Send(env proto.Envelope) error {
	t := time.NewTimer(5 * time.Second)
	defer t.Stop()
	select {
	case c.send <- env:
		return nil
	case <-t.C:
		return errors.New("send timeout")
	}
}

func (c *Conn) writer(ctx context.Context) {
	tk := time.NewTicker(20 * time.Second)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case env, ok := <-c.send:
			if !ok {
				return
			}
			wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := writeEnvelope(wctx, c.c, env)
			cancel()
			if err != nil {
				log.Printf("write %s: %v", c.AgentID, err)
				return
			}
		case <-tk.C:
			wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			_ = writeEnvelope(wctx, c.c, proto.Envelope{Type: proto.MsgHeartbeat})
			cancel()
		}
	}
}

func (c *Conn) reader(ctx context.Context) {
	for {
		_, data, err := c.c.Read(ctx)
		if err != nil {
			log.Printf("read %s: %v", c.AgentID, err)
			return
		}
		var env proto.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}
		c.hub.registry.Touch(c.AgentID)

		switch env.Type {
		case proto.MsgHeartbeat:
		case proto.MsgTaskLog, proto.MsgTaskResult:
			c.hub.deliverTask(env)
		case proto.MsgTermOpenAck, proto.MsgTermOutput, proto.MsgTermClose:
			c.hub.deliverTerm(env)
		}
	}
}

func (h *Hub) Subscribe(taskID string) chan proto.Envelope {
	h.taskMu.Lock()
	defer h.taskMu.Unlock()
	if ch, ok := h.taskSubs[taskID]; ok {
		return ch
	}
	ch := make(chan proto.Envelope, 256)
	h.taskSubs[taskID] = ch
	return ch
}

func (h *Hub) Unsubscribe(taskID string) {
	h.taskMu.Lock()
	if t, ok := h.taskTimers[taskID]; ok {
		t.Stop()
		delete(h.taskTimers, taskID)
	}
	if ch, ok := h.taskSubs[taskID]; ok {
		delete(h.taskSubs, taskID)
		close(ch)
	}
	delete(h.taskOwners, taskID)
	h.taskMu.Unlock()
}

func (h *Hub) UnsubscribeAfter(taskID string, d time.Duration) {
	h.taskMu.Lock()
	if t, ok := h.taskTimers[taskID]; ok {
		t.Stop()
	}
	h.taskTimers[taskID] = time.AfterFunc(d, func() { h.Unsubscribe(taskID) })
	h.taskMu.Unlock()
}

func (h *Hub) RegisterTaskOwner(taskID string, owner TaskOwner) {
	h.taskMu.Lock()
	h.taskOwners[taskID] = owner
	h.taskMu.Unlock()
}

func (h *Hub) TaskOwner(taskID string) (TaskOwner, bool) {
	h.taskMu.RLock()
	defer h.taskMu.RUnlock()
	o, ok := h.taskOwners[taskID]
	return o, ok
}

func (h *Hub) deliverTask(env proto.Envelope) {
	var taskID string
	switch env.Type {
	case proto.MsgTaskLog:
		var p proto.TaskLogPayload
		_ = json.Unmarshal(env.Payload, &p)
		taskID = p.TaskID
	case proto.MsgTaskResult:
		var p proto.TaskResultPayload
		_ = json.Unmarshal(env.Payload, &p)
		taskID = p.TaskID
	}
	h.taskMu.RLock()
	ch, ok := h.taskSubs[taskID]
	h.taskMu.RUnlock()
	if !ok {
		return
	}
	select {
	case ch <- env:
	default:
	}
}

func (h *Hub) SubscribeTerm(sessionID string) chan proto.Envelope {
	ch := make(chan proto.Envelope, 1024)
	h.termMu.Lock()
	if old, ok := h.termSubs[sessionID]; ok {
		close(old)
	}
	h.termSubs[sessionID] = ch
	h.termMu.Unlock()
	return ch
}

func (h *Hub) UnsubscribeTerm(sessionID string) {
	h.termMu.Lock()
	if ch, ok := h.termSubs[sessionID]; ok {
		delete(h.termSubs, sessionID)
		close(ch)
	}
	h.termMu.Unlock()
}

func (h *Hub) deliverTerm(env proto.Envelope) {
	var sessionID string
	switch env.Type {
	case proto.MsgTermOpenAck:
		var p proto.TermOpenAckPayload
		_ = json.Unmarshal(env.Payload, &p)
		sessionID = p.SessionID
	case proto.MsgTermOutput:
		var p proto.TermDataPayload
		_ = json.Unmarshal(env.Payload, &p)
		sessionID = p.SessionID
	case proto.MsgTermClose:
		var p proto.TermClosePayload
		_ = json.Unmarshal(env.Payload, &p)
		sessionID = p.SessionID
	}
	h.termMu.RLock()
	ch, ok := h.termSubs[sessionID]
	h.termMu.RUnlock()
	if !ok {
		return
	}
	select {
	case ch <- env:
	default:
	}
}

func writeEnvelope(ctx context.Context, c *websocket.Conn, env proto.Envelope) error {
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, data)
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
