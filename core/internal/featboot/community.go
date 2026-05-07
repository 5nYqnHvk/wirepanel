//go:build !pro

package featboot

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/wirepanel/wirepanel/shared/featgate"
	"github.com/wirepanel/wirepanel/shared/perms"
)

func New(cfg featgate.Config) (featgate.Provider, error) {
	users, err := newCommunityUserStore(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	if users.Count() == 0 {
		u := cfg.AdminUser
		p := cfg.AdminPass
		if u == "" { u = "admin" }
		if p == "" { p = "wirepanel" }
		if _, err := users.Create(featgate.UserCreateInput{Username: u, Password: p, RoleIDs: []string{featgate.OwnerRoleID}}); err != nil {
			return nil, err
		}
		log.Printf("featboot(community): bootstrapped owner %q", u)
	}
	auditDir := cfg.AuditDir
	if auditDir == "" {
		auditDir = filepath.Join(cfg.DataDir, "audit")
	}
	aud, err := featgate.NewJSONLAudit(auditDir)
	if err != nil {
		return nil, err
	}
	return &communityProvider{
		audit: aud,
		users: users,
		roles: &communityRoleStore{},
	}, nil
}

type communityProvider struct {
	audit featgate.Audit
	users featgate.UserStore
	roles featgate.RoleStore
}

func (p *communityProvider) Edition() featgate.Edition  { return featgate.EditionCommunity }
func (p *communityProvider) Audit() featgate.Audit      { return p.audit }
func (p *communityProvider) Users() featgate.UserStore  { return p.users }
func (p *communityProvider) Roles() featgate.RoleStore  { return p.roles }

type noopAudit struct{}

func (noopAudit) Enabled() bool                                                            { return false }
func (noopAudit) TrashDir() string                                                         { return "" }
func (noopAudit) Begin(featgate.AuditStartArgs) *featgate.AuditEntry                       { return &featgate.AuditEntry{} }
func (noopAudit) Commit(e *featgate.AuditEntry, _ error) *featgate.AuditEntry              { return e }
func (noopAudit) Recent(int) []*featgate.AuditEntry                                        { return []*featgate.AuditEntry{} }
func (noopAudit) Get(string) (*featgate.AuditEntry, bool)                                  { return nil, false }
func (noopAudit) BeginRollback(string) (*featgate.AuditEntry, error)                       { return nil, featgate.ErrAuditNotFound }
func (noopAudit) MarkRolledBack(string, string, error) (*featgate.AuditEntry, bool)        { return nil, false }

var _ featgate.Audit = noopAudit{}

var ownerRole = &featgate.Role{
	ID:          featgate.OwnerRoleID,
	Name:        "Owner",
	Color:       "#ff5d6c",
	Position:    1000,
	System:      true,
	Permissions: []perms.Permission{perms.WildcardAll},
}

type communityRoleStore struct{}

func (communityRoleStore) List() []*featgate.Role { cp := *ownerRole; return []*featgate.Role{&cp} }
func (communityRoleStore) Get(id string) (*featgate.Role, bool) {
	if id == featgate.OwnerRoleID {
		cp := *ownerRole
		return &cp, true
	}
	return nil, false
}
func (communityRoleStore) Create(string, string, int, []perms.Permission) (*featgate.Role, error) {
	return nil, errors.New("multi-role management requires Pro edition")
}
func (communityRoleStore) Update(string, func(*featgate.Role) error) (*featgate.Role, error) {
	return nil, errors.New("multi-role management requires Pro edition")
}
func (communityRoleStore) Delete(string) error {
	return errors.New("multi-role management requires Pro edition")
}

type communityUserStore struct {
	mu    sync.RWMutex
	path  string
	users map[string]*featgate.User
}

func newCommunityUserStore(dataDir string) (*communityUserStore, error) {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, err
	}
	s := &communityUserStore{
		path:  filepath.Join(dataDir, "users.json"),
		users: make(map[string]*featgate.User),
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var list []*featgate.User
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	for _, u := range list {
		s.users[u.ID] = u
	}
	return s, nil
}

func (s *communityUserStore) persistLocked() error {
	list := make([]*featgate.User, 0, len(s.users))
	for _, u := range s.users {
		list = append(list, u)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *communityUserStore) Count() int { s.mu.RLock(); defer s.mu.RUnlock(); return len(s.users) }

func (s *communityUserStore) List() []*featgate.User {
	s.mu.RLock(); defer s.mu.RUnlock()
	out := make([]*featgate.User, 0, len(s.users))
	for _, u := range s.users {
		cp := *u; cp.Hash = ""
		out = append(out, &cp)
	}
	return out
}

func (s *communityUserStore) Get(id string) (*featgate.User, bool) {
	s.mu.RLock(); defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok { return nil, false }
	cp := *u
	return &cp, true
}

func (s *communityUserStore) ByUsername(username string) (*featgate.User, bool) {
	s.mu.RLock(); defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.Username == username {
			cp := *u
			return &cp, true
		}
	}
	return nil, false
}

func (s *communityUserStore) VerifyPassword(username, password string) (*featgate.User, bool) {
	u, ok := s.ByUsername(username)
	if !ok { return nil, false }
	if err := bcrypt.CompareHashAndPassword([]byte(u.Hash), []byte(password)); err != nil {
		return nil, false
	}
	cp := *u; cp.Hash = ""
	return &cp, true
}

func (s *communityUserStore) Create(in featgate.UserCreateInput) (*featgate.User, error) {
	if in.Username == "" {
		return nil, errors.New("username required")
	}
	if in.Hash == "" {
		if in.Password == "" {
			return nil, errors.New("password required")
		}
		h, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		in.Hash = string(h)
	}
	s.mu.Lock(); defer s.mu.Unlock()
	if len(s.users) > 0 {
		return nil, errors.New("multi-user requires Pro edition")
	}
	now := time.Now().UTC()
	u := &featgate.User{
		ID:        uuid.NewString(),
		Username:  in.Username,
		Hash:      in.Hash,
		RoleIDs:   []string{featgate.OwnerRoleID},
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.users[u.ID] = u
	if err := s.persistLocked(); err != nil {
		delete(s.users, u.ID)
		return nil, err
	}
	cp := *u; cp.Hash = ""
	return &cp, nil
}

func (s *communityUserStore) Update(id string, fn func(*featgate.User) error) (*featgate.User, error) {
	s.mu.Lock(); defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok { return nil, errors.New("not found") }
	cp := *u
	if err := fn(&cp); err != nil {
		return nil, err
	}
	cp.RoleIDs = []string{featgate.OwnerRoleID}
	cp.UpdatedAt = time.Now().UTC()
	*u = cp
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	out := *u; out.Hash = ""
	return &out, nil
}

func (s *communityUserStore) Delete(string) error {
	return errors.New("cannot delete sole user in community edition")
}
