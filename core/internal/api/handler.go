package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/wirepanel/wirepanel/core/internal/agents"
	"github.com/wirepanel/wirepanel/core/internal/auth"
	"github.com/wirepanel/wirepanel/core/internal/fspath"
	"github.com/wirepanel/wirepanel/core/internal/ws"
	"github.com/wirepanel/wirepanel/shared/featgate"
	"github.com/wirepanel/wirepanel/shared/perms"
	"github.com/wirepanel/wirepanel/shared/proto"
)

type Handler struct {
	registry *agents.Registry
	hub      *ws.Hub
	provider featgate.Provider
	fsPolicy fspath.Policy
}

func NewHandler(reg *agents.Registry, hub *ws.Hub, provider featgate.Provider, fsPolicy fspath.Policy) *Handler {
	return &Handler{registry: reg, hub: hub, provider: provider, fsPolicy: fsPolicy}
}

func (h *Handler) audit() featgate.Audit { return h.provider.Audit() }

func (h *Handler) Info() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"edition":          h.provider.Edition(),
			"audit":            h.audit().Enabled(),
			"permission_catalog": perms.Catalog,
		})
	}
}

func (h *Handler) ListAgents() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(h.registry.List())
	}
}

type DispatchRequest struct {
	confirmable
	AgentID string            `json:"agent_id"`
	Kind    proto.TaskKind    `json:"kind"`
	Command string            `json:"command,omitempty"`
	Args    map[string]string `json:"args,omitempty"`
	Timeout int               `json:"timeout_sec,omitempty"`
}

type DispatchResponse struct {
	TaskID  string `json:"task_id"`
	AuditID string `json:"audit_id,omitempty"`
}

func (h *Handler) DispatchTask() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req DispatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if req.Kind != proto.TaskShell {
			http.Error(w, "use resource endpoints for fs/service/system", http.StatusBadRequest)
			return
		}
		id := auth.IdentityFromContext(r.Context())
		if !id.Has(perms.ShellExec) {
			http.Error(w, "forbidden: shell.exec required", http.StatusForbidden)
			return
		}
		if !req.Confirm {
			http.Error(w, "confirm=true required (irreversible)", http.StatusBadRequest)
			return
		}
		conn, ok := h.hub.Get(req.AgentID)
		if !ok {
			http.Error(w, "agent not found or offline", http.StatusNotFound)
			return
		}
		taskID := uuid.NewString()
		h.hub.Subscribe(taskID)
		h.hub.UnsubscribeAfter(taskID, 60*time.Second)

		meta := auditMeta(r)
		meta.AgentID = req.AgentID
		meta.Action = string(perms.ShellExec)
		meta.Target = "shell"
		meta.Args = map[string]any{"command": req.Command, "task_id": taskID}
		meta.Reversible = false
		entry := h.audit().Begin(meta)
		h.audit().Commit(entry, nil)

		payload, _ := json.Marshal(proto.TaskDispatchPayload{
			TaskID:  taskID,
			Kind:    req.Kind,
			Command: req.Command,
			Args:    req.Args,
			Timeout: req.Timeout,
		})
		env := proto.Envelope{ID: taskID, Type: proto.MsgTaskDispatch, Payload: payload}
		if err := conn.Send(env); err != nil {
			http.Error(w, "send failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if entry.ID != "" {
			w.Header().Set("X-Audit-ID", entry.ID)
		}
		json.NewEncoder(w).Encode(DispatchResponse{TaskID: taskID, AuditID: entry.ID})
	}
}

func (h *Handler) TaskStream() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		taskID := r.URL.Query().Get("task_id")
		if taskID == "" {
			http.Error(w, "task_id required", http.StatusBadRequest)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		ch := h.hub.Subscribe(taskID)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher.Flush()

		for {
			select {
			case <-r.Context().Done():
				return
			case env, ok := <-ch:
				if !ok {
					return
				}
				data, _ := json.Marshal(env)
				w.Write([]byte("data: "))
				w.Write(data)
				w.Write([]byte("\n\n"))
				flusher.Flush()
				if env.Type == proto.MsgTaskResult {
					return
				}
			}
		}
	}
}

func (h *Handler) AuditList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries := h.audit().Recent(200)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entries)
	}
}

func (h *Handler) AuditGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		e, ok := h.audit().Get(id)
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(e)
	}
}

type rollbackReq struct {
	confirmable
}

func (h *Handler) AuditRollback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req rollbackReq
		_ = json.NewDecoder(r.Body).Decode(&req)
		if !req.Confirm {
			http.Error(w, "confirm=true required", http.StatusBadRequest)
			return
		}
		entry, ok := h.audit().Get(id)
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if !entry.Reversible {
			http.Error(w, "audit entry is not reversible", http.StatusBadRequest)
			return
		}
		if entry.Status == featgate.AuditStatusRolledBack {
			http.Error(w, "already rolled back", http.StatusConflict)
			return
		}
		err := h.executeRollback(r, entry)
		who := auth.IdentityFromContext(r.Context()).Username
		updated, _ := h.audit().MarkRolledBack(id, who, err)
		if err != nil {
			http.Error(w, "rollback failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updated)
	}
}

