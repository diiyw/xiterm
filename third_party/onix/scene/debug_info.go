//go:build !onix_release

package scene

import (
	"image"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/graphics"
	"github.com/onix-engine/onix/render"
	"github.com/go-gl/mathgl/mgl32"
)

const (
	debugInfoRefreshInterval = 500 * time.Millisecond
	debugGlyphFirst          = byte(32)
	debugGlyphLast           = byte(126)
	debugGlyphColumns        = 16
	debugGlyphWidth          = 5
	debugGlyphHeight         = 7
	debugGlyphCellWidth      = 8
	debugGlyphCellHeight     = 10
	debugInfoLineCount       = 14
)

// DebugInfo enables a compact frame statistics overlay in the top-left corner.
// Zero-valued fields use the defaults returned by NewDebugInfo.
type DebugInfo struct {
	Scale           float32
	Margin          float32
	Padding         float32
	TextColor       graphics.ColorRGBA
	BackgroundColor graphics.ColorRGBA
}

func NewDebugInfo() DebugInfo {
	return DebugInfo{
		Scale:           2,
		Margin:          8,
		Padding:         7,
		TextColor:       graphics.ColorFromRGBA(232, 238, 244, 255),
		BackgroundColor: graphics.ColorFromRGBA(13, 16, 20, 192),
	}
}

func SpawnDebugInfo(world *ecs.World) ecs.Entity {
	spawner := world.NewSpawner1[DebugInfo]()
	return spawner.Spawn(NewDebugInfo())
}

func RenderDebugInfoSystem(name string) ecs.SystemConfig {
	if name == "" {
		name = "render_debug_info"
	}
	return ecs.SystemConfig{Name: name, System: renderDebugInfo}
}

type debugInfoSystemState struct {
	lastFrame     time.Time
	windowStarted time.Time
	atlas         *graphics.Texture
	text          debugInfoText
	bufferMemory  render.BufferMemoryStatistics
	windowFrames  uint64
	fps           float64
	heapBytes     uint64
}

type debugInfoSystemStateTag struct{}

func renderDebugInfo(context *ecs.SystemContext) error {
	world := context.World
	c := graphics.CanvasFor(world)
	if world == nil || c == nil {
		return nil
	}
	var component DebugInfo
	found := false
	for entity, access := range world.Query[DebugInfo]().All {
		value := access.Get()

		if visibility, ok := world.Get[Visibility](entity); ok && !visibility.Visible {
			continue
		}
		component = value
		found = true
		break
	}
	if !found {
		return nil
	}

	// Flush the game first so the snapshot excludes this overlay itself.
	c.DrawFrame()
	statistics := c.Device().Statistics()
	state := context.Local[debugInfoSystemStateTag, debugInfoSystemState]()
	state.sample(time.Now(), c.Device())
	if state.atlas == nil {
		state.atlas = newDebugGlyphAtlas()
	}
	state.text.format(statistics, state.heapBytes, state.bufferMemory, state.fps, c.Device().RendererInfo())
	drawDebugInfo(c, component.resolved(), state.atlas, &state.text)
	return nil
}

func (state *debugInfoSystemState) sample(now time.Time, device *render.Device) {
	if state.lastFrame.IsZero() {
		state.lastFrame = now
		state.windowStarted = now
		state.readMemory(device)
		return
	}
	delta := now.Sub(state.lastFrame)
	state.lastFrame = now
	state.windowFrames++
	if state.fps == 0 && delta > 0 {
		state.fps = 1 / delta.Seconds()
	}
	elapsed := now.Sub(state.windowStarted)
	if elapsed < debugInfoRefreshInterval {
		return
	}
	state.fps = float64(state.windowFrames) / elapsed.Seconds()
	state.windowStarted = now
	state.windowFrames = 0
	state.readMemory(device)
}

func (state *debugInfoSystemState) readMemory(device *render.Device) {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	state.heapBytes = memory.HeapAlloc
	state.bufferMemory = device.BufferMemoryStatistics()
}

