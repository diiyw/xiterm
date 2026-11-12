package render

// Fixed variants avoid a pipeline for every combination of material features.
const (
	meshVariantFull = iota
	meshVariantUnlit
	meshVariantDepth
	meshVariantBasic
	meshVariantCount
)

func meshBuiltinVariant(batch *MeshBatch3D, draw *MeshDraw3D) int {
	pass := int(batch.Lighting.Params[1])
	if pass == MeshPassFXAA {
		return meshVariantUnlit
	}
	if pass == MeshPassReflectionResolve {
		return meshVariantFull
	}
	depthPass := pass == MeshPassShadowDepth || pass == MeshPassOcclusionDepth || pass == MeshPassReflectionDepth
	unlit, basic, depth := true, true, true
	for i := draw.FirstInstance; i < draw.FirstInstance+draw.InstanceCount; i++ {
		instance := &batch.Instances[i]
		flags := uint32(instance.Material[3] + 0.5)
		special := instance.Material[0] < 0 || flags&(256|2048|8192) != 0
		unlit = unlit && special
		depth = depth && instance.Material[0] >= 0 && flags&(8192|2048) == 0
		basic = basic && !special && instance.Custom[0][3] <= 0 && instance.Custom[2][0] <= 0 && instance.Custom[2][1] <= 0 && instance.Custom[2][2] <= 0 && instance.Custom[3][0] <= 0
		if !unlit && !basic && (!depthPass || !depth) {
			return meshVariantFull
		}
	}
	if depth && depthPass {
		return meshVariantDepth
	}
	if unlit {
		return meshVariantUnlit
	}
	if basic {
		return meshVariantBasic
	}
	return meshVariantFull
}
