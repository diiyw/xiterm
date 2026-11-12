//go:build js && wasm

package window

import (
	"syscall/js"
	"time"

	"github.com/onix-engine/onix/input"
)

func jsInputPosition(c *Context, event js.Value) (float64, float64) {
	x, y := event.Get("clientX").Float(), event.Get("clientY").Float()
	return c.GetLogicalXY(x, y)
}

func jsWheelOffset(event js.Value) (float64, float64) {
	multiplier := float64(1)
	if event.Get("deltaMode").Int() == input.DeltaPixel {
		multiplier = 0.1
	}
	return -event.Get("deltaX").Float() * multiplier, -event.Get("deltaY").Float() * multiplier
}

func forEachJSTouch(c *Context, event js.Value, property string, yield func(float64, float64)) {
	touches := event.Get(property)
	for index := range touches.Length() {
		x, y := jsInputPosition(c, touches.Index(index))
		yield(x, y)
	}
}

func jsDroppedFiles(event js.Value) []string {
	transfer := event.Get("dataTransfer")
	if !transfer.Truthy() || transfer.Type() != js.TypeObject {
		return nil
	}
	list := transfer.Get("files")
	if !list.Truthy() || list.Type() != js.TypeObject {
		return nil
	}
	files := make([]string, 0, list.Length())
	for index := 0; index < list.Length(); index++ {
		file := list.Index(index)
		name := ""
		if relative := file.Get("webkitRelativePath"); relative.Type() == js.TypeString {
			name = relative.String()
		}
		if name == "" {
			if value := file.Get("name"); value.Type() == js.TypeString {
				name = value.String()
			}
		}
		if name != "" {
			files = append(files, name)
		}
	}
	return files
}

func toKey(event js.Value) input.KeyCode {
	key := input.KeyCode(event.Get("keyCode").Int())
	location := event.Get("location").Int()
	switch {
	case key == 16 && location == input.KeyLocationLeft:
		return input.KeyLeftShift
	case key == 16 && location == input.KeyLocationRight:
		return input.KeyRightShift
	case key == 17 && location == input.KeyLocationLeft:
		return input.KeyLeftControl
	case key == 17 && location == input.KeyLocationRight:
		return input.KeyRightControl
	case key == 18 && location == input.KeyLocationLeft:
		return input.KeyLeftAlt
	case key == 18 && location == input.KeyLocationRight:
		return input.KeyRightAlt
	default:
		return key
	}
}

func (c *Context) LoopGame(frame func() (bool, error)) error {
	var err error
	wake := make(chan struct{}, 1)
	c.wakeMutex.Lock()
	c.wake = func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	}
	c.wakeMutex.Unlock()
	defer func() { c.wakeMutex.Lock(); c.wake = nil; c.wakeMutex.Unlock() }()
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	defer timer.Stop()
	callback := js.FuncOf(func(js.Value, []js.Value) interface{} {
		go func() {
			c.ActivateRender()
			rendered, frameErr := frame()
			err = frameErr
			if err == nil && rendered {
				err = c.Device.Present()
			}
			c.animationFrameChan <- struct{}{}
		}()
		return nil
	})
	defer callback.Release()
	for {
		<-c.animationFrameChan
		if err != nil {
			return err
		}
		if wait, idle := c.idleWait(); idle {
			if wait < 0 {
				<-wake
			} else {
				timer.Reset(wait)
				select {
				case <-wake:
					timer.Stop()
				case <-timer.C:
				}
			}
		}
		c.Call("requestAnimationFrame", callback)
	}
}