func (info DebugInfo) resolved() DebugInfo {
	defaults := NewDebugInfo()
	if info.Scale <= 0 {
		info.Scale = defaults.Scale
	}
	if info.Margin <= 0 {
		info.Margin = defaults.Margin
	}
	if info.Padding <= 0 {
		info.Padding = defaults.Padding
	}
	if info.TextColor == (graphics.ColorRGBA{}) {
		info.TextColor = defaults.TextColor
	}
	if info.BackgroundColor == (graphics.ColorRGBA{}) {
		info.BackgroundColor = defaults.BackgroundColor
	}
	return info
}

type debugInfoLine struct {
	start int
	end   int
}

type debugInfoText struct {
	buffer []byte
	lines  [debugInfoLineCount]debugInfoLine
}

func (text *debugInfoText) format(statistics render.FrameStatistics, heapBytes uint64, buffers render.BufferMemoryStatistics, fps float64, info render.RendererInfo) {
	backend := strings.TrimSpace(info.Backend)
	if backend == "" {
		backend = "UNKNOWN"
	}
	device := strings.TrimSpace(info.Device)
	if device == "" {
		device = "UNKNOWN"
	}

	text.buffer = text.buffer[:0]
	text.appendLabel(0, "SURFACE     ")
	text.buffer = strconv.AppendUint(text.buffer, statistics.SurfaceWidth, 10)
	text.buffer = append(text.buffer, 'X')
	text.buffer = strconv.AppendUint(text.buffer, statistics.SurfaceHeight, 10)
	text.finishLine(0)
	text.appendLabel(1, "DRAW CALLS  ")
	text.buffer = strconv.AppendUint(text.buffer, statistics.DrawCalls, 10)
	text.finishLine(1)
	text.appendLabel(2, "DRAWS SPR/SHP/M ")
	text.buffer = appendDebugTriple(text.buffer, statistics.SpriteDrawCalls, statistics.ShapeDrawCalls, statistics.MeshDrawCalls)
	text.finishLine(2)
	text.appendLabel(3, "INST SPR/SHP/M  ")
	text.buffer = appendDebugTriple(text.buffer, statistics.Sprites, statistics.Shapes, statistics.Meshes)
	text.finishLine(3)
	text.appendLabel(4, "VERT/TRI    ")
	text.buffer = strconv.AppendUint(text.buffer, statistics.Vertices, 10)
	text.buffer = append(text.buffer, '/')
	text.buffer = strconv.AppendUint(text.buffer, statistics.Triangles, 10)
	text.finishLine(4)
	text.appendLabel(5, "BUFFER      ")
	text.buffer = appendDebugBytes(text.buffer, statistics.BufferUploadBytes)
	text.finishLine(5)
	text.appendLabel(6, "DYN MESH    ")
	text.buffer = strconv.AppendUint(text.buffer, statistics.DynamicMeshDrawCalls, 10)
	text.buffer = append(text.buffer, " / "...)
	text.buffer = appendDebugBytes(text.buffer, statistics.DynamicMeshUploadBytes)
	text.finishLine(6)
	text.appendLabel(7, "TEX UPLOAD  ")
	text.buffer = strconv.AppendUint(text.buffer, statistics.TextureUploads, 10)
	text.buffer = append(text.buffer, " / "...)
	text.buffer = appendDebugBytes(text.buffer, statistics.TextureUploadBytes)
	text.finishLine(7)
	text.appendLabel(8, "GO HEAP     ")
	text.buffer = appendDebugBytes(text.buffer, heapBytes)
	text.finishLine(8)
	text.appendLabel(9, "NATIVE BUF  ")
	text.buffer = appendDebugBytes(text.buffer, buffers.NativeBytes())
	text.finishLine(9)
	text.appendLabel(10, "GO SCRATCH  ")
	text.buffer = appendDebugBytes(text.buffer, buffers.ScratchBytes)
	text.finishLine(10)
	text.appendLabel(11, "FPS         ")
	text.buffer = strconv.AppendFloat(text.buffer, fps, 'f', 1, 64)
	text.finishLine(11)
	text.appendLabel(12, "BACKEND     ")
	text.buffer = appendDebugUpper(text.buffer, backend)
	text.finishLine(12)
	text.appendLabel(13, "GPU         ")
	text.buffer = appendDebugUpper(text.buffer, device)
	text.finishLine(13)
}

