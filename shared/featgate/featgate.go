package featgate

import (
	"encoding/json"
	"time"

	"github.com/wirepanel/wirepanel/shared/perms"
)

type Edition string

const (
	EditionCommunity  Edition = "community"
	EditionPro        Edition = "pro"
	EditionTeam       Edition = "team"
	EditionEnterprise Edition = "enterprise"
)

func (e Edition) Rank() int {
	switch e {
	case EditionCommunity:
		return 0
	case EditionPro:
		return 1
	case EditionTeam:
		return 2
	case EditionEnterprise:
		return 3
	}
	return -1
}

func (e Edition) AtLeast(min Edition) bool {
	return e.Rank() >= min.Rank()
}

type AuditStatus string

const (
	AuditStatusOK         AuditStatus = "ok"
	AuditStatusFailed     AuditStatus = "failed"
	AuditStatusRolledBack AuditStatus = "rolled_back"
)

type AuditEntry struct {
	ID         string             `json:"id"`
	Timestamp  time.Time          `json:"timestamp"`
	User       string             `json:"user"`
	Role       string             `json:"role"`
	IP         string             `json:"ip,omitempty"`
	AgentID    string             `json:"agent_id,omitempty"`
	Action     string             `json:"action"`
	Target     string             `json:"target,omitempty"`
	Args       json.RawMessage    `json:"args,omitempty"`
	PreImage   json.RawMessage    `json:"pre_image,omitempty"`
	Reversible bool               `json:"reversible"`
	Status     AuditStatus        `json:"status"`
	Error      string             `json:"error,omitempty"`
	RolledBack *AuditRollbackInfo `json:"rolled_back,omitempty"`
}

type AuditRollbackInfo struct {
	At    time.Time `json:"at"`
	By    string    `json:"by"`
	Error string    `json:"error,omitempty"`
}

type AuditStartArgs struct {
	User       string
	Role       string
	IP         string
	AgentID    string
	Action     string
	Target     string
	Args       any
	PreImage   any
	Reversible bool
}

type Audit interface {
	Enabled() bool
	TrashDir() string
	Begin(AuditStartArgs) *AuditEntry
	Commit(*AuditEntry, error) *AuditEntry
	Recent(limit int) []*AuditEntry
	Get(id string) (*AuditEntry, bool)
	MarkRolledBack(id, by string, err error) (*AuditEntry, bool)
}

type Role struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Color       string             `json:"color,omitempty"`
	Position    int                `json:"position"`
	System      bool               `json:"system"`
	Permissions []perms.Permission `json:"permissions"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

const OwnerRoleID = "owner"

type RoleStore interface {
	List() []*Role
	Get(id string) (*Role, bool)
	Create(name, color string, position int, perms []perms.Permission) (*Role, error)
	Update(id string, fn func(*Role) error) (*Role, error)
	Delete(id string) error
}

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Hash      string    `json:"hash,omitempty"`
	RoleIDs   []string  `json:"role_ids"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserCreateInput struct {
	Username string
	Password string
	Hash     string
	RoleIDs  []string
}

type UserStore interface {
	Count() int
	List() []*User
	Get(id string) (*User, bool)
	ByUsername(username string) (*User, bool)
	VerifyPassword(username, password string) (*User, bool)
	Create(UserCreateInput) (*User, error)
	Update(id string, fn func(*User) error) (*User, error)
	Delete(id string) error
}

type Provider interface {
	Edition() Edition
	Audit() Audit
	Users() UserStore
	Roles() RoleStore
}

type Config struct {
	DataDir   string
	AuditDir  string
	AdminUser string
	AdminPass string
}
