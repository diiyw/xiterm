package window

import "github.com/onix-engine/onix/input"

type callback[T any] struct {
	handler func(T)
}

func (c *callback[T]) set(handler func(T)) {
	c.handler = chainCallback(c.handler, handler)
}

func (c *callback[T]) emit(value T) {
	if c.handler != nil {
		c.handler(value)
	}
}

type textInputSupport struct {
	text        callback[string]
	composition callback[input.TextComposition]
	command     callback[input.TextCommand]
	selection   callback[input.TextSelection]
	applyConfig func(previous, current input.TextInputConfig)
	config      input.TextInputConfig
}

func (s *textInputSupport) OnTextInput(handler func(string)) {
	s.text.set(handler)
}

func (s *textInputSupport) OnTextComposition(handler func(input.TextComposition)) {
	s.composition.set(handler)
}

func (s *textInputSupport) OnTextCommand(handler func(input.TextCommand)) {
	s.command.set(handler)
}

func (s *textInputSupport) OnTextSelection(handler func(input.TextSelection)) {
	s.selection.set(handler)
}

func (s *textInputSupport) SetTextInputConfig(config input.TextInputConfig) {
	if s == nil || s.config == config {
		return
	}
	previous := s.config
	s.config = config
	if s.applyConfig != nil {
		s.applyConfig(previous, config)
	}
}

func (s *textInputSupport) TextInputConfig() input.TextInputConfig {
	if s == nil {
		return input.TextInputConfig{}
	}
	return s.config
}

func (s *textInputSupport) acceptsTextInputSession(session uint64) bool {
	return s != nil && session != 0 && s.config.Value() == session
}

func (s *textInputSupport) emitTextInput(text string) {
	if s != nil && s.config.Active && text != "" {
		s.text.emit(text)
	}
}

func (s *textInputSupport) emitTextComposition(event input.TextComposition) {
	if s == nil || event.Active && !s.config.Active {
		return
	}
	s.composition.emit(event)
}

func (s *textInputSupport) emitTextCommand(command input.TextCommand) {
	if s != nil && s.config.Active && command != 0 {
		s.command.emit(command)
	}
}

func (s *textInputSupport) emitTextSelection(selection input.TextSelection) {
	if s != nil && s.config.Active {
		s.selection.emit(selection)
	}
}

func (s *textInputSupport) applyTextSelection(session uint64, start, end int) {
	if s.acceptsTextInputSession(session) {
		s.emitTextSelection(input.TextSelection{Start: start, End: end})
	}
}
