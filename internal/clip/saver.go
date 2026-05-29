package clip

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/example/cliper/internal/config"
)

type SaveOptions struct {
	Duration time.Duration
	Output   string
}

type Saver struct {
	cfg config.Config
	mu  sync.Mutex
}

func NewSaver(cfg config.Config) *Saver {
	return &Saver{cfg: cfg}
}

func (s *Saver) Save(ctx context.Context) (string, error) {
	return s.SaveWithOptions(ctx, SaveOptions{})
}

func (s *Saver) SaveWithOptions(ctx context.Context, opts SaveOptions) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	segments, err := s.currentSegments()
	if err != nil {
		return "", err
	}
	segments, err = s.segmentsForDuration(segments, opts.Duration)
	if err != nil {
		return "", err
	}
	if len(segments) == 0 {
		return "", fmt.Errorf("no finalized segments available yet")
	}

	timestamp := time.Now().Format("20060102_150405")
	snapshotDir := filepath.Join(s.cfg.ClipsDir, ".clip_snapshot_"+timestamp)
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		return "", fmt.Errorf("create snapshot directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(snapshotDir); err != nil {
			log.Printf("failed to remove snapshot directory %s: %v", snapshotDir, err)
		}
	}()

	copied := make([]string, 0, len(segments))
	for i, segment := range segments {
		destination := filepath.Join(snapshotDir, fmt.Sprintf("segment_%03d.ts", i))
		if err := copyFile(segment, destination); err != nil {
			return "", fmt.Errorf("copy segment %s: %w", segment, err)
		}
		copied = append(copied, destination)
	}

	concatList := filepath.Join(snapshotDir, "concat.ffconcat")
	if err := writeConcatList(concatList, copied); err != nil {
		return "", err
	}

	output := opts.Output
	if output == "" {
		output = filepath.Join(s.cfg.ClipsDir, "clip_"+timestamp+".mp4")
	} else if !filepath.IsAbs(output) {
		absolute, err := filepath.Abs(output)
		if err != nil {
			return "", fmt.Errorf("resolve output path: %w", err)
		}
		output = absolute
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}
	args := []string{
		"-hide_banner",
		"-loglevel", "warning",
		"-y",
		"-safe", "0",
		"-f", "concat",
		"-i", concatList,
		"-c", "copy",
		"-movflags", "+faststart",
		output,
	}
	cmd := exec.CommandContext(ctx, s.cfg.FFmpegPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Printf("saving clip with %d segments to %s", len(copied), output)
	if err := cmd.Run(); err != nil {
		_ = os.Remove(output)
		return "", fmt.Errorf("concat segments with ffmpeg: %w", err)
	}
	return output, nil
}

func (s *Saver) segmentsForDuration(segments []string, duration time.Duration) ([]string, error) {
	if duration < 0 {
		return nil, fmt.Errorf("clip duration must be greater than or equal to zero")
	}
	if duration == 0 {
		return segments, nil
	}
	if s.cfg.SegmentDuration <= 0 {
		return nil, fmt.Errorf("segment duration must be greater than zero")
	}
	wanted := int((duration + s.cfg.SegmentDuration - time.Nanosecond) / s.cfg.SegmentDuration)
	if wanted <= 0 || wanted >= len(segments) {
		return segments, nil
	}
	return segments[len(segments)-wanted:], nil
}

func (s *Saver) currentSegments() ([]string, error) {
	file, err := os.Open(s.cfg.SegmentListPath)
	if err != nil {
		return nil, fmt.Errorf("open segment list %s: %w", s.cfg.SegmentListPath, err)
	}
	defer file.Close()

	var segments []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "file ") {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(line, "file "))
		path = strings.Trim(path, "'")
		path = strings.ReplaceAll(path, "'\\''", "'")
		if !filepath.IsAbs(path) {
			path = filepath.Join(s.cfg.BufferDir, path)
		}
		if _, err := os.Stat(path); err == nil {
			segments = append(segments, path)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read segment list: %w", err)
	}
	if len(segments) > s.cfg.MaxSegments {
		segments = segments[len(segments)-s.cfg.MaxSegments:]
	}
	return segments, nil
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	info, err := input.Stat()
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return fmt.Errorf("segment is empty")
	}

	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer output.Close()

	if _, err := output.ReadFrom(input); err != nil {
		return err
	}
	return output.Sync()
}

func writeConcatList(path string, segments []string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create concat list: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString("ffconcat version 1.0\n"); err != nil {
		return err
	}
	for _, segment := range segments {
		if _, err := fmt.Fprintf(file, "file '%s'\n", escapeFFConcatPath(segment)); err != nil {
			return err
		}
	}
	return file.Sync()
}

func escapeFFConcatPath(path string) string {
	return strings.ReplaceAll(path, "'", "'\\''")
}
