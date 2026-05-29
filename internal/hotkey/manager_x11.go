//go:build linux && cgo && x11hotkey

package hotkey

/*
#cgo linux LDFLAGS: -lX11
#include <stdlib.h>
#include <X11/Xlib.h>
#include <X11/keysym.h>

static int cliper_grab_key(Display* display, Window root, KeyCode keycode, unsigned int modifiers) {
    return XGrabKey(display, keycode, modifiers, root, True, GrabModeAsync, GrabModeAsync);
}

static void cliper_ungrab_key(Display* display, Window root, KeyCode keycode) {
    XUngrabKey(display, keycode, AnyModifier, root);
}

static int cliper_pending(Display* display) {
    return XPending(display);
}

static int cliper_next_event(Display* display) {
    XEvent event;
    XNextEvent(display, &event);
    return event.type;
}
*/
import "C"

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"unsafe"
)

type Manager interface {
	Run(context.Context, chan<- struct{}) error
	Close() error
}

type X11Manager struct {
	display *C.Display
	root    C.Window
	keycode C.KeyCode
}

func NewX11(displayName, hotkey string) (*X11Manager, error) {
	name := C.CString(displayName)
	defer C.free(unsafe.Pointer(name))

	display := C.XOpenDisplay(name)
	if display == nil {
		return nil, fmt.Errorf("connect to X11 display %q", displayName)
	}

	keysym := keysymFor(hotkey)
	keycode := C.XKeysymToKeycode(display, keysym)
	if keycode == 0 {
		C.XCloseDisplay(display)
		return nil, fmt.Errorf("hotkey %s not found in keyboard mapping", hotkey)
	}

	root := C.XDefaultRootWindow(display)
	return &X11Manager{display: display, root: root, keycode: keycode}, nil
}

func (m *X11Manager) Run(ctx context.Context, out chan<- struct{}) error {
	if err := m.grab(); err != nil {
		return err
	}
	defer m.ungrab()
	log.Printf("global hotkey registered on X11")

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if C.cliper_pending(m.display) == 0 {
			time.Sleep(30 * time.Millisecond)
			continue
		}
		if eventType := C.cliper_next_event(m.display); eventType == C.KeyPress {
			select {
			case out <- struct{}{}:
			default:
				log.Printf("clip request ignored because another request is pending")
			}
		}
	}
}

func (m *X11Manager) Close() error {
	if m.display != nil {
		C.XCloseDisplay(m.display)
		m.display = nil
	}
	return nil
}

func (m *X11Manager) grab() error {
	modifiers := []C.uint{0, C.LockMask, C.Mod2Mask, C.LockMask | C.Mod2Mask}
	for _, modifier := range modifiers {
		C.cliper_grab_key(m.display, m.root, m.keycode, modifier)
	}
	C.XFlush(m.display)
	return nil
}

func (m *X11Manager) ungrab() {
	C.cliper_ungrab_key(m.display, m.root, m.keycode)
	C.XFlush(m.display)
}

type TerminalManager struct{}

func NewTerminal() *TerminalManager { return &TerminalManager{} }

func (m *TerminalManager) Run(ctx context.Context, out chan<- struct{}) error {
	log.Printf("terminal fallback active: press Enter to save a clip")
	lines := make(chan struct{}, 1)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			lines <- struct{}{}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-lines:
			select {
			case out <- struct{}{}:
			default:
				log.Printf("clip request ignored because another request is pending")
			}
		}
	}
}

func (m *TerminalManager) Close() error { return nil }

func keysymFor(name string) C.KeySym {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "F1":
		return C.XK_F1
	case "F2":
		return C.XK_F2
	case "F3":
		return C.XK_F3
	case "F4":
		return C.XK_F4
	case "F5":
		return C.XK_F5
	case "F6":
		return C.XK_F6
	case "F7":
		return C.XK_F7
	case "F9":
		return C.XK_F9
	case "F10":
		return C.XK_F10
	case "F11":
		return C.XK_F11
	case "F12":
		return C.XK_F12
	case "F8":
		fallthrough
	default:
		return C.XK_F8
	}
}
