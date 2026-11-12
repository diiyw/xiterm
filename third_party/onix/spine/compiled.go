package spine

import (
	"fmt"
	"slices"

	"github.com/onix-engine/onix/graphics"
	"github.com/onix-engine/onix/internal/typedbin"
	"github.com/onix-engine/onix/render"
)

type compiledResource struct {
	IK          []compiledIKConstraint
	Animations  []compiledNamedAnimation
	Atlas       []compiledAtlasRegion
	Bones       []compiledBone
	Children    [][]int
	Slots       []compiledSlot
	Paths       []compiledPathConstraint
	Transforms  []compiledTransformConstraint
	Skins       []compiledSkin
	UpdateCache []compiledUpdateOperation
	Attachments []compiledAttachment
	Width       float32
	Height      float32
	Mipmapped   bool
}

type compiledBone struct {
	Name                           string
	Parent                         int
	Length                         float32
	X, Y, Rotation, ScaleX, ScaleY float32
	ShearX, ShearY                 float32
	Transform                      transformMode
}

type compiledIKConstraint struct {
	Name                       string
	Bones                      []int
	Order                      int
	Target                     int
	BendDirection              int
	Mix                        float32
	Compress, Stretch, Uniform bool
}

type compiledTransformConstraint struct {
	Name                                     string
	Bones                                    []int
	Order                                    int
	Target                                   int
	RotateMix                                float32
	OffsetRotation, OffsetX, OffsetY         float32
	OffsetScaleX, OffsetScaleY, OffsetShearY float32
	TranslateMix                             float32
	ScaleMix                                 float32
	ShearMix                                 float32
	Relative                                 bool
	Local                                    bool
}

type compiledPathConstraint struct {
	Name                             string
	PositionMode, SpacingMode        string
	RotateMode                       string
	Bones                            []int
	Order                            int
	Target                           int
	OffsetRotation, Position         float32
	Spacing, RotateMix, TranslateMix float32
}

type compiledUpdateOperation struct {
	Kind  updateKind
	Index int
}

type compiledSlot struct {
	Name       string
	Attachment string
	Bone       int
	Color      [4]float32
	Dark       [3]float32
	Blend      render.MeshBlendMode
}

type compiledAttachment struct {
	Path                  string
	Parent                string
	ParentSkin            string
	Kind                  string
	Name                  string
	Lengths               []float32
	Vertices              []float32
	Triangles             []uint32
	UVs                   []float32
	EndSlot               int
	VertexCount           int
	Color                 [4]float32
	Y                     float32
	Height                float32
	Width                 float32
	ScaleY                float32
	ParentMesh            uint32
	ScaleX                float32
	Rotation              float32
	X                     float32
	Weighted              bool
	InheritDeform         bool
	Closed, ConstantSpeed bool
}

type compiledSkin struct {
	Name  string
	Slots []compiledSkinSlot
}

type compiledSkinSlot struct {
	Entries []compiledSkinEntry
	Slot    int
}

type compiledSkinEntry struct {
	Name       string
	Attachment uint32
}

type compiledKeyframe struct {
	Name      string
	Color     [4]float32
	Curve     [4]float32
	Dark      [3]float32
	Time      float32
	X, Y      float32
	CurveKind uint8
}

type compiledBoneTimeline struct {
	Kind   string
	Frames []compiledKeyframe
	Bone   int
}

type compiledSlotTimeline struct {
	Kind   string
	Frames []compiledKeyframe
	Slot   int
}

type compiledDrawOrderFrame struct {
	Order []int
	Time  float32
}

type compiledDeformFrame struct {
	Vertices  []float32
	Curve     [4]float32
	Time      float32
	CurveKind uint8
}

type compiledDeformTimeline struct {
	Frames     []compiledDeformFrame
	Slot       int
	Attachment uint32
}

type compiledIKKeyframe struct {
	Time              float32
	Mix               float32
	BendDirection     int
	Compress, Stretch bool
	Curve             [4]float32
	CurveKind         uint8
}

type compiledIKTimeline struct {
	Frames     []compiledIKKeyframe
	Constraint int
}

