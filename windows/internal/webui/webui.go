// Package webui — локальный веб-интерфейс управления VPN.
//
// Сервер слушает только 127.0.0.1 и открывается в браузере по умолчанию.
// Никаких GUI-тулкитов: чистый net/http, поэтому не требует C-компилятора.
package webui

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wcomsat/vpn-client/internal/app"
	"github.com/wcomsat/vpn-client/internal/config"
)

// Server — HTTP-обёртка над Manager.
type Server struct {
	mgr     *app.Manager
	subPath string
}

// New создаёт сервер; файл подписки кладём рядом с приложением.
func New(mgr *app.Manager) *Server {
	dir := "."
	if exe, err := os.Executable(); err == nil {
		dir = filepath.Dir(exe)
	}
	return &Server{mgr: mgr, subPath: filepath.Join(dir, "subscription.txt")}
}

type jsonResp map[string]any

// Start поднимает HTTP-сервер и открывает браузер.
func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/load", s.handleLoad)
	mux.HandleFunc("/save", s.handleSave)
	mux.HandleFunc("/parse", s.handleParse)
	mux.HandleFunc("/connect", s.handleConnect)
	mux.HandleFunc("/disconnect", s.handleDisconnect)

	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, indexHTML)
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	running, mode, channel, server := s.mgr.Status()
	writeJSON(w, jsonResp{
		"running": running, "mode": mode, "channel": channel,
		"server": server, "admin": s.mgr.IsAdmin(),
	})
}

func (s *Server) handleLoad(w http.ResponseWriter, _ *http.Request) {
	b, _ := os.ReadFile(s.subPath)
	writeJSON(w, jsonResp{"subscription": strings.TrimSpace(string(b))})
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	var req struct{ Subscription string }
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, jsonResp{"error": err.Error()})
		return
	}
	if err := os.WriteFile(s.subPath, []byte(strings.TrimSpace(req.Subscription)+"\n"), 0o600); err != nil {
		writeJSON(w, jsonResp{"error": err.Error()})
		return
	}
	writeJSON(w, jsonResp{"error": ""})
}

func (s *Server) handleParse(w http.ResponseWriter, r *http.Request) {
	var req struct{ Subscription string }
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, jsonResp{"error": err.Error()})
		return
	}
	chans, err := config.ParseSubscription(req.Subscription)
	if err != nil {
		writeJSON(w, jsonResp{"error": err.Error()})
		return
	}
	list := make([]string, 0, len(chans))
	for _, c := range chans {
		name := c.Transport
		if name == "tcp" {
			name = "vision"
		}
		list = append(list, name)
	}
	writeJSON(w, jsonResp{"error": "", "channels": list})
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	var req struct{ Subscription, Channel, Mode string }
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, jsonResp{"error": err.Error()})
		return
	}
	if err := s.mgr.Connect(req.Subscription, req.Channel, req.Mode); err != nil {
		writeJSON(w, jsonResp{"error": err.Error()})
		return
	}
	writeJSON(w, jsonResp{"error": ""})
}

func (s *Server) handleDisconnect(w http.ResponseWriter, _ *http.Request) {
	if err := s.mgr.Disconnect(); err != nil {
		writeJSON(w, jsonResp{"error": err.Error()})
		return
	}
	writeJSON(w, jsonResp{"error": ""})
}

func readJSON(r *http.Request, v any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

// OpenBrowser открывает URL в браузере по умолчанию.
func OpenBrowser(url string) {
	// VPN_CLIENT_NO_BROWSER — не открывать браузер (тесты/серверный запуск).
	if os.Getenv("VPN_CLIENT_NO_BROWSER") != "" {
		return
	}
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
