package graphics

import (
	"errors"
	"fmt"
	"math"

	"github.com/onix-engine/onix/internal/reuse"
	"github.com/onix-engine/onix/render"
	"github.com/go-gl/mathgl/mgl32"
)

const initialBatchInstances = 256

type Canvas2D struct {
	err              error
	material2DShader *Material2DShader
	device           *render.Device
	scratch          scratchValues
	texture          *Texture
	retained         []any

	instances         []render.Instance2D
	shapes            []render.ShapeInstance2D
	draws             []render.SpriteDraw
	clipStack         []clipRect
	targetClips       []clipRect
	shapeRetention    reuse.Retention
	drawRetention     reuse.Retention
	instanceRetention reuse.Retention
	retainedRetention reuse.Retention
	rasterScale       float64
	drawStart         int
	projection        mgl32.Mat4
	material2DCustom  [4][4]float32
	viewMatrix        mgl32.Mat4
	blend             render.BlendState
	scissorRect       render.Rect

	clearColor  ColorRGBA
	height      float32
	width       float32
	projected   bool
	frameActive bool
}

// RetainUntilFlush keeps a resource owner alive while its draw commands are
// queued on the CPU. Drivers retire submitted resources after GPU completion.
func (canvas *Canvas2D) RetainUntilFlush[T comparable](value T) {
	if canvas == nil {
		return
	}
	if n := len(canvas.retained); n > 0 {
		if last, ok := canvas.retained[n-1].(T); ok && last == value {
			return
		}
	}
	canvas.retained = append(canvas.retained, value)
}

var canvas2DContext *Canvas2D

func Canvas2DContext(width, height int) *Canvas2D {
	if canvas2DContext != nil {
		if canvas2DContext.device == render.ActiveDevice() {
			canvas2DContext.Resize(width, height)
			return canvas2DContext
		}
		canvas2DContext.FinishFrame()
	}
	canvas2DContext = NewCanvas2D(render.ActiveDevice(), width, height)
	return canvas2DContext
}

func NewCanvas2D(device *render.Device, width, height int) *Canvas2D {
	return &Canvas2D{
		device:      device,
		width:       float32(width),
		height:      float32(height),
		rasterScale: 1,
		viewMatrix:  mgl32.Ident4(),
		instances:   make([]render.Instance2D, 0, initialBatchInstances),
		draws:       make([]render.SpriteDraw, 0, 64),
		shapes:      make([]render.ShapeInstance2D, 0, 256),
		blend:       render.PremultipliedAlphaBlend,
	}
}

func (c *Canvas2D) Resize(width, height int) {
	if c.width == float32(width) && c.height == float32(height) {
		return
	}
	c.FinishFrame()
	c.width, c.height = float32(width), float32(height)
}

func (c *Canvas2D) Device() *render.Device { return c.device }

// SetRasterScale sets the physical-pixel density used for rasterized 2D
// content while preserving the canvas' logical coordinate system.
func (c *Canvas2D) SetRasterScale(scale float64) {
	if c == nil || math.IsNaN(scale) || math.IsInf(scale, 0) || scale <= 0 {
		scale = 1
	}
	if c.rasterScale == scale {
		return
	}
	c.DrawFrame()
	c.rasterScale = scale
}

// RasterScale returns the number of physical pixels represented by one
// logical canvas unit.
func (c *Canvas2D) RasterScale() float64 {
	if c == nil || c.rasterScale <= 0 {
		return 1
	}
	return c.rasterScale
}

func (c *Canvas2D) SetViewMatrix(view mgl32.Mat4) {
	if c.viewMatrix == view {
		return
	}
	c.DrawFrame()
	c.viewMatrix = view
}

func (c *Canvas2D) SetProjectionMatrix(projection mgl32.Mat4) {
	if c == nil || c.projected && c.projection == projection {
		return
	}
	c.DrawFrame()
	c.projection = projection
	c.projected = true
}

