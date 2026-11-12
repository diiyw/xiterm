package scene

import (
	"cmp"
	"image"
	"iter"
	"slices"

	"github.com/onix-engine/onix/ecs"
)

func (view *compositor2D) tileIndices(bounds image.Rectangle) iter.Seq[int] {
	return func(yield func(int) bool) {
		if bounds.Empty() {
			return
		}
		x0, y0 := bounds.Min.X/compositeTileSize2D, bounds.Min.Y/compositeTileSize2D
		x1, y1 := (bounds.Max.X-1)/compositeTileSize2D, (bounds.Max.Y-1)/compositeTileSize2D
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				if !yield(y*view.columns + x) {
					return
				}
			}
		}
	}
}

func compareCompositeIndex2D(a, b uint32) int { return cmp.Compare(b, a) }

func (view *compositor2D) damageTile(index int) {
	if tile := &view.tiles[index]; !tile.changed {
		// Remove the old visible prefix before any insertion/deletion shifts
		// candidates. Each touched tile is retired once for the whole edit set.
		for _, position := range tile.visibleItems() {
			coverage := &view.coverage[position]
			coverage.visible--
			if coverage.visible == 0 && coverage.present {
				view.occluded++
			}
		}
		tile.visible = 0
		tile.changed = true
		view.damageTiles = append(view.damageTiles, index)
	}
}

// Stable retained positions let a sparse frame edit just the old/new spatial
// bins. Keeping occluded candidates allows a moved cover to reveal its content.
func (view *compositor2D) updateDamage(state *RenderWorld, camera ExtractedCamera, defaultUICamera ecs.Entity, damage []renderDamage2D) bool {
	view.damageTiles = view.damageTiles[:0]
	screen := image.Rect(0, 0, view.width, view.height)
	for _, change := range damage {
		item := &state.Items2D[change.position]
		previous := change.previous.Intersect(screen)
		var next image.Rectangle
		if renderItemForCamera2D(item, camera, defaultUICamera) {
			if item.paint == nil || !item.paint.cacheable {
				return false
			}
			next = item.paint.boundsRect(item).Intersect(screen)
		} else {
			previous = image.Rectangle{}
		}
		coverage := &view.coverage[change.position]
		if coverage.present && coverage.visible == 0 {
			view.occluded--
		}
		coverage.present = !next.Empty()
		if coverage.present && coverage.visible == 0 {
			view.occluded++
		}
		for index := range view.tileIndices(previous) {
			view.damageTile(index)
			if view.tileRect(index).Overlaps(next) {
				continue
			}
			tile := &view.tiles[index]
			if position, found := slices.BinarySearchFunc(tile.candidates, uint32(change.position), compareCompositeIndex2D); found {
				tile.candidates = slices.Delete(tile.candidates, position, position+1)
			}
		}
		for index := range view.tileIndices(next) {
			view.damageTile(index)
			if view.tileRect(index).Overlaps(previous) {
				continue
			}
			tile := &view.tiles[index]
			position, _ := slices.BinarySearchFunc(tile.candidates, uint32(change.position), compareCompositeIndex2D)
			tile.candidates = slices.Insert(tile.candidates, position, uint32(change.position))
		}
	}
	for _, index := range view.damageTiles {
		view.stats.PreparedTiles++
		tile := &view.tiles[index]
		rect := view.tileRect(index)
		tile.hash, tile.covered, tile.changed = paintMix2D(uint64(rect.Dx()), uint64(rect.Dy())), false, false
		for _, position := range tile.candidates {
			view.stats.PreparedItems++
			item := &state.Items2D[position]
			coverage := &view.coverage[position]
			if coverage.visible == 0 {
				view.occluded--
			}
			coverage.visible++
			tile.visible++
			tile.hash = paintMix2D(tile.hash, item.paint.tileHash(item, rect))
			if rect.In(item.paint.opaqueRect(item)) {
				tile.covered = true
				break
			}
		}
	}
	return true
}
