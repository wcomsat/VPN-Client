//go:build !windows

package tun

import "errors"

var errUnsupported = errors.New("TUN-режим поддерживается только в Windows")

func IsAdmin() bool { return false }
func Elevate() error { return errUnsupported }
func PrimaryInterface() (string, string, error) { return "", "", errUnsupported }
func Start(tun2socksPath, socksAddr, serverIP, ifaceAlias, gateway string) (func() error, error) {
	return nil, errUnsupported
}
