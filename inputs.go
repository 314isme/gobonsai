package gobonsai

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type InputSource struct {
	Keys           []ebiten.Key
	MouseButtons   []ebiten.MouseButton
	GamepadID      ebiten.GamepadID
	GamepadButtons []ebiten.StandardGamepadButton
	GamepadAxis    []AxisThreshold
	AxisState      map[ebiten.StandardGamepadAxis]float64
}

type AxisThreshold struct {
	Axis      ebiten.StandardGamepadAxis
	Threshold float64
	Direction int
}

type InputBinding struct {
	Name    string
	Sources map[string]*InputSource
}

type InputsManager struct {
	bindings      map[string]*InputBinding
	logger        *Logger
	scene         string
	lastActiveSrc map[string]string
}

func NewInputsManager() *InputsManager {
	return &InputsManager{
		bindings:      make(map[string]*InputBinding),
		logger:        NewLogger("bonsai:input"),
		lastActiveSrc: make(map[string]string),
	}
}

func (im *InputsManager) AddInputSource(group, name string, keys []ebiten.Key, mouseButtons []ebiten.MouseButton,
	gamepadID ebiten.GamepadID, gamepadButtons []ebiten.StandardGamepadButton, gamepadAxis []AxisThreshold) {
	if _, exists := im.bindings[group]; exists {
		if _, exists := im.bindings[group].Sources[name]; exists {
			im.logger.Warn("Input source already exists:", name)
			return
		}
	} else {
		im.bindings[group] = &InputBinding{Name: group, Sources: make(map[string]*InputSource)}
		im.logger.Debug("Input binding added:", group)
	}
	im.bindings[group].Sources[name] = &InputSource{keys, mouseButtons, gamepadID, gamepadButtons, gamepadAxis, make(map[ebiten.StandardGamepadAxis]float64)}
	if b, exists := im.bindings[group]; exists {
		b.Sources[name] = im.bindings[group].Sources[name]
		im.logger.Debug("Input source added:", name, "("+group+")")
	} else {
		im.bindings[group] = &InputBinding{Name: name, Sources: make(map[string]*InputSource)}
		im.logger.Debug("Input binding added:", group)
		im.logger.Debug("Input source added:", name, "("+group+")")
	}
}

func (im *InputsManager) Lock(scene string) {
	im.scene = scene
}

func (im *InputsManager) Unlock() {
	im.scene = ""
}

func (im *InputsManager) IsSourcePressed(group, name string) bool {
	s := im.getSource(group, name)
	return s != nil && im.isActiveScene() && im.isSourcePressed(s)
}

func (im *InputsManager) IsSourceJustPressed(group, name string) bool {
	s := im.getSource(group, name)
	return s != nil && im.isActiveScene() && im.isSourceJustPressed(s)
}

func (im *InputsManager) IsSourceJustReleased(group, name string) bool {
	s := im.getSource(group, name)
	return s != nil && im.isActiveScene() && im.isSourceJustReleased(s)
}

func (im *InputsManager) GetBindingPressed(name string) string {
	b := im.getBinding(name)
	if b == nil || !im.isActiveScene() {
		delete(im.lastActiveSrc, name)
		return ""
	}

	for srcName, src := range b.Sources {
		if im.isSourceJustPressed(src) {
			im.lastActiveSrc[name] = srcName
			return srcName
		}
	}

	if last, exists := im.lastActiveSrc[name]; exists {
		if im.isSourcePressed(b.Sources[last]) {
			return last
		}
	}

	for srcName, src := range b.Sources {
		if im.isSourcePressed(src) {
			im.lastActiveSrc[name] = srcName
			return srcName
		}
	}

	delete(im.lastActiveSrc, name)
	return ""
}

func (im *InputsManager) GetBindingJustPressed(name string) string {
	b := im.getBinding(name)
	if b == nil || !im.isActiveScene() {
		delete(im.lastActiveSrc, name)
		return ""
	}

	for srcName, src := range b.Sources {
		if im.isSourceJustPressed(src) {
			im.lastActiveSrc[name] = srcName
			return srcName
		}
	}
	return ""
}

