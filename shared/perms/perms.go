package perms

type Permission string

const (
	WildcardAll Permission = "*"

	AgentsRead   Permission = "agents.read"
	SystemRead   Permission = "system.read"

	ServicesRead    Permission = "services.read"
	ServicesStart   Permission = "services.start"
	ServicesStop    Permission = "services.stop"
	ServicesRestart Permission = "services.restart"
	ServicesEnable  Permission = "services.enable"
	ServicesDisable Permission = "services.disable"

	FSRead   Permission = "fs.read"
	FSWrite  Permission = "fs.write"
	FSMkdir  Permission = "fs.mkdir"
	FSDelete Permission = "fs.delete"

	ShellExec Permission = "shell.exec"
	Terminal  Permission = "terminal"

	AuditRead     Permission = "audit.read"
	AuditRollback Permission = "audit.rollback"

	RolesManage Permission = "roles.manage"
	UsersManage Permission = "users.manage"
)

type Group struct {
	ID    string       `json:"id"`
	Label string       `json:"label"`
	Items []Permission `json:"items"`
}

var Catalog = []Group{
	{ID: "agents",   Label: "Agents",   Items: []Permission{AgentsRead, SystemRead}},
	{ID: "services", Label: "Services", Items: []Permission{ServicesRead, ServicesStart, ServicesStop, ServicesRestart, ServicesEnable, ServicesDisable}},
	{ID: "fs",       Label: "Files",    Items: []Permission{FSRead, FSWrite, FSMkdir, FSDelete}},
	{ID: "exec",     Label: "Execution",Items: []Permission{ShellExec, Terminal}},
	{ID: "audit",    Label: "Audit",    Items: []Permission{AuditRead, AuditRollback}},
	{ID: "admin",    Label: "Admin",    Items: []Permission{RolesManage, UsersManage}},
}

func IsValid(p Permission) bool {
	if p == WildcardAll {
		return true
	}
	for _, g := range Catalog {
		for _, it := range g.Items {
			if it == p {
				return true
			}
		}
	}
	return false
}

func Has(set []Permission, want Permission) bool {
	for _, p := range set {
		if p == WildcardAll || p == want {
			return true
		}
	}
	return false
}

var ServiceActionPerm = map[string]Permission{
	"status":  ServicesRead,
	"start":   ServicesStart,
	"stop":    ServicesStop,
	"restart": ServicesRestart,
	"reload":  ServicesRestart,
	"enable":  ServicesEnable,
	"disable": ServicesDisable,
}

func IsDangerous(p Permission) bool {
	switch p {
	case FSWrite, FSMkdir, FSDelete,
		ServicesStart, ServicesStop, ServicesRestart, ServicesEnable, ServicesDisable,
		ShellExec:
		return true
	}
	return false
}
