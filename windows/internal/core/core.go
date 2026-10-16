// Package core: запуск встроенного ядра Xray-core с готовым конфигом.
//
// Ядро (xray.exe) поставляется рядом с приложением. Туннелирование системного
// трафика выполняет tun2socks (см. README), ядро лишь поднимает SOCKS-инбаунд.
package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Process — запущенный процесс ядра.
type Process struct {
	cmd     *exec.Cmd
	tmpDir  string
	logFile string
}

// XrayPath ищет xray.exe рядом с исполняемым файлом приложения.
func XrayPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(filepath.Dir(exe), "xray.exe")
	if _, err := os.Stat(candidate); err != nil {
		return "", fmt.Errorf("ядро не найдено (%s): положите xray.exe рядом с приложением", candidate)
	}
	return candidate, nil
}

// Start запускает ядро с конфигом и возвращает процесс.
func Start(xrayPath string, cfg []byte) (*Process, error) {
	dir, err := os.MkdirTemp("", "vpn-client")
	if err != nil {
		return nil, err
	}
	cfgPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfgPath, cfg, 0o600); err != nil {
		return nil, err
	}
	logPath := filepath.Join(dir, "xray.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(xrayPath, "run", "-c", cfgPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return nil, fmt.Errorf("не удалось запустить ядро: %w", err)
	}
	return &Process{cmd: cmd, tmpDir: dir, logFile: logPath}, nil
}

// Wait ожидает завершения процесса ядра.
func (p *Process) Wait() error {
	defer p.cleanup()
	return p.cmd.Wait()
}

// Stop останавливает ядро.
func (p *Process) Stop() error {
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}

// LogPath — путь к логу ядра (для диагностики).
func (p *Process) LogPath() string { return p.logFile }

func (p *Process) cleanup() {
	if p.tmpDir != "" {
		_ = os.RemoveAll(p.tmpDir)
	}
}