type compiledTransformKeyframe struct {
	Time                                        float32
	RotateMix, TranslateMix, ScaleMix, ShearMix float32
	Curve                                       [4]float32
	CurveKind                                   uint8
}

type compiledTransformTimeline struct {
	Frames     []compiledTransformKeyframe
	Constraint int
}

type compiledPathKeyframe struct {
	Time, Value, RotateMix, TranslateMix float32
	Curve                                [4]float32
	CurveKind                            uint8
}

type compiledPathTimeline struct {
	Kind       string
	Frames     []compiledPathKeyframe
	Constraint int
}

type compiledAnimation struct {
	Bones     []compiledBoneTimeline
	Slots     []compiledSlotTimeline
	Deform    []compiledDeformTimeline
	IK        []compiledIKTimeline
	Transform []compiledTransformTimeline
	Paths     []compiledPathTimeline
	DrawOrder []compiledDrawOrderFrame
	Duration  float32
}

type compiledNamedAnimation struct {
	Name      string
	Animation compiledAnimation
}

type compiledAtlasRegion struct {
	Name                          string
	X, Y, Width, Height           int
	OriginalWidth, OriginalHeight int
	OffsetX, OffsetY              int
	Rotate                        bool
}

type attachmentIndex struct {
	ids    map[*attachment]uint32
	values []*attachment
}

func (index *attachmentIndex) add(value *attachment) uint32 {
	if value == nil {
		return 0
	}
	if id := index.ids[value]; id != 0 {
		return id
	}
	id := uint32(len(index.values) + 1)
	index.ids[value] = id
	index.values = append(index.values, value)
	return id
}

// IsPreprocessed reports whether skeleton data contains Onix's compiled Spine IR.
func IsPreprocessed(data []byte) bool {
	return typedbin.IsEncoded(data)
}

// Preprocess compiles Spine skeleton and atlas source data into a linked IR.
func Preprocess(skeletonJSON, atlasData []byte) ([]byte, error) {
	page, regions, err := parseAtlas(atlasData)
	if err != nil {
		return nil, err
	}
	resource, err := compileSkeleton(skeletonJSON, regions, nil)
	if err != nil {
		return nil, err
	}
	compiled, err := compileResource(resource, page.mipmapped())
	if err != nil {
		return nil, err
	}
	data, err := typedbin.Marshal(compiled)
	if err != nil {
		return nil, fmt.Errorf("spine: encode compiled skeleton: %w", err)
	}
	return data, nil
}

func decodeCompiled(data []byte) (compiledResource, bool, error) {
	if !IsPreprocessed(data) {
		return compiledResource{}, false, nil
	}
	compiled, err := typedbin.Unmarshal[compiledResource](data)
	if err != nil {
		return compiledResource{}, true, fmt.Errorf("spine: decode compiled skeleton: %w", err)
	}
	return compiled, true, nil

}

func decodePreprocessed(data []byte, texture *graphics.Texture) (*Resource, bool, error) {
	compiled, ok, err := decodeCompiled(data)
	if err != nil || !ok {
		return nil, ok, err
	}
	resource, err := restoreResource(compiled, texture)
	return resource, true, err
}

