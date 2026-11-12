//go:build js && wasm && !wechat && !onix_directx && !onix_metal && !onix_vulkan && !onix_opengl && !onix_webgl

package render

type webGPUDataKey struct {
	mesh *webGPUMesh
	size int
}

type webGPUDataBinding struct {
	previous, next *frameCacheEntry[webGPUDataKey, webGPUDataBinding]
	groups         [webGPUFramesInFlight]webGPUTextureBindGroup
}

func unlinkWebGPUData(key webGPUDataKey, binding *webGPUDataBinding) {
	if key.mesh == nil {
		return
	}
	if binding.previous != nil {
		binding.previous.value.next = binding.next
	} else {
		key.mesh.bindGroups = binding.next
	}
	if binding.next != nil {
		binding.next.value.previous = binding.previous
	}
}

func unlinkWebGPUMaterial(pair webGPUTexturePair, binding *webGPUMaterialBinding) {
	for index, owner := range pair.textures() {
		link := &binding.links[index]
		if link.previous != nil {
			link.previous.next = link.next
		} else {
			owner.materials = link.next
		}
		if link.next != nil {
			link.next.previous = link.previous
		}
	}
}

func (d *webGPUDriver) trimBindingCaches() {
	d.meshBindGroups.trim(d.cacheFrame, unlinkWebGPUMaterial)
	d.meshDataGroups.trim(d.cacheFrame, unlinkWebGPUData)
}
