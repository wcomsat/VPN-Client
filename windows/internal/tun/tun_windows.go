//go:build windows

// Package tun — полноценный TUN-режим Windows: Wintun-адаптер + tun2socks
// направляет ВЕСЬ системный трафик (включая UDP/QUIC) в локальный SOCKS Xray.
//
// Требует прав администратора. Схема по официальной документации tun2socks:
//   tun2socks --device wintun --proxy socks5://127.0.0.1:10808 --interface "<осн.>"
//   netsh interface ipv4 set address name="wintun" ...
//   netsh interface ipv4 set dnsservers name="wintun" ...
//   netsh interface ipv4 add route 0.0.0.0/0 "wintun" <tun-ip> metric=1
//
// Критично: IP VPN-сервера исключается из туннеля host-маршрутом через
// физический шлюз — иначе бесконечная петля (server -> tun -> xray -> server).
package tun

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	tunName = "wintun"
	tunIP   = "192.168.123.1"
	tunMask = "255.255.255.0"
)

var (
	shell32           = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)

// IsAdmin — запущено ли с повышенными правами (член Administrators в активном
// токене; при UAC без повышения это false).
func IsAdmin() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY, 2,
		windows.SECURITY_BUILTIN_DOMAIN_RID, windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0, &sid,
	)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)
	member, err := windows.GetCurrentProcessToken().IsMember(sid)
	return err == nil && member
}

// Elevate перезапускает текущий процесс с правами администратора (UAC).
func Elevate() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exe)
	params := quoteArgs(os.Args[1:])
	p, _ := windows.UTF16PtrFromString(params)
	ret, _, _ := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(p)),
		0, 1, // SW_SHOWNORMAL
	)
	if ret <= 32 {
		return errors.New("запрос повышения прав отклонён")
	}
	return nil
}

func quoteArgs(args []string) string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if strings.ContainsAny(a, " \t\"") {
			out = append(out, `"`+strings.ReplaceAll(a, `"`, `\"`)+`"`)
		} else {
			out = append(out, a)
		}
	}
	return strings.Join(out, " ")
}

// PrimaryInterface возвращает имя основного интерфейса и его шлюз по умолчанию.
func PrimaryInterface() (alias, gateway string, err error) {
	ps := `$c = Get-NetIPConfiguration | Where-Object { $_.IPv4DefaultGateway -ne $null } | Select-Object -First 1; ` +
		`if ($c) { $c.InterfaceAlias; $c.IPv4DefaultGateway.NextHop }`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("не удалось определить основной интерфейс: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return "", "", errors.New("не найден основной интерфейс/шлюз")
	}
	alias = strings.TrimSpace(lines[0])
	gateway = strings.TrimSpace(lines[1])
	if alias == "" || gateway == "" {
		return "", "", errors.New("пустой интерфейс/шлюз")
	}
	return alias, gateway, nil
}

// Start запускает tun2socks и настраивает маршрутизацию. Возвращает функцию
// остановки (снимает маршруты и закрывает туннель).
func Start(tun2socksPath, socksAddr, serverIP, ifaceAlias, gateway string) (func() error, error) {
	logFile, err := os.CreateTemp("", "tun2socks-*.log")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(tun2socksPath,
		"--device", "wintun",
		"--proxy", "socks5://"+socksAddr,
		"--interface", ifaceAlias,
		"--loglevel", "warn",
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("не удалось запустить tun2socks: %w", err)
	}

	stop := func() error {
		netshQuiet("interface", "ipv4", "delete", "route", "0.0.0.0/0", tunName, tunIP)
		netshQuiet("interface", "ipv4", "delete", "route", serverIP+"/32", ifaceAlias, gateway)
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = logFile.Close()
		return nil
	}

	if !waitAdapter(tunName, 15*time.Second) {
		_ = stop()
		return nil, fmt.Errorf("интерфейс %s не появился (проверьте wintun.dll рядом с tun2socks.exe)", tunName)
	}

	steps := [][]string{
		{"interface", "ipv4", "set", "address", "name=" + tunName, "source=static", "addr=" + tunIP, "mask=" + tunMask},
		{"interface", "ipv4", "set", "dnsservers", "name=" + tunName, "static", "address=1.1.1.1", "register=none", "validate=no"},
		// host-маршрут к серверу — МИМО туннеля (иначе петля)
		{"interface", "ipv4", "add", "route", serverIP + "/32", ifaceAlias, gateway, "metric=1"},
		// весь трафик — через tun
		{"interface", "ipv4", "add", "route", "0.0.0.0/0", tunName, tunIP, "metric=1"},
	}
	for _, s := range steps {
		if out, err := netsh(s...); err != nil {
			_ = stop()
			return nil, fmt.Errorf("netsh %s: %v\n%s", strings.Join(s, " "), err, out)
		}
	}
	return stop, nil
}

func waitAdapter(name string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, _ := netsh("interface", "ipv4", "show", "interfaces")
		if strings.Contains(out, name) {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

func netsh(args ...string) (string, error) {
	cmd := exec.Command("netsh", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func netshQuiet(args ...string) { _, _ = netsh(args...) }
