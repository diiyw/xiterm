//go:build cgo && darwin && !onix_directx && !onix_vulkan && !onix_opengl && !onix_webgpu && !onix_webgl

package render

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestMetalMemoryLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "metal_memory_test")
	command := exec.CommandContext(ctx, "xcrun", "clang", "-fno-objc-arc", "-O2", "tests/metal_memory_test.m", "-framework", "Foundation", "-framework", "Metal", "-framework", "QuartzCore", "-framework", "CoreVideo", "-o", binary)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile Metal memory regression: %v\n%s", err, output)
	}
	command = exec.CommandContext(ctx, binary)
	command.Env = append(os.Environ(), "MTL_DEBUG_LAYER=1")
	output, err := command.CombinedOutput()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 77 {
		t.Skipf("Metal device unavailable: %s", output)
	}
	if err != nil {
		t.Fatalf("Metal memory regression: %v\n%s", err, output)
	}
	t.Log(string(output))
}