func compileResource(resource *Resource, mipmapped bool) (compiledResource, error) {
	if resource == nil {
		return compiledResource{}, fmt.Errorf("spine: resource is nil")
	}
	compiled := compiledResource{
		Mipmapped: mipmapped, Width: resource.Width, Height: resource.Height,
		Children: clone2D(resource.children),
	}
	compiled.Bones = mapSlice(resource.bones, func(value boneData) compiledBone {
		return compiledBone{
			Name: value.name, Parent: value.parent, Length: value.length,
			X: value.x, Y: value.y, Rotation: value.rotation, ScaleX: value.scaleX, ScaleY: value.scaleY,
			ShearX: value.shearX, ShearY: value.shearY, Transform: value.transform,
		}
	})
	compiled.Slots = mapSlice(resource.slots, func(value slotData) compiledSlot {
		return compiledSlot{Name: value.name, Bone: value.bone, Attachment: value.attachment, Color: value.color, Dark: value.dark, Blend: value.blend}
	})
	compiled.IK = mapSlice(resource.ik, func(value ikConstraintData) compiledIKConstraint {
		return compiledIKConstraint{
			Name: value.name, Order: value.order, Bones: slices.Clone(value.bones), Target: value.target,
			BendDirection: value.bendDirection, Compress: value.compress, Stretch: value.stretch, Uniform: value.uniform, Mix: value.mix,
		}
	})
	compiled.Transforms = mapSlice(resource.transforms, func(value transformConstraintData) compiledTransformConstraint {
		return compiledTransformConstraint{
			Name: value.name, Order: value.order, Bones: slices.Clone(value.bones), Target: value.target, Local: value.local, Relative: value.relative,
			OffsetRotation: value.offsetRotation, OffsetX: value.offsetX, OffsetY: value.offsetY,
			OffsetScaleX: value.offsetScaleX, OffsetScaleY: value.offsetScaleY, OffsetShearY: value.offsetShearY,
			RotateMix: value.rotateMix, TranslateMix: value.translateMix, ScaleMix: value.scaleMix, ShearMix: value.shearMix,
		}
	})
	compiled.Paths = mapSlice(resource.paths, func(value pathConstraintData) compiledPathConstraint {
		return compiledPathConstraint{
			Name: value.name, Order: value.order, Bones: slices.Clone(value.bones), Target: value.target,
			PositionMode: value.positionMode, SpacingMode: value.spacingMode, RotateMode: value.rotateMode,
			OffsetRotation: value.offsetRotation, Position: value.position, Spacing: value.spacing,
			RotateMix: value.rotateMix, TranslateMix: value.translateMix,
		}
	})
	compiled.UpdateCache = mapSlice(resource.updateCache, func(value updateOperation) compiledUpdateOperation {
		return compiledUpdateOperation{Kind: value.kind, Index: value.index}
	})

	attachments := attachmentIndex{ids: make(map[*attachment]uint32)}
	for _, skinName := range sortedStringKeys(resource.skins) {
		for _, slot := range sortedIntKeys(resource.skins[skinName]) {
			for _, name := range sortedStringKeys(resource.skins[skinName][slot]) {
				attachments.add(resource.skins[skinName][slot][name])
			}
		}
	}
	for _, animationName := range sortedStringKeys(resource.animations) {
		for _, timeline := range resource.animations[animationName].deform {
			attachments.add(timeline.attachment)
		}
	}
	for index := 0; index < len(attachments.values); index++ {
		attachments.add(attachments.values[index].parentMesh)
	}
	compiled.Attachments = mapSlice(attachments.values, func(value *attachment) compiledAttachment {
		result := compiledAttachment{
			Name: value.name, Path: value.path, Parent: value.parent, ParentSkin: value.parentSkin, Kind: value.kind,
			X: value.x, Y: value.y, Rotation: value.rotation, ScaleX: value.scaleX, ScaleY: value.scaleY,
			Width: value.width, Height: value.height, Color: value.color, Weighted: value.weighted,
			ParentMesh: attachments.add(value.parentMesh), InheritDeform: value.inheritDeform,
			VertexCount: value.vertexCount, EndSlot: value.endSlot, Closed: value.closed,
			ConstantSpeed: value.constantSpeed, Lengths: slices.Clone(value.lengths),
		}
		if value.parentMesh == nil {
			result.UVs = slices.Clone(value.uvs)
			result.Triangles = slices.Clone(value.triangles)
			result.Vertices = slices.Clone(value.vertices)
		}
		return result
	})
	compiled.Skins = make([]compiledSkin, 0, len(resource.skins))
	for _, skinName := range sortedStringKeys(resource.skins) {
		parsed := resource.skins[skinName]
		skinValue := compiledSkin{Name: skinName, Slots: make([]compiledSkinSlot, 0, len(parsed))}
		for _, slot := range sortedIntKeys(parsed) {
			entries := parsed[slot]
			slotValue := compiledSkinSlot{Slot: slot, Entries: make([]compiledSkinEntry, 0, len(entries))}
			for _, name := range sortedStringKeys(entries) {
				slotValue.Entries = append(slotValue.Entries, compiledSkinEntry{Name: name, Attachment: attachments.add(entries[name])})
			}
			skinValue.Slots = append(skinValue.Slots, slotValue)
		}
		compiled.Skins = append(compiled.Skins, skinValue)
	}
	compiled.Animations = make([]compiledNamedAnimation, 0, len(resource.animations))
	for _, name := range sortedStringKeys(resource.animations) {
		compiled.Animations = append(compiled.Animations, compiledNamedAnimation{
			Name: name, Animation: compileAnimation(resource.animations[name], &attachments),
		})
	}
	compiled.Atlas = make([]compiledAtlasRegion, 0, len(resource.atlas))
	for _, name := range sortedStringKeys(resource.atlas) {
		region := resource.atlas[name]
		compiled.Atlas = append(compiled.Atlas, compiledAtlasRegion{
			Name: name, X: region.x, Y: region.y, Width: region.width, Height: region.height,
			OriginalWidth: region.originalWidth, OriginalHeight: region.originalHeight,
			OffsetX: region.offsetX, OffsetY: region.offsetY, Rotate: region.rotate,
		})
	}
	return compiled, nil
}

