// Package config: разбор подписки (vless://) и сборка конфига Xray-core.
//
// Контракт общий для Android и Windows — см. shared/README.md. Результат
// проверяется фикстурами shared/fixtures.
package config

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Reality — публичные параметры REALITY (без приватных ключей).
type Reality struct {
	ServerName  string `json:"serverName"`
	PublicKey   string `json:"publicKey"`
	ShortID     string `json:"shortId"`
	Fingerprint string `json:"fingerprint"`
	SpiderX     string `json:"spiderX"`
}

// XHTTP — параметры транспорта xHTTP (только для transport = xhttp).
type XHTTP struct {
	Host string `json:"host"`
	Path string `json:"path"`
	Mode string `json:"mode"`
}

// Channel — нормализованное представление одного канала.
type Channel struct {
	Name      string  `json:"name"`
	Protocol  string  `json:"protocol"`
	UUID      string  `json:"uuid"`
	Address   string  `json:"address"`
	Port      int     `json:"port"`
	Transport string  `json:"transport"`
	Security  string  `json:"security"`
	Reality   Reality `json:"reality"`
	XHTTP     *XHTTP  `json:"xhttp"`
	Flow      string  `json:"flow"`
}

// ParseLink разбирает одну ссылку vless://.
func ParseLink(raw string) (*Channel, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("пустая ссылка")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("некорректная ссылка: %w", err)
	}
	if u.Scheme != "vless" {
		return nil, fmt.Errorf("ожидалась схема vless://, получено %q", u.Scheme)
	}
	if u.User == nil || u.User.Username() == "" {
		return nil, fmt.Errorf("в ссылке нет UUID")
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		return nil, fmt.Errorf("нет host:port в ссылке: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("некорректный порт %q", portStr)
	}

	q := u.Query()
	transport := q.Get("type")
	if transport == "" {
		transport = "tcp"
	}
	if transport != "xhttp" && transport != "tcp" {
		return nil, fmt.Errorf("неподдерживаемый транспорт: %q", transport)
	}

	ch := &Channel{
		Name:      u.Fragment,
		Protocol:  "vless",
		UUID:      u.User.Username(),
		Address:   host,
		Port:      port,
		Transport: transport,
		Security:  q.Get("security"),
		Reality: Reality{
			ServerName:  q.Get("sni"),
			PublicKey:   q.Get("pbk"),
			ShortID:     q.Get("sid"),
			Fingerprint: q.Get("fp"),
			SpiderX:     defaultStr(q.Get("spx"), "/"),
		},
	}
	if ch.Security == "" {
		ch.Security = "reality"
	}
	if ch.Reality.Fingerprint == "" {
		ch.Reality.Fingerprint = "firefox"
	}

	switch transport {
	case "xhttp":
		ch.XHTTP = &XHTTP{
			Host: defaultStr(q.Get("host"), ch.Reality.ServerName),
			Path: defaultStr(q.Get("path"), "/"),
			Mode: defaultStr(q.Get("mode"), "stream-one"),
		}
	case "tcp":
		ch.Flow = q.Get("flow")
	}
	return ch, nil
}

// ParseSubscription разбирает текст подписки (несколько строк).
func ParseSubscription(text string) ([]*Channel, error) {
	var out []*Channel
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ch, err := ParseLink(line)
		if err != nil {
			return nil, err
		}
		out = append(out, ch)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("в подписке нет ни одной ссылки")
	}
	return out, nil
}

// CoreConfig собирает конфиг Xray-core (SOCKS-инбаунд + VLESS-аутбаунд) для
// выбранного канала. Совпадает с shared/fixtures/expected/*.json.
func (c *Channel) CoreConfig(socksPort int) ([]byte, error) {
	user := map[string]any{"id": c.UUID, "encryption": "none"}
	if c.Flow != "" {
		user["flow"] = c.Flow
	}

	reality := map[string]any{
		"serverName":  c.Reality.ServerName,
		"fingerprint": c.Reality.Fingerprint,
		"publicKey":   c.Reality.PublicKey,
		"shortId":     c.Reality.ShortID,
		"spiderX":     c.Reality.SpiderX,
	}
	stream := map[string]any{
		"network":         c.Transport,
		"security":        "reality",
		"realitySettings": reality,
	}
	if c.Transport == "xhttp" && c.XHTTP != nil {
		stream["xhttpSettings"] = map[string]any{
			"host": c.XHTTP.Host,
			"path": c.XHTTP.Path,
			"mode": c.XHTTP.Mode,
		}
	}

	cfg := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"inbounds": []any{
			map[string]any{
				"listen":   "127.0.0.1",
				"port":     socksPort,
				"protocol": "socks",
				"settings": map[string]any{"udp": true},
				"sniffing": map[string]any{
					"enabled":      true,
					"destOverride": []string{"http", "tls", "quic"},
				},
			},
		},
		"outbounds": []any{
			map[string]any{
				"protocol": "vless",
				"settings": map[string]any{
					"vnext": []any{
						map[string]any{
							"address": c.Address,
							"port":    c.Port,
							"users":   []any{user},
						},
					},
				},
				"streamSettings": stream,
			},
			map[string]any{"protocol": "freedom", "tag": "direct"},
		},
	}
	return json.MarshalIndent(cfg, "", "  ")
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
