//go:build !windows

package main

import (
	"errors"

	"github.com/wcomsat/vpn-client/internal/app"
)

// Заглушка для не-Windows сборок (проект целится в Windows).
func runWindow(mgr *app.Manager) error {
	return errors.New("окно доступно только в Windows")
}
