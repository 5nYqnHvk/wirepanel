// Package wppro is the Pro/Team/Enterprise edition implementation.
//
// This is a STUB. The real implementation lives in a private repository
// and is not part of the public open-source distribution.
//
// Build community (default):
//   go build
// Build pro/team/enterprise (requires the private dep to be present):
//   go build -tags pro
package wppro

import (
	"errors"

	"github.com/wirepanel/wirepanel/shared/featgate"
)

func NewProvider(cfg featgate.Config) (featgate.Provider, error) {
	return nil, errors.New("Pro/Team/Enterprise build requires the private wirepanel-pro module; this stub is for community open-source builds only")
}
