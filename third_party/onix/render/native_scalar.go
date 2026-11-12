//go:build cgo && (windows || linux || android || darwin || ios)

package render

/*
#include <stdint.h>
*/
import "C"

func copyFloat32ToC(destination []C.float, source []float32) {
	for index := range source {
		destination[index] = C.float(source[index])
	}
}
