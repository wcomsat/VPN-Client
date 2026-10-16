//go:build windows

package main

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/getlantern/systray"
	"github.com/wcomsat/vpn-client/internal/app"
)

//go:embed assets/icon.ico
var trayIcon []byte

// runTray показывает иконку в системном трее (блокирует до «Выход»).
func runTray(mgr *app.Manager, openUI func()) {
	systray.Run(func() {
		systray.SetIcon(trayIcon)
		systray.SetTitle("VPN")
		systray.SetTooltip("VPN-Client")

		mOpen := systray.AddMenuItem("Открыть интерфейс", "Показать страницу управления")
		systray.AddSeparator()
		mProxy := systray.AddMenuItem("Подключить: прокси", "Браузеры и приложения")
		mTun := systray.AddMenuItem("Подключить: TUN (весь трафик)", "Нужны права администратора")
		mOff := systray.AddMenuItem("Отключить", "")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Выход", "")

		refresh := func() {
			running, mode, ch, _ := mgr.Status()
			if running {
				systray.SetTooltip("VPN: подключено (" + mode + ", " + ch + ")")
			} else {
				systray.SetTooltip("VPN: отключено")
			}
		}
		refresh()

		go func() {
			for {
				select {
				case <-mOpen.ClickedCh:
					openUI()
				case <-mProxy.ClickedCh:
					_ = connectFromTray(mgr, "proxy")
					refresh()
				case <-mTun.ClickedCh:
					_ = connectFromTray(mgr, "tun")
					refresh()
				case <-mOff.ClickedCh:
					_ = mgr.Disconnect()
					refresh()
				case <-mQuit.ClickedCh:
					systray.Quit()
					return
				}
			}
		}()
	}, func() {
		_ = mgr.Disconnect()
	})
}

func connectFromTray(mgr *app.Manager, mode string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	sub, err := os.ReadFile(filepath.Join(filepath.Dir(exe), "subscription.txt"))
	if err != nil {
		return err
	}
	return mgr.Connect(string(sub), "xhttp", mode)
}
