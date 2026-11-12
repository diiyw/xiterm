package scene

import (
	"math"
	"math/rand/v2"

	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/graphics"
	"github.com/go-gl/mathgl/mgl32"
)

// ParticleAppearance2D pairs a particle element's texture with its colour.
type ParticleAppearance2D struct {
	Texture *graphics.Texture
	Color   graphics.ColorRGBA
}

type ParticleEmitter2D struct {
	Texture     *graphics.Texture
	particles   []particleState2D
	Colors      []graphics.ColorRGBA
	Appearances []ParticleAppearance2D
	Count       int
	SizeMin     float32
	GravityX    float32
	Lifetime    float32
	SpeedMin    float32
	SpeedMax    float32
	Width       float32
	SizeMax     float32
	Direction   float32
	Spread      float32
	Rate        float32
	GravityY    float32
	AlphaStart  float32
	AlphaEnd    float32
	accumulator float32
	Height      float32
	Loop        bool
	burstDone   bool
}

type particleState2D struct {
	texture  *graphics.Texture
	clr      graphics.ColorRGBA
	x, y     float32
	vx, vy   float32
	size     float32
	life     float32
	lifetime float32
	rotation float32
	spin     float32
}

func NewParticleEmitter2D() ParticleEmitter2D {
	return ParticleEmitter2D{
		Count:      12,
		Rate:       12,
		Lifetime:   1.2,
		SpeedMin:   27,
		SpeedMax:   60,
		SizeMin:    3,
		SizeMax:    7,
		AlphaStart: 0.9,
		AlphaEnd:   0,
		Colors: []graphics.ColorRGBA{
			graphics.White,
		},
	}
}

func UpdateParticleEmitter2DSystem(name string) ecs.SystemConfig {
	if name == "" {
		name = "update_particle_emitter_2d"
	}
	return ecs.SystemConfig{
		Name:   name,
		System: updateParticleEmitters2D,
	}
}

func updateParticleEmitters2D(context *ecs.SystemContext) error {
	world := context.World
	if world == nil {
		return nil
	}
	dt := float32(1.0 / 60.0)
	if clock, ok := world.GetResource[*Time](); ok && clock != nil && clock.Delta > 0 {
		dt = clock.DeltaSeconds()
	}
	for entity, access := range world.Query[ParticleEmitter2D]().All {
		if !IsActive(world, entity) {
			continue
		}
		emitter := access.Mut()

		if emitter == nil {
			continue
		}
		emitter.Normalize()
		emitter.update(dt)
	}
	return nil
}

func (e *ParticleEmitter2D) update(dt float32) {
	if e == nil {
		return
	}
	e.spawn(dt)
	particles := e.particles
	write := 0
	for read := range particles {
		item := &particles[read]
		item.life += dt
		if item.life >= item.lifetime {
			*item = particleState2D{}
			continue
		}
		item.vx += e.GravityX * dt
		item.vy += e.GravityY * dt
		item.x += item.vx * dt
		item.y += item.vy * dt
		item.rotation += item.spin * dt
		if write != read {
			particles[write] = *item
			*item = particleState2D{}
		}
		write++
	}
	e.particles = particles[:write]
}

func (e *ParticleEmitter2D) Active() bool {
	if e == nil {
		return false
	}
	return e.Loop || !e.burstDone || len(e.particles) > 0
}

// Restart clears live particles and arms a non-looping emitter for another burst.
func (e *ParticleEmitter2D) Restart() {
	if e == nil {
		return
	}
	clear(e.particles)
	e.particles = e.particles[:0]
	e.accumulator = 0
	e.burstDone = false
}

// Stop clears live particles and disarms a burst emitter until it is restarted.
func (e *ParticleEmitter2D) Stop() {
	if e == nil {
		return
	}
	clear(e.particles)
	e.particles = e.particles[:0]
	e.accumulator = 0
	e.burstDone = true
}

// Reserve preallocates particle storage for pooled effects.
func (e *ParticleEmitter2D) Reserve(maxParticles int) {
	if e == nil || maxParticles <= cap(e.particles) {
		return
	}
	e.particles = make([]particleState2D, 0, maxParticles)
}

func (e *ParticleEmitter2D) spawn(dt float32) {
	if e == nil {
		return
	}
	if e.Loop {
		rate := e.Rate
		if rate <= 0 {
			rate = float32(e.Count)
		}
		e.accumulator += dt * rate
		for e.accumulator >= 1 {
			e.accumulator--
			if len(e.particles) >= e.Count {
				break
			}
			e.particles = append(e.particles, e.makeParticle())
		}
		return
	}

	if e.burstDone {
		return
	}
	for i := 0; i < e.Count; i++ {
		e.particles = append(e.particles, e.makeParticle())
	}
	e.burstDone = true
}

