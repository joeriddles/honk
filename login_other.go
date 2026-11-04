//go:build !darwin

package main

import "errors"

func loginSupported() bool { return false }
func loginEnabled() bool   { return false }
func setLogin(bool) error  { return errors.New("start at login is only on macOS") }
