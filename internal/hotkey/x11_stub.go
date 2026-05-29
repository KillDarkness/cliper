//go:build !linux || !cgo || !cliper_x11

package hotkey

import (
	"context"
	"fmt"
)

type X11Manager struct{}

func NewX11(displayName, hotkey string) (*X11Manager, error) {
	return nil, fmt.Errorf("X11 hotkey support is not included in this build; rebuild with CGO enabled and -tags cliper_x11")
}

func (m *X11Manager) Run(context.Context, chan<- struct{}) error {
	return fmt.Errorf("X11 hotkey support is not included in this build")
}

func (m *X11Manager) Close() error { return nil }