func (c *Canvas2D) ResetProjectionMatrix() {
	if c == nil || !c.projected {
		return
	}
	c.DrawFrame()
	c.projected = false
}

func (c *Canvas2D) ResetViewMatrix() {
	c.SetViewMatrix(mgl32.Ident4())
}

func (c *Canvas2D) ViewMatrix() mgl32.Mat4 {
	return c.viewMatrix
}

func (c *Canvas2D) ProjectionMatrix() mgl32.Mat4 {
	if c == nil {
		return mgl32.Ident4()
	}
	return c.projectionMatrix()
}

func (c *Canvas2D) Size() (float32, float32) {
	if c == nil {
		return 0, 0
	}
	return c.width, c.height
}

func (c *Canvas2D) ClearColor(clr ColorRGBA) {
	if c == nil {
		return
	}
	c.DrawFrame()
	c.clearColor = clr
	if c.frameActive {
		return
	}
	c.beginFrame()
}

// BlendState returns the active component blending factors.
func (c *Canvas2D) BlendState() render.BlendState { return c.blend }

func (c *Canvas2D) BlendSeparate(rgb, alpha render.Blend) {
	c.SetBlendState(render.BlendState{RGB: rgb, Alpha: alpha})
}

// SetBlendState flushes pending geometry before changing factors or constants.
func (c *Canvas2D) SetBlendState(next render.BlendState) {
	if c == nil {
		return
	}
	if c.blend == next {
		return
	}
	c.DrawFrame()
	c.blend = next
	if driver := c.device.Driver(); driver != nil {
		c.setError(driver.SetBlend(next))
	}
}

func (c *Canvas2D) BindTexture(texture *Texture) {
	if c == nil || texture == nil || !texture.ValidFor(c.device) {
		return
	}
	if len(c.shapes) > 0 {
		c.DrawFrame()
	}
	if c.texture == nil || c.texture.BatchKey() != texture.BatchKey() {
		c.finishSpriteDraw()
	}
	c.texture = texture
}

func (c *Canvas2D) BindMaterial2D(shader *Material2DShader) {
	if c == nil {
		return
	}
	if c.material2DShader == shader {
		if shader != nil {
			shader.RenderHandleFor(c.device)
			c.setError(shader.UploadError())
		}
		return
	}
	c.finishSpriteDraw()
	c.material2DShader = shader
	if shader == nil {
		return
	}
	shader.RenderHandleFor(c.device)
	c.setError(shader.UploadError())
}

func (c *Canvas2D) SetMaterial2DParameters(custom [4][4]float32) {
	if c == nil {
		return
	}
	c.material2DCustom = custom
}

func (c *Canvas2D) DrawMatrices(model mgl32.Mat4, texture mgl32.Mat3, alpha float32) {
	c.DrawTintedMatrices(model, texture, White, alpha)
}

// Keep the matrix entry direct so callers do not pay an adapter call per sprite.
func (c *Canvas2D) DrawTintedMatrices(model mgl32.Mat4, texture mgl32.Mat3, tint ColorRGBA, alpha float32) {
	c.DrawTintedMatricesCustom(model, texture, tint, alpha, c.material2DCustom)
}

// DrawTintedMatricesCustom submits a sprite with per-instance material data.
// The built-in HTML rounded-clip path stores its mask in custom2/custom3 while
// custom0/custom1 remain available to user Material2D shaders.
func (c *Canvas2D) DrawTintedMatricesCustom(model mgl32.Mat4, texture mgl32.Mat3, tint ColorRGBA, alpha float32, custom [4][4]float32) {
	if c == nil || c.texture == nil || !c.texture.ValidFor(c.device) || alpha <= 0 || tint.A <= 0 {
		return
	}
	if len(c.shapes) > 0 {
		c.DrawFrame()
	}
	c.instances = append(c.instances, render.Instance2D{
		Model:        affineFromMat4(model),
		TextureModel: [6]float32{texture[0], texture[3], texture[6], texture[1], texture[4], texture[7]},
		Color:        [4]float32{tint.R, tint.G, tint.B, tint.A * alpha},
		Custom:       custom,
	})
}

