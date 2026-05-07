package svc

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"

	"github.com/wirepanel/wirepanel/shared/proto"
)

var allowedActions = map[string]bool{
	"start":   true,
	"stop":    true,
	"restart": true,
	"reload":  true,
	"enable":  true,
	"disable": true,
	"status":  true,
}

func List(ctx context.Context) (json.RawMessage, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "list-units", "--type=service", "--all", "--no-pager", "--no-legend", "--plain")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var services []proto.Service
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		svc := proto.Service{
			Name:   fields[0],
			Load:   fields[1],
			Active: fields[2],
			Sub:    fields[3],
		}
		if len(fields) > 4 {
			svc.Description = strings.Join(fields[4:], " ")
		}
		services = append(services, svc)
	}
	return json.Marshal(proto.ServiceListResult{Services: services})
}

func State(ctx context.Context, name string) (json.RawMessage, error) {
	if name == "" {
		return nil, errors.New("service name required")
	}
	active := strings.TrimSpace(runOutput(ctx, "systemctl", "is-active", name))
	enabled := strings.TrimSpace(runOutput(ctx, "systemctl", "is-enabled", name))
	return json.Marshal(map[string]string{
		"name":    name,
		"active":  active,
		"enabled": enabled,
	})
}

func Action(ctx context.Context, name, action string) (json.RawMessage, error) {
	if name == "" {
		return nil, errors.New("service name required")
	}
	if !allowedActions[action] {
		return nil, errors.New("unsupported action")
	}
	cmd := exec.CommandContext(ctx, "systemctl", action, name)
	out, err := cmd.CombinedOutput()
	res := map[string]any{
		"name":    name,
		"action":  action,
		"output":  string(out),
		"success": err == nil,
	}
	if err != nil {
		res["error"] = err.Error()
	}
	return json.Marshal(res)
}

func runOutput(ctx context.Context, name string, args ...string) string {
	cmd := exec.CommandContext(ctx, name, args...)
	out, _ := cmd.Output()
	return string(out)
}
