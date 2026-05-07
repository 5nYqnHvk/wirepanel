//go:build pro

package featboot

import (
	"github.com/wirepanel/wirepanel/shared/featgate"
	wppro "github.com/wirepanel/wirepanel-pro"
)

func New(cfg featgate.Config) (featgate.Provider, error) {
	return wppro.NewProvider(cfg)
}
