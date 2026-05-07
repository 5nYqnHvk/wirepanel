package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/wirepanel/wirepanel/shared/proto"
)

type termClientMsg struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

func (h *Handler) Terminal() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID := r.PathValue("id")
		conn, ok := h.hub.Get(agentID)
		if !ok {
			http.Error(w, "agent offline", http.StatusNotFound)
			return
		}
		bws, err := websocket.Accept(w, r, h.hub.AcceptOptions())
		if err != nil {
			return
		}
		defer bws.CloseNow()

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		sessionID := uuid.NewString()

		meta := auditMeta(r)
		meta.AgentID = agentID
		meta.Action = "terminal.open"
		meta.Target = sessionID
		meta.Reversible = false
		entry := h.audit().Begin(meta)
		h.audit().Commit(entry, nil)

		_ = entry
		ch := h.hub.SubscribeTerm(sessionID)
		defer h.hub.UnsubscribeTerm(sessionID)

		openPayload, _ := json.Marshal(proto.TermOpenPayload{
			SessionID: sessionID, Cols: 80, Rows: 24,
		})
		if err := conn.Send(proto.Envelope{ID: sessionID, Type: proto.MsgTermOpen, Payload: openPayload}); err != nil {
			log.Printf("term open send: %v", err)
			return
		}

		go func() {
			defer cancel()
			for {
				_, data, err := bws.Read(ctx)
				if err != nil {
					return
				}
				var msg termClientMsg
				if err := json.Unmarshal(data, &msg); err != nil {
					continue
				}
				switch msg.Type {
				case "input":
					p, _ := json.Marshal(proto.TermDataPayload{SessionID: sessionID, Data: msg.Data})
					_ = conn.Send(proto.Envelope{ID: sessionID, Type: proto.MsgTermInput, Payload: p})
				case "resize":
					p, _ := json.Marshal(proto.TermResizePayload{SessionID: sessionID, Cols: msg.Cols, Rows: msg.Rows})
					_ = conn.Send(proto.Envelope{ID: sessionID, Type: proto.MsgTermResize, Payload: p})
				}
			}
		}()

		for {
			select {
			case <-ctx.Done():
				closeP, _ := json.Marshal(proto.TermClosePayload{SessionID: sessionID})
				_ = conn.Send(proto.Envelope{ID: sessionID, Type: proto.MsgTermClose, Payload: closeP})
				return
			case env, ok := <-ch:
				if !ok {
					return
				}
				wctx, wc := context.WithTimeout(ctx, 5*time.Second)
				_ = bws.Write(wctx, websocket.MessageText, mustJSON(env))
				wc()
				if env.Type == proto.MsgTermClose {
					return
				}
			}
		}
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
