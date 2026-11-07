//go:build !darwin

package app

func (a *App) installNativeInput() func() { return func() {} }
