package gobonsai

import (
	"bytes"
	"fmt"
	"image/color"
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

type Font struct {
	name       string
	data       []byte
	faceSource *text.GoTextFaceSource
	faces      map[float64]*text.GoTextFace
}

func newFont(name string, data []byte, faceSource *text.GoTextFaceSource) *Font {
	return &Font{
		name:       name,
		data:       data,
		faceSource: faceSource,
		faces:      make(map[float64]*text.GoTextFace),
	}
}

func (f *Font) GetFace(size float64) *text.GoTextFace {
	if face, exists := f.faces[size]; exists {
		return face
	}

	face := &text.GoTextFace{
		Source: f.faceSource,
		Size:   size,
	}
	f.faces[size] = face
	return face
}

type TextOptions struct {
	Color       color.Color
	LineSpacing float64
	Align       text.Align
}

type FontsManager struct {
	fonts      map[string]*Font
	logger     *Logger
	mu         sync.RWMutex
	defaultKey string
}

func NewFontsManager() *FontsManager {
	return &FontsManager{
		fonts:  make(map[string]*Font),
		logger: NewLogger("bonsai:fonts"),
	}
}

func (fm *FontsManager) AddFont(name string, path string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	data := Membeds.GetFile(path)
	if data == nil {
		fm.logger.Warn("No font data:", path)
		return fmt.Errorf("font data not found: %s", path)
	}

	faceSource, err := text.NewGoTextFaceSource(bytes.NewReader(data))
	if err != nil {
		fm.logger.Error("Failed to create face source:", err)
		return err
	}

	font := newFont(name, data, faceSource)

	commonSizes := []float64{8, 10, 12, 14, 16, 18, 20, 24, 32, 48}
	for _, size := range commonSizes {
		font.GetFace(size)
	}

	fm.fonts[name] = font

	if fm.defaultKey == "" {
		fm.defaultKey = name
	}

	fm.logger.Info("Font added:", name)
	return nil
}

func (fm *FontsManager) GetFont(name string) *Font {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if font, exists := fm.fonts[name]; exists {
		return font
	}
	return fm.fonts[fm.defaultKey]
}

func (fm *FontsManager) SetDefaultFont(name string) bool {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, exists := fm.fonts[name]; exists {
		fm.defaultKey = name
		return true
	}
	return false
}

func (fm *FontsManager) DrawText(screen *ebiten.Image, txt string, fontName string, size float64, x, y float64, align string, color color.Color) {
	font := fm.GetFont(fontName)
	if font == nil {
		fm.logger.Error("Font not found:", fontName)
		return
	}

	face := font.GetFace(size)
	op := &text.DrawOptions{}

	textWidth, _ := text.Measure(txt, face, size)

	switch align {
	case "center":
		x -= textWidth / 2
	case "end":
		x -= textWidth
	}

	roundedX := math.Round(x)
	roundedY := math.Round(y)

	op.GeoM.Translate(roundedX, roundedY)
	op.ColorScale.ScaleWithColor(color)

	text.Draw(screen, txt, face, op)
}