// DrawAffine submits a sprite using [a, b, tx, c, d, ty] 2D affine matrices.
func (c *Canvas2D) DrawAffine(model, texture [6]float32, alpha float32) {
	c.DrawTintedAffine(model, texture, White, alpha)
}

// DrawTintedAffine submits a tinted sprite using [a, b, tx, c, d, ty] matrices.
func (c *Canvas2D) DrawTintedAffine(model, texture [6]float32, tint ColorRGBA, alpha float32) {
	c.drawTintedAffineValues(
		model[0], model[1], model[2], model[3], model[4], model[5],
		texture[0], texture[1], texture[2], texture[3], texture[4], texture[5],
		tint, alpha,
	)
}

// drawTintedAffineValues keeps the per-sprite path in the 2D affine domain;
// depth is carried by ordering, not by the sprite transform.
func (c *Canvas2D) drawTintedAffineValues(
	model0, model1, model2, model3, model4, model5 float32,
	texture0, texture1, texture2, texture3, texture4, texture5 float32,
	tint ColorRGBA,
	alpha float32,
) {
	if c == nil || c.texture == nil || !c.texture.ValidFor(c.device) || alpha <= 0 || tint.A <= 0 {
		return
	}
	if len(c.shapes) > 0 {
		c.DrawFrame()
	}
	c.instances = append(c.instances, render.Instance2D{
		Model:        [6]float32{model0, model1, model2, model3, model4, model5},
		TextureModel: [6]float32{texture0, texture1, texture2, texture3, texture4, texture5},
		Color:        [4]float32{tint.R, tint.G, tint.B, tint.A * alpha},
		Custom:       c.material2DCustom,
	})
}

func (c *Canvas2D) DrawShapeMatrices(
	model mgl32.Mat4,
	local mgl32.Mat3,
	width, height float32,
	kind render.ShapeKind,
	fill, border ColorRGBA,
	radius, borderWidth, alpha float32,
) {
	c.DrawShapeAffine(
		affineFromMat4(model),
		affineFromMat3(local),
		width, height, kind, fill, border, radius, borderWidth, alpha,
	)
}

func (c *Canvas2D) DrawShapeAffine(
	model, local [6]float32,
	width, height float32,
	kind render.ShapeKind,
	fill, border ColorRGBA,
	radius, borderWidth, alpha float32,
) {
	if c == nil || width <= 0 || height <= 0 || kind == 0 || alpha <= 0 || (fill.A <= 0 && border.A <= 0) {
		return
	}
	if len(c.instances) > 0 {
		c.DrawFrame()
	}
	model = multiplyAffine(affineFromMat4(c.viewMatrix), model)
	if kind == render.ShapeRoundedRect {
		model, local = c.shapeCoverageQuad(model, local)
	}
	c.shapes = append(c.shapes, render.ShapeInstance2D{
		Model:      model,
		LocalModel: local,
		Fill:       [4]float32{fill.R, fill.G, fill.B, fill.A * alpha},
		Border:     [4]float32{border.R, border.G, border.B, border.A * alpha},
		Params:     [4]float32{width, height, maxFloat32(radius, 0), maxFloat32(borderWidth, 0)},
		Kind:       kind,
	})
}

func affineFromMat4(matrix mgl32.Mat4) [6]float32 {
	return [6]float32{
		matrix[0], matrix[4], matrix[12],
		matrix[1], matrix[5], matrix[13],
	}
}

func affineFromMat3(matrix mgl32.Mat3) [6]float32 {
	return [6]float32{
		matrix[0], matrix[3], matrix[6],
		matrix[1], matrix[4], matrix[7],
	}
}

func multiplyAffine(left, right [6]float32) [6]float32 {
	return [6]float32{
		left[0]*right[0] + left[1]*right[3],
		left[0]*right[1] + left[1]*right[4],
		left[0]*right[2] + left[1]*right[5] + left[2],
		left[3]*right[0] + left[4]*right[3],
		left[3]*right[1] + left[4]*right[4],
		left[3]*right[2] + left[4]*right[5] + left[5],
	}
}

