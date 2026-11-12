package scene

import (
	"fmt"
	"math"

	"github.com/onix-engine/onix/ecs"
	"github.com/onix-engine/onix/internal/entitycache"
	"github.com/onix-engine/onix/internal/reuse"
	"github.com/go-gl/mathgl/mgl32"
)

type UITransform struct {
	Translation Val2
	Scale       mgl32.Vec2
	Rotation    float32
	Skew        mgl32.Vec2
	// Pivot is the local point around which rotation, skew, and scale are applied.
	Pivot mgl32.Vec2
	// LinearMatrix overrides rotation, skew, and scale when nonzero. It preserves
	// exact composition, including singular scales; pivot and translation still apply.
	LinearMatrix mgl32.Mat4
}

type UIGlobalTransform struct {
	Matrix mgl32.Mat4
	clip   clipRect2D
}

func (t UIGlobalTransform) renderMatrix2D() mgl32.Mat4 { return t.Matrix }
func (t UIGlobalTransform) renderClip2D() clipRect2D   { return t.clip }

// ClipRect returns the inherited clipping bounds in world UI coordinates.
// The boolean distinguishes an empty clipping rectangle from no clipping.
func (t UIGlobalTransform) ClipRect() (ClipRect2D, bool) {
	if !t.clip.valid {
		return ClipRect2D{}, false
	}
	return ClipRect2D{X: t.clip.minX, Y: t.clip.minY, Width: t.clip.maxX - t.clip.minX, Height: t.clip.maxY - t.clip.minY}, true
}

func NewUITransform() UITransform {
	return UITransform{Translation: Px2(0, 0), Scale: mgl32.Vec2{1, 1}}
}

func UITransformFromXY(x, y float32) UITransform {
	transform := NewUITransform()
	transform.Translation = Px2(x, y)
	return transform
}

func (t UITransform) LocalMatrix(context UIResolveContext) mgl32.Mat4 {
	translation := t.Translation.Resolve(context)
	transformed := t.LinearMatrix
	if transformed == (mgl32.Mat4{}) {
		transformed = mgl32.HomogRotate3DZ(t.Rotation).
			Mul4(mgl32.Mat4{
				1, float32(math.Tan(float64(t.Skew.Y()))), 0, 0,
				float32(math.Tan(float64(t.Skew.X()))), 1, 0, 0,
				0, 0, 1, 0,
				0, 0, 0, 1,
			}).
			Mul4(mgl32.Scale3D(t.Scale.X(), t.Scale.Y(), 1))
	}
	if t.Pivot.X() != 0 || t.Pivot.Y() != 0 {
		transformed = mgl32.Translate3D(t.Pivot.X(), t.Pivot.Y(), 0).
			Mul4(transformed).
			Mul4(mgl32.Translate3D(-t.Pivot.X(), -t.Pivot.Y(), 0))
	}
	return mgl32.Translate3D(translation.X(), translation.Y(), 0).Mul4(transformed)
}

type resolvedUITransform struct {
	inheritedClip clipRect2D
	matrix        mgl32.Mat4
	clip          clipRect2D
	revision      uint64
}

type uiTransformNode struct {
	inheritedClip    clipRect2D
	revision         uint64
	parent           ecs.Entity
	seen             uint64
	parentRevision   uint64
	transform        UITransform
	local            mgl32.Mat4
	localRoundedClip RoundedClip2D
	localClip        ClipRect2D
	baseSize         mgl32.Vec2
	hasClip          bool
	state            uint8
	parentClipExempt bool
	clipExempt       bool
	hasRoundedClip   bool
	dirty            bool
}

type uiTransformBindings struct {
	world     *ecs.World
	transform ecs.ComponentBinding[UITransform]
	parent    ecs.ComponentBinding[Parent]
	clip      ecs.ComponentBinding[ClipRect2D]
	rounded   ecs.ComponentBinding[RoundedClip2D]
	exempt    ecs.ComponentBinding[ClipExempt2D]
	size      ecs.ComponentBinding[Size2D]
	sprite    ecs.ComponentBinding[Sprite2D]
	text      ecs.ComponentBinding[Text2D]
	global    ecs.ComponentBinding[UIGlobalTransform]
}

