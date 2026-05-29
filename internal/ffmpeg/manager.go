package ffmpeg

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/example/cliper/internal/config"
)

type Manager struct {
	cfg    config.Config
	mu     sync.Mutex
	cmd    *exec.Cmd
	cancel context.CancelFunc
}

func NewManager(cfg config.Config) *Manager {
	return &Manager{cfg: cfg}
}

func (m *Manager) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			m.Stop()
			return
		default:
		}

		runCtx, cancel := context.WithCancel(ctx)
		cmd := exec.CommandContext(runCtx, m.cfg.FFmpegPath, m.recordArgs()...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		m.mu.Lock()
		m.cmd = cmd
		m.cancel = cancel
		m.mu.Unlock()

		log.Printf("starting ffmpeg recorder with %s backend", m.cfg.Backend)
		if err := cmd.Start(); err != nil {
			log.Printf("failed to start ffmpeg: %v", err)
			cancel()
		} else if err := cmd.Wait(); err != nil && ctx.Err() == nil {
			log.Printf("ffmpeg exited: %v", err)
		} else if ctx.Err() == nil {
			log.Printf("ffmpeg exited without error")
		}

		m.mu.Lock()
		if m.cmd == cmd {
			m.cmd = nil
			m.cancel = nil
		}
		m.mu.Unlock()
		cancel()

		if ctx.Err() != nil {
			return
		}
		log.Printf("restarting ffmpeg in %s", m.cfg.RestartDelay)
		time.Sleep(m.cfg.RestartDelay)
	}
}

func (m *Manager) Stop() {
	m.mu.Lock()
	cancel := m.cancel
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (m *Manager) CleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(m.cfg.CleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.CleanupSegments(); err != nil {
				log.Printf("segment cleanup failed: %v", err)
			}
		}
	}
}

func (m *Manager) CleanupSegments() error {
	entries, err := filepath.Glob(filepath.Join(m.cfg.BufferDir, "segment_*.ts"))
	if err != nil {
		return fmt.Errorf("list buffer segments: %w", err)
	}
	if len(entries) <= m.cfg.MaxSegments+1 {
		return nil
	}

	type segment struct {
		path string
		mod  time.Time
	}
	segments := make([]segment, 0, len(entries))
	for _, entry := range entries {
		info, err := os.Stat(entry)
		if err != nil {
			continue
		}
		segments = append(segments, segment{path: entry, mod: info.ModTime()})
	}
	sort.Slice(segments, func(i, j int) bool { return segments[i].mod.Before(segments[j].mod) })

	keep := m.cfg.MaxSegments + 1 // include one likely in-progress segment outside the final playlist
	for _, segment := range segments[:max(0, len(segments)-keep)] {
		if err := os.Remove(segment.path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove old segment %s: %w", segment.path, err)
		}
	}
	return nil
}

func (m *Manager) recordArgs() []string {
	args := []string{"-hide_banner", "-loglevel", "warning", "-y"}
	args = append(args, m.inputArgs()...)
	args = append(args,
		"-an",
		"-c:v", m.cfg.VideoCodec,
	)
	args = appendOptionalFFmpegOption(args, "-preset", m.cfg.VideoPreset)
	args = appendOptionalFFmpegOption(args, "-tune", m.cfg.VideoTune)
	args = appendOptionalFFmpegOption(args, "-crf", m.cfg.VideoCRF)
	args = appendOptionalFFmpegOption(args, "-pix_fmt", m.cfg.PixelFormat)
	args = append(args,
		"-f", "segment",
		"-segment_time", fmt.Sprint(m.cfg.SegmentSeconds()),
		"-segment_format", "mpegts",
		"-segment_list", m.cfg.SegmentListPath,
		"-segment_list_type", "ffconcat",
		"-segment_list_size", fmt.Sprint(m.cfg.MaxSegments),
		"-segment_list_flags", "+live",
		"-reset_timestamps", "1",
		"-strftime", "1",
		m.cfg.SegmentName,
	)
	return args
}

func (m *Manager) inputArgs() []string {
	switch m.cfg.Backend {
	case config.BackendPipeWire:
		return []string{"-f", "pipewire", "-framerate", fmt.Sprint(m.cfg.FPS), "-video_size", m.cfg.VideoSize, "-i", "0"}
	case config.BackendKMSGrab:
		return []string{"-f", "kmsgrab", "-framerate", fmt.Sprint(m.cfg.FPS), "-i", "-", "-vf", fmt.Sprintf("hwmap=derive_device=vaapi,scale_vaapi=w=%s:h=%s:format=nv12", width(m.cfg.VideoSize), height(m.cfg.VideoSize))}
	case config.BackendX11:
		fallthrough
	default:
		return []string{"-f", "x11grab", "-framerate", fmt.Sprint(m.cfg.FPS), "-video_size", m.cfg.VideoSize, "-i", m.cfg.Display + m.cfg.CaptureOffset}
	}
}

func width(videoSize string) string {
	for i, r := range videoSize {
		if r == 'x' || r == 'X' {
			return videoSize[:i]
		}
	}
	return "1920"
}

func height(videoSize string) string {
	for i, r := range videoSize {
		if r == 'x' || r == 'X' {
			return videoSize[i+1:]
		}
	}
	return "1080"
}

func appendOptionalFFmpegOption(args []string, key, value string) []string {
	if value == "" {
		return args
	}
	return append(args, key, value)
}