func appendDebugTriple(dst []byte, first, second, third uint64) []byte {
	dst = strconv.AppendUint(dst, first, 10)
	dst = append(dst, '/')
	dst = strconv.AppendUint(dst, second, 10)
	dst = append(dst, '/')
	return strconv.AppendUint(dst, third, 10)
}

func (text *debugInfoText) appendLabel(index int, label string) {
	text.lines[index].start = len(text.buffer)
	text.buffer = append(text.buffer, label...)
}

func (text *debugInfoText) finishLine(index int) {
	text.lines[index].end = len(text.buffer)
}

func (text *debugInfoText) line(index int) []byte {
	line := text.lines[index]
	return text.buffer[line.start:line.end]
}

func appendDebugBytes(dst []byte, bytes uint64) []byte {
	if bytes < 1024 {
		dst = strconv.AppendUint(dst, bytes, 10)
		return append(dst, " B"...)
	}
	units := [...]string{"KIB", "MIB", "GIB", "TIB"}
	value := float64(bytes)
	unit := 0
	for value >= 1024 && unit < len(units) {
		value /= 1024
		unit++
	}
	dst = strconv.AppendFloat(dst, value, 'f', 1, 64)
	dst = append(dst, ' ')
	return append(dst, units[unit-1]...)
}

func appendDebugUpper(dst []byte, value string) []byte {
	for _, current := range value {
		dst = utf8.AppendRune(dst, unicode.ToUpper(current))
	}
	return dst
}

type visibleDebugInfoLine struct {
	end  int
	dots int
}

func drawDebugInfo(c *graphics.Canvas2D, info DebugInfo, atlas *graphics.Texture, text *debugInfoText) {
	if c == nil || atlas == nil || text == nil {
		return
	}
	c.ResetViewMatrix()
	c.ResetProjectionMatrix()
	width, _ := c.Size()
	advance := float32(debugGlyphWidth+1) * info.Scale
	maxCharacters := int((width - 2*info.Margin - 2*info.Padding + info.Scale) / advance)
	if maxCharacters <= 0 {
		return
	}
	var visible [debugInfoLineCount]visibleDebugInfoLine
	maxLineRunes := 0
	for index := range text.lines {
		end, dots, runeCount := debugInfoLineBounds(text.line(index), maxCharacters)
		visible[index] = visibleDebugInfoLine{end: end, dots: dots}
		maxLineRunes = max(maxLineRunes, runeCount)
	}
	if maxLineRunes == 0 {
		return
	}
	textWidth := (float32(maxLineRunes*(debugGlyphWidth+1)) - 1) * info.Scale
	lineHeight := float32(debugGlyphHeight+2) * info.Scale
	textHeight := float32(debugGlyphHeight)*info.Scale + float32(len(text.lines)-1)*lineHeight
	panelWidth := textWidth + info.Padding*2
	panelHeight := textHeight + info.Padding*2
	model := mgl32.Translate3D(info.Margin, info.Margin, 0).Mul4(mgl32.Scale3D(panelWidth, panelHeight, 1))
	c.DrawShapeMatrices(
		model,
		mgl32.Ident3(),
		panelWidth,
		panelHeight,
		render.ShapeRoundedRect,
		info.BackgroundColor,
		graphics.ColorRGBA{},
		4,
		0,
		1,
	)
	c.BindMaterial2D(nil)
	c.BindTexture(atlas)
	x := info.Margin + info.Padding
	y := info.Margin + info.Padding
	for index := range text.lines {
		line := text.line(index)
		display := visible[index]
		drawDebugInfoLine(c, atlas, line[:display.end], display.dots, x, y, info.Scale, info.TextColor)
		y += lineHeight
	}
}

func debugInfoLineBounds(value []byte, limit int) (end, dots, runeCount int) {
	if limit <= 0 {
		return 0, 0, 0
	}
	runeCount = utf8.RuneCount(value)
	if runeCount <= limit {
		return len(value), 0, runeCount
	}
	if limit <= 3 {
		return 0, limit, limit
	}
	runeCount = limit
	for count := 0; count < limit-3; count++ {
		_, size := utf8.DecodeRune(value[end:])
		end += size
	}
	return end, 3, runeCount
}

