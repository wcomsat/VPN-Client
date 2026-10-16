//go:build !windows

package main

import "github.com/wcomsat/vpn-client/internal/app"

// Заглушка для не-Windows сборок (проект целится в Windows).
func runTray(mgr *app.Manager, openUI func()) {}
