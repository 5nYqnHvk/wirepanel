package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/wirepanel/wirepanel/core/internal/auth"
	"github.com/wirepanel/wirepanel/shared/featgate"
	"github.com/wirepanel/wirepanel/shared/perms"
	"github.com/wirepanel/wirepanel/shared/proto"
)

type confirmable struct {
	Confirm     bool   `json:"confirm"`
	ConfirmPath string `json:"confirm_path"`
}

func clientIP(r *http.Request) string {
	if x := r.Header.Get("X-Forwarded-For"); x != "" {
		if i := strings.IndexByte(x, ','); i > 0 {
			return strings.TrimSpace(x[:i])
		}
		return strings.TrimSpace(x)
	}
	host := r.RemoteAddr
	if i := strings.LastIndexByte(host, ':'); i > 0 {
		host = host[:i]
	}
	return host
}

func auditMeta(r *http.Request) featgate.AuditStartArgs {
	id := auth.IdentityFromContext(r.Context())
	return featgate.AuditStartArgs{
		User: id.Username,
		Role: strings.Join(id.RoleIDs, ","),
		IP:   clientIP(r),
	}
}

type fsListReq struct {
	Path string `json:"path"`
}

func (h *Handler) FSList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID := r.PathValue("id")
		var req fsListReq
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Path == "" {
			req.Path = r.URL.Query().Get("path")
		}
		if req.Path == "" {
			req.Path = "/"
		}
		if err := h.fsPolicy.Check(req.Path); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		res, err := DispatchSync(r.Context(), h.hub, agentID, proto.TaskFSList, map[string]string{"path": req.Path}, "", 15)
		writeSyncResult(w, res, err)
	}
}

type fsReadReq struct {
	Path string `json:"path"`
}

func (h *Handler) FSRead() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID := r.PathValue("id")
		var req fsReadReq
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Path == "" {
			req.Path = r.URL.Query().Get("path")
		}
		if err := h.fsPolicy.Check(req.Path); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		res, err := DispatchSync(r.Context(), h.hub, agentID, proto.TaskFSRead, map[string]string{"path": req.Path}, "", 30)
		writeSyncResult(w, res, err)
	}
}

type fsWriteReq struct {
	confirmable
	Path    string `json:"path"`
	Content string `json:"content"`
	Mode    string `json:"mode,omitempty"`
}

func (h *Handler) FSWrite() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID := r.PathValue("id")
		var req fsWriteReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if !req.Confirm {
			http.Error(w, "confirm=true required", http.StatusBadRequest)
			return
		}
		if err := h.fsPolicy.Check(req.Path); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}

		meta := auditMeta(r)
		meta.AgentID = agentID
		meta.Action = string(perms.FSWrite)
		meta.Target = req.Path
		meta.Args = map[string]string{"path": req.Path, "mode": req.Mode}
		meta.Reversible = h.audit().Enabled()

		if h.audit().Enabled() {
			meta.PreImage = h.capturePreImageWrite(r.Context(), agentID, req.Path)
		}

		args := map[string]string{"path": req.Path}
		if req.Mode != "" {
			args["mode"] = req.Mode
		}
		entry := h.audit().Begin(meta)
		res, err := DispatchSync(r.Context(), h.hub, agentID, proto.TaskFSWrite, args, req.Content, 30)
		err = combineErr(err, res)
		h.audit().Commit(entry, err)
		if entry.ID != "" {
			w.Header().Set("X-Audit-ID", entry.ID)
		}
		writeSyncResult(w, res, err)
	}
}

type fsDeleteReq struct {
	confirmable
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
}

func (h *Handler) FSDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID := r.PathValue("id")
		var req fsDeleteReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if !req.Confirm {
			http.Error(w, "confirm=true required", http.StatusBadRequest)
			return
		}
		abs, err := filepath.Abs(req.Path)
		if err != nil {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		if req.ConfirmPath == "" || req.ConfirmPath != abs {
			http.Error(w, "confirm_path must equal absolute target path", http.StatusBadRequest)
			return
		}
		if err := h.fsPolicy.Check(req.Path); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}

		meta := auditMeta(r)
		meta.AgentID = agentID
		meta.Action = string(perms.FSDelete)
		meta.Target = abs
		meta.Args = map[string]any{"path": abs, "recursive": req.Recursive}
		meta.Reversible = h.audit().Enabled()

		entry := h.audit().Begin(meta)
		args := map[string]string{"path": abs}
		if req.Recursive {
			args["recursive"] = "true"
		}
		if h.audit().Enabled() && entry.ID != "" {
			args["trash_id"] = entry.ID
			args["trash_dir"] = h.audit().TrashDir()
		}
		res, derr := DispatchSync(r.Context(), h.hub, agentID, proto.TaskFSDelete, args, "", 60)
		derr = combineErr(derr, res)
		if derr == nil && h.audit().Enabled() && entry.ID != "" {
			pre := map[string]any{
				"original_path": abs,
				"trash_path":    filepath.Join(h.audit().TrashDir(), entry.ID),
			}
			if b, jerr := json.Marshal(pre); jerr == nil {
				entry.PreImage = b
			}
		}
		h.audit().Commit(entry, derr)
		if entry.ID != "" {
			w.Header().Set("X-Audit-ID", entry.ID)
		}
		writeSyncResult(w, res, derr)
	}
}