func drawDebugInfoLine(c *graphics.Canvas2D, atlas *graphics.Texture, value []byte, dots int, x, y, scale float32, color graphics.ColorRGBA) {
	atlasWidth := float32(atlas.Width)
	atlasHeight := float32(atlas.Height)
	for len(value) > 0 || dots > 0 {
		current := rune('.')
		if len(value) > 0 {
			var size int
			current, size = utf8.DecodeRune(value)
			value = value[size:]
		} else {
			dots--
		}
		glyph := byte(current)
		if glyph != ' ' && (current < rune(debugGlyphFirst) || current > rune(debugGlyphLast) || debugGlyphPatterns[glyph] == ([debugGlyphHeight]byte{})) {
			glyph = '?'
		}
		if glyph != ' ' {
			index := int(glyph - debugGlyphFirst)
			column := index % debugGlyphColumns
			row := index / debugGlyphColumns
			texture := mgl32.Translate2D(
				float32(column*debugGlyphCellWidth+1)/atlasWidth,
				float32(row*debugGlyphCellHeight+1)/atlasHeight,
			).Mul3(mgl32.Scale2D(debugGlyphWidth/atlasWidth, debugGlyphHeight/atlasHeight))
			model := mgl32.Translate3D(x, y, 0).Mul4(mgl32.Scale3D(debugGlyphWidth*scale, debugGlyphHeight*scale, 1))
			c.DrawTintedMatrices(model, texture, color, 1)
		}
		x += float32(debugGlyphWidth+1) * scale
	}
}

func newDebugGlyphAtlas() *graphics.Texture {
	glyphCount := int(debugGlyphLast-debugGlyphFirst) + 1
	rows := (glyphCount + debugGlyphColumns - 1) / debugGlyphColumns
	atlas := image.NewRGBA(image.Rect(0, 0, debugGlyphColumns*debugGlyphCellWidth, rows*debugGlyphCellHeight))
	for glyph, pattern := range debugGlyphPatterns {
		index := int(glyph - debugGlyphFirst)
		x := index%debugGlyphColumns*debugGlyphCellWidth + 1
		y := index/debugGlyphColumns*debugGlyphCellHeight + 1
		for row, bits := range pattern {
			for column := range debugGlyphWidth {
				if bits&(1<<uint(debugGlyphWidth-column-1)) == 0 {
					continue
				}
				offset := (y+row)*atlas.Stride + (x+column)*4
				atlas.Pix[offset] = 255
				atlas.Pix[offset+1] = 255
				atlas.Pix[offset+2] = 255
				atlas.Pix[offset+3] = 255
			}
		}
	}
	return graphics.CreateTextureBySharedImage(atlas, false, graphics.ResourceUsageDefault)
}

