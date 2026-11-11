//go:build !privacy

package main

import (
	"explorer/internal/authv2"
	"os"
)

func startV2IfSelected() (bool, error) {
	if os.Getenv("EXPLORER_AUTH_PROTOCOL") != "" {
		return true, authv2.ErrInvalid
	}
	return false, nil
}
