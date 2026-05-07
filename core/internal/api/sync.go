package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/wirepanel/wirepanel/core/internal/ws"
	"github.com/wirepanel/wirepanel/shared/proto"
)

type SyncResult struct {
	ExitCode int
	Error    string
	Data     json.RawMessage
}

func DispatchSync(ctx context.Context, hub *ws.Hub, agentID string, kind proto.TaskKind, args map[string]string, body string, timeoutSec int) (*SyncResult, error) {
	conn, ok := hub.Get(agentID)
	if !ok {
		return nil, errors.New("agent offline")
	}
	taskID := uuid.NewString()
	payload, _ := json.Marshal(proto.TaskDispatchPayload{
		TaskID: taskID, Kind: kind, Args: args, Body: body, Timeout: timeoutSec,
	})
	ch := hub.Subscribe(taskID)
	defer hub.Unsubscribe(taskID)
	if err := conn.Send(proto.Envelope{ID: taskID, Type: proto.MsgTaskDispatch, Payload: payload}); err != nil {
		return nil, err
	}
	wait := 30 * time.Second
	if timeoutSec > 0 {
		wait = time.Duration(timeoutSec+5) * time.Second
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, errors.New("task timeout")
		case env, ok := <-ch:
			if !ok {
				return nil, errors.New("subscription closed")
			}
			if env.Type == proto.MsgTaskResult {
				var r proto.TaskResultPayload
				if err := json.Unmarshal(env.Payload, &r); err != nil {
					return nil, err
				}
				return &SyncResult{ExitCode: r.ExitCode, Error: r.Error, Data: r.Data}, nil
			}
		}
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeSyncResult(w http.ResponseWriter, res *SyncResult, err error) {
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if res.ExitCode != 0 || res.Error != "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":     res.Error,
			"exit_code": res.ExitCode,
		})
		return
	}
	if len(res.Data) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(res.Data)
}