func compileAnimation(value animation, attachments *attachmentIndex) compiledAnimation {
	result := compiledAnimation{Duration: value.duration}
	result.Bones = mapSlice(value.bones, func(timeline boneTimeline) compiledBoneTimeline {
		return compiledBoneTimeline{Bone: timeline.bone, Kind: timeline.kind, Frames: mapSlice(timeline.frames, compileKeyframe)}
	})
	slices.SortFunc(result.Bones, func(left, right compiledBoneTimeline) int {
		if left.Bone != right.Bone {
			return left.Bone - right.Bone
		}
		return compareString(left.Kind, right.Kind)
	})
	result.Slots = mapSlice(value.slots, func(timeline slotTimeline) compiledSlotTimeline {
		return compiledSlotTimeline{Slot: timeline.slot, Kind: timeline.kind, Frames: mapSlice(timeline.frames, compileKeyframe)}
	})
	slices.SortFunc(result.Slots, func(left, right compiledSlotTimeline) int {
		if left.Slot != right.Slot {
			return left.Slot - right.Slot
		}
		return compareString(left.Kind, right.Kind)
	})
	result.Deform = mapSlice(value.deform, func(timeline deformTimeline) compiledDeformTimeline {
		frames := mapSlice(timeline.frames, func(frame deformFrame) compiledDeformFrame {
			return compiledDeformFrame{Time: frame.time, Vertices: slices.Clone(frame.vertices), Curve: frame.curve, CurveKind: frame.curveKind}
		})
		return compiledDeformTimeline{Slot: timeline.slot, Attachment: attachments.add(timeline.attachment), Frames: frames}
	})
	slices.SortFunc(result.Deform, func(left, right compiledDeformTimeline) int {
		if left.Slot != right.Slot {
			return left.Slot - right.Slot
		}
		return int(left.Attachment) - int(right.Attachment)
	})
	result.IK = mapSlice(value.ik, func(timeline ikTimeline) compiledIKTimeline {
		frames := mapSlice(timeline.frames, func(frame ikKeyframe) compiledIKKeyframe {
			return compiledIKKeyframe{
				Time: frame.time, Mix: frame.mix, BendDirection: frame.bendDirection,
				Compress: frame.compress, Stretch: frame.stretch, Curve: frame.curve, CurveKind: frame.curveKind,
			}
		})
		return compiledIKTimeline{Constraint: timeline.constraint, Frames: frames}
	})
	slices.SortFunc(result.IK, func(left, right compiledIKTimeline) int { return left.Constraint - right.Constraint })
	result.Transform = mapSlice(value.transform, func(timeline transformTimeline) compiledTransformTimeline {
		frames := mapSlice(timeline.frames, func(frame transformKeyframe) compiledTransformKeyframe {
			return compiledTransformKeyframe{
				Time: frame.time, RotateMix: frame.rotateMix, TranslateMix: frame.translateMix,
				ScaleMix: frame.scaleMix, ShearMix: frame.shearMix, Curve: frame.curve, CurveKind: frame.curveKind,
			}
		})
		return compiledTransformTimeline{Constraint: timeline.constraint, Frames: frames}
	})
	slices.SortFunc(result.Transform, func(left, right compiledTransformTimeline) int { return left.Constraint - right.Constraint })
	result.Paths = mapSlice(value.paths, func(timeline pathTimeline) compiledPathTimeline {
		frames := mapSlice(timeline.frames, func(frame pathKeyframe) compiledPathKeyframe {
			return compiledPathKeyframe{
				Time: frame.time, Value: frame.value, RotateMix: frame.rotateMix, TranslateMix: frame.translateMix,
				Curve: frame.curve, CurveKind: frame.curveKind,
			}
		})
		return compiledPathTimeline{Constraint: timeline.constraint, Kind: timeline.kind, Frames: frames}
	})
	slices.SortFunc(result.Paths, func(left, right compiledPathTimeline) int {
		if left.Constraint != right.Constraint {
			return left.Constraint - right.Constraint
		}
		return compareString(left.Kind, right.Kind)
	})
	result.DrawOrder = mapSlice(value.drawOrder, func(frame drawOrderFrame) compiledDrawOrderFrame {
		return compiledDrawOrderFrame{Time: frame.time, Order: slices.Clone(frame.order)}
	})
	return result
}

