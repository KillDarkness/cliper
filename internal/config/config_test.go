package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAcceptsManualCaptureAndRecordingSettings(t *testing.T) {
	t.Setenv("CLIPER_BUFFER_DIR", filepath.Join(t.TempDir(), "buffer"))
	t.Setenv("CLIPER_CLIPS_DIR", filepath.Join(t.TempDir(), "clips"))
	t.Setenv("CLIPER_VIDEO_SIZE", "1366x768")
	t.Setenv("CLIPER_CAPTURE_OFFSET", "+10,20")
	t.Setenv("CLIPER_FPS", "30")
	t.Setenv("CLIPER_VIDEO_PRESET", "fast")
	t.Setenv("CLIPER_VIDEO_CRF", "23")
	t.Setenv("CLIPER_SEGMENT_DURATION", "3s")
	t.Setenv("CLIPER_MAX_SEGMENTS", "40")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.VideoSize != "1366x768" || cfg.VideoSizeSource != "CLIPER_VIDEO_SIZE" {
		t.Fatalf("VideoSize = %q (%s), want 1366x768 from CLIPER_VIDEO_SIZE", cfg.VideoSize, cfg.VideoSizeSource)
	}
	if cfg.CaptureOffset != "+10,20" {
		t.Fatalf("CaptureOffset = %q, want +10,20", cfg.CaptureOffset)
	}
	if cfg.FPS != 30 || cfg.VideoPreset != "fast" || cfg.VideoCRF != "23" {
		t.Fatalf("recording settings = fps %d preset %q crf %q", cfg.FPS, cfg.VideoPreset, cfg.VideoCRF)
	}
	if cfg.SegmentDuration != 3*time.Second || cfg.MaxSegments != 40 || cfg.ClipDuration() != 120*time.Second {
		t.Fatalf("buffer settings = segment %s max %d clip %s", cfg.SegmentDuration, cfg.MaxSegments, cfg.ClipDuration())
	}
}

func TestLoadRejectsInvalidCaptureOffset(t *testing.T) {
	t.Setenv("CLIPER_BUFFER_DIR", filepath.Join(t.TempDir(), "buffer"))
	t.Setenv("CLIPER_CLIPS_DIR", filepath.Join(t.TempDir(), "clips"))
	t.Setenv("CLIPER_VIDEO_SIZE", "1366x768")
	t.Setenv("CLIPER_CAPTURE_OFFSET", "10,20")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid capture offset error")
	}
}
