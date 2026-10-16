// Package app — управление подключением (общее для CLI и веб-UI).
package app

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/wcomsat/vpn-client/internal/config"
	"github.com/wcomsat/vpn-client/internal/core"
	"github.com/wcomsat/vpn-client/internal/proxy"
	"github.com/wcomsat/vpn-client/internal/tun"
)

const (
	SocksPort = 10808
	HTTPPort  = 10809
)

// Manager держит состояние активного подключения.
type Manager struct {
	mu       sync.Mutex
	coreProc *core.Process
	restore  func() error
	stopTun  func() error
	running  bool
	mode     string
	channel  string
	server   string
}

func New() *Manager { return &Manager{} }

// PickChannel выбирает канал из подписки: «vision» — это транспорт tcp.
func PickChannel(chans []*config.Channel, channel string) (*config.Channel, error) {
	want := "xhttp"
	switch channel {
	case "", "xhttp":
		want = "xhttp"
	case "vision":
		want = "tcp"
	default:
		want = channel
	}
	for _, c := range chans {
		if c.Transport == want {
			return c, nil
		}
	}
	return nil, fmt.Errorf("в подписке нет канала %q", channel)
}

// Connect поднимает канал. mode: "proxy" | "tun".
func (m *Manager) Connect(subText, channel, mode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return errors.New("уже подключено — сначала отключите")
	}
	if mode == "" {
		mode = "proxy"
	}

	chans, err := config.ParseSubscription(subText)
	if err != nil {
		return err
	}
	ch, err := PickChannel(chans, channel)
	if err != nil {
		return err
	}
	cfg, err := ch.CoreConfig(SocksPort, HTTPPort)
	if err != nil {
		return err
	}
	xray, err := core.XrayPath()
	if err != nil {
		return err
	}
	proc, err := core.Start(xray, cfg)
	if err != nil {
		return err
	}
	if !waitPort(SocksPort, 10*time.Second) {
		_ = proc.Stop()
		return fmt.Errorf("ядро Xray не подняло SOCKS на 127.0.0.1:%d (см. %s)", SocksPort, proc.LogPath())
	}

	switch mode {
	case "proxy":
		restore, err := proxy.Set(fmt.Sprintf("127.0.0.1:%d", HTTPPort))
		if err != nil {
			_ = proc.Stop()
			return err
		}
		m.restore = restore
	case "tun":
		if !tun.IsAdmin() {
			_ = proc.Stop()
			return errors.New("режим TUN требует прав администратора — запустите приложение от имени администратора")
		}
		serverIP, err := resolveIPv4(ch.Address)
		if err != nil {
			_ = proc.Stop()
			return err
		}
		tun2socks, err := core.Beside("tun2socks.exe")
		if err != nil {
			_ = proc.Stop()
			return err
		}
		alias, gw, err := tun.PrimaryInterface()
		if err != nil {
			_ = proc.Stop()
			return err
		}
		stop, err := tun.Start(tun2socks, fmt.Sprintf("127.0.0.1:%d", SocksPort), serverIP, alias, gw)
		if err != nil {
			_ = proc.Stop()
			return err
		}
		m.stopTun = stop
	default:
		_ = proc.Stop()
		return fmt.Errorf("неизвестный режим %q", mode)
	}

	m.coreProc = proc
	m.running = true
	m.mode = mode
	m.channel = ch.Transport
	m.server = ch.Address
	return nil
}

// Disconnect снимает туннель/прокси и останавливает ядро.
func (m *Manager) Disconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopTun != nil {
		_ = m.stopTun()
		m.stopTun = nil
	}
	if m.restore != nil {
		_ = m.restore()
		m.restore = nil
	}
	if m.coreProc != nil {
		_ = m.coreProc.Stop()
		m.coreProc = nil
	}
	m.running = false
	m.mode, m.channel, m.server = "", "", ""
	return nil
}

// Status возвращает текущее состояние.
func (m *Manager) Status() (running bool, mode, channel, server string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running, m.mode, m.channel, m.server
}

// IsAdmin — можно ли использовать TUN-режим.
func (m *Manager) IsAdmin() bool { return tun.IsAdmin() }

func waitPort(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

func resolveIPv4(host string) (string, error) {
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			return v4.String(), nil
		}
		return "", fmt.Errorf("%s: нужен IPv4-адрес сервера", host)
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", fmt.Errorf("не удалось разрешить %s: %w", host, err)
	}
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			return v4.String(), nil
		}
	}
	return "", fmt.Errorf("у %s нет IPv4-адреса", host)
}
