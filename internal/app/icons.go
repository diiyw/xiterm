package app

import (
	"fmt"

	"github.com/diiyw/xiterm/assets"
	"github.com/diiyw/xiterm/internal/workspace"
	"github.com/onix-engine/onix/graphics"
	svgcodec "github.com/onix-engine/onix/graphics/codec/svg"
)

func (a *App) loadIcons() error {
	a.icons = make(map[string]*graphics.Texture)
	entries, err := assets.UIIcons.ReadDir("icons/ui")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		data, err := assets.UIIcons.ReadFile("icons/ui/" + entry.Name())
		if err != nil {
			a.disposeIcons()
			return err
		}
		texture, err := svgcodec.DecodeAtScale(data, 3, graphics.TextureOptions{Mipmap: true, Usage: graphics.ResourceUsageDefault})
		if err != nil {
			a.disposeIcons()
			return fmt.Errorf("load icon %s: %w", entry.Name(), err)
		}
		a.icons[entry.Name()[:len(entry.Name())-4]] = texture
	}
	return nil
}

func (a *App) disposeIcons() {
	for _, texture := range a.icons {
		texture.Dispose()
	}
}

func (a *App) drawIcon(name string, r workspace.Rect, color graphics.ColorRGBA) {
	if texture := a.icons[name]; texture != nil {
		a.paint.canvas.BindTexture(texture)
		a.paint.canvas.DrawTintedAffine([6]float32{r.W, 0, r.X, 0, r.H, r.Y}, [6]float32{1, 0, 0, 0, 1, 0}, color, 1)
	}
}

func (a *App) addIconButton(name string, r workspace.Rect, action func()) {
	a.buttons = append(a.buttons, button{rect: r, label: name, action: action})
	color := muted
	if r.Contains(a.mouseX, a.mouseY) {
		a.paint.rect(r, surface)
		color = foreground
	}
	a.drawIcon(name, workspace.Rect{X: r.X + (r.W-16)/2, Y: r.Y + (r.H-16)/2, W: 16, H: 16}, color)
}
