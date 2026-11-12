package spine

import (
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/onix-engine/onix/graphics"
	"github.com/onix-engine/onix/internal/sourcejson"
	"github.com/onix-engine/onix/render"
)

type Resource struct {
	Texture     *graphics.Texture
	atlas       map[string]atlasRegion
	animations  map[string]animation
	skins       map[string]skin
	ik          []ikConstraintData
	slots       []slotData
	children    [][]int
	transforms  []transformConstraintData
	paths       []pathConstraintData
	updateCache []updateOperation
	bones       []boneData
	Height      float32
	Width       float32
}

type Player struct {
	Resource  *Resource
	pose      *pose
	bounds    map[animationBoundsKey]animationBounds
	Animation string
	Skin      string
	path      pathWorkspace
	mesh      meshWorkspace
	Speed     float32
	Alpha     float32
	time      float32
	Loop      bool
}

type pathWorkspace struct {
	spaces        []float32
	lengths       []float32
	positions     []float32
	worldVertices []float32
	world         []float32
	curves        []float32
	segments      []float32
}

type meshWorkspace struct {
	positions []float32
	uvs       []float32
	clipper   skeletonClipper
}

type animationBoundsKey struct {
	animation string
	skin      string
}

type animationBounds struct {
	minX, minY, maxX, maxY float32
	valid                  bool
}

type boneData struct {
	name                                           string
	parent                                         int
	length                                         float32
	x, y, rotation, scaleX, scaleY, shearX, shearY float32
	transform                                      transformMode
}

type transformMode uint8

const (
	transformNormal transformMode = iota
	transformOnlyTranslation
	transformNoRotationOrReflection
	transformNoScale
	transformNoScaleOrReflection
)

type ikConstraintData struct {
	name                       string
	bones                      []int
	order                      int
	target                     int
	bendDirection              int
	mix                        float32
	compress, stretch, uniform bool
}

type transformConstraintData struct {
	name                                     string
	bones                                    []int
	order                                    int
	target                                   int
	rotateMix                                float32
	offsetRotation, offsetX, offsetY         float32
	offsetScaleX, offsetScaleY, offsetShearY float32
	translateMix                             float32
	scaleMix                                 float32
	shearMix                                 float32
	relative                                 bool
	local                                    bool
}

type pathConstraintData struct {
	name                             string
	positionMode, spacingMode        string
	rotateMode                       string
	bones                            []int
	order                            int
	target                           int
	offsetRotation, position         float32
	spacing, rotateMix, translateMix float32
}

type updateKind uint8

const (
	updateBone updateKind = iota
	updateIK
	updateTransform
	updatePath
)

type updateOperation struct {
	kind  updateKind
	index int
}

type orderedConstraint struct {
	order int
	kind  updateKind
	index int
}

type slotData struct {
	name       string
	attachment string
	bone       int
	color      [4]float32
	dark       [3]float32
	blend      render.MeshBlendMode
}

type attachment struct {
	parentMesh            *attachment
	parentSkin            string
	parent                string
	kind                  string
	name                  string
	path                  string
	uvs                   []float32
	lengths               []float32
	vertices              []float32
	triangles             []uint32
	endSlot               int
	vertexCount           int
	color                 [4]float32
	y                     float32
	height                float32
	width                 float32
	scaleY                float32
	scaleX                float32
	rotation              float32
	x                     float32
	weighted              bool
	inheritDeform         bool
	closed, constantSpeed bool
}

type skin map[int]map[string]*attachment

type keyframe struct {
	name      string
	color     [4]float32
	curve     [4]float32
	dark      [3]float32
	time      float32
	x, y      float32
	curveKind uint8
}

func (f keyframe) timelineTime() float32              { return f.time }
func (f keyframe) timelineCurve() (uint8, [4]float32) { return f.curveKind, f.curve }

type boneTimeline struct {
	kind   string
	frames []keyframe
	bone   int
}
type slotTimeline struct {
	kind   string
	frames []keyframe
	slot   int
}
type drawOrderFrame struct {
	order []int
	time  float32
}
type deformFrame struct {
	vertices  []float32
	curve     [4]float32
	time      float32
	curveKind uint8
}

func (f deformFrame) timelineTime() float32              { return f.time }
func (f deformFrame) timelineCurve() (uint8, [4]float32) { return f.curveKind, f.curve }

type deformTimeline struct {
	attachment *attachment
	frames     []deformFrame
	slot       int
}
type ikKeyframe struct {
	time              float32
	mix               float32
	bendDirection     int
	compress, stretch bool
	curve             [4]float32
	curveKind         uint8
}

func (f ikKeyframe) timelineTime() float32              { return f.time }
func (f ikKeyframe) timelineCurve() (uint8, [4]float32) { return f.curveKind, f.curve }

type ikTimeline struct {
	frames     []ikKeyframe
	constraint int
}
type transformKeyframe struct {
	time                                        float32
	rotateMix, translateMix, scaleMix, shearMix float32
	curve                                       [4]float32
	curveKind                                   uint8
}

func (f transformKeyframe) timelineTime() float32              { return f.time }
func (f transformKeyframe) timelineCurve() (uint8, [4]float32) { return f.curveKind, f.curve }

type transformTimeline struct {
	frames     []transformKeyframe
	constraint int
}
type pathKeyframe struct {
	time, value, rotateMix, translateMix float32
	curve                                [4]float32
	curveKind                            uint8
}

func (f pathKeyframe) timelineTime() float32              { return f.time }
func (f pathKeyframe) timelineCurve() (uint8, [4]float32) { return f.curveKind, f.curve }

type pathTimeline struct {
	kind       string
	frames     []pathKeyframe
	constraint int
}
type animation struct {
	bones     []boneTimeline
	slots     []slotTimeline
	deform    []deformTimeline
	ik        []ikTimeline
	transform []transformTimeline
	paths     []pathTimeline
	drawOrder []drawOrderFrame
	duration  float32
}

type bonePose struct {
	x, y, rotation, scaleX, scaleY, shearX, shearY float32
	ax, ay, arotation, ascaleX, ascaleY            float32
	ashearX, ashearY                               float32
	appliedValid                                   bool
	world                                          affine
}
type slotPose struct {
	attachment string
	deform     []float32
	color      [4]float32
	dark       [3]float32
}
type ikPose struct {
	bendDirection     int
	compress, stretch bool
	mix               float32
}
type transformPose struct {
	rotateMix, translateMix, scaleMix, shearMix float32
}
type pathPose struct {
	position, spacing, rotateMix, translateMix float32
}
type pose struct {
	bones      []bonePose
	slots      []slotPose
	ik         []ikPose
	transforms []transformPose
	paths      []pathPose
	drawOrder  []int
}
type affine struct{ a, b, c, d, x, y float32 }

// Mesh pipelines use reverse depth and clear to zero, so 2D geometry must sit in front of the clear plane.
const meshDepth = float32(-1)
const radToDeg = float32(180 / math.Pi)
const degToRad = float32(math.Pi / 180)

type rawSkeleton struct {
	Skins      map[string]map[string]map[string]sourcejson.RawMessage `json:"skins"`
	Animations map[string]sourcejson.RawMessage                       `json:"animations"`
	Bones      []struct {
		ScaleX, ScaleY          *float32
		Name, Parent, Transform string
		Length, X, Y, Rotation  float32
		ShearX, ShearY          float32
	} `json:"bones"`
	IK []struct {
		BendPositive               *bool
		Mix                        *float32
		Name, Target               string
		Bones                      []string
		Order                      int
		Compress, Stretch, Uniform bool
	} `json:"ik"`
	Transform []struct {
		ShearMix     *float32
		ScaleMix     *float32
		TranslateMix *float32
		RotateMix    *float32
		Name, Target string
		Bones        []string
		Order        int
		Y            float32
		ScaleX       float32
		ScaleY       float32
		ShearY       float32
		X            float32
		Rotation     float32
		Relative     bool
		Local        bool
	} `json:"transform"`
	Path []struct {
		RotateMix, TranslateMix                             *float32
		Name, Target, PositionMode, SpacingMode, RotateMode string
		Bones                                               []string
		Order                                               int
		Rotation, Position, Spacing                         float32
	} `json:"path"`
	Slots    []struct{ Name, Bone, Attachment, Color, Dark, Blend string } `json:"slots"`
	Skeleton struct {
		Width  float32 `json:"width"`
		Height float32 `json:"height"`
	} `json:"skeleton"`
}

type rawAttachment struct {
	ScaleX        *float32
	ConstantSpeed *bool `json:"constantSpeed"`
	Deform        *bool `json:"deform"`
	ScaleY        *float32
	Name          string
	Path          string
	Parent        string
	Skin          string
	Color         string
	End           string
	Type          string
	UVs           []float32 `json:"uvs"`
	Vertices      []float32 `json:"vertices"`
	Triangles     []uint32  `json:"triangles"`
	Lengths       []float32 `json:"lengths"`
	VertexCount   int
	Height        float32
	Width         float32
	Rotation      float32
	Y             float32
	X             float32
	Closed        bool `json:"closed"`
}

func Decode(skeletonJSON, atlasData, textureData []byte, texturePath string, options graphics.TextureOptions, decoder graphics.ImageDecoder) (*Resource, error) {
	if decoder == nil {
		return nil, fmt.Errorf("spine: image decoder is not configured")
	}
	var texture *graphics.Texture
	resource, err := LoadWithTexture(skeletonJSON, atlasData, options, func(resolved graphics.TextureOptions) (*graphics.Texture, error) {
		var err error
		texture, err = decoder.DecodeImage(textureData, texturePath, resolved)
		if err != nil {
			return nil, fmt.Errorf("spine: decode texture: %w", err)
		}
		return texture, nil
	})
	if err != nil {
		if texture != nil {
			_ = texture.Dispose()
		}
		return nil, err
	}
	return resource, nil
}

// LoadWithTexture parses a skeleton once, resolves its texture options, and
// asks loadTexture for a texture that may come from a shared cache. The caller
// retains ownership of the returned texture.
func LoadWithTexture(skeletonJSON, atlasData []byte, options graphics.TextureOptions, loadTexture func(graphics.TextureOptions) (*graphics.Texture, error)) (*Resource, error) {
	if loadTexture == nil {
		return nil, fmt.Errorf("spine: texture loader is not configured")
	}
	compiled, preprocessed, err := decodeCompiled(skeletonJSON)
	if err != nil {
		return nil, err
	}
	if !preprocessed && !sourcejson.Enabled {
		return nil, fmt.Errorf("spine: release builds require preprocessed Spine IR")
	}
	var regions map[string]atlasRegion
	if preprocessed {
		options.Mipmap = compiled.Mipmapped
	} else {
		page, parsedRegions, err := parseAtlas(atlasData)
		if err != nil {
			return nil, err
		}
		options.Mipmap = page.mipmapped()
		regions = parsedRegions
	}
	texture, err := loadTexture(options)
	if err != nil {
		return nil, err
	}
	if texture == nil || !texture.Valid() {
		return nil, fmt.Errorf("spine: texture is invalid")
	}
	if preprocessed {
		return restoreResource(compiled, texture)
	}
	return compileSkeleton(skeletonJSON, regions, texture)
}

func Load(skeletonJSON, atlasData []byte, texture *graphics.Texture) (*Resource, error) {
	if texture == nil || !texture.Valid() {
		return nil, fmt.Errorf("spine: texture is invalid")
	}
	if resource, preprocessed, err := decodePreprocessed(skeletonJSON, texture); preprocessed {
		return resource, err
	}
	if !sourcejson.Enabled {
		return nil, fmt.Errorf("spine: release builds require preprocessed Spine IR")
	}
	_, regions, err := parseAtlas(atlasData)
	if err != nil {
		return nil, err
	}
	return compileSkeleton(skeletonJSON, regions, texture)
}