func compileKeyframe(frame keyframe) compiledKeyframe {
	return compiledKeyframe{
		Time: frame.time, X: frame.x, Y: frame.y, Color: frame.color, Dark: frame.dark,
		Name: frame.name, Curve: frame.curve, CurveKind: frame.curveKind,
	}
}

func restoreResource(compiled compiledResource, texture *graphics.Texture) (*Resource, error) {
	if len(compiled.Bones) == 0 {
		return nil, fmt.Errorf("spine: compiled skeleton has no bones")
	}
	resource := &Resource{
		Texture: texture, Width: compiled.Width, Height: compiled.Height,
		children: clone2D(compiled.Children), skins: make(map[string]skin, len(compiled.Skins)),
		animations: make(map[string]animation, len(compiled.Animations)), atlas: make(map[string]atlasRegion, len(compiled.Atlas)),
	}
	resource.bones = mapSlice(compiled.Bones, func(value compiledBone) boneData {
		return boneData{
			name: value.Name, parent: value.Parent, length: value.Length,
			x: value.X, y: value.Y, rotation: value.Rotation, scaleX: value.ScaleX, scaleY: value.ScaleY,
			shearX: value.ShearX, shearY: value.ShearY, transform: value.Transform,
		}
	})
	resource.slots = mapSlice(compiled.Slots, func(value compiledSlot) slotData {
		return slotData{name: value.Name, bone: value.Bone, attachment: value.Attachment, color: value.Color, dark: value.Dark, blend: value.Blend}
	})
	resource.ik = mapSlice(compiled.IK, func(value compiledIKConstraint) ikConstraintData {
		return ikConstraintData{
			name: value.Name, order: value.Order, bones: slices.Clone(value.Bones), target: value.Target,
			bendDirection: value.BendDirection, compress: value.Compress, stretch: value.Stretch, uniform: value.Uniform, mix: value.Mix,
		}
	})
	resource.transforms = mapSlice(compiled.Transforms, func(value compiledTransformConstraint) transformConstraintData {
		return transformConstraintData{
			name: value.Name, order: value.Order, bones: slices.Clone(value.Bones), target: value.Target, local: value.Local, relative: value.Relative,
			offsetRotation: value.OffsetRotation, offsetX: value.OffsetX, offsetY: value.OffsetY,
			offsetScaleX: value.OffsetScaleX, offsetScaleY: value.OffsetScaleY, offsetShearY: value.OffsetShearY,
			rotateMix: value.RotateMix, translateMix: value.TranslateMix, scaleMix: value.ScaleMix, shearMix: value.ShearMix,
		}
	})
	resource.paths = mapSlice(compiled.Paths, func(value compiledPathConstraint) pathConstraintData {
		return pathConstraintData{
			name: value.Name, order: value.Order, bones: slices.Clone(value.Bones), target: value.Target,
			positionMode: value.PositionMode, spacingMode: value.SpacingMode, rotateMode: value.RotateMode,
			offsetRotation: value.OffsetRotation, position: value.Position, spacing: value.Spacing,
			rotateMix: value.RotateMix, translateMix: value.TranslateMix,
		}
	})
	resource.updateCache = mapSlice(compiled.UpdateCache, func(value compiledUpdateOperation) updateOperation {
		return updateOperation{kind: value.Kind, index: value.Index}
	})

	attachments := make([]*attachment, len(compiled.Attachments))
	for index, value := range compiled.Attachments {
		attachments[index] = &attachment{
			name: value.Name, path: value.Path, parent: value.Parent, parentSkin: value.ParentSkin, kind: value.Kind,
			x: value.X, y: value.Y, rotation: value.Rotation, scaleX: value.ScaleX, scaleY: value.ScaleY,
			width: value.Width, height: value.Height, color: value.Color,
			uvs: slices.Clone(value.UVs), triangles: slices.Clone(value.Triangles), vertices: slices.Clone(value.Vertices), weighted: value.Weighted,
			inheritDeform: value.InheritDeform, vertexCount: value.VertexCount, endSlot: value.EndSlot,
			closed: value.Closed, constantSpeed: value.ConstantSpeed, lengths: slices.Clone(value.Lengths),
		}
	}
	states := make([]uint8, len(attachments))
	var linkAttachment func(int) error
	linkAttachment = func(index int) error {
		if index < 0 || index >= len(attachments) {
			return fmt.Errorf("spine: compiled attachment index %d is invalid", index)
		}
		switch states[index] {
		case 1:
			return fmt.Errorf("spine: compiled attachment parent cycle at %d", index)
		case 2:
			return nil
		}
		states[index] = 1
		parentID := compiled.Attachments[index].ParentMesh
		if parentID != 0 {
			parentIndex := int(parentID - 1)
			if err := linkAttachment(parentIndex); err != nil {
				return err
			}
			parent := attachments[parentIndex]
			child := attachments[index]
			child.parentMesh = parent
			child.uvs, child.triangles, child.vertices, child.weighted = parent.uvs, parent.triangles, parent.vertices, parent.weighted
		}
		states[index] = 2
		return nil
	}
	for index := range attachments {
		if err := linkAttachment(index); err != nil {
			return nil, err
		}
	}
	attachmentAt := func(id uint32) (*attachment, error) {
		if id == 0 || id > uint32(len(attachments)) {
			return nil, fmt.Errorf("spine: compiled attachment ID %d is invalid", id)
		}
		return attachments[id-1], nil
	}
	for _, skinValue := range compiled.Skins {
		if _, exists := resource.skins[skinValue.Name]; exists {
			return nil, fmt.Errorf("spine: compiled skin %q is duplicated", skinValue.Name)
		}
		parsed := make(skin, len(skinValue.Slots))
		for _, slotValue := range skinValue.Slots {
			if slotValue.Slot < 0 || slotValue.Slot >= len(resource.slots) {
				return nil, fmt.Errorf("spine: compiled skin %q slot %d is invalid", skinValue.Name, slotValue.Slot)
			}
			entries := make(map[string]*attachment, len(slotValue.Entries))
			for _, entry := range slotValue.Entries {
				value, err := attachmentAt(entry.Attachment)
				if err != nil {
					return nil, err
				}
				entries[entry.Name] = value
			}
			parsed[slotValue.Slot] = entries
		}
		resource.skins[skinValue.Name] = parsed
	}
	for _, named := range compiled.Animations {
		if _, exists := resource.animations[named.Name]; exists {
			return nil, fmt.Errorf("spine: compiled animation %q is duplicated", named.Name)
		}
		value, err := restoreAnimation(named.Animation, attachmentAt)
		if err != nil {
			return nil, fmt.Errorf("spine: compiled animation %q: %w", named.Name, err)
		}
		resource.animations[named.Name] = value
	}
	for _, value := range compiled.Atlas {
		if _, exists := resource.atlas[value.Name]; exists {
			return nil, fmt.Errorf("spine: compiled atlas region %q is duplicated", value.Name)
		}
		resource.atlas[value.Name] = atlasRegion{
			name: value.Name, x: value.X, y: value.Y, width: value.Width, height: value.Height,
			originalWidth: value.OriginalWidth, originalHeight: value.OriginalHeight,
			offsetX: value.OffsetX, offsetY: value.OffsetY, rotate: value.Rotate,
		}
	}
	return resource, nil
}

