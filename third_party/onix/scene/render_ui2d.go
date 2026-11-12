package scene

import (
	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/graphics"
	"github.com/go-gl/mathgl/mgl32"
)

// UIImageMode controls how a UI image is fitted into its node.
type UIImageMode uint8

const (
	// UIImageAuto preserves the source aspect ratio and fits inside the node.
	UIImageAuto UIImageMode = iota
	// UIImageStretch stretches the source image to the node bounds.
	UIImageStretch
	// UIImageSliced stretches the center while preserving the configured borders.
	UIImageSliced
)

// UIBackground2D is the background layer of a UI node.
type UIBackground2D struct {
	Shape Shape2D
	Alpha float32
}

// UIImage2D is the image layer of a UI node.
type UIImage2D struct {
	Texture *graphics.Texture
	Slice   *NineSlice2D
	Mode    UIImageMode
	Tint    graphics.ColorRGBA
	Alpha   float32
}

// UIBorder2D is the border layer of a UI node.
type UIBorder2D struct {
	Shape Shape2D
	Alpha float32
}

func NewUIBackground2D(shape Shape2D) UIBackground2D {
	return UIBackground2D{Shape: shape, Alpha: 1}
}

func NewUIImage2D(texture *graphics.Texture) UIImage2D {
	return UIImage2D{Texture: texture, Mode: UIImageAuto, Tint: graphics.White, Alpha: 1}
}

func NewUIBorder2D(shape Shape2D) UIBorder2D {
	return UIBorder2D{Shape: shape, Alpha: 1}
}

type uiShapeLayer2D interface {
	uiShape() Shape2D
	uiAlpha() float32
}

func (background UIBackground2D) uiShape() Shape2D { return background.Shape }
func (background UIBackground2D) uiAlpha() float32 { return background.Alpha }
func (border UIBorder2D) uiShape() Shape2D         { return border.Shape }
func (border UIBorder2D) uiAlpha() float32         { return border.Alpha }

func appendUIVisualRenderItems2D(c *graphics.Canvas2D, world *ecs.World, snapshots *renderSnapshots2D, hierarchy *renderHierarchy2DCache, items []renderItem2D, cull bool) []renderItem2D {
	items = snapshots.appendUIShapeLayer2D[UIBackground2D](c, world, hierarchy, items, cull)
	items = appendUIImageLayer2D(c, world, snapshots, hierarchy, items, cull)
	return snapshots.appendUIShapeLayer2D[UIBorder2D](c, world, hierarchy, items, cull)
}

func (snapshots *renderSnapshots2D) appendUIShapeLayer2D[T uiShapeLayer2D](c *graphics.Canvas2D, world *ecs.World, hierarchy *renderHierarchy2DCache, items []renderItem2D, cull bool) []renderItem2D {
	viewport := renderViewport2DForSpace(c, world, true)
	for entity, access := range world.Query3[T, UIGlobalTransform, Visibility]().All {
		layer := access.A.Ref()
		transform := access.B.Ref()
		visibility := access.C.Ref()

		if layer == nil || transform == nil || visibility == nil || !visibility.Visible {
			continue
		}
		shape := (*layer).uiShape()
		alpha := (*layer).uiAlpha()
		width, height := uiVisualSize2D(world, entity, 0, 0)
		if alpha <= 0 || shape.Kind == 0 || (shape.Fill.A <= 0 && shape.Border.A <= 0) || width <= 0 || height <= 0 {
			continue
		}
		matrix, clip := transform.renderMatrix2D(), transform.renderClip2D()
		anchor := anchorForEntity2D(world, entity)
		if !renderItemVisible2D(viewport, matrix, anchor, width, height, true, clip, cull) {
			continue
		}
		sprite := Sprite2D{Shape: shape, Width: width, Height: height, Alpha: alpha}
		info := renderEntityInfo2D(world, entity, true, 0, hierarchy)
		item := newSpriteRenderItem2D(uint32(len(items)), entity, snapshots.addSprite(sprite), nil, anchor, matrix, clip, width, height, true, info.sortIndex)
		item.entity, item.layers, item.target, item.scene, item.group = entity, info.layers, info.target, info.scene, info.group
		items = append(items, item)
	}
	return items
}

func appendUIImageLayer2D(c *graphics.Canvas2D, world *ecs.World, snapshots *renderSnapshots2D, hierarchy *renderHierarchy2DCache, items []renderItem2D, cull bool) []renderItem2D {
	viewport := renderViewport2DForSpace(c, world, true)
	for entity, access := range world.Query3[UIImage2D, UIGlobalTransform, Visibility]().All {
		image := access.A.Ref()
		transform := access.B.Ref()
		visibility := access.C.Ref()

		if image == nil || image.Alpha <= 0 || image.Tint.A <= 0 || image.Texture == nil || !image.Texture.ValidFor(c.Device()) || transform == nil || visibility == nil || !visibility.Visible {
			continue
		}
		naturalWidth := float32(image.Texture.LogicalWidth())
		naturalHeight := float32(image.Texture.LogicalHeight())
		width, height := uiVisualSize2D(world, entity, naturalWidth, naturalHeight)
		if width <= 0 || height <= 0 {
			continue
		}
		matrix := transform.renderMatrix2D()
		if image.Mode == UIImageAuto && naturalWidth > 0 && naturalHeight > 0 {
			scale := min(width/naturalWidth, height/naturalHeight)
			drawWidth, drawHeight := naturalWidth*scale, naturalHeight*scale
			matrix = matrix.Mul4(mgl32.Translate3D((width-drawWidth)*0.5, (height-drawHeight)*0.5, 0))
			width, height = drawWidth, drawHeight
		}
		clip := transform.renderClip2D()
		anchor := anchorForEntity2D(world, entity)
		if !renderItemVisible2D(viewport, matrix, anchor, width, height, true, clip, cull) {
			continue
		}
		sprite := NewSprite2D()
		sprite.Texture = image.Texture
		if image.Mode == UIImageSliced {
			sprite.Slice = image.Slice
		}
		sprite.Width, sprite.Height = width, height
		sprite.Tint = image.Tint
		sprite.Alpha = image.Alpha
		info := renderEntityInfo2D(world, entity, true, 0, hierarchy)
		item := newSpriteRenderItem2D(uint32(len(items)), entity, snapshots.addSprite(sprite), snapshots.addMaterial(material2DForEntity(world, entity)), anchor, matrix, clip, width, height, true, info.sortIndex)
		item.entity, item.layers, item.target, item.scene, item.group = entity, info.layers, info.target, info.scene, info.group
		items = append(items, item)
	}
	return items
}

func uiVisualSize2D(world *ecs.World, entity ecs.Entity, fallbackWidth, fallbackHeight float32) (float32, float32) {
	if size, ok := world.Get[Size2D](entity); ok {
		return size.Width, size.Height
	}
	return fallbackWidth, fallbackHeight
}
