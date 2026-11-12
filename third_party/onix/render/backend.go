package render

import "fmt"

// Backend is selected at compile time with an onix_<backend> build tag.
type Backend string

const (
	BackendDirectX Backend = "directx"
	BackendMetal   Backend = "metal"
	BackendVulkan  Backend = "vulkan"
	BackendOpenGL  Backend = "opengl"
	BackendWebGPU  Backend = "webgpu"
	BackendWebGL   Backend = "webgl"
)

// Backends lists a platform's renderers, with its default first.
func Backends(platform string) []Backend {
	switch platform {
	case "windows":
		return []Backend{BackendDirectX, BackendVulkan, BackendOpenGL}
	case "darwin", "macos":
		return []Backend{BackendMetal, BackendVulkan, BackendOpenGL}
	case "linux":
		return []Backend{BackendOpenGL, BackendVulkan}
	case "js", "web":
		return []Backend{BackendWebGPU, BackendWebGL}
	case "wechat":
		return []Backend{BackendWebGL}
	case "android":
		return []Backend{BackendVulkan, BackendOpenGL}
	case "ios":
		return []Backend{BackendMetal}
	default:
		return nil
	}
}

func ResolveBackend(platform string, backend Backend) (Backend, error) {
	available := Backends(platform)
	if backend == "" && len(available) != 0 {
		return available[0], nil
	}
	for _, candidate := range available {
		if candidate == backend {
			return backend, nil
		}
	}
	return "", fmt.Errorf("render: backend %q is not supported on %s", backend, platform)
}

func (backend Backend) BuildTag() string { return "onix_" + string(backend) }

func (backend Backend) String() string {
	switch backend {
	case BackendDirectX:
		return "DirectX"
	case BackendMetal:
		return "Metal"
	case BackendVulkan:
		return "Vulkan"
	case BackendOpenGL:
		return "OpenGL"
	case BackendWebGPU:
		return "WebGPU"
	case BackendWebGL:
		return "WebGL"
	default:
		return string(backend)
	}
}