func (c *Canvas2D) DrawFrame() {
	if c == nil {
		return
	}
	c.finishSpriteDraw()
	if len(c.draws) == 0 && len(c.shapes) == 0 {
		c.retainedRetention.Record(len(c.retained))
		clear(c.retained)
		c.retained = c.retained[:0]
		return
	}
	driver := c.device.Driver()
	if driver == nil {
		c.resetDrawBatch()
		return
	}
	c.ensureFrame()
	if c.err != nil || !c.frameActive {
		c.resetDrawBatch()
		return
	}
	projection := c.projectionMatrix()
	if len(c.draws) > 0 {
		applyViewAffineBatch(c.instances, affineFromMat4(c.viewMatrix))
		c.setError(driver.DrawSprites(render.SpriteBatch{
			Projection: [16]float32(projection),
			Instances:  c.instances,
			Draws:      c.draws,
		}))
	}
	if len(c.shapes) > 0 {
		c.setError(driver.DrawShapes(render.ShapeBatch{
			Projection: [16]float32(projection),
			Instances:  c.shapes,
		}))
	}
	c.resetDrawBatch()
}

func (c *Canvas2D) resetDrawBatch() {
	c.instanceRetention.Record(len(c.instances))
	c.drawRetention.Record(len(c.draws))
	c.shapeRetention.Record(len(c.shapes))
	c.retainedRetention.Record(len(c.retained))
	c.instances = c.instances[:0]
	c.draws = c.draws[:0]
	c.drawStart = 0
	c.shapes = c.shapes[:0]
	clear(c.retained)
	c.retained = c.retained[:0]
}

// DrawMeshes flushes pending 2D work and submits a 3D render phase into the
// same frame. Callers control phase ordering by the order of submissions.
func (c *Canvas2D) DrawMeshes(batch render.MeshBatch3D) error {
	if c == nil || len(batch.Draws) == 0 {
		return nil
	}
	c.DrawFrame()
	c.ensureFrame()
	if c.err != nil || !c.frameActive {
		return c.err
	}
	c.submitMeshes(batch)
	return c.err
}

// DrawTo renders a logical 2D canvas into a target without starting another
// frame. Pending batches are flushed on both sides of the target boundary.
func (c *Canvas2D) DrawTo(target *render.RenderTarget, clear render.RenderTargetClear, draw func() error) (resultErr error) {
	if c == nil || target == nil {
		return nil
	}
	c.DrawFrame()
	c.ensureFrame()
	if c.err != nil || !c.frameActive {
		return c.err
	}
	driver := c.device.Driver()
	viewport := driver.Viewport()
	width, height := c.width, c.height
	view, projection, projected := c.viewMatrix, c.projection, c.projected
	clips, blend, clearColor := c.clipStack, c.blend, c.clearColor
	if err := target.BeginFor(c.device, clear); err != nil {
		return err
	}
	w, h := target.Size()
	c.width, c.height = float32(w), float32(h)
	c.viewMatrix, c.projected, c.clipStack = mgl32.Ident4(), false, c.targetClips[:0]
	driver.SetViewport(render.Rect{Width: int32(w), Height: int32(h)})
	driver.SetScissor(nil)
	defer func() {
		c.DrawFrame()
		resultErr = errors.Join(resultErr, c.err, target.End())
		c.width, c.height = width, height
		c.viewMatrix, c.projection, c.projected = view, projection, projected
		c.targetClips = c.clipStack[:0]
		c.clipStack, c.blend = clips, blend
		c.clearColor = clearColor
		driver.SetViewport(viewport)
		c.setError(driver.SetBlend(blend))
		if len(clips) == 0 {
			driver.SetScissor(nil)
		} else {
			c.applyClipRect(clips[len(clips)-1])
		}
	}()
	return draw()
}

