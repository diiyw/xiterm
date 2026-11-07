package app

/*
#cgo CFLAGS: -fblocks
#cgo LDFLAGS: -framework Cocoa
#include <stdint.h>
void xiterm_install_key_modifiers(uintptr_t window);
int xiterm_next_key_modifiers(int key);
void xiterm_remove_key_modifiers(void);
*/
import "C"

import (
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/onix-engine/onix/input"
)

func (a *App) installNativeInput() func() {
	C.xiterm_install_key_modifiers(C.uintptr_t(a.win.NativeHandle()))
	a.keyModifiers = func(key input.KeyCode) (uv.KeyMod, bool) {
		flags := int(C.xiterm_next_key_modifiers(C.int(key)))
		if flags < 0 {
			return 0, false
		}
		var mods uv.KeyMod
		for i, mod := range []uv.KeyMod{uv.ModShift, uv.ModCtrl, uv.ModAlt, uv.ModSuper} {
			if flags&(1<<i) != 0 {
				mods |= mod
			}
		}
		return mods, true
	}
	return func() {
		a.keyModifiers = nil
		C.xiterm_remove_key_modifiers()
	}
}
