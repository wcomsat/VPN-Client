// Команда vpn-client — минималистичный клиент под личный VPN-сервис.
//
// Это каркас: парсинг подписки и сборка конфига ядра уже реализованы и
// покрыты тестами; подключение системного трафика (tun2socks + WinTun) и трей
// добавляются на следующем шаге.
//
// Примеры:
//
//	vpn-client version
//	vpn-client parse   --sub subscription.txt
//	vpn-client config  --sub subscription.txt --channel xhttp --out config.json
//	vpn-client connect --sub subscription.txt [--channel vision]
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/wcomsat/vpn-client/internal/config"
	"github.com/wcomsat/vpn-client/internal/core"
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
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "неизвестная команда: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`vpn-client — клиент личного VPN

Команды:
  version
  parse   --sub <файл>                          показать каналы из подписки
  config  --sub <файл> [--channel xhttp|vision] --out <файл>
  connect --sub <файл> [--channel xhttp|vision]   поднять канал (каркас)

Флаги:
  --sub      файл подписки (две ссылки vless://)
  --channel  канал: xhttp (по умолчанию) или vision
  --out      куда записать конфиг ядра
  --socks    порт локального SOCKS (по умолчанию 10808)
`)
}

// loadSubscription читает подписку и выбирает канал.
func loadSubscription(subPath, channel string) (*config.Channel, error) {
	data, err := os.ReadFile(subPath)
	if err != nil {
		return nil, fmt.Errorf("не читается подписка %s: %w", subPath, err)
	}
	chans, err := config.ParseSubscription(string(data))
	if err != nil {
		return nil, err
	}
	want := "xhttp"
	if channel != "" {
		want = channel
	}
	for _, c := range chans {
		if c.Transport == want {
			return c, nil
		}
	}
	return nil, fmt.Errorf("в подписке нет канала %q", want)
}

func cmdParse(args []string) {
	fs := flag.NewFlagSet("parse", flag.ExitOnError)
	sub := fs.String("sub", "", "файл подписки")
	_ = fs.Parse(args)
	if *sub == "" {
		fatal("укажите --sub <файл>")
	}
	data, err := os.ReadFile(*sub)
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
	sub := fs.String("sub", "", "файл подписки")
	channel := fs.String("channel", "xhttp", "канал: xhttp|vision")
	out := fs.String("out", "", "куда записать конфиг")
	socks := fs.Int("socks", 10808, "порт локального SOCKS")
	_ = fs.Parse(args)
	if *sub == "" || *out == "" {
		fatal("нужны --sub <файл> и --out <файл>")
	}
	ch, err := loadSubscription(*sub, *channel)
	if err != nil {
		fatal("%v", err)
	}
	cfg, err := ch.CoreConfig(*socks)
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
	sub := fs.String("sub", "", "файл подписки")
	channel := fs.String("channel", "xhttp", "канал: xhttp|vision")
	socks := fs.Int("socks", 10808, "порт локального SOCKS")
	_ = fs.Parse(args)
	if *sub == "" {
		fatal("укажите --sub <файл>")
	}
	ch, err := loadSubscription(*sub, *channel)
	if err != nil {
		fatal("%v", err)
	}
	cfg, err := ch.CoreConfig(*socks)
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
	fmt.Printf("канал %s поднят; SOCKS на 127.0.0.1:%d\n", ch.Transport, *socks)
	fmt.Println("ВАЖНО: маршрутизация системного трафика (tun2socks + WinTun) — следующий шаг.")
	fmt.Printf("Лог ядра: %s\n\nCtrl+C — остановить.\n", proc.LogPath())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	_ = proc.Stop()
	fmt.Println("\nостановлено")
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "ошибка: "+format+"\n", a...)
	os.Exit(1)
}