func (b *uiTransformBindings) bind(world *ecs.World) {
	if b.world == world {
		return
	}
	b.world = world
	b.transform = world.BindComponent[UITransform]()
	b.parent = world.BindComponent[Parent]()
	b.clip = world.BindComponent[ClipRect2D]()
	b.rounded = world.BindComponent[RoundedClip2D]()
	b.exempt = world.BindComponent[ClipExempt2D]()
	b.size = world.BindComponent[Size2D]()
	b.sprite = world.BindComponent[Sprite2D]()
	b.text = world.BindComponent[Text2D]()
	b.global = world.BindComponent[UIGlobalTransform]()
}

type uiTransformWorkspace struct {
	scope        *ecs.QueryScope
	bindings     uiTransformBindings
	changes      ecs.SystemContext
	queue        []ecs.Entity
	nodes        entitycache.Store[uiTransformNode]
	structures   [9]uint64
	versions     [9]uint64
	retention    reuse.Retention
	scopeVersion uint64
	generation   uint64
	recomputed   int
	viewport     mgl32.Vec2
	lastRun      ecs.Tick
	scaleFactor  float32
	initialized  bool
}

func uiTransformRevisions(world *ecs.World) (versions, structures [9]uint64) {
	versions[0], structures[0] = world.ComponentValueRevision[UITransform]()
	versions[1], structures[1] = world.ComponentValueRevision[Parent]()
	versions[2], structures[2] = world.ComponentValueRevision[ClipRect2D]()
	versions[3], structures[3] = world.ComponentValueRevision[RoundedClip2D]()
	versions[4], structures[4] = world.ComponentValueRevision[ClipExempt2D]()
	versions[5], structures[5] = world.ComponentValueRevision[Size2D]()
	versions[6], structures[6] = world.ComponentValueRevision[Sprite2D]()
	versions[7], structures[7] = world.ComponentValueRevision[Text2D]()
	versions[8], structures[8] = world.ComponentValueRevision[UIGlobalTransform]()
	return
}

type uiTransformWorkspaceScratch struct{}

const (
	uiTransformUnvisited uint8 = iota
	uiTransformVisiting
	uiTransformResolved
)

func SyncUIGlobalTransformSystem(name string) ecs.SystemConfig {
	if name == "" {
		name = "sync_ui_global_transform"
	}
	return ecs.SystemConfig{Name: name, System: syncUIGlobalTransforms}
}

