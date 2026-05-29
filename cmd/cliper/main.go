package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/example/cliper/internal/clip"
	"github.com/example/cliper/internal/config"
	"github.com/example/cliper/internal/ffmpeg"
	"github.com/example/cliper/internal/hotkey"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}
	log.Printf("buffer directory: %s", cfg.BufferDir)
	log.Printf("clips directory: %s", cfg.ClipsDir)
	log.Printf("target buffer: %s (%d x %s segments)", cfg.ClipDuration(), cfg.MaxSegments, cfg.SegmentDuration)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	recorder := ffmpeg.NewManager(cfg)
	go recorder.Run(ctx)
	go recorder.CleanupLoop(ctx)

	saver := clip.NewSaver(cfg)
	clipRequests := make(chan struct{}, 1)
	manager := buildHotkeyManager(cfg)
	defer manager.Close()
	go func() {
		if err := manager.Run(ctx, clipRequests); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("hotkey manager stopped: %v", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			log.Printf("shutting down")
			recorder.Stop()
			return
		case <-clipRequests:
			go func() {
				saveCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()
				output, err := saver.Save(saveCtx)
				if err != nil {
					log.Printf("failed to save clip: %v", err)
					return
				}
				log.Printf("clip saved: %s", output)
			}()
		}
	}
}

func buildHotkeyManager(cfg config.Config) hotkey.Manager {
	if os.Getenv("DISPLAY") == "" || cfg.Backend != config.BackendX11 {
		return hotkey.NewTerminal()
	}
	manager, err := hotkey.NewX11(cfg.Hotkey)
	if err != nil {
		log.Printf("failed to initialize X11 hotkey %s: %v", cfg.Hotkey, err)
		return hotkey.NewTerminal()
	}
	log.Printf("press %s to save the latest clip", cfg.Hotkey)
	return manager
}