// DrawMeshesTo submits a 3D phase to an offscreen target, then restores the
// swapchain viewport and scissor used by subsequent canvas work.
func (c *Canvas2D) DrawMeshesTo(target *render.RenderTarget, viewport render.Rect, clear render.RenderTargetClear, batch render.MeshBatch3D) error {
	return c.drawMeshesTo(target, viewport, clear, batch, false)
}

// DrawMeshPickingTo submits a uint32 ID phase to an offscreen picking target.
func (c *Canvas2D) DrawMeshPickingTo(target *render.RenderTarget, viewport render.Rect, clear render.RenderTargetClear, batch render.MeshBatch3D) error {
	return c.drawMeshesTo(target, viewport, clear, batch, true)
}

func (c *Canvas2D) drawMeshesTo(target *render.RenderTarget, viewport render.Rect, clear render.RenderTargetClear, batch render.MeshBatch3D, picking bool) error {
	if c == nil || target == nil {
		return nil
	}
	if picking {
		clear.Color = render.Color{}
		clear.ClearColor = true
	}
	c.DrawFrame()
	c.ensureFrame()
	if c.err != nil || !c.frameActive {
		return c.err
	}
	driver := c.device.Driver()
	previousViewport := driver.Viewport()
	if viewport.Empty() {
		width, height := target.Size()
		viewport = render.Rect{Width: int32(width), Height: int32(height)}
	}
	if err := target.BeginFor(c.device, clear); err != nil {
		if picking {
			return err
		}
		c.setError(err)
		return c.err
	}
	driver.SetViewport(viewport)
	c.scissorRect = viewport
	driver.SetScissor(&c.scissorRect)
	var pickingErr error
	if len(batch.Draws) != 0 {
		if picking {
			pickingDriver, ok := driver.(render.MeshPickingDriver)
			if !ok {
				pickingErr = fmt.Errorf("graphics: render driver does not support mesh picking")
			} else {
				pickingErr = pickingDriver.DrawMeshPicking(batch)
			}
		} else {
			c.submitMeshes(batch)
		}
	}
	endErr := target.End()
	if !picking {
		c.setError(endErr)
	}
	driver.SetViewport(previousViewport)
	if len(c.clipStack) == 0 {
		driver.SetScissor(nil)
	} else {
		c.applyClipRect(c.clipStack[len(c.clipStack)-1])
	}
	if picking {
		return errors.Join(pickingErr, endErr)
	}
	return c.err
}

func (c *Canvas2D) submitMeshes(batch render.MeshBatch3D) {
	driver, ok := c.device.Driver().(render.MeshDriver)
	if !ok {
		c.setError(fmt.Errorf("graphics: render driver does not support 3D meshes"))
		return
	}
	if len(batch.Deformations) != 0 {
		if gpuDriver, supported := driver.(render.MeshGPUDeformationDriver); supported {
			c.setError(gpuDriver.DrawMeshesDeformed(batch))
			return
		}
	}
	c.setError(driver.DrawMeshes(batch))
}

func (c *Canvas2D) finishSpriteDraw() {
	if c == nil || c.texture == nil || c.drawStart == len(c.instances) {
		return
	}
	count := len(c.instances) - c.drawStart
	shader := render.Material2DShaderHandle{}
	if c.material2DShader != nil {
		shader = c.material2DShader.RenderHandleFor(c.device)
	}
	texture := c.texture.RenderHandleFor(c.device)
	if owner := c.texture.baseTexture(); owner.colorOwner != nil {
		c.RetainUntilFlush(owner)
	}
	if len(c.draws) > 0 {
		previous := &c.draws[len(c.draws)-1]
		if previous.Shader == shader && previous.FirstInstance+previous.InstanceCount == uint32(c.drawStart) {
			for slot := range render.SpriteTextureSlots {
				if shader.Valid() && slot > 0 {
					break
				}
				if previous.TextureAt(slot) != texture {
					if slot == 0 || previous.AdditionalTextures[slot-1].Valid() {
						continue
					}
					previous.AdditionalTextures[slot-1] = texture
				}
				for index := c.drawStart; index < len(c.instances); index++ {
					c.instances[index].TextureSlot = float32(slot)
				}
				previous.InstanceCount += uint32(count)
				c.drawStart = len(c.instances)
				return
			}
		}
	}
	c.draws = append(c.draws, render.SpriteDraw{
		Shader:        shader,
		Texture:       texture,
		FirstInstance: uint32(c.drawStart),
		InstanceCount: uint32(count),
	})
	c.drawStart = len(c.instances)
}

