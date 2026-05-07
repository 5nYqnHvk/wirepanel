package fspath

import (
	"errors"
	"path/filepath"
	"strings"
)

var sensitiveDeny = []string{
	"/etc/shadow",
	"/etc/gshadow",
	"/etc/sudoers",
	"/etc/sudoers.d",
	"/root/.ssh",
	"/root/.bash_history",
	"/boot",
	"/sys",
	"/proc/sys",
	"/dev/mem",
	"/dev/kmem",
}

var sensitiveSuffix = []string{
	"/.ssh",
	"/.aws/credentials",
	"/.gnupg",
}

type Policy struct {
	AllowSensitive bool
}

func (p Policy) Check(path string) error {
	if path == "" {
		return errors.New("empty path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if p.AllowSensitive {
		return nil
	}
	clean := filepath.Clean(abs)
	for _, deny := range sensitiveDeny {
		if clean == deny || strings.HasPrefix(clean, deny+"/") {
			return errors.New("path is in sensitive denylist (set WP_FS_ALLOW_SENSITIVE=true to override)")
		}
	}
	for _, suf := range sensitiveSuffix {
		if strings.HasSuffix(clean, suf) || strings.Contains(clean, suf+"/") {
			return errors.New("path matches sensitive suffix denylist")
		}
	}
	return nil
}
