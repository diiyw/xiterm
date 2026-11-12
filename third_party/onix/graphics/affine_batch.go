package graphics

import "github.com/onix-engine/onix/render"

func applyViewAffineBatch(instances []render.Instance2D, view [6]float32) {
	applyViewAffineBatchArch(instances, view)
}

func applyViewAffineBatchGo(instances []render.Instance2D, view [6]float32) {
	for index := range instances {
		model := instances[index].Model
		instances[index].Model = [6]float32{
			view[0]*model[0] + view[1]*model[3],
			view[0]*model[1] + view[1]*model[4],
			view[0]*model[2] + view[1]*model[5] + view[2],
			view[3]*model[0] + view[4]*model[3],
			view[3]*model[1] + view[4]*model[4],
			view[3]*model[2] + view[4]*model[5] + view[5],
		}
	}
}