func (im *InputsManager) GetBindingJustReleased(name string) string {
	b := im.getBinding(name)
	if b == nil || !im.isActiveScene() {
		delete(im.lastActiveSrc, name)
		return ""
	}

	for srcName, src := range b.Sources {
		if im.isSourceJustReleased(src) {
			delete(im.lastActiveSrc, name)
			return srcName
		}
	}
	return ""
}

func (im *InputsManager) getSource(group, name string) *InputSource {
	s, ok := im.bindings[group].Sources[name]
	if !ok {
		im.logger.Warn("Input source not found:", name)
		return nil
	}
	return s
}

func (im *InputsManager) getBinding(name string) *InputBinding {
	b, ok := im.bindings[name]
	if !ok {
		im.logger.Warn("Input binding not found:", name)
		return nil
	}
	return b
}

func (im *InputsManager) isActiveScene() bool {
	return im.scene == "" || im.scene == Mscenes.GetScene()
}

func (im *InputsManager) isSourcePressed(s *InputSource) bool {
	for _, k := range s.Keys {
		if ebiten.IsKeyPressed(k) {
			return true
		}
	}
	for _, mb := range s.MouseButtons {
		if ebiten.IsMouseButtonPressed(mb) {
			return true
		}
	}
	for _, b := range s.GamepadButtons {
		if ebiten.IsStandardGamepadButtonPressed(s.GamepadID, b) {
			return true
		}
	}
	for _, a := range s.GamepadAxis {
		value := ebiten.StandardGamepadAxisValue(s.GamepadID, a.Axis)
		if (a.Direction > 0 && value > a.Threshold) || (a.Direction < 0 && value < -a.Threshold) {
			return true
		}
	}
	return false
}

func (im *InputsManager) isSourceJustPressed(s *InputSource) bool {
	for _, k := range s.Keys {
		if inpututil.IsKeyJustPressed(k) {
			return true
		}
	}
	for _, mb := range s.MouseButtons {
		if inpututil.IsMouseButtonJustPressed(mb) {
			return true
		}
	}
	for _, b := range s.GamepadButtons {
		if inpututil.IsStandardGamepadButtonJustPressed(s.GamepadID, b) {
			return true
		}
	}
	for _, a := range s.GamepadAxis {
		currentValue := ebiten.StandardGamepadAxisValue(s.GamepadID, a.Axis)
		previousValue, existed := s.AxisState[a.Axis]

		if !existed {
			s.AxisState[a.Axis] = currentValue
			continue
		}

		if a.Direction > 0 {
			if previousValue <= a.Threshold && currentValue > a.Threshold {
				s.AxisState[a.Axis] = currentValue
				return true
			}
		} else {
			if previousValue >= -a.Threshold && currentValue < -a.Threshold {
				s.AxisState[a.Axis] = currentValue
				return true
			}
		}

		s.AxisState[a.Axis] = currentValue
	}
	return false
}

func (im *InputsManager) isSourceJustReleased(s *InputSource) bool {
	for _, k := range s.Keys {
		if inpututil.IsKeyJustReleased(k) {
			return true
		}
	}
	for _, mb := range s.MouseButtons {
		if inpututil.IsMouseButtonJustReleased(mb) {
			return true
		}
	}
	for _, b := range s.GamepadButtons {
		if inpututil.IsStandardGamepadButtonJustReleased(s.GamepadID, b) {
			return true
		}
	}
	for _, a := range s.GamepadAxis {
		currentValue := ebiten.StandardGamepadAxisValue(s.GamepadID, a.Axis)
		previousValue, existed := s.AxisState[a.Axis]

		if !existed {
			s.AxisState[a.Axis] = currentValue
			continue
		}

		if a.Direction > 0 {
			if previousValue > a.Threshold && currentValue <= a.Threshold {
				s.AxisState[a.Axis] = currentValue
				return true
			}
		} else {
			if previousValue < -a.Threshold && currentValue >= -a.Threshold {
				s.AxisState[a.Axis] = currentValue
				return true
			}
		}

		s.AxisState[a.Axis] = currentValue
	}
	return false
}