var debugGlyphPatterns = map[byte][debugGlyphHeight]byte{
	'!':  {0x04, 0x04, 0x04, 0x04, 0x04, 0x00, 0x04},
	'#':  {0x0A, 0x1F, 0x0A, 0x0A, 0x1F, 0x0A, 0x00},
	'%':  {0x19, 0x19, 0x02, 0x04, 0x08, 0x13, 0x13},
	'&':  {0x0C, 0x12, 0x14, 0x08, 0x15, 0x12, 0x0D},
	'(':  {0x02, 0x04, 0x08, 0x08, 0x08, 0x04, 0x02},
	')':  {0x08, 0x04, 0x02, 0x02, 0x02, 0x04, 0x08},
	'*':  {0x00, 0x15, 0x0E, 0x1F, 0x0E, 0x15, 0x00},
	'+':  {0x00, 0x04, 0x04, 0x1F, 0x04, 0x04, 0x00},
	',':  {0x00, 0x00, 0x00, 0x00, 0x04, 0x04, 0x08},
	'-':  {0x00, 0x00, 0x00, 0x1F, 0x00, 0x00, 0x00},
	'.':  {0x00, 0x00, 0x00, 0x00, 0x00, 0x0C, 0x0C},
	'/':  {0x01, 0x02, 0x02, 0x04, 0x08, 0x08, 0x10},
	'0':  {0x0E, 0x11, 0x13, 0x15, 0x19, 0x11, 0x0E},
	'1':  {0x04, 0x0C, 0x14, 0x04, 0x04, 0x04, 0x1F},
	'2':  {0x0E, 0x11, 0x01, 0x02, 0x04, 0x08, 0x1F},
	'3':  {0x1E, 0x01, 0x01, 0x0E, 0x01, 0x01, 0x1E},
	'4':  {0x02, 0x06, 0x0A, 0x12, 0x1F, 0x02, 0x02},
	'5':  {0x1F, 0x10, 0x10, 0x1E, 0x01, 0x01, 0x1E},
	'6':  {0x0E, 0x10, 0x10, 0x1E, 0x11, 0x11, 0x0E},
	'7':  {0x1F, 0x01, 0x02, 0x04, 0x08, 0x08, 0x08},
	'8':  {0x0E, 0x11, 0x11, 0x0E, 0x11, 0x11, 0x0E},
	'9':  {0x0E, 0x11, 0x11, 0x0F, 0x01, 0x01, 0x0E},
	':':  {0x00, 0x0C, 0x0C, 0x00, 0x0C, 0x0C, 0x00},
	';':  {0x00, 0x0C, 0x0C, 0x00, 0x0C, 0x04, 0x08},
	'=':  {0x00, 0x00, 0x1F, 0x00, 0x1F, 0x00, 0x00},
	'?':  {0x0E, 0x11, 0x01, 0x02, 0x04, 0x00, 0x04},
	'@':  {0x0E, 0x11, 0x17, 0x15, 0x17, 0x10, 0x0E},
	'A':  {0x0E, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
	'B':  {0x1E, 0x11, 0x11, 0x1E, 0x11, 0x11, 0x1E},
	'C':  {0x0E, 0x11, 0x10, 0x10, 0x10, 0x11, 0x0E},
	'D':  {0x1E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x1E},
	'E':  {0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x1F},
	'F':  {0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x10},
	'G':  {0x0E, 0x11, 0x10, 0x17, 0x11, 0x11, 0x0F},
	'H':  {0x11, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
	'I':  {0x0E, 0x04, 0x04, 0x04, 0x04, 0x04, 0x0E},
	'J':  {0x07, 0x02, 0x02, 0x02, 0x02, 0x12, 0x0C},
	'K':  {0x11, 0x12, 0x14, 0x18, 0x14, 0x12, 0x11},
	'L':  {0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x1F},
	'M':  {0x11, 0x1B, 0x15, 0x15, 0x11, 0x11, 0x11},
	'N':  {0x11, 0x19, 0x19, 0x15, 0x13, 0x13, 0x11},
	'O':  {0x0E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E},
	'P':  {0x1E, 0x11, 0x11, 0x1E, 0x10, 0x10, 0x10},
	'Q':  {0x0E, 0x11, 0x11, 0x11, 0x15, 0x12, 0x0D},
	'R':  {0x1E, 0x11, 0x11, 0x1E, 0x14, 0x12, 0x11},
	'S':  {0x0F, 0x10, 0x10, 0x0E, 0x01, 0x01, 0x1E},
	'T':  {0x1F, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04},
	'U':  {0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E},
	'V':  {0x11, 0x11, 0x11, 0x11, 0x11, 0x0A, 0x04},
	'W':  {0x11, 0x11, 0x11, 0x15, 0x15, 0x15, 0x0A},
	'X':  {0x11, 0x11, 0x0A, 0x04, 0x0A, 0x11, 0x11},
	'Y':  {0x11, 0x11, 0x0A, 0x04, 0x04, 0x04, 0x04},
	'Z':  {0x1F, 0x01, 0x02, 0x04, 0x08, 0x10, 0x1F},
	'[':  {0x0E, 0x08, 0x08, 0x08, 0x08, 0x08, 0x0E},
	'\\': {0x10, 0x08, 0x08, 0x04, 0x02, 0x02, 0x01},
	']':  {0x0E, 0x02, 0x02, 0x02, 0x02, 0x02, 0x0E},
	'_':  {0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x1F},
	'|':  {0x04, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04},
}
