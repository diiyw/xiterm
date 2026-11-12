package scene

import (
	"github.com/onix-engine/onix/graphics"
)

type renderSnapshots2DScratch struct{}

type renderSnapshots2D struct {
	materials           []graphics.Material2DData
	sprites             []Sprite2D
	emitters            []ParticleEmitter2D
	tilemaps            []tilemapSnapshot2D
	texts               []textPaint2D
	spines              []Spine2D
	nineSlices          []NineSlice2D
	particleColors      []graphics.ColorRGBA
	particles           []particleState2D
	particleAppearances []ParticleAppearance2D
	previousMaterials   int
	previousSprites     int
	previousEmitters    int
	previousTilemaps    int
	previousTexts       int
	previousSpines      int
	previousAppearances int
	previousParticles   int
	borrow              bool
}

func (s *renderSnapshots2D) capacity() int {
	return cap(s.materials) + cap(s.sprites) + cap(s.emitters) + cap(s.tilemaps) + cap(s.texts) + cap(s.spines) +
		cap(s.nineSlices) + cap(s.particleColors) + cap(s.particles) + cap(s.particleAppearances)
}

func (s *renderSnapshots2D) length() int {
	return len(s.materials) + len(s.sprites) + len(s.emitters) + len(s.tilemaps) + len(s.texts) + len(s.spines) +
		len(s.nineSlices) + len(s.particleColors) + len(s.particles) + len(s.particleAppearances)
}

// Borrow components only while a retained collector builds its temporary item
// list. The retained owner copies them before any drawing or structural writes.
func (s *renderSnapshots2D) capture[T any](value *T, snapshot func(T) *T) *T {
	if s.borrow {
		return value
	}
	return snapshot(*value)
}

func (s *renderSnapshots2D) reset() {
	s.previousMaterials = len(s.materials)
	s.materials = s.materials[:0]
	s.previousSprites, s.previousEmitters = len(s.sprites), len(s.emitters)
	s.previousTilemaps, s.previousTexts = len(s.tilemaps), len(s.texts)
	s.previousSpines = len(s.spines)
	s.previousAppearances, s.previousParticles = len(s.particleAppearances), len(s.particles)
	s.sprites = s.sprites[:0]
	s.emitters = s.emitters[:0]
	s.tilemaps = s.tilemaps[:0]
	s.texts = s.texts[:0]
	s.spines = s.spines[:0]
	s.nineSlices = s.nineSlices[:0]
	s.particleColors = resetLayoutBuffer(s.particleColors, len(s.particleColors))
	s.particleAppearances = resetLayoutBuffer(s.particleAppearances, len(s.particleAppearances))
	s.particles = resetLayoutBuffer(s.particles, len(s.particles))
}

func (s *renderSnapshots2D) releaseUnused() {
	clearSnapshotTail(s.materials, s.previousMaterials)
	clearSnapshotTail(s.sprites, s.previousSprites)
	clearSnapshotTail(s.emitters, s.previousEmitters)
	clearSnapshotTail(s.tilemaps, s.previousTilemaps)
	clearSnapshotTail(s.texts, s.previousTexts)
	clearSnapshotTail(s.spines, s.previousSpines)
	clearSnapshotTail(s.particleAppearances, s.previousAppearances)
	clearSnapshotTail(s.particles, s.previousParticles)
}

func clearSnapshotTail[T any](values []T, previousLen int) {
	end := min(previousLen, cap(values))
	if end > len(values) {
		clear(values[:end][len(values):])
	}
}

func (s *renderSnapshots2D) addSprite(value Sprite2D) *Sprite2D {
	if value.Slice != nil {
		s.nineSlices = append(s.nineSlices, *value.Slice)
		value.Slice = &s.nineSlices[len(s.nineSlices)-1]
	}
	s.sprites = append(s.sprites, value)
	return &s.sprites[len(s.sprites)-1]
}

func (s *renderSnapshots2D) addEmitter(value ParticleEmitter2D) *ParticleEmitter2D {
	value.Colors = appendSnapshotValues(&s.particleColors, value.Colors)
	value.Appearances = appendSnapshotValues(&s.particleAppearances, value.Appearances)
	value.particles = appendSnapshotValues(&s.particles, value.particles)
	s.emitters = append(s.emitters, value)
	return &s.emitters[len(s.emitters)-1]
}

func (s *renderSnapshots2D) addTilemap(value Tilemap2D) *Tilemap2D {
	index := len(s.tilemaps)
	var previous tilemapSnapshot2D
	if index < cap(s.tilemaps) {
		previous = s.tilemaps[:index+1][index]
	}
	var source *int
	if len(value.Tiles) != 0 {
		source = &value.Tiles[0]
	}
	if value.Revision != 0 && value.Revision == previous.value.Revision && previous.source == source && len(value.Tiles) == len(previous.value.Tiles) {
		value.Tiles = previous.value.Tiles
	} else {
		value.Tiles = append(previous.value.Tiles[:0], value.Tiles...)
	}
	s.tilemaps = append(s.tilemaps, tilemapSnapshot2D{value: value, source: source})
	return &s.tilemaps[len(s.tilemaps)-1].value
}

type tilemapSnapshot2D struct {
	source *int
	value  Tilemap2D
}

func (s *renderSnapshots2D) addText(value Text2D) *textPaint2D {
	s.texts = append(s.texts, value.paintSnapshot())
	return &s.texts[len(s.texts)-1]
}

func (s *renderSnapshots2D) addSpine(value Spine2D) *Spine2D {
	s.spines = append(s.spines, value)
	return &s.spines[len(s.spines)-1]
}

func appendSnapshotValues[T any](buffer *[]T, values []T) []T {
	start := len(*buffer)
	*buffer = append(*buffer, values...)
	return (*buffer)[start:]
}

func (s *renderSnapshots2D) addMaterial(value graphics.Material2DData) *graphics.Material2DData {
	if value == (graphics.Material2DData{}) {
		return nil
	}
	if n := len(s.materials); n > 0 && s.materials[n-1] == value {
		return &s.materials[n-1]
	}
	s.materials = append(s.materials, value)
	return &s.materials[len(s.materials)-1]
}