func (c *Canvas2D) projectionMatrix() mgl32.Mat4 {
	if c.projected {
		return c.projection
	}
	return mgl32.Ortho2D(0, c.width, c.height, 0)
}

func (c *Canvas2D) FinishFrame() error {
	if c == nil {
		return nil
	}
	c.DrawFrame()
	c.frameActive = false
	c.instances = c.instanceRetention.Trim(c.instances, initialBatchInstances)
	c.draws = c.drawRetention.Trim(c.draws, 64)
	c.shapes = c.shapeRetention.Trim(c.shapes, 256)
	c.retained = c.retainedRetention.Trim(c.retained, 32)
	c.setError(maintainColorTextures(c.device))
	err := c.err
	c.err = nil
	return err
}

func (c *Canvas2D) ensureFrame() {
	if !c.frameActive {
		c.beginFrame()
	}
}

func (c *Canvas2D) beginFrame() {
	driver := c.device.Driver()
	if driver == nil {
		return
	}
	c.setError(driver.BeginFrame(render.Color(c.clearColor)))
	if c.err != nil {
		return
	}
	c.frameActive = true
	c.setError(driver.SetBlend(c.blend))
}

func (c *Canvas2D) PushClipRect(x, y, width, height float32) {
	if c == nil {
		return
	}
	c.DrawFrame()
	next := clipRect{x: x, y: y, w: width, h: height}
	if len(c.clipStack) > 0 {
		next = intersectClipRect(c.clipStack[len(c.clipStack)-1], next)
	}
	c.clipStack = append(c.clipStack, next)
	c.applyClipRect(next)
}

func (c *Canvas2D) PopClipRect() {
	if c == nil || len(c.clipStack) == 0 {
		return
	}
	c.DrawFrame()
	c.clipStack = c.clipStack[:len(c.clipStack)-1]
	if driver := c.device.Driver(); driver != nil {
		if len(c.clipStack) == 0 {
			driver.SetScissor(nil)
			return
		}
		c.applyClipRect(c.clipStack[len(c.clipStack)-1])
	}
}

func (c *Canvas2D) applyClipRect(clip clipRect) {
	driver := c.device.Driver()
	if driver == nil {
		return
	}
	viewport := driver.Viewport()
	scaleX := float32(viewport.Width) / c.width
	scaleY := float32(viewport.Height) / c.height
	x0 := floorToInt32(float32(viewport.X) + maxFloat32(clip.x, 0)*scaleX)
	y0 := floorToInt32(float32(viewport.Y) + maxFloat32(clip.y, 0)*scaleY)
	x1 := ceilToInt32(float32(viewport.X) + minFloat32(clip.x+clip.w, c.width)*scaleX)
	y1 := ceilToInt32(float32(viewport.Y) + minFloat32(clip.y+clip.h, c.height)*scaleY)
	c.scissorRect = render.Rect{X: x0, Y: y0, Width: max(x1-x0, 0), Height: max(y1-y0, 0)}
	driver.SetScissor(&c.scissorRect)
}

func (c *Canvas2D) setError(err error) {
	if c.err == nil {
		c.err = err
	}
}

func floorToInt32(value float32) int32 {
	result := int32(value)
	if float32(result) > value {
		result--
	}
	return result
}

func ceilToInt32(value float32) int32 {
	result := int32(value)
	if float32(result) < value {
		result++
	}
	return result
}
