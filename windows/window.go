//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxn/walk"
	"github.com/wcomsat/vpn-client/internal/app"
	"github.com/wcomsat/vpn-client/internal/config"
)

// runWindow показывает нативное окно управления (блокирует до закрытия).
func runWindow(mgr *app.Manager) error {
	mw, err := walk.NewMainWindow()
	if err != nil {
		return err
	}
	mw.SetTitle("VPN")
	mw.SetSize(walk.Size{Width: 520, Height: 480})
	mw.SetLayout(walk.NewVBoxLayout())

	exe, _ := os.Executable()
	subPath := filepath.Join(filepath.Dir(exe), "subscription.txt")

	subLabel, _ := walk.NewLabel(mw)
	subLabel.SetText("Подписка (две ссылки vless:// — по одной в строке):")

	subEdit, _ := walk.NewTextEdit(mw)
	subEdit.SetMinMaxSize(walk.Size{Width: 0, Height: 96}, walk.Size{})
	if b, err := os.ReadFile(subPath); err == nil {
		_ = subEdit.SetText(strings.TrimSpace(string(b)))
	}

	row, _ := walk.NewComposite(mw)
	_ = row.SetLayout(walk.NewHBoxLayout())

	chLabel, _ := walk.NewLabel(row)
	chLabel.SetText("Канал:")
	chCombo, _ := walk.NewComboBox(row)
	chCombo.SetModel([]string{"xhttp"})
	chCombo.SetCurrentIndex(0)

	modeLabel, _ := walk.NewLabel(row)
	modeLabel.SetText("Режим:")
	modeCombo, _ := walk.NewComboBox(row)
	modeCombo.SetModel([]string{"proxy", "tun"})
	modeCombo.SetCurrentIndex(0)
	if !mgr.IsAdmin() {
		modeCombo.SetModel([]string{"proxy"})
	}

	btns, _ := walk.NewComposite(mw)
	_ = btns.SetLayout(walk.NewHBoxLayout())
	saveBtn, _ := walk.NewPushButton(btns)
	saveBtn.SetText("Сохранить")
	checkBtn, _ := walk.NewPushButton(btns)
	checkBtn.SetText("Проверить")
	onBtn, _ := walk.NewPushButton(btns)
	onBtn.SetText("Подключить")
	offBtn, _ := walk.NewPushButton(btns)
	offBtn.SetText("Отключить")
	offBtn.SetEnabled(false)

	statusLbl, _ := walk.NewLabel(mw)
	statusLbl.SetText("Отключено")

	populateChannels := func() {
		chans, err := config.ParseSubscription(subEdit.Text())
		if err != nil {
			chCombo.SetModel([]string{})
			return
		}
		seen := map[string]bool{}
		var names []string
		for _, c := range chans {
			n := c.Transport
			if n == "tcp" {
				n = "vision"
			}
			if !seen[n] {
				seen[n] = true
				names = append(names, n)
			}
		}
		chCombo.SetModel(names)
		if len(names) > 0 {
			chCombo.SetCurrentIndex(0)
		}
	}
	populateChannels()

	saveBtn.Clicked().Attach(func() {
		if err := os.WriteFile(subPath, []byte(subEdit.Text()), 0o600); err != nil {
			statusLbl.SetText("Не сохранить: " + err.Error())
			return
		}
		populateChannels()
		statusLbl.SetText("Сохранено. Каналы: " + strings.Join(chCombo.Model().([]string), ", "))
	})

	checkBtn.Clicked().Attach(func() {
		populateChannels()
		names := chCombo.Model().([]string)
		if len(names) == 0 {
			statusLbl.SetText("Ошибка: не удалось разобрать подписку")
			return
		}
		statusLbl.SetText("Каналы в подписке: " + strings.Join(names, ", "))
	})

	onBtn.Clicked().Attach(func() {
		channel := "xhttp"
		if chCombo.CurrentIndex() == 1 {
			channel = "vision"
		}
		mode := "proxy"
		if modeCombo.CurrentIndex() == 1 {
			mode = "tun"
		}
		onBtn.SetEnabled(false)
		statusLbl.SetText("Подключение…")
		go func() {
			err := mgr.Connect(subEdit.Text(), channel, mode)
			mw.Synchronize(func() {
				if err != nil {
					statusLbl.SetText("Ошибка: " + err.Error())
					onBtn.SetEnabled(true)
					return
				}
				_, m, c, s := mgr.Status()
				statusLbl.SetText(fmt.Sprintf("Подключено — %s, канал %s, %s", m, c, s))
				offBtn.SetEnabled(true)
			})
		}()
	})

	offBtn.Clicked().Attach(func() {
		_ = mgr.Disconnect()
		statusLbl.SetText("Отключено")
		offBtn.SetEnabled(false)
		onBtn.SetEnabled(true)
	})

	mw.Closing().Attach(func(canceled *bool, _ walk.CloseReason) {
		_ = mgr.Disconnect()
	})

	mw.Show()
	mw.Run()
	return nil
}
