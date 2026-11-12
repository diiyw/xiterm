package graphics

import (
	"sync"
	"weak"

	"github.com/onix-engine/onix/internal/reuse"
	"github.com/onix-engine/onix/render"
)

// A live texture owns this record. The cache keeps only a weak texture reference;
// the independent residencies let the render thread retire an abandoned texture.
type colorTextureEntry struct {
	texture     weak.Pointer[Texture]
	residencies render.DeviceValues[render.TextureHandle]
	key         int
}

var colorTextures struct {
	entries      map[int]*colorTextureEntry
	slots        []*colorTextureEntry
	retention    reuse.Retention
	mapRetention reuse.MapRetention
	sweep        int
	sync.Mutex
}

func CreateTextureByColor(clr ColorRGBA) *Texture {
	key := clr.Int32()
	colorTextures.Lock()
	defer colorTextures.Unlock()
	if entry := colorTextures.entries[key]; entry != nil {
		if texture := entry.texture.Value(); texture != nil && texture.Valid() {
			return texture
		}
	}
	// CPU-only callers can retire dead CPU records without issuing GPU calls.
	_ = sweepColorTextures(nil, 16)
	texture := newTexture(1, 1, true, ResourceUsageDefault, []byte{
		uint8(clr.R * 255), uint8(clr.G * 255), uint8(clr.B * 255), uint8(clr.A * 255),
	})
	entry := &colorTextureEntry{texture: weak.Make(texture), key: key}
	texture.colorOwner = entry
	if colorTextures.entries == nil {
		colorTextures.entries = make(map[int]*colorTextureEntry)
	}
	colorTextures.entries[key] = entry
	colorTextures.slots = append(colorTextures.slots, entry)
	colorTextures.retention.Record(len(colorTextures.slots))
	colorTextures.mapRetention.Record(len(colorTextures.entries))
	return texture
}

func (t *Texture) deviceValues() *render.DeviceValues[render.TextureHandle] {
	if t.colorOwner != nil {
		return &t.colorOwner.residencies
	}
	return &t.residencies
}

// Called after CPU draw submission, on the thread owning this device. Resources
// for another live device stay queued until that device completes a frame.
func maintainColorTextures(device *render.Device) error {
	colorTextures.Lock()
	defer colorTextures.Unlock()
	err := sweepColorTextures(device, 64)
	colorTextures.slots = colorTextures.retention.Trim(colorTextures.slots, 32)
	colorTextures.entries = colorTextures.mapRetention.Trim(colorTextures.entries, 32)
	return err
}

// Requires colorTextures' lock. Swap removal preserves the entry addresses
// borrowed by live textures, and the cursor bounds work without rescanning keys.
func sweepColorTextures(device *render.Device, budget int) error {
	for range min(budget, len(colorTextures.slots)) {
		if colorTextures.sweep >= len(colorTextures.slots) {
			colorTextures.sweep = 0
		}
		index := colorTextures.sweep
		entry := colorTextures.slots[index]
		if entry.texture.Value() != nil {
			colorTextures.sweep++
			continue
		}
		if handle, ok := entry.residencies.Remove(device); ok && device.Driver() != nil {
			if err := device.Driver().DestroyTexture(handle); err != nil {
				entry.residencies.Set(device, handle)
				return err
			}
		}
		resident := false
		_ = entry.residencies.Each(func(*render.Device, render.TextureHandle) error { resident = true; return nil })
		if resident {
			colorTextures.sweep++
			continue
		}
		if colorTextures.entries[entry.key] == entry {
			delete(colorTextures.entries, entry.key)
		}
		last := len(colorTextures.slots) - 1
		colorTextures.slots[index] = colorTextures.slots[last]
		colorTextures.slots[last] = nil
		colorTextures.slots = colorTextures.slots[:last]
	}
	return nil
}