func restoreAnimation(value compiledAnimation, attachmentAt func(uint32) (*attachment, error)) (animation, error) {
	result := animation{duration: value.Duration}
	result.bones = mapSlice(value.Bones, func(timeline compiledBoneTimeline) boneTimeline {
		return boneTimeline{bone: timeline.Bone, kind: timeline.Kind, frames: mapSlice(timeline.Frames, restoreKeyframe)}
	})
	result.slots = mapSlice(value.Slots, func(timeline compiledSlotTimeline) slotTimeline {
		return slotTimeline{slot: timeline.Slot, kind: timeline.Kind, frames: mapSlice(timeline.Frames, restoreKeyframe)}
	})
	result.deform = make([]deformTimeline, 0, len(value.Deform))
	for _, timeline := range value.Deform {
		attachment, err := attachmentAt(timeline.Attachment)
		if err != nil {
			return animation{}, err
		}
		frames := mapSlice(timeline.Frames, func(frame compiledDeformFrame) deformFrame {
			return deformFrame{time: frame.Time, vertices: slices.Clone(frame.Vertices), curve: frame.Curve, curveKind: frame.CurveKind}
		})
		result.deform = append(result.deform, deformTimeline{slot: timeline.Slot, attachment: attachment, frames: frames})
	}
	result.ik = mapSlice(value.IK, func(timeline compiledIKTimeline) ikTimeline {
		frames := mapSlice(timeline.Frames, func(frame compiledIKKeyframe) ikKeyframe {
			return ikKeyframe{
				time: frame.Time, mix: frame.Mix, bendDirection: frame.BendDirection,
				compress: frame.Compress, stretch: frame.Stretch, curve: frame.Curve, curveKind: frame.CurveKind,
			}
		})
		return ikTimeline{constraint: timeline.Constraint, frames: frames}
	})
	result.transform = mapSlice(value.Transform, func(timeline compiledTransformTimeline) transformTimeline {
		frames := mapSlice(timeline.Frames, func(frame compiledTransformKeyframe) transformKeyframe {
			return transformKeyframe{
				time: frame.Time, rotateMix: frame.RotateMix, translateMix: frame.TranslateMix,
				scaleMix: frame.ScaleMix, shearMix: frame.ShearMix, curve: frame.Curve, curveKind: frame.CurveKind,
			}
		})
		return transformTimeline{constraint: timeline.Constraint, frames: frames}
	})
	result.paths = mapSlice(value.Paths, func(timeline compiledPathTimeline) pathTimeline {
		frames := mapSlice(timeline.Frames, func(frame compiledPathKeyframe) pathKeyframe {
			return pathKeyframe{
				time: frame.Time, value: frame.Value, rotateMix: frame.RotateMix, translateMix: frame.TranslateMix,
				curve: frame.Curve, curveKind: frame.CurveKind,
			}
		})
		return pathTimeline{constraint: timeline.Constraint, kind: timeline.Kind, frames: frames}
	})
	result.drawOrder = mapSlice(value.DrawOrder, func(frame compiledDrawOrderFrame) drawOrderFrame {
		return drawOrderFrame{time: frame.Time, order: slices.Clone(frame.Order)}
	})
	return result, nil
}

func restoreKeyframe(frame compiledKeyframe) keyframe {
	return keyframe{
		time: frame.Time, x: frame.X, y: frame.Y, color: frame.Color, dark: frame.Dark,
		name: frame.Name, curve: frame.Curve, curveKind: frame.CurveKind,
	}
}

func mapSlice[S, T any](source []S, convert func(S) T) []T {
	if source == nil {
		return nil
	}
	result := make([]T, len(source))
	for index, value := range source {
		result[index] = convert(value)
	}
	return result
}

func clone2D[T any](source [][]T) [][]T {
	return mapSlice(source, slices.Clone)
}

func sortedStringKeys[V any](values map[string]V) []string {
	result := mapKeys(values)
	slices.Sort(result)
	return result
}

func sortedIntKeys[V any](values map[int]V) []int {
	result := mapKeys(values)
	slices.Sort(result)
	return result
}

func compareString(left, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
