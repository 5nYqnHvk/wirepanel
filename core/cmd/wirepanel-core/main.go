package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wirepanel/wirepanel/core/internal/agents"
	"github.com/wirepanel/wirepanel/core/internal/api"
	"github.com/wirepanel/wirepanel/core/internal/auth"
	"github.com/wirepanel/wirepanel/core/internal/config"
	"github.com/wirepanel/wirepanel/core/internal/featboot"
	"github.com/wirepanel/wirepanel/core/internal/fspath"
	"github.com/wirepanel/wirepanel/core/internal/ws"
	"github.com/wirepanel/wirepanel/shared/featgate"
	"github.com/wirepanel/wirepanel/shared/perms"
)

func main() {
	cfg := config.Load()

	provider, err := featboot.New(featgate.Config{
		DataDir:   cfg.DataDir,
		AuditDir:  cfg.AuditDir,
		AdminUser: cfg.AdminUser,
		AdminPass: cfg.AdminPass,
	})
	if err != nil {
		log.Fatalf("featboot: %v", err)
	}

	registry := agents.NewRegistry()
	hub := ws.NewHub(registry, cfg.AgentToken, cfg.AllowedOrigins)
	authH := auth.NewHandler(cfg.JWTSecret, cfg.Env, provider)
	apiH := api.NewHandler(registry, hub, provider, fspath.Policy{AllowSensitive: cfg.FSAllowSensitive})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.Handle(cfg.WSPath, hub.Handler())
	mux.Handle("POST /api/auth/login", authH.LoginHandler())
	mux.Handle("GET /api/info", apiH.Info())

	gate := func(p perms.Permission, handler http.HandlerFunc) http.Handler {
		return authH.Middleware(authH.Require(p, handler))
	}
	authed := func(handler http.HandlerFunc) http.Handler {
		return authH.Middleware(handler)
	}

	mux.Handle("GET /api/auth/me", authed(authH.Me()))

	mux.Handle("GET /api/agents", gate(perms.AgentsRead, apiH.ListAgents()))
	mux.Handle("POST /api/tasks", authed(apiH.DispatchTask()))
	mux.Handle("GET /api/tasks/stream", authed(apiH.TaskStream()))

	mux.Handle("GET /api/agents/{id}/system", gate(perms.SystemRead, apiH.SystemInfo()))
	mux.Handle("GET /api/agents/{id}/services", gate(perms.ServicesRead, apiH.ServiceList()))
	mux.Handle("POST /api/agents/{id}/services/{name}/action", authed(apiH.ServiceAction()))

	mux.Handle("GET /api/agents/{id}/fs/list", gate(perms.FSRead, apiH.FSList()))
	mux.Handle("POST /api/agents/{id}/fs/list", gate(perms.FSRead, apiH.FSList()))
	mux.Handle("POST /api/agents/{id}/fs/read", gate(perms.FSRead, apiH.FSRead()))
	mux.Handle("POST /api/agents/{id}/fs/write", gate(perms.FSWrite, apiH.FSWrite()))
	mux.Handle("POST /api/agents/{id}/fs/delete", gate(perms.FSDelete, apiH.FSDelete()))
	mux.Handle("POST /api/agents/{id}/fs/mkdir", gate(perms.FSMkdir, apiH.FSMkdir()))

	mux.Handle("GET /api/agents/{id}/terminal", gate(perms.Terminal, apiH.Terminal()))

	if provider.Audit().Enabled() {
		mux.Handle("GET /api/audit", gate(perms.AuditRead, apiH.AuditList()))
		mux.Handle("GET /api/audit/{id}", gate(perms.AuditRead, apiH.AuditGet()))
		mux.Handle("POST /api/audit/{id}/rollback", gate(perms.AuditRollback, apiH.AuditRollback()))
	}

	if provider.Edition().AtLeast(featgate.EditionTeam) {
		mux.Handle("GET /api/roles", gate(perms.RolesManage, apiH.RolesList()))
		mux.Handle("POST /api/roles", gate(perms.RolesManage, apiH.RolesCreate()))
		mux.Handle("PUT /api/roles/{id}", gate(perms.RolesManage, apiH.RolesUpdate()))
		mux.Handle("DELETE /api/roles/{id}", gate(perms.RolesManage, apiH.RolesDelete()))

		mux.Handle("GET /api/users", gate(perms.UsersManage, apiH.UsersList()))
		mux.Handle("POST /api/users", gate(perms.UsersManage, apiH.UsersCreate()))
		mux.Handle("PUT /api/users/{id}", gate(perms.UsersManage, apiH.UsersUpdate()))
		mux.Handle("DELETE /api/users/{id}", gate(perms.UsersManage, apiH.UsersDelete()))
	}

	corsOrigins := cfg.AllowedOrigins
	if len(corsOrigins) == 0 {
		corsOrigins = []string{"*"}
	}
	handler := api.SecurityHeaders(api.CORS(corsOrigins)(mux))
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if cfg.TLSCert != "" && cfg.TLSKey != "" {
			log.Printf("WirePanel Core (TLS) %s edition=%s", cfg.HTTPAddr, provider.Edition())
			if err := srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey); err != nil && err != http.ErrServerClosed {
				log.Fatalf("https: %v", err)
			}
			return
		}
		if cfg.Env == "production" {
			log.Println("WARNING: running without TLS in production")
		}
		log.Printf("WirePanel Core %s edition=%s env=%s audit=%v", cfg.HTTPAddr, provider.Edition(), cfg.Env, provider.Audit().Enabled())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
