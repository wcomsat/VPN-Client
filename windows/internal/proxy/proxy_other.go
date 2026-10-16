//go:build !windows

package proxy

import "errors"

// Set/Disable на не-Windows платформах недоступны (эта сборка — только Windows).
func Set(addr string) (func() error, error) {
	return nil, errors.New("системный прокси поддерживается только в Windows")
}

func Disable() error {
	return errors.New("системный прокси поддерживается только в Windows")
}