func compileSkeleton(skeletonJSON []byte, regions map[string]atlasRegion, texture *graphics.Texture) (*Resource, error) {
	var raw rawSkeleton
	if err := sourcejson.Unmarshal(skeletonJSON, &raw); err != nil {
		return nil, fmt.Errorf("spine: parse skeleton: %w", err)
	}
	resource := &Resource{Texture: texture, Width: raw.Skeleton.Width, Height: raw.Skeleton.Height, atlas: regions, skins: make(map[string]skin), animations: make(map[string]animation)}
	boneIndex := make(map[string]int, len(raw.Bones))
	for i, item := range raw.Bones {
		parent := -1
		if item.Parent != "" {
			var ok bool
			parent, ok = boneIndex[item.Parent]
			if !ok {
				return nil, fmt.Errorf("spine: bone %q has unknown parent %q", item.Name, item.Parent)
			}
		}
		boneIndex[item.Name] = i
		resource.bones = append(resource.bones, boneData{
			name: item.Name, parent: parent, length: item.Length,
			x: item.X, y: item.Y, rotation: item.Rotation, scaleX: valueOr(item.ScaleX, float32(1)), scaleY: valueOr(item.ScaleY, float32(1)), shearX: item.ShearX, shearY: item.ShearY,
			transform: parseTransformMode(item.Transform),
		})
	}
	resource.children = make([][]int, len(resource.bones))
	for bone, data := range resource.bones {
		if data.parent >= 0 {
			resource.children[data.parent] = append(resource.children[data.parent], bone)
		}
	}
	slotIndex := make(map[string]int, len(raw.Slots))
	for i, item := range raw.Slots {
		bone, ok := boneIndex[item.Bone]
		if !ok {
			return nil, fmt.Errorf("spine: slot %q has unknown bone %q", item.Name, item.Bone)
		}
		resource.slots = append(resource.slots, slotData{
			name: item.Name, bone: bone, attachment: item.Attachment,
			color: parseColor(item.Color), dark: parseDarkColor(item.Dark), blend: parseBlendMode(item.Blend),
		})
		slotIndex[item.Name] = i
	}
	ikIndex := make(map[string]int, len(raw.IK))
	for _, item := range raw.IK {
		if len(item.Bones) < 1 || len(item.Bones) > 2 {
			return nil, fmt.Errorf("spine: IK constraint %q must use one or two bones", item.Name)
		}
		target, ok := boneIndex[item.Target]
		if !ok {
			return nil, fmt.Errorf("spine: IK constraint %q has unknown target %q", item.Name, item.Target)
		}
		bones := make([]int, len(item.Bones))
		for i, name := range item.Bones {
			bone, ok := boneIndex[name]
			if !ok {
				return nil, fmt.Errorf("spine: IK constraint %q has unknown bone %q", item.Name, name)
			}
			bones[i] = bone
		}
		bendDirection := 1
		if item.BendPositive != nil && !*item.BendPositive {
			bendDirection = -1
		}
		mix := float32(1)
		if item.Mix != nil {
			mix = *item.Mix
		}
		ikIndex[item.Name] = len(resource.ik)
		resource.ik = append(resource.ik, ikConstraintData{
			name: item.Name, order: item.Order, bones: bones, target: target, bendDirection: bendDirection,
			compress: item.Compress, stretch: item.Stretch, uniform: item.Uniform, mix: mix,
		})
	}
	transformIndex := make(map[string]int, len(raw.Transform))
	for _, item := range raw.Transform {
		if len(item.Bones) == 0 {
			return nil, fmt.Errorf("spine: transform constraint %q has no bones", item.Name)
		}
		target, ok := boneIndex[item.Target]
		if !ok {
			return nil, fmt.Errorf("spine: transform constraint %q has unknown target %q", item.Name, item.Target)
		}
		bones := make([]int, len(item.Bones))
		for i, name := range item.Bones {
			bone, ok := boneIndex[name]
			if !ok {
				return nil, fmt.Errorf("spine: transform constraint %q has unknown bone %q", item.Name, name)
			}
			if item.Local && resource.bones[bone].parent < 0 {
				return nil, fmt.Errorf("spine: local transform constraint %q cannot constrain a root bone", item.Name)
			}
			bones[i] = bone
		}
		transformIndex[item.Name] = len(resource.transforms)
		resource.transforms = append(resource.transforms, transformConstraintData{
			name: item.Name, order: item.Order, bones: bones, target: target, local: item.Local, relative: item.Relative,
			offsetRotation: item.Rotation, offsetX: item.X, offsetY: item.Y,
			offsetScaleX: item.ScaleX, offsetScaleY: item.ScaleY, offsetShearY: item.ShearY,
			rotateMix: valueOr(item.RotateMix, float32(1)), translateMix: valueOr(item.TranslateMix, float32(1)),
			scaleMix: valueOr(item.ScaleMix, float32(1)), shearMix: valueOr(item.ShearMix, float32(1)),
		})
	}
	pathIndex := make(map[string]int, len(raw.Path))
	for _, item := range raw.Path {
		if len(item.Bones) == 0 {
			return nil, fmt.Errorf("spine: path constraint %q has no bones", item.Name)
		}
		target, ok := slotIndex[item.Target]
		if !ok {
			return nil, fmt.Errorf("spine: path constraint %q has unknown target slot %q", item.Name, item.Target)
		}
		bones := make([]int, len(item.Bones))
		for i, name := range item.Bones {
			bone, ok := boneIndex[name]
			if !ok {
				return nil, fmt.Errorf("spine: path constraint %q has unknown bone %q", item.Name, name)
			}
			bones[i] = bone
		}
		pathIndex[item.Name] = len(resource.paths)
		resource.paths = append(resource.paths, pathConstraintData{
			name: item.Name, order: item.Order, bones: bones, target: target,
			positionMode: valueOrEmpty(item.PositionMode, "percent"), spacingMode: valueOrEmpty(item.SpacingMode, "length"),
			rotateMode: valueOrEmpty(item.RotateMode, "tangent"), offsetRotation: item.Rotation,
			position: item.Position, spacing: item.Spacing,
			rotateMix: valueOr(item.RotateMix, float32(1)), translateMix: valueOr(item.TranslateMix, float32(1)),
		})
	}
	for skinName, rawSkin := range raw.Skins {
		parsed := make(skin)
		for slotName, entries := range rawSkin {
			si, ok := slotIndex[slotName]
			if !ok {
				return nil, fmt.Errorf("spine: skin %q uses unknown slot %q", skinName, slotName)
			}
			parsed[si] = make(map[string]*attachment)
			for name, data := range entries {
				var value rawAttachment
				if err := sourcejson.Unmarshal(data, &value); err != nil {
					return nil, fmt.Errorf("spine: attachment %q: %w", name, err)
				}
				if value.Type == "" {
					value.Type = "region"
				}
				if value.Name == "" {
					value.Name = name
				}
				if value.Path == "" && value.Type != "linkedmesh" {
					value.Path = value.Name
				}
				if value.Type != "region" && value.Type != "mesh" && value.Type != "linkedmesh" && value.Type != "clipping" && value.Type != "path" {
					continue
				}
				endSlot := -1
				if value.End != "" {
					var ok bool
					endSlot, ok = slotIndex[value.End]
					if !ok {
						return nil, fmt.Errorf("spine: clipping attachment %q has unknown end slot %q", name, value.End)
					}
				}
				inheritDeform := value.Deform == nil || *value.Deform
				parsed[si][name] = &attachment{
					name: value.Name, path: value.Path, parent: value.Parent, parentSkin: value.Skin, kind: value.Type,
					x: value.X, y: value.Y, rotation: value.Rotation, scaleX: valueOr(value.ScaleX, float32(1)), scaleY: valueOr(value.ScaleY, float32(1)),
					width: value.Width, height: value.Height, color: parseColor(value.Color), uvs: value.UVs, triangles: value.Triangles,
					vertices: value.Vertices, inheritDeform: inheritDeform, vertexCount: value.VertexCount, endSlot: endSlot,
					closed: value.Closed, constantSpeed: valueOr(value.ConstantSpeed, true), lengths: value.Lengths,
				}
			}
		}
		resource.skins[skinName] = parsed
	}
	for _, parsed := range resource.skins {
		for _, entries := range parsed {
			for _, a := range entries {
				if a.kind == "mesh" || a.kind == "clipping" || a.kind == "path" {
					worldVerticesLength := len(a.uvs)
					if a.kind == "clipping" || a.kind == "path" {
						worldVerticesLength = a.vertexCount * 2
					}
					a.weighted = len(a.vertices) != worldVerticesLength
				}
			}
		}
	}
	for skinName, parsed := range resource.skins {
		for slotIndex, entries := range parsed {
			for _, child := range entries {
				if child.kind != "linkedmesh" {
					continue
				}
				parentSkin := skinName
				if child.parentSkin != "" {
					parentSkin = child.parentSkin
				}
				parent := resource.skins[parentSkin][slotIndex][child.parent]
				if parent == nil {
					return nil, fmt.Errorf("spine: linked mesh %q parent %q not found", child.name, child.parent)
				}
				child.kind, child.path = "mesh", parent.path
				child.parentMesh = parent
				child.uvs, child.triangles, child.vertices, child.weighted = parent.uvs, parent.triangles, parent.vertices, parent.weighted
			}
		}
	}
	resource.buildUpdateCache()
	for name, data := range raw.Animations {
		anim, err := parseAnimation(data, resource, boneIndex, slotIndex, ikIndex, transformIndex, pathIndex)
		if err != nil {
			return nil, fmt.Errorf("spine: animation %q: %w", name, err)
		}
		resource.animations[name] = anim
	}
	if len(resource.bones) == 0 {
		return nil, fmt.Errorf("spine: skeleton has no bones")
	}
	return resource, nil
}

func (a *Resource) buildUpdateCache() {
	a.updateCache = a.updateCache[:0]
	constraints := make([]orderedConstraint, 0, len(a.ik)+len(a.transforms)+len(a.paths))
	for i, constraint := range a.ik {
		constraints = append(constraints, orderedConstraint{order: constraint.order, kind: updateIK, index: i})
	}
	for i, constraint := range a.transforms {
		constraints = append(constraints, orderedConstraint{order: constraint.order, kind: updateTransform, index: i})
	}
	for i, constraint := range a.paths {
		constraints = append(constraints, orderedConstraint{order: constraint.order, kind: updatePath, index: i})
	}
	slices.SortStableFunc(constraints, func(a, b orderedConstraint) int { return a.order - b.order })

	sorted := make([]bool, len(a.bones))
	var sortBone func(int)
	sortBone = func(bone int) {
		if bone < 0 || sorted[bone] {
			return
		}
		sortBone(a.bones[bone].parent)
		sorted[bone] = true
		a.updateCache = append(a.updateCache, updateOperation{kind: updateBone, index: bone})
	}
	var sortReset func([]int)
	sortReset = func(bones []int) {
		for _, bone := range bones {
			if sorted[bone] {
				sortReset(a.children[bone])
			}
			sorted[bone] = false
		}
	}

	for _, ref := range constraints {
		switch ref.kind {
		case updateIK:
			constraint := a.ik[ref.index]
			sortBone(constraint.target)
			parent := constraint.bones[0]
			sortBone(parent)
			a.updateCache = append(a.updateCache, updateOperation{kind: updateIK, index: ref.index})
			sortReset(a.children[parent])
			sorted[constraint.bones[len(constraint.bones)-1]] = true
		case updateTransform:
			constraint := a.transforms[ref.index]
			sortBone(constraint.target)
			for _, bone := range constraint.bones {
				if constraint.local {
					sortBone(a.bones[bone].parent)
				} else {
					sortBone(bone)
				}
			}
			a.updateCache = append(a.updateCache, updateOperation{kind: updateTransform, index: ref.index})
			for _, bone := range constraint.bones {
				sortReset(a.children[bone])
				sorted[bone] = true
			}
		case updatePath:
			constraint := a.paths[ref.index]
			for _, parsed := range a.skins {
				for _, attachment := range parsed[constraint.target] {
					if attachment.kind != "path" {
						continue
					}
					if attachment.weighted {
						for _, bone := range weightedAttachmentBones(attachment.vertices) {
							sortBone(bone)
						}
					} else {
						sortBone(a.slots[constraint.target].bone)
					}
				}
			}
			for _, bone := range constraint.bones {
				sortBone(bone)
			}
			a.updateCache = append(a.updateCache, updateOperation{kind: updatePath, index: ref.index})
			for _, bone := range constraint.bones {
				sortReset(a.children[bone])
				sorted[bone] = true
			}
		}
	}
	for bone := range a.bones {
		sortBone(bone)
	}
}

func weightedAttachmentBones(vertices []float32) []int {
	unique := make(map[int]struct{})
	result := make([]int, 0)
	for cursor := 0; cursor < len(vertices); {
		count := int(vertices[cursor])
		cursor++
		for range count {
			if cursor+3 >= len(vertices) {
				return result
			}
			bone := int(vertices[cursor])
			if _, ok := unique[bone]; !ok {
				unique[bone] = struct{}{}
				result = append(result, bone)
			}
			cursor += 4
		}
	}
	return result
}

