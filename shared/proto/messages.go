package proto

import "encoding/json"

type MsgType string

const (
	MsgRegister     MsgType = "register"
	MsgRegisterAck  MsgType = "register.ack"
	MsgHeartbeat    MsgType = "heartbeat"
	MsgTaskDispatch MsgType = "task.dispatch"
	MsgTaskLog      MsgType = "task.log"
	MsgTaskResult   MsgType = "task.result"
	MsgTermOpen     MsgType = "term.open"
	MsgTermOpenAck  MsgType = "term.open.ack"
	MsgTermInput    MsgType = "term.input"
	MsgTermOutput   MsgType = "term.output"
	MsgTermResize   MsgType = "term.resize"
	MsgTermClose    MsgType = "term.close"
	MsgError        MsgType = "error"
)

type Envelope struct {
	ID      string          `json:"id"`
	Type    MsgType         `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type RegisterPayload struct {
	AgentID  string `json:"agent_id"`
	Token    string `json:"token"`
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Version  string `json:"version"`
}

type RegisterAckPayload struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

type TaskKind string

const (
	TaskShell        TaskKind = "shell"
	TaskFSList       TaskKind = "fs.list"
	TaskFSRead       TaskKind = "fs.read"
	TaskFSWrite      TaskKind = "fs.write"
	TaskFSDelete     TaskKind = "fs.delete"
	TaskFSMkdir      TaskKind = "fs.mkdir"
	TaskFSStat       TaskKind = "fs.stat"
	TaskFSRestore    TaskKind = "fs.restore"
	TaskServiceList  TaskKind = "service.list"
	TaskServiceCtl   TaskKind = "service.ctl"
	TaskServiceState TaskKind = "service.state"
	TaskSystemInfo   TaskKind = "system.info"
)

type TaskDispatchPayload struct {
	TaskID  string            `json:"task_id"`
	Kind    TaskKind          `json:"kind"`
	Args    map[string]string `json:"args,omitempty"`
	Command string            `json:"command,omitempty"`
	Body    string            `json:"body,omitempty"`
	Timeout int               `json:"timeout_sec,omitempty"`
}

type TaskLogPayload struct {
	TaskID string `json:"task_id"`
	Stream string `json:"stream"`
	Data   string `json:"data"`
}

type TaskResultPayload struct {
	TaskID   string          `json:"task_id"`
	ExitCode int             `json:"exit_code"`
	Error    string          `json:"error,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
}

type FSEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	IsDir   bool   `json:"is_dir"`
	ModTime string `json:"mod_time"`
	Owner   string `json:"owner,omitempty"`
}

type FSListResult struct {
	Path    string    `json:"path"`
	Entries []FSEntry `json:"entries"`
}

type FSReadResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int64  `json:"size"`
}

type Service struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Load        string `json:"load,omitempty"`
	Active      string `json:"active,omitempty"`
	Sub         string `json:"sub,omitempty"`
}

type ServiceListResult struct {
	Services []Service `json:"services"`
}

type SystemInfoResult struct {
	Hostname    string  `json:"hostname"`
	OS          string  `json:"os"`
	Arch        string  `json:"arch"`
	Kernel      string  `json:"kernel,omitempty"`
	Distro      string  `json:"distro,omitempty"`
	Uptime      int64   `json:"uptime_sec"`
	CPUCount    int     `json:"cpu_count"`
	LoadAvg1    float64 `json:"load_avg_1"`
	LoadAvg5    float64 `json:"load_avg_5"`
	LoadAvg15   float64 `json:"load_avg_15"`
	MemTotalKB  int64   `json:"mem_total_kb"`
	MemFreeKB   int64   `json:"mem_free_kb"`
	MemAvailKB  int64   `json:"mem_avail_kb"`
}

type TermOpenPayload struct {
	SessionID string `json:"session_id"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
	Shell     string `json:"shell,omitempty"`
}

type TermOpenAckPayload struct {
	SessionID string `json:"session_id"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
}

type TermDataPayload struct {
	SessionID string `json:"session_id"`
	Data      string `json:"data"`
}

type TermResizePayload struct {
	SessionID string `json:"session_id"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}

type TermClosePayload struct {
	SessionID string `json:"session_id"`
	Reason    string `json:"reason,omitempty"`
}
