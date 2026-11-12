package render

// MaxMeshLights is the fixed number of extracted lights uploaded to a mesh
// shader. Keeping the limit stable lets every backend use stack or reused
// storage and avoids per-frame growth while still allowing mixed light kinds.
const MaxMeshLights = 16

const meshLightingOffset = 32 + MaxMeshLights*16 + 4
const meshLightRightOffset = meshLightingOffset + 12
const meshLightUpOffset = meshLightRightOffset + MaxMeshLights*4
const meshShadowMatrixOffset = meshLightUpOffset + MaxMeshLights*4
const meshShadowRectOffset = meshShadowMatrixOffset + MaxShadowMaps*16
const meshShadowParamsOffset = meshShadowRectOffset + MaxShadowMaps*4
const meshLightShadowOffset = meshShadowParamsOffset + MaxShadowMaps*4
const meshOcclusionInverseOffset = meshLightShadowOffset + MaxMeshLights*4
const meshLightUniformFloatCount = meshOcclusionInverseOffset + 16
const meshLightUniformByteCount = meshLightUniformFloatCount * 4
const meshLightUniformStride = (meshLightUniformByteCount + 255) &^ 255

// writeMeshLightUniforms writes the backend-neutral light arrays into a mesh
// uniform block. The first 32 floats are the existing projection/camera and
// legacy directional fields; the light arrays start at float 32.
// Direction.w carries the Light3DKind while Params keeps range, intensity,
// and spot cone values in the same four-float record on every backend.
func writeMeshLightUniforms(dst []float32, lights []Light3D) int {
	if len(dst) < meshLightUniformFloatCount {
		panic("render: mesh light uniform buffer is too small")
	}
	clear(dst[32:])
	count := min(len(lights), MaxMeshLights)
	for index := 0; index < count; index++ {
		light := lights[index]
		positionOffset := 32 + index*4
		directionOffset := 32 + MaxMeshLights*4 + index*4
		colorOffset := 32 + MaxMeshLights*8 + index*4
		paramsOffset := 32 + MaxMeshLights*12 + index*4
		copy(dst[positionOffset:positionOffset+4], light.PositionDirection[:])
		copy(dst[directionOffset:directionOffset+4], light.Direction[:])
		copy(dst[colorOffset:colorOffset+4], light.Color[:])
		copy(dst[paramsOffset:paramsOffset+4], light.Params[:])
		copy(dst[meshLightRightOffset+index*4:], light.Right[:])
		copy(dst[meshLightUpOffset+index*4:], light.Up[:])
		dst[directionOffset+3] = float32(light.Kind)
	}
	dst[32+MaxMeshLights*16] = float32(count)
	return count
}

func writeMeshLighting(dst []float32, value MeshLighting) {
	copy(dst[meshLightingOffset:meshLightingOffset+4], value.Environment[:])
	copy(dst[meshLightingOffset+4:meshLightingOffset+8], value.Params[:])
	copy(dst[meshLightingOffset+8:meshLightingOffset+12], value.CameraForward[:])
	// SSAO uses reserved components, keeping GLES fragment uniforms within
	// the minimum limit while retaining every light and shadow map.
	dst[32+MaxMeshLights*16+2] = value.OcclusionParams[0]
	dst[32+MaxMeshLights*16+3] = value.OcclusionParams[1]
	dst[meshLightingOffset+6] = value.OcclusionParams[2]
	dst[meshLightingOffset+11] = value.OcclusionParams[3]
	copy(dst[meshOcclusionInverseOffset:], value.OcclusionInverseProjection[:])
	for index := range value.Shadows {
		shadow := &value.Shadows[index]
		copy(dst[meshShadowMatrixOffset+index*16:], shadow.Matrix[:])
		copy(dst[meshShadowRectOffset+index*4:], shadow.Rect[:])
		copy(dst[meshShadowParamsOffset+index*4:], shadow.Params[:])
		copy(dst[meshLightShadowOffset+index*4:], value.LightShadows[index][:])
	}
	// Camera color/fog occupies the previously reserved W components. Light
	// mapping XYZ, all sixteen lights/maps and the 3584-byte ABI stay intact.
	for index, component := range [...]float32{value.FogColor[0], value.FogColor[1], value.FogColor[2], value.FogParams[0], value.FogParams[1], value.FogParams[2], value.FogParams[3], value.ToneMapping} {
		dst[meshLightShadowOffset+index*4+3] = component
	}
	// Atlas encoding occupies another reserved W component. The native
	// uniform layout and light/shadow XYZ records remain unchanged.
	dst[meshLightShadowOffset+8*4+3] = float32(value.EnvironmentEncoding)
}

// writeMeshSceneUniforms fills the camera, material lighting and light arrays.
// Projection stays with the backend so WebGPU can apply its viewport transform.
func writeMeshSceneUniforms(dst []float32, batch *MeshBatch3D) int {
	copy(dst[16:20], batch.CameraPosition[:])
	copy(dst[20:24], batch.AmbientLight[:])
	copy(dst[24:28], batch.LightDirection[:])
	copy(dst[28:32], batch.DirectionalLight[:])
	count := writeMeshLightUniforms(dst, batch.Lights)
	writeMeshLighting(dst, batch.Lighting)
	return count
}