func syncUIGlobalTransforms(context *ecs.SystemContext) error {
	world := context.World
	if world == nil {
		return nil
	}

	workspace := world.ScopeValue[uiTransformWorkspaceScratch, uiTransformWorkspace]()
	workspace.bindings.bind(world)
	scope, version := world.QueryScopeKey()
	if workspace.scope != scope || workspace.scopeVersion != version {
		workspace.nodes.Reset()
		workspace.initialized = false
		workspace.scope, workspace.scopeVersion = scope, version
	}
	workspace.nodes.Trim()
	workspace.queue = workspace.retention.Trim(workspace.queue[:0], 64)
	workspace.recomputed = 0
	viewport := mgl32.Vec2{}
	scaleFactor := float32(1)
	if size, ok := world.GetResource[ViewportSize](); ok {
		viewport = mgl32.Vec2{size.Width, size.Height}
		if size.ScaleFactor > 0 {
			scaleFactor = size.ScaleFactor
		}
	}
	versions, structures := uiTransformRevisions(world)
	viewportChanged := workspace.viewport != viewport || workspace.scaleFactor != scaleFactor
	if workspace.initialized && workspace.structures == structures && workspace.versions == versions && !viewportChanged {
		return nil
	}
	workspace.generation++
	workspace.queue = workspace.queue[:0]
	if !workspace.initialized {
		for entity := range workspace.nodes.All {
			node := workspace.node(entity)
			node.dirty, node.state = false, uiTransformUnvisited
		}
	}
	hierarchy := HierarchyFor(world)
	if !workspace.initialized || workspace.structures != structures {
		for entity := range workspace.nodes.All {
			if parent := workspace.node(workspace.node(entity).parent); parent != nil && !world.Has[UITransform](workspace.node(entity).parent) {
				workspace.mark(entity)
			}
		}
		workspace.nodes.Prune(func(entity ecs.Entity, _ *uiTransformNode) bool { return world.Has[UITransform](entity) })
		for entity, _ := range world.Query[UITransform]().All {
			workspace.refresh(world, entity, viewport, scaleFactor, !workspace.initialized || viewportChanged)
		}
	} else if viewportChanged {
		for entity := range workspace.nodes.All {
			workspace.refresh(world, entity, viewport, scaleFactor, true)
		}
	} else {
		changes := &workspace.changes
		since := workspace.lastRun
		if since == world.ChangeTick() {
			since--
		}
		*changes = world.ChangesSince(since)
		refresh := func(entity ecs.Entity) {
			workspace.refresh(world, entity, viewport, scaleFactor, false)
		}
		if workspace.versions[0] != versions[0] {
			collectUITransformChanges[UITransform](world, changes, refresh)
		}
		if workspace.versions[1] != versions[1] {
			collectUITransformChanges[Parent](world, changes, refresh)
		}
		if workspace.versions[2] != versions[2] {
			collectUITransformChanges[ClipRect2D](world, changes, refresh)
		}
		if workspace.versions[3] != versions[3] {
			collectUITransformChanges[RoundedClip2D](world, changes, refresh)
		}
		if workspace.versions[4] != versions[4] {
			collectUITransformChanges[ClipExempt2D](world, changes, refresh)
		}
		if workspace.versions[5] != versions[5] {
			collectUITransformChanges[Size2D](world, changes, refresh)
		}
		if workspace.versions[6] != versions[6] {
			collectUITransformChanges[Sprite2D](world, changes, refresh)
		}
		if workspace.versions[7] != versions[7] {
			collectUITransformChanges[Text2D](world, changes, refresh)
		}

	}
	if workspace.initialized && workspace.versions[8] != versions[8] {
		since := workspace.lastRun
		if since == world.ChangeTick() {
			since--
		}
		changes := world.ChangesSince(since)
		for entity, access := range world.Query[UIGlobalTransform](changes.Changed[UIGlobalTransform](), ecs.With[UITransform]()).All {
			node := workspace.node(entity)
			if node == nil || node.dirty {
				continue
			}
			expected := composeUITransform(node, workspace.current(node.parent))
			value := access.Get()
			if expected.matrix != value.Matrix || expected.clip != value.clip {
				workspace.mark(entity)
			}
		}
	}
	for index := 0; index < len(workspace.queue); index++ {
		node := workspace.node(workspace.queue[index])
		if node == nil {
			continue
		}
		for child := hierarchy.FirstChild(workspace.queue[index]); child != 0; child = hierarchy.NextSibling(child) {
			workspace.mark(child)
		}
	}

	for _, entity := range workspace.queue {
		if workspace.node(entity) == nil {
			continue
		}
		if _, err := resolveUITransform(workspace, entity); err != nil {
			workspace.initialized = false
			return err
		}
	}
	workspace.initialized = true
	workspace.versions, workspace.structures = uiTransformRevisions(world)
	workspace.retention.Record(len(workspace.queue))
	workspace.lastRun = world.ChangeTick()
	workspace.viewport, workspace.scaleFactor = viewport, scaleFactor
	return nil
}

func collectUITransformChanges[T any](world *ecs.World, changes *ecs.SystemContext, refresh func(ecs.Entity)) {
	for entity, _ := range world.Query[T](changes.Changed[T](), ecs.With[UITransform]()).All {
		refresh(entity)
	}
}

func (workspace *uiTransformWorkspace) node(entity ecs.Entity) *uiTransformNode {
	return workspace.nodes.Get(entity)
}

func (workspace *uiTransformWorkspace) mark(entity ecs.Entity) {
	if node := workspace.node(entity); node != nil && !node.dirty {
		node.dirty, node.state = true, uiTransformUnvisited
		workspace.queue = append(workspace.queue, entity)
	}
}

func (workspace *uiTransformWorkspace) refresh(world *ecs.World, entity ecs.Entity, viewport mgl32.Vec2, scaleFactor float32, force bool) {
	node := workspace.nodes.Ensure(entity)
	if node.seen == workspace.generation {
		return
	}
	row := world.Row(entity)
	b := &workspace.bindings
	transform, _ := b.transform.Get(row)
	parent, _ := b.parent.Get(row)
	clip, hasClip := b.clip.Get(row)
	rounded, hasRounded := b.rounded.Get(row)
	exemption := b.exempt.Access(row).Ref()
	exempt := exemption != nil
	parentExempt := exempt && exemption.ParentOnly
	baseSize := b.baseSize(row)
	_, hasGlobal := b.global.Get(row)
	topologyChanged := node.seen == 0 || node.parent != parent.Entity
	dirty := force || topologyChanged || !hasGlobal || node.transform != transform || node.baseSize != baseSize ||
		node.localClip != clip || node.hasClip != hasClip || node.localRoundedClip != rounded || node.hasRoundedClip != hasRounded || node.clipExempt != exempt || node.parentClipExempt != parentExempt
	node.seen = workspace.generation
	node.transform, node.parent, node.baseSize = transform, parent.Entity, baseSize
	node.localClip, node.hasClip, node.localRoundedClip, node.hasRoundedClip, node.clipExempt = clip, hasClip, rounded, hasRounded, exempt
	node.parentClipExempt = parentExempt
	if dirty {
		node.local = transform.LocalMatrix(UIResolveContext{ScaleFactor: scaleFactor, BaseSize: baseSize, Viewport: viewport})
		workspace.mark(entity)
	}
}

