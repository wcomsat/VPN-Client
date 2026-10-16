// Команда vpn-client — минималистичный клиент личного VPN-сервиса.
//
// Режим Windows: локальные SOCKS/HTTP-инбаунды Xray + системный HTTP-прокси.
// Покрывает браузеры и приложения, уважающие системный прокси (без драйвера и
// прав администратора). Полноценный TUN (весь трафик, включая UDP) — следующий шаг.
//
// Примеры:
//
//	vpn-client version
//	vpn-client parse   --sub subscription.txt
//	vpn-client config  --sub subscription.txt --channel xhttp --out config.json
//	vpn-client connect --sub https://example/sub/ivan   # вкл. системный прокси
//	vpn-client proxy off                                 # выкл. системный прокси
package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/wcomsat/vpn-client/internal/config"
	"github.com/wcomsat/vpn-client/internal/core"
	"github.com/wcomsat/vpn-client/internal/proxy"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "version", "-v", "--version":
		fmt.Println("vpn-client", version)
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
  version
  parse   --sub <файл|URL>                      показать каналы из подписки
  config  --sub <файл|URL> [--channel xhttp|vision] --out <файл>
  connect --sub <файл|URL> [--channel xhttp|vision] [--no-proxy]
  proxy   off                                    выключить системный прокси

Флаги connect/config:
  --channel  канал: xhttp (по умолчанию) или vision
  --socks    порт локального SOCKS  (по умолчанию 10808)
  --http     порт локального HTTP   (по умолчанию 10809, для системного прокси)
  --no-proxy не трогать системный прокси Windows

Для работы connect/config рядом с exe нужен xray.exe (ядро Xray-core).
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

func loadSubscription(src, channel string) (*config.Channel, error) {
	data, err := readSource(src)
	if err != nil {
		return nil, err
	}
	chans, err := config.ParseSubscription(string(data))
	if err != nil {
		return nil, err
	}
	// «vision» — это имя нашего резервного канала; его транспорт в ссылке — tcp.
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
	socks := fs.Int("socks", 10808, "порт локального SOCKS")
	httpPort := fs.Int("http", 10809, "порт локального HTTP")
	_ = fs.Parse(args)
	if *sub == "" || *out == "" {
		fatal("нужны --sub <файл|URL> и --out <файл>")
	}
	ch, err := loadSubscription(*sub, *channel)
	if err != nil {
		fatal("%v", err)
	}
	cfg, err := ch.CoreConfig(*socks, *httpPort)
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
	socks := fs.Int("socks", 10808, "порт локального SOCKS")
	httpPort := fs.Int("http", 10809, "порт локального HTTP")
	noProxy := fs.Bool("no-proxy", false, "не трогать системный прокси Windows")
	_ = fs.Parse(args)
	if *sub == "" {
		fatal("укажите --sub <файл|URL>")
	}

	ch, err := loadSubscription(*sub, *channel)
	if err != nil {
		fatal("%v", err)
	}
	cfg, err := ch.CoreConfig(*socks, *httpPort)
	if err != nil {
		fatal("%v", err)
	}
	xray, err := core.XrayPath()
	if err != nil {
		fatal("%v", err)
	}
	proc, err := core.Start(xray, cfg)
	if err != nil {
		fatal("%v", err)
	}

	if !waitPort(*socks, 10*time.Second) {
		_ = proc.Stop()
		fatal("ядро Xray не подняло SOCKS на 127.0.0.1:%d (см. %s)", *socks, proc.LogPath())
	}

	var restore func() error
	if !*noProxy {
		restore, err = proxy.Set(fmt.Sprintf("127.0.0.1:%d", *httpPort))
		if err != nil {
			_ = proc.Stop()
			fatal("не удалось включить системный прокси: %v", err)
		}
		fmt.Printf("Системный прокси включён: 127.0.0.1:%d\n", *httpPort)
	}

	fmt.Printf("Канал: %s (%s:%d, sni=%s)\n", ch.Transport, ch.Address, ch.Port, ch.Reality.ServerName)
	fmt.Printf("SOCKS: 127.0.0.1:%d   HTTP: 127.0.0.1:%d\n", *socks, *httpPort)
	fmt.Printf("Лог ядра: %s\n\nCtrl+C — отключить.\n", proc.LogPath())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	_ = proc.Stop()
	if restore != nil {
		if err := restore(); err != nil {
			fmt.Fprintf(os.Stderr, "внимание: не удалось восстановить прокси: %v\n", err)
		}
	}
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

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "ошибка: "+format+"\n", a...)
	os.Exit(1)
}
