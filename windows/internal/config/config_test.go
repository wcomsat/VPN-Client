package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func readFixture(t *testing.T, parts ...string) []byte {
	t.Helper()
	path := filepath.Join(append([]string{"..", "..", "..", "shared", "fixtures"}, parts...)...)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("не читается фикстура %s: %v", path, err)
	}
	return b
}

func TestParseSubscription(t *testing.T) {
	chans, err := ParseSubscription(string(readFixture(t, "sample-links.txt")))
	if err != nil {
		t.Fatalf("ParseSubscription: %v", err)
	}
	if len(chans) != 2 {
		t.Fatalf("ожидалось 2 канала, получено %d", len(chans))
	}

	xhttp := chans[0]
	if xhttp.Transport != "xhttp" || xhttp.Port != 443 {
		t.Errorf("xhttp: transport=%q port=%d", xhttp.Transport, xhttp.Port)
	}
	if xhttp.XHTTP == nil || xhttp.XHTTP.Mode != "stream-one" || xhttp.XHTTP.Path != "/api/v1/updates" {
		t.Errorf("xhttp: неверные параметры xhttp: %+v", xhttp.XHTTP)
	}
	if xhttp.Reality.ServerName != "www.github.com" || xhttp.Reality.Fingerprint != "firefox" {
		t.Errorf("xhttp: неверный reality: %+v", xhttp.Reality)
	}

	vision := chans[1]
	if vision.Transport != "tcp" || vision.Port != 8443 {
		t.Errorf("vision: transport=%q port=%d", vision.Transport, vision.Port)
	}
	if vision.Flow != "xtls-rprx-vision" {
		t.Errorf("vision: flow=%q", vision.Flow)
	}
	if vision.XHTTP != nil {
		t.Errorf("vision: xhttp не должен быть задан для tcp")
	}
}

func TestCoreConfigMatchesFixtures(t *testing.T) {
	chans, err := ParseSubscription(string(readFixture(t, "sample-links.txt")))
	if err != nil {
		t.Fatalf("ParseSubscription: %v", err)
	}
	cases := []struct {
		channel *Channel
		fixture []string
	}{
		{chans[0], []string{"expected", "xhttp.json"}},
		{chans[1], []string{"expected", "vision.json"}},
	}
	for _, tc := range cases {
		got, err := tc.channel.CoreConfig(10808, 10809)
		if err != nil {
			t.Fatalf("CoreConfig(%s): %v", tc.channel.Transport, err)
		}
		var gotAny, wantAny map[string]any
		if err := json.Unmarshal(got, &gotAny); err != nil {
			t.Fatalf("сгенерированный JSON невалиден: %v", err)
		}
		if err := json.Unmarshal(readFixture(t, tc.fixture...), &wantAny); err != nil {
			t.Fatalf("фикстура невалидна: %v", err)
		}
		delete(wantAny, "_comment")
		if !reflect.DeepEqual(gotAny, wantAny) {
			gotPretty, _ := json.MarshalIndent(gotAny, "", "  ")
			wantPretty, _ := json.MarshalIndent(wantAny, "", "  ")
			t.Errorf("конфиг для %s не совпал с фикстурой\n--- got ---\n%s\n--- want ---\n%s",
				tc.channel.Transport, gotPretty, wantPretty)
		}
	}
}

func TestParseLinkRejectsBadInput(t *testing.T) {
	bad := []string{
		"",
		"https://example.com",
		"vless://uuid-only",
		"vless://11111111-1111-4111-8111-111111111111@host:0?type=tcp",
	}
	for _, s := range bad {
		if _, err := ParseLink(s); err == nil {
			t.Errorf("ожидалась ошибка для %q", s)
		}
	}
}
