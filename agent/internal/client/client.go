package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"runtime"
	"time"

	"github.com/coder/websocket"
	"github.com/wirepanel/wirepanel/agent/internal/config"
	"github.com/wirepanel/wirepanel/agent/internal/exec"
	"github.com/wirepanel/wirepanel/agent/internal/term"
	"github.com/wirepanel/wirepanel/shared/proto"
)

type Client struct {
	cfg  *config.Config
	conn *websocket.Conn
	exec *exec.Executor
	term *term.Manager

	writeCh chan proto.Envelope
}

func New(cfg *config.Config) *Client {
	return &Client{cfg: cfg, exec: exec.New(), term: term.NewManager()}
}

func (c *Client) Run(ctx context.Context) error {
	for {
		err := c.connect(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		log.Printf("disconnected: %v, reconnecting in 5s", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func (c *Client) connect(ctx context.Context) error {
	conn, _, err := websocket.Dial(ctx, c.cfg.CoreURL, nil)
	if err != nil {
		return err
	}
	c.conn = conn
	c.writeCh = make(chan proto.Envelope, 256)
	conn.SetReadLimit(8 << 20)
	defer conn.CloseNow()

	if err := c.register(ctx); err != nil {
		return err
	}
	log.Printf("registered as %s", c.cfg.AgentID)

	wctx, wcancel := context.WithCancel(ctx)
	defer wcancel()
	go c.writer(wctx)

	return c.readLoop(ctx)
}

func (c *Client) register(ctx context.Context) error {
	payload, _ := json.Marshal(proto.RegisterPayload{
		AgentID:  c.cfg.AgentID,
		Token:    c.cfg.Token,
		Hostname: c.cfg.Hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Version:  "0.1.0",
	})
	env := proto.Envelope{ID: "reg-" + c.cfg.AgentID, Type: proto.MsgRegister, Payload: payload}
	if err := c.directWrite(ctx, env); err != nil {
		return err
	}

	_, data, err := c.conn.Read(ctx)
	if err != nil {
		return err
	}
	var resp proto.Envelope
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}
	if resp.Type == proto.MsgRegisterAck {
		var ack proto.RegisterAckPayload
		json.Unmarshal(resp.Payload, &ack)
		if !ack.OK {
			return &RegError{Msg: ack.Message}
		}
	}
	return nil
}

func (c *Client) readLoop(ctx context.Context) error {
	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			return err
		}
		var env proto.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}
		switch env.Type {
		case proto.MsgHeartbeat:
			c.send(proto.Envelope{Type: proto.MsgHeartbeat})
		case proto.MsgTaskDispatch:
			go c.handleTask(ctx, env)
		case proto.MsgTermOpen:
			go c.handleTermOpen(ctx, env)
		case proto.MsgTermInput:
			go c.handleTermInput(env)
		case proto.MsgTermResize:
			go c.handleTermResize(env)
		case proto.MsgTermClose:
			go c.handleTermClose(env)
		}
	}
}

func (c *Client) handleTask(ctx context.Context, env proto.Envelope) {
	var task proto.TaskDispatchPayload
	if err := json.Unmarshal(env.Payload, &task); err != nil {
		return
	}

	logFn := func(stream, data string) {
		payload, _ := json.Marshal(proto.TaskLogPayload{
			TaskID: task.TaskID, Stream: stream, Data: data,
		})
		c.send(proto.Envelope{ID: task.TaskID, Type: proto.MsgTaskLog, Payload: payload})
	}

	res := c.exec.Run(ctx, task, logFn)

	errStr := ""
	if res.Err != nil {
		errStr = res.Err.Error()
	}
	payload, _ := json.Marshal(proto.TaskResultPayload{
		TaskID: task.TaskID, ExitCode: res.ExitCode, Error: errStr, Data: res.Data,
	})
	c.send(proto.Envelope{ID: task.TaskID, Type: proto.MsgTaskResult, Payload: payload})
}

func (c *Client) handleTermOpen(ctx context.Context, env proto.Envelope) {
	var p proto.TermOpenPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return
	}
	sess, err := c.term.Open(ctx, p.SessionID, p.Shell, p.Cols, p.Rows)
	ack := proto.TermOpenAckPayload{SessionID: p.SessionID, OK: err == nil}
	if err != nil {
		ack.Error = err.Error()
	}
	ackPayload, _ := json.Marshal(ack)
	c.send(proto.Envelope{ID: p.SessionID, Type: proto.MsgTermOpenAck, Payload: ackPayload})
	if err != nil {
		return
	}
	go c.pumpTermOutput(sess)
}

func (c *Client) pumpTermOutput(sess *term.Session) {
	buf := make([]byte, 4096)
	for {
		n, err := sess.Read(buf)
		if n > 0 {
			data := base64.StdEncoding.EncodeToString(buf[:n])
			payload, _ := json.Marshal(proto.TermDataPayload{SessionID: sess.ID, Data: data})
			c.send(proto.Envelope{ID: sess.ID, Type: proto.MsgTermOutput, Payload: payload})
		}
		if err != nil {
			closePayload, _ := json.Marshal(proto.TermClosePayload{SessionID: sess.ID, Reason: err.Error()})
			c.send(proto.Envelope{ID: sess.ID, Type: proto.MsgTermClose, Payload: closePayload})
			c.term.Close(sess.ID)
			return
		}
	}
}

func (c *Client) handleTermInput(env proto.Envelope) {
	var p proto.TermDataPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return
	}
	sess, ok := c.term.Get(p.SessionID)
	if !ok {
		return
	}
	data, err := base64.StdEncoding.DecodeString(p.Data)
	if err != nil {
		return
	}
	_, _ = sess.Write(data)
}

func (c *Client) handleTermResize(env proto.Envelope) {
	var p proto.TermResizePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return
	}
	sess, ok := c.term.Get(p.SessionID)
	if !ok {
		return
	}
	_ = sess.Resize(p.Cols, p.Rows)
}

func (c *Client) handleTermClose(env proto.Envelope) {
	var p proto.TermClosePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return
	}
	c.term.Close(p.SessionID)
}

func (c *Client) writer(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case env, ok := <-c.writeCh:
			if !ok {
				return
			}
			wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := c.directWrite(wctx, env)
			cancel()
			if err != nil {
				log.Printf("write: %v", err)
				return
			}
		}
	}
}

func (c *Client) send(env proto.Envelope) {
	select {
	case c.writeCh <- env:
	default:
		log.Printf("write buffer full, dropping %s", env.Type)
	}
}

func (c *Client) directWrite(ctx context.Context, env proto.Envelope) error {
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return c.conn.Write(ctx, websocket.MessageText, data)
}

type RegError struct {
	Msg string
}

func (e *RegError) Error() string { return "register failed: " + e.Msg }
