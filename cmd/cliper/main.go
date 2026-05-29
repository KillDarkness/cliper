package main

import (
	"context"
	"errors"
	"flag"
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

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "save", "clip", "join":
			runSaveCommand(cfg, os.Args[1:])
			return
		}
	}

	runRecorder(cfg)
}

func runRecorder(cfg config.Config) {
	log.Printf("buffer directory: %s", cfg.BufferDir)
	log.Printf("clips directory: %s", cfg.ClipsDir)
	log.Printf("capture size: %s (%s)", cfg.VideoSize, cfg.VideoSizeSource)
	if cfg.Backend == config.BackendX11 {
		log.Printf("capture input: %s%s", cfg.Display, cfg.CaptureOffset)
	}
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

func runSaveCommand(cfg config.Config, args []string) {
	flags := flag.NewFlagSet(args[0], flag.ExitOnError)
	flags.Usage = func() {
		log.Printf("usage: cliper %s [-duration 30s] [-output clip.mp4] [-timeout 10m]", args[0])
		flags.PrintDefaults()
	}

	duration := flags.Duration("duration", 0, "clip duration to save from the end of the buffer; 0 saves every finalized segment in the playlist")
	flags.DurationVar(duration, "d", 0, "short alias for -duration")
	output := flags.String("output", "", "output MP4 path; defaults to CLIPER_CLIPS_DIR/clip_TIMESTAMP.mp4")
	flags.StringVar(output, "o", "", "short alias for -output")
	timeout := flags.Duration("timeout", 10*time.Minute, "maximum time to wait for FFmpeg to concatenate the clip")

	if err := flags.Parse(args[1:]); err != nil {
		log.Fatalf("parse %s flags: %v", args[0], err)
	}
	if flags.NArg() != 0 {
		log.Fatalf("unexpected positional arguments: %v", flags.Args())
	}
	if *timeout <= 0 {
		log.Fatalf("-timeout must be greater than zero")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	log.Printf("buffer directory: %s", cfg.BufferDir)
	log.Printf("clips directory: %s", cfg.ClipsDir)
	clipSaver := clip.NewSaver(cfg)
	path, err := clipSaver.SaveWithOptions(ctx, clip.SaveOptions{
		Duration: *duration,
		Output:   *output,
	})
	if err != nil {
		log.Fatalf("failed to save clip: %v", err)
	}
	log.Printf("clip saved: %s", path)
}

func buildHotkeyManager(cfg config.Config) hotkey.Manager {
	if cfg.Display == "" || cfg.Backend != config.BackendX11 {
		return hotkey.NewTerminal()
	}
	manager, err := hotkey.NewX11(cfg.Display, cfg.Hotkey)
	if err != nil {
		log.Printf("failed to initialize X11 hotkey %s: %v", cfg.Hotkey, err)
		return hotkey.NewTerminal()
	}
	log.Printf("press %s to save the latest clip", cfg.Hotkey)
	return manager
}
