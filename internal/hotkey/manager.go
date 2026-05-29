package hotkey

import (
	"bufio"
	"context"
	"log"
	"os"
)

type Manager interface {
	Run(context.Context, chan<- struct{}) error
	Close() error
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