type fsMkdirReq struct {
	confirmable
	Path string `json:"path"`
	Mode string `json:"mode,omitempty"`
}

func (h *Handler) FSMkdir() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID := r.PathValue("id")
		var req fsMkdirReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if !req.Confirm {
			http.Error(w, "confirm=true required", http.StatusBadRequest)
			return
		}
		if err := h.fsPolicy.Check(req.Path); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		args := map[string]string{"path": req.Path}
		if req.Mode != "" {
			args["mode"] = req.Mode
		}

		meta := auditMeta(r)
		meta.AgentID = agentID
		meta.Action = string(perms.FSMkdir)
		meta.Target = req.Path
		meta.Args = args

		entry := h.audit().Begin(meta)
		res, err := DispatchSync(r.Context(), h.hub, agentID, proto.TaskFSMkdir, args, "", 15)
		err = combineErr(err, res)
		existed := false
		if err == nil && res != nil && len(res.Data) > 0 {
			var d struct{ Existed bool `json:"existed"` }
			_ = json.Unmarshal(res.Data, &d)
			existed = d.Existed
		}
		entry.Reversible = h.audit().Enabled() && !existed && err == nil
		h.audit().Commit(entry, err)
		if entry.ID != "" {
			w.Header().Set("X-Audit-ID", entry.ID)
		}
		writeSyncResult(w, res, err)
	}
}

func (h *Handler) SystemInfo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID := r.PathValue("id")
		res, err := DispatchSync(r.Context(), h.hub, agentID, proto.TaskSystemInfo, nil, "", 10)
		writeSyncResult(w, res, err)
	}
}

func (h *Handler) ServiceList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID := r.PathValue("id")
		res, err := DispatchSync(r.Context(), h.hub, agentID, proto.TaskServiceList, nil, "", 30)
		writeSyncResult(w, res, err)
	}
}

type serviceActionReq struct {
	confirmable
	Action string `json:"action"`
}

func (h *Handler) ServiceAction() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID := r.PathValue("id")
		name := r.PathValue("name")
		var req serviceActionReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		permP, ok := perms.ServiceActionPerm[req.Action]
		if !ok {
			http.Error(w, "unsupported action", http.StatusBadRequest)
			return
		}
		id := auth.IdentityFromContext(r.Context())
		if !id.Has(permP) {
			http.Error(w, "forbidden: missing "+string(permP), http.StatusForbidden)
			return
		}
		if perms.IsDangerous(permP) && !req.Confirm {
			http.Error(w, "confirm=true required", http.StatusBadRequest)
			return
		}

		meta := auditMeta(r)
		meta.AgentID = agentID
		meta.Action = string(permP)
		meta.Target = name
		meta.Args = map[string]string{"action": req.Action, "name": name}
		meta.Reversible = h.audit().Enabled() && isReversibleSvcAction(req.Action)

		var preImage json.RawMessage
		if meta.Reversible {
			st, _ := DispatchSync(r.Context(), h.hub, agentID, proto.TaskServiceState, map[string]string{"name": name}, "", 10)
			if st != nil && len(st.Data) > 0 {
				preImage = st.Data
			}
		}
		meta.PreImage = json.RawMessage(preImage)

		entry := h.audit().Begin(meta)
		res, err := DispatchSync(r.Context(), h.hub, agentID, proto.TaskServiceCtl, map[string]string{
			"name": name, "action": req.Action,
		}, "", 30)
		err = combineErr(err, res)
		h.audit().Commit(entry, err)
		if entry.ID != "" {
			w.Header().Set("X-Audit-ID", entry.ID)
		}
		writeSyncResult(w, res, err)
	}
}

func isReversibleSvcAction(a string) bool {
	switch a {
	case "start", "stop", "enable", "disable", "restart", "reload":
		return true
	}
	return false
}

func combineErr(transportErr error, res *SyncResult) error {
	if transportErr != nil {
		return transportErr
	}
	if res == nil {
		return errors.New("nil result")
	}
	if res.Error != "" {
		return errors.New(res.Error)
	}
	if res.ExitCode != 0 {
		return errors.New("exit code != 0")
	}
	return nil
}

func (h *Handler) capturePreImageWrite(ctx context.Context, agentID, path string) json.RawMessage {
	res, err := DispatchSync(ctx, h.hub, agentID, proto.TaskFSRead, map[string]string{"path": path}, "", 15)
	if err != nil || res == nil || res.Error != "" {
		b, _ := json.Marshal(map[string]any{"existed": false})
		return b
	}
	var prior struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Size    int64  `json:"size"`
	}
	if jerr := json.Unmarshal(res.Data, &prior); jerr != nil {
		b, _ := json.Marshal(map[string]any{"existed": false})
		return b
	}
	b, _ := json.Marshal(map[string]any{
		"existed":       true,
		"prior_path":    prior.Path,
		"prior_size":    prior.Size,
		"prior_content": prior.Content,
	})
	return b
}