func (h *Handler) executeRollback(r *http.Request, e *featgate.AuditEntry) error {
	switch e.Action {
	case string(perms.FSWrite):
		var pre struct {
			Existed      bool   `json:"existed"`
			PriorPath    string `json:"prior_path"`
			PriorContent string `json:"prior_content"`
		}
		if err := json.Unmarshal(e.PreImage, &pre); err != nil {
			return err
		}
		if !pre.Existed {
			res, err := DispatchSync(r.Context(), h.hub, e.AgentID, proto.TaskFSDelete, map[string]string{"path": e.Target}, "", 30)
			return combineErr(err, res)
		}
		res, err := DispatchSync(r.Context(), h.hub, e.AgentID, proto.TaskFSWrite, map[string]string{"path": pre.PriorPath}, pre.PriorContent, 30)
		return combineErr(err, res)

	case string(perms.FSDelete):
		var pre struct {
			OriginalPath string `json:"original_path"`
			TrashPath    string `json:"trash_path"`
		}
		if err := json.Unmarshal(e.PreImage, &pre); err != nil {
			return err
		}
		res, err := DispatchSync(r.Context(), h.hub, e.AgentID, proto.TaskFSRestore, map[string]string{
			"trash_path":    pre.TrashPath,
			"original_path": pre.OriginalPath,
		}, "", 30)
		return combineErr(err, res)

	case string(perms.FSMkdir):
		res, err := DispatchSync(r.Context(), h.hub, e.AgentID, proto.TaskFSDelete, map[string]string{"path": e.Target}, "", 15)
		return combineErr(err, res)

	case string(perms.ServicesStart), string(perms.ServicesStop),
		string(perms.ServicesRestart), string(perms.ServicesEnable), string(perms.ServicesDisable):
		var pre struct {
			Active  string `json:"active"`
			Enabled string `json:"enabled"`
		}
		if err := json.Unmarshal(e.PreImage, &pre); err != nil {
			return err
		}
		var args map[string]any
		_ = json.Unmarshal(e.Args, &args)
		curAction, _ := args["action"].(string)
		var inverse string
		switch curAction {
		case "start":
			inverse = "stop"
		case "stop":
			inverse = "start"
		case "restart", "reload":
			if pre.Active == "active" || pre.Active == "activating" {
				inverse = "restart"
			} else {
				inverse = "stop"
			}
		case "enable":
			inverse = "disable"
		case "disable":
			inverse = "enable"
		default:
			return errors.New("no inverse for action: " + curAction)
		}
		res, err := DispatchSync(r.Context(), h.hub, e.AgentID, proto.TaskServiceCtl, map[string]string{
			"name": e.Target, "action": inverse,
		}, "", 30)
		return combineErr(err, res)

	default:
		return errors.New("rollback not implemented for action: " + e.Action)
	}
}

type roleInput struct {
	Name        string             `json:"name"`
	Color       string             `json:"color,omitempty"`
	Position    int                `json:"position"`
	Permissions []perms.Permission `json:"permissions"`
}

func (h *Handler) RolesList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(h.provider.Roles().List())
	}
}

func (h *Handler) RolesCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in roleInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		for _, p := range in.Permissions {
			if !perms.IsValid(p) {
				http.Error(w, "invalid permission: "+string(p), http.StatusBadRequest)
				return
			}
		}
		role, err := h.provider.Roles().Create(in.Name, in.Color, in.Position, in.Permissions)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(role)
	}
}

func (h *Handler) RolesUpdate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var in roleInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		for _, p := range in.Permissions {
			if !perms.IsValid(p) {
				http.Error(w, "invalid permission: "+string(p), http.StatusBadRequest)
				return
			}
		}
		role, err := h.provider.Roles().Update(id, func(role *featgate.Role) error {
			if in.Name != "" {
				role.Name = in.Name
			}
			role.Color = in.Color
			role.Position = in.Position
			role.Permissions = in.Permissions
			return nil
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(role)
	}
}

func (h *Handler) RolesDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := h.provider.Roles().Delete(id); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type userInput struct {
	Username string   `json:"username"`
	Password string   `json:"password,omitempty"`
	RoleIDs  []string `json:"role_ids"`
}

func (h *Handler) UsersList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(h.provider.Users().List())
	}
}

func (h *Handler) UsersCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in userInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		u, err := h.provider.Users().Create(featgate.UserCreateInput{
			Username: in.Username, Password: in.Password, RoleIDs: in.RoleIDs,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(u)
	}
}

func (h *Handler) UsersUpdate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var in userInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		u, err := h.provider.Users().Update(id, func(u *featgate.User) error {
			if in.Username != "" {
				u.Username = in.Username
			}
			if in.Password != "" {
				h, err := bcryptHash(in.Password)
				if err != nil {
					return err
				}
				u.Hash = h
			}
			if in.RoleIDs != nil {
				u.RoleIDs = in.RoleIDs
			}
			return nil
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(u)
	}
}

func (h *Handler) UsersDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := h.provider.Users().Delete(id); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