func resolveUITransform(workspace *uiTransformWorkspace, entity ecs.Entity) (resolvedUITransform, error) {
	node := workspace.node(entity)
	if node == nil {
		return resolvedUITransform{matrix: mgl32.Ident4()}, nil
	}
	if node.state == uiTransformResolved {
		return workspace.current(entity), nil
	}
	if node.state == uiTransformVisiting {
		return resolvedUITransform{}, fmt.Errorf("ecs: cycle detected in UI Parent hierarchy at entity %d", entity)
	}
	node.state = uiTransformVisiting
	parentTransform := resolvedUITransform{matrix: mgl32.Ident4()}
	if node.parent != 0 {
		var err error
		parentTransform, err = resolveUITransform(workspace, node.parent)
		if err != nil {
			return resolvedUITransform{}, err
		}
	}
	if !node.dirty && node.parentRevision == parentTransform.revision {
		node.state = uiTransformResolved
		return workspace.current(entity), nil
	}
	resolved := composeUITransform(node, parentTransform)
	world := workspace.bindings.world
	access := workspace.bindings.global.Access(world.Row(entity))
	value := UIGlobalTransform{Matrix: resolved.matrix, clip: resolved.clip}
	if access.Ref() != nil {
		ecs.CommitComponent(access, value)
	} else if err := world.Insert(entity, value); err != nil {
		return resolvedUITransform{}, err
	}
	node.inheritedClip, node.revision = resolved.inheritedClip, resolved.revision
	node.dirty = false
	node.parentRevision = parentTransform.revision
	workspace.recomputed++
	node.state = uiTransformResolved
	return resolved, nil
}

func (workspace *uiTransformWorkspace) current(entity ecs.Entity) resolvedUITransform {
	node := workspace.node(entity)
	if node == nil {
		return resolvedUITransform{matrix: mgl32.Ident4()}
	}
	global, _ := workspace.bindings.global.Get(workspace.bindings.world.Row(entity))
	return resolvedUITransform{matrix: global.Matrix, clip: global.clip, inheritedClip: node.inheritedClip, revision: node.revision}
}

func composeUITransform(node *uiTransformNode, parent resolvedUITransform) resolvedUITransform {
	resolved := resolvedUITransform{matrix: parent.matrix.Mul4(node.local), clip: parent.clip, revision: node.revision + 1}
	if node.clipExempt {
		resolved.clip = clipRect2D{}
		if node.parentClipExempt {
			resolved.clip = parent.inheritedClip
		}
	}
	resolved.inheritedClip = resolved.clip
	if node.hasClip {
		resolved.clip = intersectClipRect2D(resolved.clip, clipRectFromLocal2D(resolved.matrix, node.localClip))
	}
	if node.hasRoundedClip {
		resolved.clip = addRoundedClip2D(resolved.clip, roundedClipFromLocal2D(resolved.matrix, node.localRoundedClip))
	}
	return resolved
}

func (b *uiTransformBindings) baseSize(row ecs.EntityRow) mgl32.Vec2 {
	if size, ok := b.size.Get(row); ok {
		return mgl32.Vec2{size.Width, size.Height}
	}
	if sprite := b.sprite.Access(row).Ref(); sprite != nil {
		width, height := sprite.Size()
		return mgl32.Vec2{width, height}
	}
	if text := b.text.Access(row).Ref(); text != nil {
		width, height, _ := MeasureText2D(text)
		return mgl32.Vec2{width, height}
	}
	if clip, ok := b.clip.Get(row); ok {
		return mgl32.Vec2{clip.Width, clip.Height}
	}
	return mgl32.Vec2{}
}
