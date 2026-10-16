// Команда vpn-client — клиент личного VPN-сервиса (Windows).
//
// Режимы подключения:
//   proxy — системный HTTP-прокси (браузеры/приложения; без админа);
//   tun   — полноценный туннель Wintun+tun2socks (весь трафик; нужен админ).
//
// Интерфейс:  vpn-client ui   — локальная веб-страница с кнопками.
//
// Примеры:
//
//	vpn-client ui
//	vpn-client parse   --sub subscription.txt
//	vpn-client connect --sub subscription.txt --channel xhttp        # proxy
//	vpn-client connect --sub subscription.txt --channel xhttp --mode tun
//	vpn-client proxy off
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/wcomsat/vpn-client/internal/app"
	"github.com/wcomsat/vpn-client/internal/config"
	"github.com/wcomsat/vpn-client/internal/proxy"
	"github.com/wcomsat/vpn-client/internal/webui"
)

const version = "0.3.1"

func main() {
	// Запуск без аргументов (двойным кликом) — открываем интерфейс.
	if len(os.Args) < 2 {
		cmdUI(nil)
		return
	}
	switch os.Args[1] {
	case "version", "-v", "--version":
		fmt.Println("vpn-client", version)
	case "ui":
		cmdUI(os.Args[2:])
	case "parse":
		cmdParse(os.Args[2:])
	case "config":
		cmdConfig(os.Args[2:])
	case "connect":
		cmdConnect(os.Args[2:])
	case "proxy":
		cmdProxy(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "неизвестная команда: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`vpn-client — клиент личного VPN (Windows)

Команды:
  ui                                             открыть интерфейс (браузер)
  version
  parse   --sub <файл|URL>                       показать каналы из подписки
  config  --sub <файл|URL> [--channel xhttp|vision] --out <файл>
  connect --sub <файл|URL> [--channel xhttp|vision] [--mode proxy|tun]
  proxy   off                                    выключить системный прокси

Рядом с exe нужны: xray.exe; для --mode tun ещё tun2socks.exe и wintun.dll.
`)
}

// readSource читает подписку из файла, URL (http/https) или как сырой текст.
func readSource(src string) ([]byte, error) {
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		client := &http.Client{Timeout: 20 * time.Second}
		resp, err := client.Get(src)
		if err != nil {
			return nil, fmt.Errorf("не скачать подписку по URL: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("подписка недоступна: HTTP %d", resp.StatusCode)
		}
		return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	}
	if strings.Contains(src, "vless://") {
		return []byte(src), nil
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return nil, fmt.Errorf("не читается подписка %s: %w", src, err)
	}
	return b, nil
}

func cmdUI(args []string) {
	fs := flag.NewFlagSet("ui", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8787", "адрес локального интерфейса")
	_ = fs.Parse(args)

	mgr := app.New()
	srv := webui.New(mgr)
	go func() {
		if err := srv.Start(*addr); err != nil {
			fmt.Fprintf(os.Stderr, "веб-интерфейс остановлен: %v\n", err)
		}
	}()
	fmt.Printf("Интерфейс открыт в браузере: http://%s/\n", *addr)
	fmt.Println("Ctrl+C — закрыть.")
	waitSignal()
	_ = mgr.Disconnect()
	fmt.Println("\nвыход")
}

func cmdParse(args []string) {
	fs := flag.NewFlagSet("parse", flag.ExitOnError)
	sub := fs.String("sub", "", "файл или URL подписки")
	_ = fs.Parse(args)
	if *sub == "" {
		fatal("укажите --sub <файл|URL>")
	}
	data, err := readSource(*sub)
	if err != nil {
		fatal("%v", err)
	}
	chans, err := config.ParseSubscription(string(data))
	if err != nil {
		fatal("%v", err)
	}
	for _, c := range chans {
		fmt.Printf("%-8s %s:%d  sni=%s  %s\n", c.Transport, c.Address, c.Port, c.Reality.ServerName, c.Name)
	}
}

func cmdConfig(args []string) {
	fs := flag.NewFlagSet("config", flag.ExitOnError)
	sub := fs.String("sub", "", "файл или URL подписки")
	channel := fs.String("channel", "xhttp", "канал: xhttp|vision")
	out := fs.String("out", "", "куда записать конфиг")
	_ = fs.Parse(args)
	if *sub == "" || *out == "" {
		fatal("нужны --sub <файл|URL> и --out <файл>")
	}
	data, err := readSource(*sub)
	if err != nil {
		fatal("%v", err)
	}
	chans, err := config.ParseSubscription(string(data))
	if err != nil {
		fatal("%v", err)
	}
	ch, err := app.PickChannel(chans, *channel)
	if err != nil {
		fatal("%v", err)
	}
	cfg, err := ch.CoreConfig(app.SocksPort, app.HTTPPort)
	if err != nil {
		fatal("%v", err)
	}
	if err := os.WriteFile(*out, cfg, 0o600); err != nil {
		fatal("%v", err)
	}
	fmt.Printf("конфиг (%s) записан в %s\n", ch.Transport, *out)
}

func cmdConnect(args []string) {
	fs := flag.NewFlagSet("connect", flag.ExitOnError)
	sub := fs.String("sub", "", "файл или URL подписки")
	channel := fs.String("channel", "xhttp", "канал: xhttp|vision")
	mode := fs.String("mode", "proxy", "режим: proxy|tun")
	_ = fs.Parse(args)
	if *sub == "" {
		fatal("укажите --sub <файл|URL>")
	}
	data, err := readSource(*sub)
	if err != nil {
		fatal("%v", err)
	}

	mgr := app.New()
	if err := mgr.Connect(string(data), *channel, *mode); err != nil {
		fatal("%v", err)
	}
	_, m, ch, server := mgr.Status()
	fmt.Printf("Подключено: режим %s, канал %s, сервер %s\n", m, ch, server)
	if m == "proxy" {
		fmt.Printf("Системный прокси: 127.0.0.1:%d\n", app.HTTPPort)
	}
	fmt.Println("Ctrl+C — отключить.")
	waitSignal()
	_ = mgr.Disconnect()
	fmt.Println("\nотключено")
}

func cmdProxy(args []string) {
	if len(args) == 0 || args[0] != "off" {
		fatal("использование: vpn-client proxy off")
	}
	if err := proxy.Disable(); err != nil {
		fatal("%v", err)
	}
	fmt.Println("системный прокси выключен")
}

func waitSignal() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "ошибка: "+format+"\n", a...)
	os.Exit(1)
}