func (a *Resource) Animations() []string {
	result := mapKeys(a.animations)
	slices.Sort(result)
	return result
}
func (a *Resource) Skins() []string { result := mapKeys(a.skins); slices.Sort(result); return result }
func mapKeys[K comparable, V any](values map[K]V) []K {
	result := make([]K, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	return result
}

func NewPlayer(resource *Resource) *Player {
	p := &Player{Resource: resource, Loop: true, Speed: 1, Alpha: 1, Skin: "default"}
	if resource != nil {
		if names := resource.Skins(); len(names) > 0 {
			p.Skin = names[0]
		}
		if names := resource.Animations(); len(names) > 0 {
			p.Animation = names[0]
		}
		p.resetPose()
	}
	return p
}

func (p *Player) SetAnimation(name string, loop bool) error {
	if p == nil || p.Resource == nil {
		return fmt.Errorf("spine: player has no resource")
	}
	if _, ok := p.Resource.animations[name]; !ok {
		return fmt.Errorf("spine: animation %q not found", name)
	}
	p.Animation, p.Loop, p.time = name, loop, 0
	return nil
}

func (p *Player) AnimationDuration() float32 {
	if p == nil || p.Resource == nil {
		return 0
	}
	return p.Resource.animations[p.Animation].duration
}

func (p *Player) AnimationTime() float32 {
	if p == nil {
		return 0
	}
	return p.time
}

func (p *Player) AnimationComplete() bool {
	return p != nil && !p.Loop && p.time >= p.AnimationDuration()
}

func (p *Player) SetSkin(name string) error {
	if p == nil || p.Resource == nil {
		return fmt.Errorf("spine: player has no resource")
	}
	if _, ok := p.Resource.skins[name]; !ok {
		return fmt.Errorf("spine: skin %q not found", name)
	}
	p.Skin = name
	if _, ok := p.Resource.animations[p.Animation]; ok {
		p.evaluate()
	} else {
		p.resetPose()
	}
	return nil
}

func (p *Player) Update(delta float32) {
	if p == nil || p.Resource == nil {
		return
	}
	if p.Speed == 0 {
		p.Speed = 1
	}
	p.time += max(delta, 0) * p.Speed
	p.evaluate()
}

func (p *Player) resetPose() {
	if p == nil || p.Resource == nil {
		return
	}
	if p.pose == nil {
		p.pose = &pose{}
	}
	p.pose.bones = resizeSlice(p.pose.bones, len(p.Resource.bones))
	p.pose.slots = resizeSlice(p.pose.slots, len(p.Resource.slots))
	p.pose.ik = resizeSlice(p.pose.ik, len(p.Resource.ik))
	p.pose.transforms = resizeSlice(p.pose.transforms, len(p.Resource.transforms))
	p.pose.paths = resizeSlice(p.pose.paths, len(p.Resource.paths))
	p.pose.drawOrder = resizeSlice(p.pose.drawOrder, len(p.Resource.slots))
	for i, b := range p.Resource.bones {
		p.pose.bones[i] = bonePose{x: b.x, y: b.y, rotation: b.rotation, scaleX: b.scaleX, scaleY: b.scaleY, shearX: b.shearX, shearY: b.shearY}
	}
	for i, s := range p.Resource.slots {
		deform := p.pose.slots[i].deform[:0]
		p.pose.slots[i] = slotPose{attachment: s.attachment, color: s.color, dark: s.dark, deform: deform}
		p.pose.drawOrder[i] = i
	}
	for i, constraint := range p.Resource.ik {
		p.pose.ik[i] = ikPose{bendDirection: constraint.bendDirection, compress: constraint.compress, stretch: constraint.stretch, mix: constraint.mix}
	}
	for i, constraint := range p.Resource.transforms {
		p.pose.transforms[i] = transformPose{
			rotateMix: constraint.rotateMix, translateMix: constraint.translateMix,
			scaleMix: constraint.scaleMix, shearMix: constraint.shearMix,
		}
	}
	for i, constraint := range p.Resource.paths {
		p.pose.paths[i] = pathPose{
			position: constraint.position, spacing: constraint.spacing,
			rotateMix: constraint.rotateMix, translateMix: constraint.translateMix,
		}
	}
	p.updateWorld()
}

func resizeSlice[T any](values []T, length int) []T {
	if cap(values) < length {
		return make([]T, length)
	}
	return values[:length]
}

func resetSlice[T any](values []T, capacity int) []T {
	if cap(values) < capacity {
		return make([]T, 0, capacity)
	}
	return values[:0]
}

func (p *Player) evaluate() {
	p.resetPose()
	anim, ok := p.Resource.animations[p.Animation]
	if !ok {
		return
	}
	t := p.time
	if p.Loop && anim.duration > 0 {
		t = float32(math.Mod(float64(t), float64(anim.duration)))
	} else if t > anim.duration {
		t = anim.duration
	}
	for _, timeline := range anim.bones {
		value, ok := sample(timeline.frames, t, timeline.kind == "rotate")
		if !ok {
			continue
		}
		b := &p.pose.bones[timeline.bone]
		setup := p.Resource.bones[timeline.bone]
		switch timeline.kind {
		case "rotate":
			b.rotation = setup.rotation + wrapDegrees(value.x)
		case "translate":
			b.x = setup.x + value.x
			b.y = setup.y + value.y
		case "scale":
			b.scaleX = setup.scaleX * value.x
			b.scaleY = setup.scaleY * value.y
		case "shear":
			b.shearX = setup.shearX + value.x
			b.shearY = setup.shearY + value.y
		}
	}
	for _, timeline := range anim.slots {
		value, ok := sample(timeline.frames, t, false)
		if !ok {
			continue
		}
		s := &p.pose.slots[timeline.slot]
		switch timeline.kind {
		case "attachment":
			s.attachment = value.name
		case "color":
			s.color = value.color
		case "twoColor":
			s.color = value.color
			s.dark = value.dark
		}
	}
	for _, timeline := range anim.ik {
		value, ok := sampleIK(timeline.frames, t)
		if !ok {
			continue
		}
		p.pose.ik[timeline.constraint] = value
	}
	for _, timeline := range anim.transform {
		value, ok := sampleTransform(timeline.frames, t)
		if !ok {
			continue
		}
		p.pose.transforms[timeline.constraint] = value
	}
	for _, timeline := range anim.paths {
		value, ok := samplePath(timeline.frames, t, timeline.kind)
		if !ok {
			continue
		}
		state := &p.pose.paths[timeline.constraint]
		switch timeline.kind {
		case "position":
			state.position = value.value
		case "spacing":
			state.spacing = value.value
		case "mix":
			state.rotateMix, state.translateMix = value.rotateMix, value.translateMix
		}
	}
	for _, timeline := range anim.deform {
		p.applyDeform(timeline, t)
	}
	for _, frame := range anim.drawOrder {
		if frame.time <= t {
			copy(p.pose.drawOrder, frame.order)
		} else {
			break
		}
	}
	p.updateWorld()
}

func (p *Player) updateWorld() {
	for i := range p.pose.bones {
		p.resetApplied(i)
	}
	for _, operation := range p.Resource.updateCache {
		switch operation.kind {
		case updateBone:
			p.updateWorldBone(operation.index)
		case updateIK:
			p.applyIK(operation.index)
		case updateTransform:
			p.applyTransform(operation.index)
		case updatePath:
			p.applyPath(operation.index)
		}
	}
}

func (p *Player) resetApplied(index int) {
	b := &p.pose.bones[index]
	b.ax, b.ay, b.arotation = b.x, b.y, b.rotation
	b.ascaleX, b.ascaleY = b.scaleX, b.scaleY
	b.ashearX, b.ashearY, b.appliedValid = b.shearX, b.shearY, true
}

func (p *Player) updateWorldBone(index int) {
	b := p.pose.bones[index]
	p.updateWorldWith(index, b.x, b.y, b.rotation, b.scaleX, b.scaleY, b.shearX, b.shearY)
}

func (p *Player) updateWorldWith(index int, x, y, rotation, scaleX, scaleY, shearX, shearY float32) {
	data := p.Resource.bones[index]
	b := &p.pose.bones[index]
	b.ax, b.ay, b.arotation = x, y, rotation
	b.ascaleX, b.ascaleY = scaleX, scaleY
	b.ashearX, b.ashearY, b.appliedValid = shearX, shearY, true
	if data.parent < 0 {
		rotationY := rotation + 90 + shearY
		b.world = affine{
			a: cosDeg(rotation+shearX) * scaleX,
			b: cosDeg(rotationY) * scaleY,
			c: sinDeg(rotation+shearX) * scaleX,
			d: sinDeg(rotationY) * scaleY,
			x: x, y: y,
		}
		return
	}
	parent := p.pose.bones[data.parent].world
	worldX := parent.a*x + parent.b*y + parent.x
	worldY := parent.c*x + parent.d*y + parent.y
	pa, pb, pc, pd := parent.a, parent.b, parent.c, parent.d
	switch data.transform {
	case transformOnlyTranslation:
		rotationY := rotation + 90 + shearY
		b.world = affine{
			a: cosDeg(rotation+shearX) * scaleX,
			b: cosDeg(rotationY) * scaleY,
			c: sinDeg(rotation+shearX) * scaleX,
			d: sinDeg(rotationY) * scaleY,
			x: worldX, y: worldY,
		}
	case transformNoRotationOrReflection:
		s := pa*pa + pc*pc
		var prx float32
		if s > .0001 {
			s = abs(pa*pd-pb*pc) / s
			pb, pd = pc*s, pa*s
			prx = atan2Deg(pc, pa)
		} else {
			pa, pc = 0, 0
			prx = 90 - atan2Deg(pd, pb)
		}
		rx, ry := rotation+shearX-prx, rotation+shearY-prx+90
		la, lb := cosDeg(rx)*scaleX, cosDeg(ry)*scaleY
		lc, ld := sinDeg(rx)*scaleX, sinDeg(ry)*scaleY
		b.world = affine{
			a: pa*la - pb*lc, b: pa*lb - pb*ld,
			c: pc*la + pd*lc, d: pc*lb + pd*ld,
			x: worldX, y: worldY,
		}
	case transformNoScale, transformNoScaleOrReflection:
		cosine, sine := cosDeg(rotation), sinDeg(rotation)
		za := pa*cosine + pb*sine
		zc := pc*cosine + pd*sine
		s := sqrt(za*za + zc*zc)
		if s > .00001 {
			s = 1 / s
		}
		za, zc = za*s, zc*s
		s = sqrt(za*za + zc*zc)
		if data.transform == transformNoScale && (pa*pd-pb*pc < 0) {
			s = -s
		}
		r := float32(math.Pi/2) + atan2(zc, za)
		zb, zd := cos(r)*s, sin(r)*s
		la, lb := cosDeg(shearX)*scaleX, cosDeg(90+shearY)*scaleY
		lc, ld := sinDeg(shearX)*scaleX, sinDeg(90+shearY)*scaleY
		b.world = affine{
			a: za*la + zb*lc, b: za*lb + zb*ld,
			c: zc*la + zd*lc, d: zc*lb + zd*ld,
			x: worldX, y: worldY,
		}
	default:
		rotationY := rotation + 90 + shearY
		la, lb := cosDeg(rotation+shearX)*scaleX, cosDeg(rotationY)*scaleY
		lc, ld := sinDeg(rotation+shearX)*scaleX, sinDeg(rotationY)*scaleY
		b.world = affine{
			a: pa*la + pb*lc, b: pa*lb + pb*ld,
			c: pc*la + pd*lc, d: pc*lb + pd*ld,
			x: worldX, y: worldY,
		}
	}
}

func (p *Player) ensureApplied(index int) {
	if p.pose.bones[index].appliedValid {
		return
	}
	p.updateApplied(index)
}

func (p *Player) updateApplied(index int) {
	data := p.Resource.bones[index]
	b := &p.pose.bones[index]
	b.appliedValid = true
	if data.parent < 0 {
		b.ax, b.ay = b.world.x, b.world.y
		b.arotation = atan2Deg(b.world.c, b.world.a)
		b.ascaleX = sqrt(b.world.a*b.world.a + b.world.c*b.world.c)
		b.ascaleY = sqrt(b.world.b*b.world.b + b.world.d*b.world.d)
		b.ashearX = 0
		b.ashearY = atan2Deg(b.world.a*b.world.b+b.world.c*b.world.d, b.world.a*b.world.d-b.world.b*b.world.c)
		return
	}
	parent := p.pose.bones[data.parent].world
	pid := float32(1) / (parent.a*parent.d - parent.b*parent.c)
	dx, dy := b.world.x-parent.x, b.world.y-parent.y
	b.ax = (dx*parent.d - dy*parent.b) * pid
	b.ay = (dy*parent.a - dx*parent.c) * pid
	ia, id, ib, ic := pid*parent.d, pid*parent.a, pid*parent.b, pid*parent.c
	ra := ia*b.world.a - ib*b.world.c
	rb := ia*b.world.b - ib*b.world.d
	rc := id*b.world.c - ic*b.world.a
	rd := id*b.world.d - ic*b.world.b
	b.ashearX = 0
	b.ascaleX = sqrt(ra*ra + rc*rc)
	if b.ascaleX > .0001 {
		determinant := ra*rd - rb*rc
		b.ascaleY = determinant / b.ascaleX
		b.ashearY = atan2Deg(ra*rb+rc*rd, determinant)
		b.arotation = atan2Deg(rc, ra)
		return
	}
	b.ascaleX = 0
	b.ascaleY = sqrt(rb*rb + rd*rd)
	b.ashearY = 0
	b.arotation = 90 - atan2Deg(rd, rb)
}

func (p *Player) applyTransform(index int) {
	data := p.Resource.transforms[index]
	state := p.pose.transforms[index]
	if data.local {
		if data.relative {
			p.applyRelativeLocalTransform(data, state)
		} else {
			p.applyAbsoluteLocalTransform(data, state)
		}
		return
	}
	if data.relative {
		p.applyRelativeWorldTransform(data, state)
	} else {
		p.applyAbsoluteWorldTransform(data, state)
	}
}

func (p *Player) applyAbsoluteWorldTransform(data transformConstraintData, state transformPose) {
	target := p.pose.bones[data.target].world
	offsetRotation, offsetShearY := data.offsetRotation*degToRad, data.offsetShearY*degToRad
	if target.a*target.d-target.b*target.c < 0 {
		offsetRotation, offsetShearY = -offsetRotation, -offsetShearY
	}
	for _, index := range data.bones {
		bone := &p.pose.bones[index]
		modified := false
		if state.rotateMix != 0 {
			a, b, c, d := bone.world.a, bone.world.b, bone.world.c, bone.world.d
			r := wrapRadians(atan2(target.c, target.a)-atan2(c, a)+offsetRotation) * state.rotateMix
			cosine, sine := cos(r), sin(r)
			bone.world.a, bone.world.b = cosine*a-sine*c, cosine*b-sine*d
			bone.world.c, bone.world.d = sine*a+cosine*c, sine*b+cosine*d
			modified = true
		}
		if state.translateMix != 0 {
			x, y := transform(target, data.offsetX, data.offsetY)
			bone.world.x += (x - bone.world.x) * state.translateMix
			bone.world.y += (y - bone.world.y) * state.translateMix
			modified = true
		}
		if state.scaleMix > 0 {
			s := sqrt(bone.world.a*bone.world.a + bone.world.c*bone.world.c)
			targetScale := sqrt(target.a*target.a + target.c*target.c)
			if s > .00001 {
				s = (s + (targetScale-s+data.offsetScaleX)*state.scaleMix) / s
			}
			bone.world.a, bone.world.c = bone.world.a*s, bone.world.c*s
			s = sqrt(bone.world.b*bone.world.b + bone.world.d*bone.world.d)
			targetScale = sqrt(target.b*target.b + target.d*target.d)
			if s > .00001 {
				s = (s + (targetScale-s+data.offsetScaleY)*state.scaleMix) / s
			}
			bone.world.b, bone.world.d = bone.world.b*s, bone.world.d*s
			modified = true
		}
		if state.shearMix > 0 {
			b, d := bone.world.b, bone.world.d
			boneY := atan2(d, b)
			r := atan2(target.d, target.b) - atan2(target.c, target.a) - (boneY - atan2(bone.world.c, bone.world.a))
			r = boneY + (wrapRadians(r)+offsetShearY)*state.shearMix
			s := sqrt(b*b + d*d)
			bone.world.b, bone.world.d = cos(r)*s, sin(r)*s
			modified = true
		}
		if modified {
			bone.appliedValid = false
		}
	}
}

func (p *Player) applyRelativeWorldTransform(data transformConstraintData, state transformPose) {
	target := p.pose.bones[data.target].world
	offsetRotation, offsetShearY := data.offsetRotation*degToRad, data.offsetShearY*degToRad
	if target.a*target.d-target.b*target.c < 0 {
		offsetRotation, offsetShearY = -offsetRotation, -offsetShearY
	}
	for _, index := range data.bones {
		bone := &p.pose.bones[index]
		modified := false
		if state.rotateMix != 0 {
			a, b, c, d := bone.world.a, bone.world.b, bone.world.c, bone.world.d
			r := wrapRadians(atan2(target.c, target.a)+offsetRotation) * state.rotateMix
			cosine, sine := cos(r), sin(r)
			bone.world.a, bone.world.b = cosine*a-sine*c, cosine*b-sine*d
			bone.world.c, bone.world.d = sine*a+cosine*c, sine*b+cosine*d
			modified = true
		}
		if state.translateMix != 0 {
			x, y := transform(target, data.offsetX, data.offsetY)
			bone.world.x += x * state.translateMix
			bone.world.y += y * state.translateMix
			modified = true
		}
		if state.scaleMix > 0 {
			s := (sqrt(target.a*target.a+target.c*target.c)-1+data.offsetScaleX)*state.scaleMix + 1
			bone.world.a, bone.world.c = bone.world.a*s, bone.world.c*s
			s = (sqrt(target.b*target.b+target.d*target.d)-1+data.offsetScaleY)*state.scaleMix + 1
			bone.world.b, bone.world.d = bone.world.b*s, bone.world.d*s
			modified = true
		}
		if state.shearMix > 0 {
			r := wrapRadians(atan2(target.d, target.b) - atan2(target.c, target.a))
			b, d := bone.world.b, bone.world.d
			r = atan2(d, b) + (r-float32(math.Pi/2)+offsetShearY)*state.shearMix
			s := sqrt(b*b + d*d)
			bone.world.b, bone.world.d = cos(r)*s, sin(r)*s
			modified = true
		}
		if modified {
			bone.appliedValid = false
		}
	}
}

func (p *Player) applyAbsoluteLocalTransform(data transformConstraintData, state transformPose) {
	p.ensureApplied(data.target)
	target := p.pose.bones[data.target]
	for _, index := range data.bones {
		p.ensureApplied(index)
		bone := p.pose.bones[index]
		rotation := bone.arotation
		if state.rotateMix != 0 {
			rotation += wrapDegrees(target.arotation-rotation+data.offsetRotation) * state.rotateMix
		}
		x, y := bone.ax, bone.ay
		if state.translateMix != 0 {
			x += (target.ax - x + data.offsetX) * state.translateMix
			y += (target.ay - y + data.offsetY) * state.translateMix
		}
		scaleX, scaleY := bone.ascaleX, bone.ascaleY
		if state.scaleMix != 0 {
			if scaleX > .00001 {
				scaleX = (scaleX + (target.ascaleX-scaleX+data.offsetScaleX)*state.scaleMix) / scaleX
			}
			if scaleY > .00001 {
				scaleY = (scaleY + (target.ascaleY-scaleY+data.offsetScaleY)*state.scaleMix) / scaleY
			}
		}
		shearY := bone.ashearY
		if state.shearMix != 0 {
			shearY += wrapDegrees(target.ashearY-shearY+data.offsetShearY) * state.shearMix
		}
		p.updateWorldWith(index, x, y, rotation, scaleX, scaleY, bone.ashearX, shearY)
	}
}

func (p *Player) applyRelativeLocalTransform(data transformConstraintData, state transformPose) {
	p.ensureApplied(data.target)
	target := p.pose.bones[data.target]
	for _, index := range data.bones {
		p.ensureApplied(index)
		bone := p.pose.bones[index]
		rotation := bone.arotation + (target.arotation+data.offsetRotation)*state.rotateMix
		x := bone.ax + (target.ax+data.offsetX)*state.translateMix
		y := bone.ay + (target.ay+data.offsetY)*state.translateMix
		scaleX := bone.ascaleX * ((target.ascaleX-1+data.offsetScaleX)*state.scaleMix + 1)
		scaleY := bone.ascaleY * ((target.ascaleY-1+data.offsetScaleY)*state.scaleMix + 1)
		shearY := bone.ashearY + (target.ashearY+data.offsetShearY)*state.shearMix
		p.updateWorldWith(index, x, y, rotation, scaleX, scaleY, bone.ashearX, shearY)
	}
}

func wrapRadians(value float32) float32 {
	if value > math.Pi {
		return value - 2*math.Pi
	}
	if value < -math.Pi {
		return value + 2*math.Pi
	}
	return value
}

func (p *Player) applyPath(index int) {
	data, state := p.Resource.paths[index], p.pose.paths[index]
	slot := p.pose.slots[data.target]
	path := skinAttachment(p.Resource.skins[p.Skin], p.Resource.skins["default"], data.target, slot.attachment)
	if path == nil || path.kind != "path" || (state.rotateMix == 0 && state.translateMix == 0) {
		return
	}

	percentSpacing := data.spacingMode == "percent"
	tangents := data.rotateMode == "tangent"
	scale := data.rotateMode == "chainScale"
	spacesCount := len(data.bones) + 1
	if tangents {
		spacesCount--
	}
	spaces := resizeSlice(p.path.spaces, spacesCount)
	lengths := resizeSlice(p.path.lengths, len(data.bones))
	clear(spaces)
	clear(lengths)
	p.path.spaces, p.path.lengths = spaces, lengths
	if scale || !percentSpacing {
		lengthSpacing := data.spacingMode == "length"
		for i := 0; i < spacesCount-1; i++ {
			bone := p.pose.bones[data.bones[i]]
			setupLength := p.Resource.bones[data.bones[i]].length
			if setupLength < 0.00001 {
				spaces[i+1] = 0
				continue
			}
			worldLength := hypot(setupLength*bone.world.a, setupLength*bone.world.c)
			if scale {
				lengths[i] = worldLength
			}
			if percentSpacing {
				spaces[i+1] = state.spacing
			} else {
				spacing := state.spacing
				if lengthSpacing {
					spacing += setupLength
				}
				spaces[i+1] = spacing * worldLength / setupLength
			}
		}
	} else {
		for i := 1; i < spacesCount; i++ {
			spaces[i] = state.spacing
		}
	}

	positions := p.computePathWorldPositions(data.target, path, spaces, state.position, tangents, data.positionMode == "percent", percentSpacing)
	if len(positions) < len(data.bones)*3+2 {
		return
	}
	boneX, boneY := positions[0], positions[1]
	offsetRotation := data.offsetRotation
	tip := offsetRotation == 0 && data.rotateMode == "chain"
	if offsetRotation != 0 {
		target := p.pose.bones[p.Resource.slots[data.target].bone].world
		if target.a*target.d-target.b*target.c < 0 {
			offsetRotation = -offsetRotation
		}
		offsetRotation *= degToRad
	}
	for i, boneIndex := range data.bones {
		bone := &p.pose.bones[boneIndex]
		bone.world.x += (boneX - bone.world.x) * state.translateMix
		bone.world.y += (boneY - bone.world.y) * state.translateMix
		next := (i + 1) * 3
		x, y := positions[next], positions[next+1]
		dx, dy := x-boneX, y-boneY
		if scale && lengths[i] != 0 {
			s := (hypot(dx, dy)/lengths[i]-1)*state.rotateMix + 1
			bone.world.a *= s
			bone.world.c *= s
		}
		boneX, boneY = x, y
		if state.rotateMix != 0 {
			r := float32(0)
			if tangents {
				r = positions[next-1]
			} else if spaces[i+1] == 0 {
				r = positions[next+2]
			} else {
				r = atan2(dy, dx)
			}
			r -= atan2(bone.world.c, bone.world.a)
			if tip {
				cosine, sine := float32(math.Cos(float64(r))), float32(math.Sin(float64(r)))
				length := p.Resource.bones[boneIndex].length
				boneX += (length*(cosine*bone.world.a-sine*bone.world.c) - dx) * state.rotateMix
				boneY += (length*(sine*bone.world.a+cosine*bone.world.c) - dy) * state.rotateMix
			} else {
				r += offsetRotation
			}
			if r > math.Pi {
				r -= 2 * math.Pi
			} else if r < -math.Pi {
				r += 2 * math.Pi
			}
			r *= state.rotateMix
			cosine, sine := float32(math.Cos(float64(r))), float32(math.Sin(float64(r)))
			a, b, c, d := bone.world.a, bone.world.b, bone.world.c, bone.world.d
			bone.world.a, bone.world.b = cosine*a-sine*c, cosine*b-sine*d
			bone.world.c, bone.world.d = sine*a+cosine*c, sine*b+cosine*d
		}
		bone.appliedValid = false
	}
}

func (p *Player) computePathWorldPositions(slotIndex int, path *attachment, spaces []float32, position float32, tangents, percentPosition, percentSpacing bool) []float32 {
	const none, before, after = -1, -2, -3
	verticesLength := path.vertexCount * 2
	curveCount := verticesLength / 6
	out := resizeSlice(p.path.positions, len(spaces)*3+2)
	clear(out)
	p.path.positions = out
	worldVertices := p.vertexAttachmentWorldVerticesInto(p.path.worldVertices, slotIndex, path, path.vertexCount)
	p.path.worldVertices = worldVertices
	if len(worldVertices) < verticesLength || curveCount == 0 {
		return nil
	}

	if !path.constantSpeed {
		if path.closed {
			curveCount--
		} else {
			curveCount -= 2
		}
		if curveCount < 0 || curveCount >= len(path.lengths) {
			return nil
		}
		pathLength := path.lengths[curveCount]
		if percentPosition {
			position *= pathLength
		}
		if percentSpacing {
			for i := 1; i < len(spaces); i++ {
				spaces[i] *= pathLength
			}
		}
		world := resizeSlice(p.path.world, 8)
		clear(world)
		p.path.world = world
		previousCurve, curve := none, 0
		for i, space := range spaces {
			o := i * 3
			position += space
			point := position
			if path.closed {
				point = modPositive(point, pathLength)
				curve = 0
			} else if point < 0 {
				if previousCurve != before {
					previousCurve = before
					copy(world, worldVertices[2:6])
				}
				addBeforePathPosition(point, world, 0, out, o)
				continue
			} else if point > pathLength {
				if previousCurve != after {
					previousCurve = after
					copy(world, worldVertices[verticesLength-6:verticesLength-2])
				}
				addAfterPathPosition(point-pathLength, world, 0, out, o)
				continue
			}
			for curve < len(path.lengths) && point > path.lengths[curve] {
				curve++
			}
			if curve >= len(path.lengths) {
				return nil
			}
			if curve == 0 {
				point /= path.lengths[curve]
			} else {
				previous := path.lengths[curve-1]
				point = (point - previous) / (path.lengths[curve] - previous)
			}
			if curve != previousCurve {
				previousCurve = curve
				if path.closed && curve == curveCount {
					copy(world[:4], worldVertices[verticesLength-4:verticesLength])
					copy(world[4:], worldVertices[:4])
				} else {
					start := curve*6 + 2
					copy(world, worldVertices[start:start+8])
				}
			}
			addCurvePathPosition(point, world, out, o, tangents || (i > 0 && space == 0))
		}
		return out
	}

	var world []float32
	if path.closed {
		verticesLength += 2
		world = resizeSlice(p.path.world, verticesLength)
		clear(world)
		copy(world, worldVertices[2:])
		copy(world[verticesLength-4:], worldVertices[:2])
		world[verticesLength-2], world[verticesLength-1] = world[0], world[1]
	} else {
		curveCount--
		verticesLength -= 4
		if curveCount <= 0 || verticesLength <= 0 {
			return nil
		}
		world = resizeSlice(p.path.world, verticesLength)
		copy(world, worldVertices[2:2+verticesLength])
	}
	p.path.world = world

	curves := resizeSlice(p.path.curves, curveCount)
	p.path.curves = curves
	pathLength := float32(0)
	x1, y1 := world[0], world[1]
	for curve, w := 0, 2; curve < curveCount; curve, w = curve+1, w+6 {
		cx1, cy1, cx2, cy2, x2, y2 := world[w], world[w+1], world[w+2], world[w+3], world[w+4], world[w+5]
		tmpX, tmpY := (x1-cx1*2+cx2)*0.1875, (y1-cy1*2+cy2)*0.1875
		dddfX, dddfY := ((cx1-cx2)*3-x1+x2)*0.09375, ((cy1-cy2)*3-y1+y2)*0.09375
		ddfX, ddfY := tmpX*2+dddfX, tmpY*2+dddfY
		dfX := (cx1-x1)*0.75 + tmpX + dddfX*0.16666667
		dfY := (cy1-y1)*0.75 + tmpY + dddfY*0.16666667
		pathLength += hypot(dfX, dfY)
		dfX, dfY = dfX+ddfX, dfY+ddfY
		ddfX, ddfY = ddfX+dddfX, ddfY+dddfY
		pathLength += hypot(dfX, dfY)
		dfX, dfY = dfX+ddfX, dfY+ddfY
		pathLength += hypot(dfX, dfY)
		dfX, dfY = dfX+ddfX+dddfX, dfY+ddfY+dddfY
		pathLength += hypot(dfX, dfY)
		curves[curve] = pathLength
		x1, y1 = x2, y2
	}
	if pathLength == 0 {
		return nil
	}
	if percentPosition {
		position *= pathLength
	} else if len(path.lengths) >= curveCount && path.lengths[curveCount-1] != 0 {
		position *= pathLength / path.lengths[curveCount-1]
	}
	if percentSpacing {
		for i := 1; i < len(spaces); i++ {
			spaces[i] *= pathLength
		}
	}

	segments := resizeSlice(p.path.segments, 10)
	p.path.segments = segments
	previousCurve, curve, segment := none, 0, 0
	curveLength := float32(0)
	var cx1, cy1, cx2, cy2, x2, y2 float32
	for i, space := range spaces {
		o := i * 3
		position += space
		point := position
		if path.closed {
			point = modPositive(point, pathLength)
			curve = 0
		} else if point < 0 {
			addBeforePathPosition(point, world, 0, out, o)
			continue
		} else if point > pathLength {
			addAfterPathPosition(point-pathLength, world, verticesLength-4, out, o)
			continue
		}
		for curve < len(curves) && point > curves[curve] {
			curve++
		}
		if curve >= len(curves) {
			return nil
		}
		if curve == 0 {
			point /= curves[curve]
		} else {
			previous := curves[curve-1]
			point = (point - previous) / (curves[curve] - previous)
		}
		if curve != previousCurve {
			previousCurve = curve
			w := curve * 6
			x1, y1 = world[w], world[w+1]
			cx1, cy1, cx2, cy2, x2, y2 = world[w+2], world[w+3], world[w+4], world[w+5], world[w+6], world[w+7]
			tmpX, tmpY := (x1-cx1*2+cx2)*0.03, (y1-cy1*2+cy2)*0.03
			dddfX, dddfY := ((cx1-cx2)*3-x1+x2)*0.006, ((cy1-cy2)*3-y1+y2)*0.006
			ddfX, ddfY := tmpX*2+dddfX, tmpY*2+dddfY
			dfX := (cx1-x1)*0.3 + tmpX + dddfX*0.16666667
			dfY := (cy1-y1)*0.3 + tmpY + dddfY*0.16666667
			curveLength = hypot(dfX, dfY)
			segments[0] = curveLength
			for ii := 1; ii < 8; ii++ {
				dfX, dfY = dfX+ddfX, dfY+ddfY
				ddfX, ddfY = ddfX+dddfX, ddfY+dddfY
				curveLength += hypot(dfX, dfY)
				segments[ii] = curveLength
			}
			dfX, dfY = dfX+ddfX, dfY+ddfY
			curveLength += hypot(dfX, dfY)
			segments[8] = curveLength
			dfX, dfY = dfX+ddfX+dddfX, dfY+ddfY+dddfY
			curveLength += hypot(dfX, dfY)
			segments[9] = curveLength
			segment = 0
		}
		point *= curveLength
		for segment < len(segments) && point > segments[segment] {
			segment++
		}
		if segment >= len(segments) {
			return nil
		}
		if segment == 0 {
			point /= segments[segment]
		} else {
			previous := segments[segment-1]
			point = float32(segment) + (point-previous)/(segments[segment]-previous)
		}
		curveWorld := [8]float32{x1, y1, cx1, cy1, cx2, cy2, x2, y2}
		addCurvePathPosition(point*0.1, curveWorld[:], out, o, tangents || (i > 0 && space == 0))
	}
	return out
}

func addBeforePathPosition(distance float32, world []float32, start int, out []float32, offset int) {
	x1, y1 := world[start], world[start+1]
	r := atan2(world[start+3]-y1, world[start+2]-x1)
	out[offset], out[offset+1], out[offset+2] = x1+distance*cosRad(r), y1+distance*sinRad(r), r
}

func addAfterPathPosition(distance float32, world []float32, start int, out []float32, offset int) {
	x1, y1 := world[start+2], world[start+3]
	r := atan2(y1-world[start+1], x1-world[start])
	out[offset], out[offset+1], out[offset+2] = x1+distance*cosRad(r), y1+distance*sinRad(r), r
}

func addCurvePathPosition(percent float32, world, out []float32, offset int, tangents bool) {
	x1, y1, cx1, cy1, cx2, cy2, x2, y2 := world[0], world[1], world[2], world[3], world[4], world[5], world[6], world[7]
	if percent == 0 || math.IsNaN(float64(percent)) {
		out[offset], out[offset+1], out[offset+2] = x1, y1, atan2(cy1-y1, cx1-x1)
		return
	}
	tt, ttt := percent*percent, percent*percent*percent
	u := 1 - percent
	uu, uuu := u*u, u*u*u
	ut3 := u * percent * 3
	uut3, utt3 := u*ut3, ut3*percent
	x := x1*uuu + cx1*uut3 + cx2*utt3 + x2*ttt
	y := y1*uuu + cy1*uut3 + cy2*utt3 + y2*ttt
	out[offset], out[offset+1] = x, y
	if tangents {
		if percent < 0.001 {
			out[offset+2] = atan2(cy1-y1, cx1-x1)
		} else {
			out[offset+2] = atan2(y-(y1*uu+cy1*u*percent*2+cy2*tt), x-(x1*uu+cx1*u*percent*2+cx2*tt))
		}
	}
}

func hypot(x, y float32) float32   { return float32(math.Hypot(float64(x), float64(y))) }
func cosRad(value float32) float32 { return float32(math.Cos(float64(value))) }
func sinRad(value float32) float32 { return float32(math.Sin(float64(value))) }
func modPositive(value, divisor float32) float32 {
	if divisor == 0 {
		return 0
	}
	value = float32(math.Mod(float64(value), float64(divisor)))
	if value < 0 {
		value += divisor
	}
	return value
}

func (p *Player) applyIK(index int) {
	data, state := p.Resource.ik[index], p.pose.ik[index]
	if len(data.bones) == 0 {
		return
	}
	target := p.pose.bones[data.target].world
	if len(data.bones) == 1 {
		p.applyIK1(data.bones[0], target.x, target.y, state.compress, state.stretch, data.uniform, state.mix)
		return
	}
	parent, child := data.bones[0], data.bones[len(data.bones)-1]
	p.applyIK2(parent, child, target.x, target.y, state.bendDirection, state.stretch, state.mix)
}

func (p *Player) applyIK1(index int, targetX, targetY float32, compress, stretch, uniform bool, alpha float32) {
	data := p.Resource.bones[index]
	if data.parent < 0 {
		return
	}
	p.ensureApplied(index)
	bone := p.pose.bones[index]
	parent := p.pose.bones[data.parent].world
	id := float32(1) / (parent.a*parent.d - parent.b*parent.c)
	x, y := targetX-parent.x, targetY-parent.y
	tx := (x*parent.d-y*parent.b)*id - bone.ax
	ty := (y*parent.a-x*parent.c)*id - bone.ay
	rotationIK := atan2Deg(ty, tx) - bone.ashearX - bone.arotation
	if bone.ascaleX < 0 {
		rotationIK += 180
	}
	rotationIK = wrapDegrees(rotationIK)
	scaleX, scaleY := bone.ascaleX, bone.ascaleY
	if compress || stretch {
		length, distance := data.length*scaleX, sqrt(tx*tx+ty*ty)
		if (compress && distance < length) || (stretch && distance > length && length > .0001) {
			scale := (distance/length-1)*alpha + 1
			scaleX *= scale
			if uniform {
				scaleY *= scale
			}
		}
	}
	p.updateWorldWith(index, bone.ax, bone.ay, bone.arotation+rotationIK*alpha, scaleX, scaleY, bone.ashearX, bone.ashearY)
}

func (p *Player) applyIK2(parentIndex, childIndex int, targetX, targetY float32, bendDirection int, stretch bool, alpha float32) {
	if alpha == 0 {
		p.updateWorldBone(childIndex)
		return
	}
	p.ensureApplied(parentIndex)
	p.ensureApplied(childIndex)
	parent, child := p.pose.bones[parentIndex], p.pose.bones[childIndex]
	parentData, childData := p.Resource.bones[parentIndex], p.Resource.bones[childIndex]
	px, py := parent.ax, parent.ay
	psx, scaleX, psy, csx := parent.ascaleX, parent.ascaleX, parent.ascaleY, child.ascaleX
	o1, o2, s2 := float32(0), float32(0), float32(1)
	if psx < 0 {
		psx, o1, s2 = -psx, 180, -1
	}
	if psy < 0 {
		psy, s2 = -psy, -s2
	}
	if csx < 0 {
		csx, o2 = -csx, 180
	}
	cx := child.ax
	uniformScale := abs(psx-psy) <= .0001
	cy := float32(0)
	cwx := parent.world.a*cx + parent.world.x
	cwy := parent.world.c*cx + parent.world.y
	if uniformScale {
		cy = child.ay
		cwx = parent.world.a*cx + parent.world.b*cy + parent.world.x
		cwy = parent.world.c*cx + parent.world.d*cy + parent.world.y
	}
	grandParent := affine{a: 1, d: 1}
	if parentData.parent >= 0 {
		grandParent = p.pose.bones[parentData.parent].world
	}
	id := float32(1) / (grandParent.a*grandParent.d - grandParent.b*grandParent.c)
	x, y := targetX-grandParent.x, targetY-grandParent.y
	tx := (x*grandParent.d-y*grandParent.b)*id - px
	ty := (y*grandParent.a-x*grandParent.c)*id - py
	distanceSquared := tx*tx + ty*ty
	x, y = cwx-grandParent.x, cwy-grandParent.y
	dx := (x*grandParent.d-y*grandParent.b)*id - px
	dy := (y*grandParent.a-x*grandParent.c)*id - py
	l1 := sqrt(dx*dx + dy*dy)
	l2 := childData.length * csx
	var a1, a2 float32
	if uniformScale {
		l2 *= psx
		cosine := (distanceSquared - l1*l1 - l2*l2) / (2 * l1 * l2)
		cosine = min(max(cosine, -1), 1)
		if cosine == 1 && stretch && l1+l2 > .0001 {
			scaleX *= (sqrt(distanceSquared)/(l1+l2)-1)*alpha + 1
		}
		a2 = acos(cosine) * float32(bendDirection)
		a, b := l1+l2*cosine, l2*sin(a2)
		a1 = atan2(ty*a-tx*b, tx*a+ty*b)
	} else {
		a, b := psx*l2, psy*l2
		aa, bb, ll := a*a, b*b, l1*l1
		targetAngle := atan2(ty, tx)
		c0, c1, c2 := bb*ll+aa*distanceSquared-aa*bb, -2*bb*l1, bb-aa
		discriminant := c1*c1 - 4*c2*c0
		solved := false
		if discriminant >= 0 {
			q := sqrt(discriminant)
			if c1 < 0 {
				q = -q
			}
			q = -(c1 + q) / 2
			r0, r1 := q/c2, c0/q
			r := r1
			if abs(r0) < abs(r1) {
				r = r0
			}
			if r*r <= distanceSquared {
				y := sqrt(distanceSquared-r*r) * float32(bendDirection)
				a1 = targetAngle - atan2(y, r)
				a2 = atan2(y/psy, (r-l1)/psx)
				solved = true
			}
		}
		if !solved {
			minAngle, minX, minY := float32(math.Pi), l1-a, float32(0)
			maxAngle, maxX, maxY := float32(0), l1+a, float32(0)
			minDistance, maxDistance := minX*minX, maxX*maxX
			c0 = -a * l1 / (aa - bb)
			if c0 >= -1 && c0 <= 1 {
				angle := acos(c0)
				x, y := a*cos(angle)+l1, b*sin(angle)
				distance := x*x + y*y
				if distance < minDistance {
					minAngle, minDistance, minX, minY = angle, distance, x, y
				}
				if distance > maxDistance {
					maxAngle, maxDistance, maxX, maxY = angle, distance, x, y
				}
			}
			if distanceSquared <= (minDistance+maxDistance)/2 {
				a1, a2 = targetAngle-atan2(minY*float32(bendDirection), minX), minAngle*float32(bendDirection)
			} else {
				a1, a2 = targetAngle-atan2(maxY*float32(bendDirection), maxX), maxAngle*float32(bendDirection)
			}
		}
	}
	offset := atan2(cy, cx) * s2
	a1 = (a1-offset)*radToDeg + o1 - parent.arotation
	p.updateWorldWith(parentIndex, px, py, parent.arotation+wrapDegrees(a1)*alpha, scaleX, parent.ascaleY, 0, 0)
	a2 = ((a2+offset)*radToDeg-child.ashearX)*s2 + o2 - child.arotation
	p.updateWorldWith(childIndex, cx, cy, child.arotation+wrapDegrees(a2)*alpha, child.ascaleX, child.ascaleY, child.ashearX, child.ashearY)
}

func (p *Player) applyDeform(timeline deformTimeline, time float32) {
	slot := &p.pose.slots[timeline.slot]
	active := skinAttachment(p.Resource.skins[p.Skin], p.Resource.skins["default"], timeline.slot, slot.attachment)
	if active == nil || active != timeline.attachment && (!active.inheritDeform || active.parentMesh != timeline.attachment) {
		return
	}
	deform, ok := sampleDeformInto(slot.deform, timeline.frames, time)
	if ok {
		slot.deform = deform
	}
}

func transform(a affine, x, y float32) (float32, float32) {
	return a.a*x + a.b*y + a.x, a.c*x + a.d*y + a.y
}

type drawableGeometry struct {
	attach    *attachment
	positions []float32
	uvs       []float32
	indices   []uint32
	slot      slotPose
	slotIndex int
}

func (p *Player) forEachDrawable(yield func(drawableGeometry)) {
	skin, fallback := p.Resource.skins[p.Skin], p.Resource.skins["default"]
	workspace := &p.mesh
	clipper := &workspace.clipper
	clipper.reset()
	for _, slotIndex := range p.pose.drawOrder {
		slot := p.pose.slots[slotIndex]
		a := skinAttachment(skin, fallback, slotIndex, slot.attachment)
		if a == nil {
			clipper.endWithSlot(slotIndex)
			continue
		}
		if a.kind == "clipping" {
			workspace.positions = p.vertexAttachmentWorldVerticesInto(workspace.positions, slotIndex, a, a.vertexCount)
			clipper.start(workspace.positions, a.endSlot)
			continue
		}
		region, ok := p.Resource.atlas[a.path]
		if !ok {
			region, ok = p.Resource.atlas[a.name]
		}
		if ok {
			var indices []uint32
			workspace.positions, workspace.uvs, indices = p.attachmentGeometryInto(workspace.positions, workspace.uvs, slotIndex, a, region)
			positions, uvs := workspace.positions, workspace.uvs
			if clipper.active() {
				positions, uvs, indices = clipper.clipTriangles(positions, uvs, indices)
			}
			if len(indices) > 0 {
				yield(drawableGeometry{slotIndex: slotIndex, slot: slot, attach: a, positions: positions, uvs: uvs, indices: indices})
			}
		}
		clipper.endWithSlot(slotIndex)
	}
}

func (p *Player) MeshBatch(projection, model mgl32.Mat4) render.MeshBatch3D {
	var batch render.MeshBatch3D
	p.MeshBatchInto(&batch, projection, model)
	return batch
}

// MeshBatchInto writes the current mesh into batch while retaining its slice
// capacity and returns the generated local-space bounds.
func (p *Player) MeshBatchInto(batch *render.MeshBatch3D, projection, model mgl32.Mat4) (minX, minY, maxX, maxY float32, valid bool) {
	if batch == nil {
		return 0, 0, 0, 0, false
	}
	initial := render.MeshBatch3D{ViewProjection: [16]float32(projection), CameraPosition: [4]float32{0, 0, 1, 1}, AmbientLight: [4]float32{1, 1, 1, 1}}
	vertices, indices := batch.Vertices[:0], batch.Indices[:0]
	instances, draws := batch.Instances[:0], batch.Draws[:0]
	*batch = initial
	batch.Vertices, batch.Indices = vertices, indices
	batch.Instances, batch.Draws = instances, draws
	return p.appendMeshBatchInto(batch, projection, model, false)
}

// AppendMeshBatchInto appends directly to a shared batch, baking the model into
// positions so adjacent players can use one identity instance. Bounds stay local.
func (p *Player) AppendMeshBatchInto(batch *render.MeshBatch3D, projection, model mgl32.Mat4) (minX, minY, maxX, maxY float32, valid bool) {
	if batch == nil {
		return 0, 0, 0, 0, false
	}
	return p.appendMeshBatchInto(batch, projection, model, true)
}

func (p *Player) appendMeshBatchInto(batch *render.MeshBatch3D, projection, model mgl32.Mat4, bake bool) (minX, minY, maxX, maxY float32, valid bool) {
	if p == nil || p.Alpha <= 0 || p.Resource == nil || p.pose == nil || p.Resource.Texture == nil {
		return 0, 0, 0, 0, false
	}
	texture := p.Resource.Texture
	instanceModel := model
	if bake {
		instanceModel = mgl32.Ident4()
	}
	if len(batch.Instances) == 0 {
		batch.ViewProjection = [16]float32(projection)
		batch.CameraPosition = [4]float32{0, 0, 1, 1}
		batch.AmbientLight = [4]float32{1, 1, 1, 1}
	}
	p.forEachDrawable(func(geometry drawableGeometry) {
		color := multiplyColor(geometry.slot.color, geometry.attach.color)
		color[3] *= p.Alpha
		if color[3] <= 0 {
			return
		}
		if len(batch.Instances) == 0 {
			batch.Instances = append(batch.Instances, render.Instance3D{
				Model:       [16]float32(instanceModel),
				BaseColor:   [4]float32{1, 1, 1, 1},
				Material:    [4]float32{-2, 0, 0, 2},
				UVTransform: [4]float32{1, 1, 0, 0},
				Custom:      [4][4]float32{{0, 0, 0, boolFloat(texture.PremultipliedAlpha())}},
			})
		}
		baseVertex := int32(len(batch.Vertices))
		firstIndex := uint32(len(batch.Indices))
		encodedLight := packRGB(color[0], color[1], color[2])
		encodedDark := packRGB(geometry.slot.dark[0], geometry.slot.dark[1], geometry.slot.dark[2])
		for i := 0; i < len(geometry.positions); i += 2 {
			x, y := geometry.positions[i], geometry.positions[i+1]
			if !valid {
				minX, minY, maxX, maxY, valid = x, y, x, y, true
			} else {
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
			position := mgl32.Vec4{x, y, meshDepth, 1}
			if bake {
				position = model.Mul4x1(position)
			}
			batch.Vertices = append(batch.Vertices, render.Vertex3D{
				Position: [3]float32{position[0], position[1], position[2]},
				Normal:   [3]float32{encodedLight, color[3], encodedDark},
				UV:       [2]float32{geometry.uvs[i], geometry.uvs[i+1]},
			})
		}
		for _, index := range geometry.indices {
			batch.Indices = append(batch.Indices, uint32(baseVertex)+index)
		}
		draw := render.MeshDraw3D{
			Texture: texture.RenderHandle(), FirstIndex: firstIndex, IndexCount: uint32(len(geometry.indices)),
			FirstInstance: 0, InstanceCount: 1, AlphaMode: 2, BlendMode: p.Resource.slots[geometry.slotIndex].blend,
		}
		if len(batch.Draws) != 0 {
			previous := &batch.Draws[len(batch.Draws)-1]
			if previous.Texture == draw.Texture && previous.BlendMode == draw.BlendMode &&
				previous.FirstIndex+previous.IndexCount == draw.FirstIndex {
				previous.IndexCount += draw.IndexCount
				return
			}
		}
		batch.Draws = append(batch.Draws, draw)
	})
	return minX, minY, maxX, maxY, valid
}

func packRGB(red, green, blue float32) float32 {
	pack := func(value float32) uint32 {
		return uint32(min(max(value, 0), 1)*255 + 0.5)
	}
	return float32(pack(red) | pack(green)<<8 | pack(blue)<<16)
}

func boolFloat(value bool) float32 {
	if value {
		return 1
	}
	return 0
}

func (p *Player) Bounds() (float32, float32, float32, float32, bool) {
	if p == nil || p.Resource == nil || p.pose == nil {
		return 0, 0, 0, 0, false
	}
	var minX, minY, maxX, maxY float32
	found := false
	p.forEachDrawable(func(geometry drawableGeometry) {
		for i := 0; i+1 < len(geometry.positions); i += 2 {
			x, y := geometry.positions[i], geometry.positions[i+1]
			if !found {
				minX, minY, maxX, maxY, found = x, y, x, y, true
			} else {
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
		}
	})
	return minX, minY, maxX, maxY, found
}

// AnimationBounds returns stable bounds for the player's current animation and skin.
// The bounds cover the complete animation instead of only the current pose.
func (p *Player) AnimationBounds() (float32, float32, float32, float32, bool) {
	if p == nil || p.Resource == nil || p.pose == nil {
		return 0, 0, 0, 0, false
	}
	animation, ok := p.Resource.animations[p.Animation]
	if !ok {
		return p.Bounds()
	}
	key := animationBoundsKey{animation: p.Animation, skin: p.Skin}
	if bounds, ok := p.bounds[key]; ok {
		return bounds.minX, bounds.minY, bounds.maxX, bounds.maxY, bounds.valid
	}

	sampler := NewPlayer(p.Resource)
	if err := sampler.SetSkin(p.Skin); err != nil {
		return 0, 0, 0, 0, false
	}
	if err := sampler.SetAnimation(p.Animation, false); err != nil {
		return 0, 0, 0, 0, false
	}
	times := animationSampleTimes(animation)
	bounds := animationBounds{}
	for _, time := range times {
		sampler.time = time
		sampler.evaluate()
		minX, minY, maxX, maxY, valid := sampler.Bounds()
		if !valid {
			continue
		}
		bounds.include(minX, minY, maxX, maxY)
	}
	if p.bounds == nil {
		p.bounds = make(map[animationBoundsKey]animationBounds)
	}
	p.bounds[key] = bounds
	return bounds.minX, bounds.minY, bounds.maxX, bounds.maxY, bounds.valid
}

func (b *animationBounds) include(minX, minY, maxX, maxY float32) {
	if !b.valid {
		b.minX, b.minY, b.maxX, b.maxY, b.valid = minX, minY, maxX, maxY, true
		return
	}
	b.minX, b.minY = min(b.minX, minX), min(b.minY, minY)
	b.maxX, b.maxY = max(b.maxX, maxX), max(b.maxY, maxY)
}

func animationSampleTimes(animation animation) []float32 {
	const samplesPerSecond = 60
	times := make(map[float32]struct{})
	add := func(time float32) {
		times[min(max(time, 0), animation.duration)] = struct{}{}
	}
	add(0)
	add(animation.duration)
	for frame := 1; frame < int(math.Ceil(float64(animation.duration*samplesPerSecond))); frame++ {
		add(float32(frame) / samplesPerSecond)
	}
	for _, timeline := range animation.bones {
		addTimelineTimes(times, animation.duration, timeline.frames)
	}
	for _, timeline := range animation.slots {
		addTimelineTimes(times, animation.duration, timeline.frames)
	}
	for _, timeline := range animation.deform {
		addTimelineTimes(times, animation.duration, timeline.frames)
	}
	for _, timeline := range animation.ik {
		addTimelineTimes(times, animation.duration, timeline.frames)
	}
	for _, timeline := range animation.transform {
		addTimelineTimes(times, animation.duration, timeline.frames)
	}
	for _, timeline := range animation.paths {
		addTimelineTimes(times, animation.duration, timeline.frames)
	}
	for _, frame := range animation.drawOrder {
		add(frame.time)
	}
	return mapKeys(times)
}

func addTimelineTimes[T interface{ timelineTime() float32 }](times map[float32]struct{}, duration float32, frames []T) {
	for _, frame := range frames {
		times[min(max(frame.timelineTime(), 0), duration)] = struct{}{}
	}
}

var regionAttachmentIndices = [...]uint32{0, 1, 2, 2, 3, 0}

func (p *Player) attachmentGeometryInto(dstPositions, dstUVs []float32, slotIndex int, a *attachment, r atlasRegion) ([]float32, []float32, []uint32) {
	bone := p.pose.bones[p.Resource.slots[slotIndex].bone].world
	if a.kind == "region" {
		w, h := a.width, a.height
		if w == 0 {
			w = float32(r.originalWidth)
		}
		if h == 0 {
			h = float32(r.originalHeight)
		}
		regionScaleX := w / float32(r.originalWidth) * a.scaleX
		regionScaleY := h / float32(r.originalHeight) * a.scaleY
		localX := -w/2*a.scaleX + float32(r.offsetX)*regionScaleX
		localY := -h/2*a.scaleY + float32(r.offsetY)*regionScaleY
		localX2 := localX + float32(r.width)*regionScaleX
		localY2 := localY + float32(r.height)*regionScaleY
		cosine, sine := cosDeg(a.rotation), sinDeg(a.rotation)
		localXCos, localXSin := localX*cosine+a.x, localX*sine
		localYCos, localYSin := localY*cosine+a.y, localY*sine
		localX2Cos, localX2Sin := localX2*cosine+a.x, localX2*sine
		localY2Cos, localY2Sin := localY2*cosine+a.y, localY2*sine
		offsets := [8]float32{
			localXCos - localYSin, localYCos + localXSin,
			localXCos - localY2Sin, localY2Cos + localXSin,
			localX2Cos - localY2Sin, localY2Cos + localX2Sin,
			localX2Cos - localYSin, localYCos + localX2Sin,
		}
		positions := resizeSlice(dstPositions, len(offsets))
		for i := 0; i < len(offsets); i += 2 {
			positions[i], positions[i+1] = transform(bone, offsets[i], offsets[i+1])
		}
		uvs := regionAttachmentUVsInto(dstUVs, r, p.Resource.Texture.Width, p.Resource.Texture.Height)
		return positions, uvs, regionAttachmentIndices[:]
	}
	positions := p.vertexAttachmentWorldVerticesInto(dstPositions, slotIndex, a, len(a.uvs)/2)
	uvs := meshAttachmentUVsInto(dstUVs, r, p.Resource.Texture.Width, p.Resource.Texture.Height, a.uvs)
	return positions, uvs, a.triangles
}

func (p *Player) vertexAttachmentWorldVerticesInto(dst []float32, slotIndex int, a *attachment, vertexCount int) []float32 {
	positions := resetSlice(dst, vertexCount*2)
	deform := p.pose.slots[slotIndex].deform
	if a.weighted {
		values := a.vertices
		cursor, deformCursor := 0, 0
		for range vertexCount {
			if cursor >= len(values) {
				break
			}
			count := int(values[cursor])
			cursor++
			x, y := float32(0), float32(0)
			for range count {
				if cursor+3 >= len(values) {
					return positions
				}
				bi := int(values[cursor])
				vx, vy, w := values[cursor+1], values[cursor+2], values[cursor+3]
				cursor += 4
				if deformCursor+1 < len(deform) {
					vx += deform[deformCursor]
					vy += deform[deformCursor+1]
				}
				deformCursor += 2
				if bi < 0 || bi >= len(p.pose.bones) {
					continue
				}
				wx, wy := transform(p.pose.bones[bi].world, vx, vy)
				x += wx * w
				y += wy * w
			}
			positions = append(positions, x, y)
		}
	} else {
		bone := p.pose.bones[p.Resource.slots[slotIndex].bone].world
		vertices := a.vertices
		if len(deform) == len(vertices) {
			vertices = deform
		}
		for i := 0; i+1 < len(vertices); i += 2 {
			x, y := transform(bone, vertices[i], vertices[i+1])
			positions = append(positions, x, y)
		}
	}
	return positions
}

type clipPoint struct{ x, y float32 }
type clipVertex struct{ x, y, u, v float32 }

type skeletonClipper struct {
	points           []clipPoint
	polygons         [][]clipPoint
	trianglePoints   []clipPoint
	remaining        []int
	clipInput        []clipVertex
	clipOutput       []clipVertex
	clippedPositions []float32
	clippedUVs       []float32
	clippedIndices   []uint32
	endSlot          int
	started          bool
}

func (c *skeletonClipper) active() bool { return c.started }

func (c *skeletonClipper) reset() {
	c.started = false
	c.endSlot = -1
	c.points = c.points[:0]
	c.polygons = c.polygons[:0]
	c.trianglePoints = c.trianglePoints[:0]
	c.remaining = c.remaining[:0]
	c.clipInput = c.clipInput[:0]
	c.clipOutput = c.clipOutput[:0]
	c.clippedPositions = c.clippedPositions[:0]
	c.clippedUVs = c.clippedUVs[:0]
	c.clippedIndices = c.clippedIndices[:0]
}

func (c *skeletonClipper) start(vertices []float32, endSlot int) {
	if c.started || len(vertices) < 6 {
		return
	}
	points := resetSlice(c.points, len(vertices)/2)
	for i := 0; i+1 < len(vertices); i += 2 {
		point := clipPoint{vertices[i], vertices[i+1]}
		if len(points) == 0 || !sameClipPoint(points[len(points)-1], point) {
			points = append(points, point)
		}
	}
	if len(points) > 2 && sameClipPoint(points[0], points[len(points)-1]) {
		points = points[:len(points)-1]
	}
	if len(points) < 3 {
		c.points = points
		return
	}
	c.points = points
	c.polygons = resetSlice(c.polygons, max(1, len(points)-2))
	c.trianglePoints = resetSlice(c.trianglePoints, max(0, len(points)-2)*3)
	c.started, c.endSlot = true, endSlot
	if convexClipPolygon(points) {
		c.polygons = append(c.polygons, points)
	} else {
		c.triangulateClipPolygon(points)
	}
}

func (c *skeletonClipper) endWithSlot(slot int) {
	if c.started && c.endSlot == slot {
		c.started = false
		c.endSlot = -1
	}
}

func (c *skeletonClipper) clipTriangles(positions, uvs []float32, indices []uint32) ([]float32, []float32, []uint32) {
	if !c.started {
		return positions, uvs, indices
	}
	clippedPositions := resetSlice(c.clippedPositions, len(positions))
	clippedUVs := resetSlice(c.clippedUVs, len(uvs))
	clippedIndices := resetSlice(c.clippedIndices, len(indices))
	for triangle := 0; triangle+2 < len(indices); triangle += 3 {
		var input [3]clipVertex
		valid := true
		for corner := range 3 {
			vertex := int(indices[triangle+corner]) * 2
			if vertex+1 >= len(positions) || vertex+1 >= len(uvs) {
				valid = false
				break
			}
			input[corner] = clipVertex{positions[vertex], positions[vertex+1], uvs[vertex], uvs[vertex+1]}
		}
		if !valid {
			continue
		}
		for _, polygon := range c.polygons {
			output := c.clipAgainstConvexPolygon(input[:], polygon)
			if len(output) < 3 {
				continue
			}
			first := uint32(len(clippedPositions) / 2)
			for _, vertex := range output {
				clippedPositions = append(clippedPositions, vertex.x, vertex.y)
				clippedUVs = append(clippedUVs, vertex.u, vertex.v)
			}
			for index := 1; index+1 < len(output); index++ {
				clippedIndices = append(clippedIndices, first, first+uint32(index), first+uint32(index+1))
			}
		}
	}
	c.clippedPositions = clippedPositions
	c.clippedUVs = clippedUVs
	c.clippedIndices = clippedIndices
	return clippedPositions, clippedUVs, clippedIndices
}

func sameClipPoint(a, b clipPoint) bool {
	const epsilon = float32(1e-5)
	return abs(a.x-b.x) <= epsilon && abs(a.y-b.y) <= epsilon
}

func clipCross(a, b, point clipPoint) float32 {
	return (b.x-a.x)*(point.y-a.y) - (b.y-a.y)*(point.x-a.x)
}

func clipPolygonOrientation(points []clipPoint) float32 {
	area := float32(0)
	for i, point := range points {
		next := points[(i+1)%len(points)]
		area += point.x*next.y - next.x*point.y
	}
	if area < 0 {
		return -1
	}
	return 1
}

func convexClipPolygon(points []clipPoint) bool {
	orientation := clipPolygonOrientation(points)
	for i := range points {
		previous := points[(i+len(points)-1)%len(points)]
		if clipCross(previous, points[i], points[(i+1)%len(points)])*orientation < -1e-5 {
			return false
		}
	}
	return true
}

func (c *skeletonClipper) triangulateClipPolygon(points []clipPoint) {
	orientation := clipPolygonOrientation(points)
	remaining := resizeSlice(c.remaining, len(points))
	for i := range remaining {
		remaining[i] = i
	}
	for len(remaining) > 3 {
		found := false
		for i, current := range remaining {
			previous := remaining[(i+len(remaining)-1)%len(remaining)]
			next := remaining[(i+1)%len(remaining)]
			a, b, d := points[previous], points[current], points[next]
			if clipCross(a, b, d)*orientation <= 1e-5 {
				continue
			}
			ear := true
			for _, candidate := range remaining {
				if candidate != previous && candidate != current && candidate != next && pointInClipTriangle(points[candidate], a, b, d, orientation) {
					ear = false
					break
				}
			}
			if !ear {
				continue
			}
			start := len(c.trianglePoints)
			c.trianglePoints = append(c.trianglePoints, a, b, d)
			c.polygons = append(c.polygons, c.trianglePoints[start:start+3])
			remaining = slices.Delete(remaining, i, i+1)
			found = true
			break
		}
		if !found {
			c.remaining = remaining
			return
		}
	}
	if len(remaining) == 3 {
		start := len(c.trianglePoints)
		c.trianglePoints = append(c.trianglePoints, points[remaining[0]], points[remaining[1]], points[remaining[2]])
		c.polygons = append(c.polygons, c.trianglePoints[start:start+3])
	}
	c.remaining = remaining
}

func pointInClipTriangle(point, a, b, c clipPoint, orientation float32) bool {
	return clipCross(a, b, point)*orientation >= -1e-5 &&
		clipCross(b, c, point)*orientation >= -1e-5 &&
		clipCross(c, a, point)*orientation >= -1e-5
}

func (c *skeletonClipper) clipAgainstConvexPolygon(input []clipVertex, polygon []clipPoint) []clipVertex {
	if len(input) < 3 || len(polygon) < 3 {
		return nil
	}
	orientation := clipPolygonOrientation(polygon)
	bufferCapacity := len(input) + len(polygon)
	previous := append(resetSlice(c.clipInput, bufferCapacity), input...)
	output := resetSlice(c.clipOutput, bufferCapacity)
	for edge, a := range polygon {
		b := polygon[(edge+1)%len(polygon)]
		output = output[:0]
		if len(previous) == 0 {
			break
		}
		start := previous[len(previous)-1]
		startDistance := clipCross(a, b, clipPoint{start.x, start.y}) * orientation
		for _, end := range previous {
			endDistance := clipCross(a, b, clipPoint{end.x, end.y}) * orientation
			startInside, endInside := startDistance >= -1e-5, endDistance >= -1e-5
			if startInside != endInside {
				mix := startDistance / (startDistance - endDistance)
				output = append(output, interpolateClipVertex(start, end, mix))
			}
			if endInside {
				output = append(output, end)
			}
			start, startDistance = end, endDistance
		}
		previous, output = output, previous
	}
	c.clipInput = previous
	c.clipOutput = output[:0]
	return c.clipInput
}

func interpolateClipVertex(a, b clipVertex, mix float32) clipVertex {
	return clipVertex{
		x: a.x + (b.x-a.x)*mix, y: a.y + (b.y-a.y)*mix,
		u: a.u + (b.u-a.u)*mix, v: a.v + (b.v-a.v)*mix,
	}
}

func regionAttachmentUVsInto(dst []float32, r atlasRegion, textureWidth, textureHeight int) []float32 {
	out := resizeSlice(dst, 8)
	u := float32(r.x) / float32(textureWidth)
	v := float32(r.y) / float32(textureHeight)
	if r.rotate {
		u2 := float32(r.x+r.height) / float32(textureWidth)
		v2 := float32(r.y+r.width) / float32(textureHeight)
		out[0], out[1], out[2], out[3] = u2, v2, u, v2
		out[4], out[5], out[6], out[7] = u, v, u2, v
		return out
	}
	u2 := float32(r.x+r.width) / float32(textureWidth)
	v2 := float32(r.y+r.height) / float32(textureHeight)
	out[0], out[1], out[2], out[3] = u, v2, u, v
	out[4], out[5], out[6], out[7] = u2, v, u2, v2
	return out
}

func meshAttachmentUVsInto(dst []float32, r atlasRegion, textureWidth, textureHeight int, input []float32) []float32 {
	out := resizeSlice(dst, len(input))
	tw, th := float32(textureWidth), float32(textureHeight)
	if r.rotate {
		u := float32(r.x)/tw - float32(r.originalHeight-r.offsetY-r.height)/tw
		v := float32(r.y)/th - float32(r.originalWidth-r.offsetX-r.width)/th
		width := float32(r.originalHeight) / tw
		height := float32(r.originalWidth) / th
		for i := 0; i+1 < len(input); i += 2 {
			out[i] = u + input[i+1]*width
			out[i+1] = v + height - input[i]*height
		}
		return out
	}
	u := float32(r.x-r.offsetX) / tw
	v := float32(r.y)/th - float32(r.originalHeight-r.offsetY-r.height)/th
	width := float32(r.originalWidth) / tw
	height := float32(r.originalHeight) / th
	for i := 0; i+1 < len(input); i += 2 {
		out[i] = u + input[i]*width
		out[i+1] = v + input[i+1]*height
	}
	return out
}

func weightedDeformLength(vertices []float32) int {
	length := 0
	for cursor := 0; cursor < len(vertices); {
		count := int(vertices[cursor])
		cursor += 1 + count*4
		length += count * 2
	}
	return length
}

func parseTransformMode(value string) transformMode {
	switch value {
	case "onlyTranslation":
		return transformOnlyTranslation
	case "noRotationOrReflection":
		return transformNoRotationOrReflection
	case "noScale":
		return transformNoScale
	case "noScaleOrReflection":
		return transformNoScaleOrReflection
	default:
		return transformNormal
	}
}

func wrapDegrees(value float32) float32 {
	for value > 180 {
		value -= 360
	}
	for value < -180 {
		value += 360
	}
	return value
}

func cosDeg(value float32) float32  { return cos(value / radToDeg) }
func sinDeg(value float32) float32  { return sin(value / radToDeg) }
func atan2Deg(y, x float32) float32 { return atan2(y, x) * radToDeg }
func cos(value float32) float32     { return float32(math.Cos(float64(value))) }
func sin(value float32) float32     { return float32(math.Sin(float64(value))) }
func acos(value float32) float32    { return float32(math.Acos(float64(value))) }
func atan2(y, x float32) float32    { return float32(math.Atan2(float64(y), float64(x))) }
func sqrt(value float32) float32    { return float32(math.Sqrt(float64(value))) }
func skinAttachment(current, fallback skin, slot int, name string) *attachment {
	if entries := current[slot]; entries != nil {
		if a := entries[name]; a != nil {
			return a
		}
	}
	if entries := fallback[slot]; entries != nil {
		return entries[name]
	}
	return nil
}
func multiplyColor(a, b [4]float32) [4]float32 {
	return [4]float32{a[0] * b[0], a[1] * b[1], a[2] * b[2], a[3] * b[3]}
}

func parseColor(value string) [4]float32 {
	if value == "" {
		return [4]float32{1, 1, 1, 1}
	}
	if len(value) == 6 {
		value += "ff"
	}
	if len(value) != 8 {
		return [4]float32{1, 1, 1, 1}
	}
	var result [4]float32
	for i := range 4 {
		n, _ := strconv.ParseUint(value[i*2:i*2+2], 16, 8)
		result[i] = float32(n) / 255
	}
	return result
}

func parseDarkColor(value string) [3]float32 {
	if value == "" {
		return [3]float32{}
	}
	color := parseColor(value)
	return [3]float32{color[0], color[1], color[2]}
}

func parseBlendMode(value string) render.MeshBlendMode {
	switch value {
	case "additive":
		return render.MeshBlendAdditive
	case "multiply":
		return render.MeshBlendMultiply
	case "screen":
		return render.MeshBlendScreen
	default:
		return render.MeshBlendNormal
	}
}

type timelineFrame interface {
	timelineTime() float32
	timelineCurve() (uint8, [4]float32)
}

func locateFrame[T timelineFrame](frames []T, time float32) (int, float32, bool) {
	if len(frames) == 0 || time < frames[0].timelineTime() {
		return 0, 0, false
	}
	index := len(frames) - 1
	for i := 1; i < len(frames); i++ {
		if time < frames[i].timelineTime() {
			index = i - 1
			break
		}
	}
	if index == len(frames)-1 {
		return index, 0, true
	}
	a, b := frames[index], frames[index+1]
	mix := (time - a.timelineTime()) / (b.timelineTime() - a.timelineTime())
	curveKind, curve := a.timelineCurve()
	if curveKind == 1 {
		mix = 0
	} else if curveKind == 2 {
		mix = bezierY(curve, mix)
	}
	return index, mix, true
}

func sample(frames []keyframe, time float32, rotation bool) (keyframe, bool) {
	index, mix, ok := locateFrame(frames, time)
	if !ok {
		return keyframe{}, false
	}
	a := frames[index]
	if index == len(frames)-1 {
		return a, true
	}
	b := frames[index+1]
	deltaX := b.x - a.x
	if rotation {
		deltaX = wrapDegrees(deltaX)
	}
	a.x += deltaX * mix
	a.y += (b.y - a.y) * mix
	for i := range 4 {
		a.color[i] += (b.color[i] - a.color[i]) * mix
	}
	for i := range 3 {
		a.dark[i] += (b.dark[i] - a.dark[i]) * mix
	}
	return a, true
}

func sampleIK(frames []ikKeyframe, time float32) (ikPose, bool) {
	index, mix, ok := locateFrame(frames, time)
	if !ok {
		return ikPose{}, false
	}
	a := frames[index]
	value := ikPose{bendDirection: a.bendDirection, compress: a.compress, stretch: a.stretch, mix: a.mix}
	if index != len(frames)-1 {
		value.mix += (frames[index+1].mix - value.mix) * mix
	}
	return value, true
}

func sampleTransform(frames []transformKeyframe, time float32) (transformPose, bool) {
	index, mix, ok := locateFrame(frames, time)
	if !ok {
		return transformPose{}, false
	}
	a := frames[index]
	value := transformPose{
		rotateMix: a.rotateMix, translateMix: a.translateMix,
		scaleMix: a.scaleMix, shearMix: a.shearMix,
	}
	if index != len(frames)-1 {
		b := frames[index+1]
		value.rotateMix += (b.rotateMix - value.rotateMix) * mix
		value.translateMix += (b.translateMix - value.translateMix) * mix
		value.scaleMix += (b.scaleMix - value.scaleMix) * mix
		value.shearMix += (b.shearMix - value.shearMix) * mix
	}
	return value, true
}

func samplePath(frames []pathKeyframe, time float32, kind string) (pathKeyframe, bool) {
	index, mix, ok := locateFrame(frames, time)
	if !ok {
		return pathKeyframe{}, false
	}
	value := frames[index]
	if index == len(frames)-1 {
		return value, true
	}
	next := frames[index+1]
	if kind == "mix" {
		value.rotateMix += (next.rotateMix - value.rotateMix) * mix
		value.translateMix += (next.translateMix - value.translateMix) * mix
	} else {
		value.value += (next.value - value.value) * mix
	}
	return value, true
}

func sampleDeformInto(dst []float32, frames []deformFrame, time float32) ([]float32, bool) {
	index, mix, ok := locateFrame(frames, time)
	if !ok {
		return nil, false
	}
	a := frames[index].vertices
	result := resizeSlice(dst, len(a))
	copy(result, a)
	if index == len(frames)-1 {
		return result, true
	}
	b := frames[index+1].vertices
	for i := range result {
		result[i] += (b[i] - result[i]) * mix
	}
	return result, true
}
func bezierY(curve [4]float32, percent float32) float32 {
	percent = min(max(percent, 0), 1)
	cx1, cy1, cx2, cy2 := float64(curve[0]), float64(curve[1]), float64(curve[2]), float64(curve[3])
	tmpX, tmpY := (-cx1*2+cx2)*.03, (-cy1*2+cy2)*.03
	dddfX, dddfY := ((cx1-cx2)*3+1)*.006, ((cy1-cy2)*3+1)*.006
	ddfX, ddfY := tmpX*2+dddfX, tmpY*2+dddfY
	dfX := cx1*.3 + tmpX + dddfX*.16666667
	dfY := cy1*.3 + tmpY + dddfY*.16666667
	x, y := dfX, dfY
	previousX, previousY := float32(0), float32(0)
	for range 9 {
		currentX, currentY := float32(x), float32(y)
		if currentX >= percent {
			return previousY + (currentY-previousY)*(percent-previousX)/(currentX-previousX)
		}
		previousX, previousY = currentX, currentY
		dfX += ddfX
		dfY += ddfY
		ddfX += dddfX
		ddfY += dddfY
		x += dfX
		y += dfY
	}
	return previousY + (1-previousY)*(percent-previousX)/(1-previousX)
}

func parseAnimation(data []byte, resource *Resource, bones, slots, ik, transforms, paths map[string]int) (animation, error) {
	var root map[string]sourcejson.RawMessage
	if err := sourcejson.Unmarshal(data, &root); err != nil {
		return animation{}, err
	}
	result := animation{}
	var boneMaps map[string]map[string][]map[string]sourcejson.RawMessage
	_ = sourcejson.Unmarshal(root["bones"], &boneMaps)
	for name, timelines := range boneMaps {
		bi, ok := bones[name]
		if !ok {
			return result, fmt.Errorf("unknown bone %q", name)
		}
		for kind, rawFrames := range timelines {
			frames := parseFrames(rawFrames, kind)
			result.bones = append(result.bones, boneTimeline{bone: bi, kind: kind, frames: frames})
			result.duration = max(result.duration, lastTime(frames))
		}
	}
	var slotMaps map[string]map[string][]map[string]sourcejson.RawMessage
	_ = sourcejson.Unmarshal(root["slots"], &slotMaps)
	for name, timelines := range slotMaps {
		si, ok := slots[name]
		if !ok {
			return result, fmt.Errorf("unknown slot %q", name)
		}
		for kind, rawFrames := range timelines {
			frames := parseFrames(rawFrames, kind)
			result.slots = append(result.slots, slotTimeline{slot: si, kind: kind, frames: frames})
			result.duration = max(result.duration, lastTime(frames))
		}
	}
	var ikMaps map[string][]map[string]sourcejson.RawMessage
	_ = sourcejson.Unmarshal(root["ik"], &ikMaps)
	for name, rawFrames := range ikMaps {
		constraint, ok := ik[name]
		if !ok {
			return result, fmt.Errorf("unknown IK constraint %q", name)
		}
		frames := parseIKFrames(rawFrames)
		result.ik = append(result.ik, ikTimeline{constraint: constraint, frames: frames})
		result.duration = max(result.duration, lastTimelineTime(frames))
	}
	var transformMaps map[string][]map[string]sourcejson.RawMessage
	_ = sourcejson.Unmarshal(root["transform"], &transformMaps)
	for name, rawFrames := range transformMaps {
		constraint, ok := transforms[name]
		if !ok {
			return result, fmt.Errorf("unknown transform constraint %q", name)
		}
		frames := parseTransformFrames(rawFrames)
		result.transform = append(result.transform, transformTimeline{constraint: constraint, frames: frames})
		result.duration = max(result.duration, lastTimelineTime(frames))
	}
	var pathMaps map[string]map[string][]map[string]sourcejson.RawMessage
	_ = sourcejson.Unmarshal(root["paths"], &pathMaps)
	for name, timelines := range pathMaps {
		constraint, ok := paths[name]
		if !ok {
			return result, fmt.Errorf("unknown path constraint %q", name)
		}
		for kind, rawFrames := range timelines {
			if kind != "position" && kind != "spacing" && kind != "mix" {
				return result, fmt.Errorf("unknown path timeline %q", kind)
			}
			frames := parsePathFrames(rawFrames, kind)
			result.paths = append(result.paths, pathTimeline{constraint: constraint, kind: kind, frames: frames})
			result.duration = max(result.duration, lastTimelineTime(frames))
		}
	}
	var deformMaps map[string]map[string]map[string][]map[string]sourcejson.RawMessage
	_ = sourcejson.Unmarshal(root["deform"], &deformMaps)
	for skinName, slotMaps := range deformMaps {
		skin := resource.skins[skinName]
		if skin == nil {
			return result, fmt.Errorf("unknown deform skin %q", skinName)
		}
		for slotName, attachmentMaps := range slotMaps {
			slot, ok := slots[slotName]
			if !ok {
				return result, fmt.Errorf("unknown deform slot %q", slotName)
			}
			for attachmentName, rawFrames := range attachmentMaps {
				attachment := skin[slot][attachmentName]
				if attachment == nil {
					continue
				}
				frames := parseDeformFrames(rawFrames, attachment)
				result.deform = append(result.deform, deformTimeline{slot: slot, attachment: attachment, frames: frames})
				result.duration = max(result.duration, lastTimelineTime(frames))
			}
		}
	}
	key := "drawOrder"
	if len(root[key]) == 0 {
		key = "draworder"
	}
	var orders []struct {
		Offsets []struct {
			Slot   string
			Offset int
		}
		Time float32
	}
	_ = sourcejson.Unmarshal(root[key], &orders)
	for _, f := range orders {
		order := make([]int, len(resource.slots))
		for i := range order {
			order[i] = -1
		}
		unchanged := make([]int, 0, len(resource.slots))
		original := 0
		for _, off := range f.Offsets {
			si, ok := slots[off.Slot]
			if !ok {
				continue
			}
			for original < si {
				unchanged = append(unchanged, original)
				original++
			}
			target := original + off.Offset
			if target >= 0 && target < len(order) {
				order[target] = original
			}
			original++
		}
		for original < len(resource.slots) {
			unchanged = append(unchanged, original)
			original++
		}
		for i := len(resource.slots) - 1; i >= 0; i-- {
			if order[i] < 0 {
				order[i] = unchanged[len(unchanged)-1]
				unchanged = unchanged[:len(unchanged)-1]
			}
		}
		result.drawOrder = append(result.drawOrder, drawOrderFrame{time: f.Time, order: order})
		result.duration = max(result.duration, f.Time)
	}
	return result, nil
}

func parseFrames(items []map[string]sourcejson.RawMessage, kind string) []keyframe {
	result := make([]keyframe, 0, len(items))
	for _, item := range items {
		f := keyframe{color: [4]float32{1, 1, 1, 1}}
		decode(item, "time", &f.time)
		decode(item, "x", &f.x)
		decode(item, "y", &f.y)
		if kind == "rotate" {
			decode(item, "angle", &f.x)
		}
		if kind == "attachment" {
			decode(item, "name", &f.name)
		}
		if kind == "color" {
			var value string
			decode(item, "color", &value)
			f.color = parseColor(value)
		}
		if kind == "twoColor" {
			var light, dark string
			decode(item, "light", &light)
			decode(item, "dark", &dark)
			f.color = parseColor(light)
			f.dark = parseDarkColor(dark)
		}
		parseCurve(item, &f.curveKind, &f.curve)
		result = append(result, f)
	}
	return result
}

func parseIKFrames(items []map[string]sourcejson.RawMessage) []ikKeyframe {
	result := make([]ikKeyframe, 0, len(items))
	for _, item := range items {
		frame := ikKeyframe{mix: 1, bendDirection: 1}
		decode(item, "time", &frame.time)
		decode(item, "mix", &frame.mix)
		var bendPositive *bool
		decode(item, "bendPositive", &bendPositive)
		if bendPositive != nil && !*bendPositive {
			frame.bendDirection = -1
		}
		decode(item, "compress", &frame.compress)
		decode(item, "stretch", &frame.stretch)
		parseCurve(item, &frame.curveKind, &frame.curve)
		result = append(result, frame)
	}
	return result
}

func parseTransformFrames(items []map[string]sourcejson.RawMessage) []transformKeyframe {
	result := make([]transformKeyframe, 0, len(items))
	for _, item := range items {
		frame := transformKeyframe{rotateMix: 1, translateMix: 1, scaleMix: 1, shearMix: 1}
		decode(item, "time", &frame.time)
		decode(item, "rotateMix", &frame.rotateMix)
		decode(item, "translateMix", &frame.translateMix)
		decode(item, "scaleMix", &frame.scaleMix)
		decode(item, "shearMix", &frame.shearMix)
		parseCurve(item, &frame.curveKind, &frame.curve)
		result = append(result, frame)
	}
	return result
}

func parsePathFrames(items []map[string]sourcejson.RawMessage, kind string) []pathKeyframe {
	result := make([]pathKeyframe, 0, len(items))
	for _, item := range items {
		frame := pathKeyframe{rotateMix: 1, translateMix: 1}
		decode(item, "time", &frame.time)
		if kind == "mix" {
			decode(item, "rotateMix", &frame.rotateMix)
			decode(item, "translateMix", &frame.translateMix)
		} else {
			decode(item, kind, &frame.value)
		}
		parseCurve(item, &frame.curveKind, &frame.curve)
		result = append(result, frame)
	}
	return result
}

func parseDeformFrames(items []map[string]sourcejson.RawMessage, attachment *attachment) []deformFrame {
	length := len(attachment.vertices)
	if attachment.weighted {
		length = weightedDeformLength(attachment.vertices)
	}
	result := make([]deformFrame, 0, len(items))
	for _, item := range items {
		frame := deformFrame{vertices: make([]float32, length)}
		decode(item, "time", &frame.time)
		var values []float32
		decode(item, "vertices", &values)
		if len(values) == 0 && !attachment.weighted {
			copy(frame.vertices, attachment.vertices)
		} else {
			start := 0
			decode(item, "offset", &start)
			if start < len(frame.vertices) {
				copy(frame.vertices[start:], values)
			}
			if !attachment.weighted {
				for i := range frame.vertices {
					frame.vertices[i] += attachment.vertices[i]
				}
			}
		}
		parseCurve(item, &frame.curveKind, &frame.curve)
		result = append(result, frame)
	}
	return result
}

func parseCurve(item map[string]sourcejson.RawMessage, kind *uint8, curve *[4]float32) {
	data := item["curve"]
	if len(data) == 0 {
		return
	}
	var name string
	if sourcejson.Unmarshal(data, &name) == nil && name == "stepped" {
		*kind = 1
		return
	}
	var values []float32
	if sourcejson.Unmarshal(data, &values) == nil && len(values) == 4 {
		copy(curve[:], values)
		*kind = 2
	}
}
func decode[T any](item map[string]sourcejson.RawMessage, key string, value *T) {
	if data := item[key]; len(data) > 0 {
		_ = sourcejson.Unmarshal(data, value)
	}
}
func valueOr[T any](value *T, fallback T) T {
	if value == nil {
		return fallback
	}
	return *value
}
func valueOrEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
func lastTime(frames []keyframe) float32 {
	if len(frames) == 0 {
		return 0
	}
	return frames[len(frames)-1].time
}
func lastTimelineTime[T timelineFrame](frames []T) float32 {
	if len(frames) == 0 {
		return 0
	}
	return frames[len(frames)-1].timelineTime()
}
func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
