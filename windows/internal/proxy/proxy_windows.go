//go:build windows

// Package proxy управляет системным прокси Windows (WinINET): включает
// HTTP-прокси на локальный инбаунд Xray и корректно восстанавливает прежние
// настройки при отключении.
package proxy

import (
	"fmt"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const regSubKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

var (
	wininet               = windows.NewLazySystemDLL("wininet.dll")
	procInternetSetOption = wininet.NewProc("InternetSetOptionW")
)

const (
	internetOptionRefresh         = 37
	internetOptionSettingsChanged = 39
)

type snapshot struct {
	hadEnable bool
	enable    uint32
	hadServer bool
	server    string
	hadOver   bool
	override  string
}

// Set включает системный HTTP-прокси addr (например "127.0.0.1:10809").
// Возвращает функцию восстановления прежних настроек.
func Set(addr string) (func() error, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, regSubKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть настройки прокси: %w", err)
	}
	defer k.Close()

	var snap snapshot
	if v, _, err := k.GetIntegerValue("ProxyEnable"); err == nil {
		snap.hadEnable, snap.enable = true, uint32(v)
	}
	if v, _, err := k.GetStringValue("ProxyServer"); err == nil {
		snap.hadServer, snap.server = true, v
	}
	if v, _, err := k.GetStringValue("ProxyOverride"); err == nil {
		snap.hadOver, snap.override = true, v
	}

	if err := k.SetDWordValue("ProxyEnable", 1); err != nil {
		return nil, err
	}
	if err := k.SetStringValue("ProxyServer", addr); err != nil {
		return nil, err
	}
	if err := k.SetStringValue("ProxyOverride", "<local>"); err != nil {
		return nil, err
	}
	notify()

	return func() error {
		kk, err := registry.OpenKey(registry.CURRENT_USER, regSubKey, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer kk.Close()

		if snap.hadEnable {
			_ = kk.SetDWordValue("ProxyEnable", snap.enable)
		} else {
			_ = kk.DeleteValue("ProxyEnable")
		}
		if snap.hadServer {
			_ = kk.SetStringValue("ProxyServer", snap.server)
		} else {
			_ = kk.DeleteValue("ProxyServer")
		}
		if snap.hadOver {
			_ = kk.SetStringValue("ProxyOverride", snap.override)
		} else {
			_ = kk.DeleteValue("ProxyOverride")
		}
		notify()
		return nil
	}, nil
}

// Disable выключает системный прокси (ProxyEnable=0).
func Disable() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, regSubKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.SetDWordValue("ProxyEnable", 0); err != nil {
		return err
	}
	notify()
	return nil
}

func notify() {
	procInternetSetOption.Call(0, internetOptionSettingsChanged, 0, 0)
	procInternetSetOption.Call(0, internetOptionRefresh, 0, 0)
}