func (e *ParticleEmitter2D) makeParticle() particleState2D {
	speed := e.SpeedMin + rand.Float32()*(e.SpeedMax-e.SpeedMin)
	angle := mgl32.DegToRad(e.Direction + (rand.Float32()-0.5)*e.Spread)
	size := e.SizeMin + rand.Float32()*(e.SizeMax-e.SizeMin)
	var clr graphics.ColorRGBA
	texture := e.Texture
	if len(e.Appearances) != 0 {
		appearance := e.Appearances[rand.IntN(len(e.Appearances))]
		clr, texture = appearance.Color, appearance.Texture
	} else {
		clr = e.Colors[rand.IntN(len(e.Colors))]
	}
	return particleState2D{
		x:        (rand.Float32() - 0.5) * e.Width,
		y:        (rand.Float32() - 0.5) * e.Height,
		vx:       float32(math.Cos(float64(angle))) * speed,
		vy:       float32(math.Sin(float64(angle))) * speed,
		size:     size,
		lifetime: e.Lifetime,
		spin:     (rand.Float32() - 0.5) * 180,
		clr:      clr,
		texture:  texture,
	}
}

func (e *ParticleEmitter2D) alphaFor(item particleState2D) float32 {
	if e == nil || item.lifetime <= 0 {
		return 0
	}
	progress := item.life / item.lifetime
	return e.AlphaStart + (e.AlphaEnd-e.AlphaStart)*progress
}

func (e *ParticleEmitter2D) texture() *graphics.Texture {
	if e == nil {
		return nil
	}
	if e.Texture != nil {
		return e.Texture
	}
	return graphics.CreateTextureByColor(graphics.White)
}

func ParticleEmitterBounds2D(emitter *ParticleEmitter2D, transform mgl32.Mat4, ui bool) (float32, float32, float32, float32, bool) {
	if emitter == nil {
		return 0, 0, 0, 0, false
	}
	if len(emitter.particles) == 0 {
		width := emitter.Width
		height := emitter.Height
		if width <= 0 {
			width = max(emitter.SizeMax, emitter.SizeMin)
		}
		if height <= 0 {
			height = max(emitter.SizeMax, emitter.SizeMin)
		}
		if width <= 0 {
			width = 1
		}
		if height <= 0 {
			height = 1
		}
		return Bounds2D(transform, AnchorCenter, width, height)
	}

	first := true
	var minX, minY, maxX, maxY float32
	for _, particle := range emitter.particles {
		if particle.size <= 0 || emitter.alphaFor(particle) <= 0 {
			continue
		}
		x0, y0, x1, y1, ok := boundsFromBase2D(particleModelMatrix(transform, particle, ui), 1, 1)
		if !ok {
			continue
		}
		if first {
			minX, minY, maxX, maxY = x0, y0, x1, y1
			first = false
			continue
		}
		if x0 < minX {
			minX = x0
		}
		if y0 < minY {
			minY = y0
		}
		if x1 > maxX {
			maxX = x1
		}
		if y1 > maxY {
			maxY = y1
		}
	}
	if first {
		return 0, 0, 0, 0, false
	}
	return minX, minY, maxX, maxY, true
}

func (e *ParticleEmitter2D) Normalize() {
	if e == nil {
		return
	}
	if e.Count <= 0 {
		e.Count = 12
	}
	if e.Lifetime <= 0 {
		e.Lifetime = 1.2
	}
	if e.SpeedMax <= 0 {
		e.SpeedMax = 60
	}
	if e.SpeedMin <= 0 {
		e.SpeedMin = e.SpeedMax * 0.45
	}
	if e.SizeMin <= 0 {
		e.SizeMin = 3
	}
	if e.SizeMax <= 0 {
		e.SizeMax = e.SizeMin + 4
	}
	if e.AlphaStart <= 0 {
		e.AlphaStart = 0.9
	}
	if e.Width < 0 {
		e.Width = 0
	}
	if e.Height < 0 {
		e.Height = 0
	}
	if len(e.Colors) == 0 {
		e.Colors = []graphics.ColorRGBA{graphics.White}
	}
}

func particleModelMatrix(transform mgl32.Mat4, particle particleState2D, ui bool) mgl32.Mat4 {
	if ui {
		transform = transform.Mul4(mgl32.Scale3D(1, -1, 1))
	}
	return transform.
		Mul4(mgl32.Translate3D(particle.x, particle.y, 0)).
		Mul4(mgl32.HomogRotate3DZ(mgl32.DegToRad(particle.rotation))).
		Mul4(mgl32.Translate3D(-particle.size*0.5, particle.size*0.5, 0)).
		Mul4(mgl32.Scale3D(particle.size, -particle.size, 1))
}
