package gobonsai

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/hajimehoshi/ebiten/v2"
)

type AnimationsManager struct {
	animations map[string]map[string]*Animation
	logger     *Logger
}

func NewAnimationsManager() *AnimationsManager {
	return &AnimationsManager{
		animations: make(map[string]map[string]*Animation),
		logger:     NewLogger("bonsai:animations"),
	}
}

func (am *AnimationsManager) AddAnimation(groupname, animname, spriteSheetPath string, fw, fh int, fd float64, indices []int, loop bool) {
	data := Membeds.GetFile(spriteSheetPath)
	if data == nil {
		am.logger.Warn("No sprite sheet data:", spriteSheetPath)
	}
	if len(indices) == 0 {
		am.logger.Warn("No frame indices provided")
	}
	frames := am.extractFrames(data, fw, fh, indices)
	if len(frames) == 0 {
		am.logger.Warn("No valid frames extracted")
	}
	am.logger.Info("Animation added")
	if _, ok := am.animations[groupname]; !ok {
		am.animations[groupname] = make(map[string]*Animation)
	}
	am.animations[groupname][animname] = &Animation{
		frames:        frames,
		frameDuration: fd,
		loop:          loop,
	}
}

func (am *AnimationsManager) GetAnimation(groupname, animname string) *Animation {
	if group, ok := am.animations[groupname]; ok {
		if anim, ok := group[animname]; ok {
			framesCopy := make([]*ebiten.Image, len(anim.frames))
			copy(framesCopy, anim.frames)
			return &Animation{
				frames:        framesCopy,
				frameDuration: anim.frameDuration,
				loop:          anim.loop,
			}
		}
	}
	return nil
}

func (am *AnimationsManager) GetAnimations(groupname string) map[string]*Animation {
	if group, ok := am.animations[groupname]; ok {
		copiedGroup := make(map[string]*Animation, len(group))
		for animname, anim := range group {
			framesCopy := make([]*ebiten.Image, len(anim.frames))
			copy(framesCopy, anim.frames)
			copiedGroup[animname] = &Animation{
				frames:        framesCopy,
				frameDuration: anim.frameDuration,
				loop:          anim.loop,
			}
		}
		return copiedGroup
	}
	return nil
}

func (am *AnimationsManager) extractFrames(data []byte, fw, fh int, indices []int) []*ebiten.Image {
	if fw <= 0 || fh <= 0 {
		am.logger.Error("Invalid frame dimensions")
		return nil
	}
	img := am.decodeImage(bytes.NewReader(data))
	if img == nil {
		return nil
	}
	si := ebiten.NewImageFromImage(img)
	sw, sh := si.Bounds().Dx(), si.Bounds().Dy()
	perRow := (sw + fw - 1) / fw
	frames := make([]*ebiten.Image, 0, len(indices))

	for _, idx := range indices {
		x, y := (idx%perRow)*fw, (idx/perRow)*fh
		if x >= sw || y >= sh {
			am.logger.Warn("Frame index out of bounds", idx)
			continue
		}
		right, bottom := min(x+fw, sw), min(y+fh, sh)
		if eimg, ok := si.SubImage(image.Rect(x, y, right, bottom)).(*ebiten.Image); ok {
			frames = append(frames, eimg)
		} else {
			am.logger.Error("Failed to convert frame:", idx)
		}
	}
	return frames
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (am *AnimationsManager) decodeImage(r *bytes.Reader) image.Image {
	decoders := []func(io.Reader) (image.Image, error){
		png.Decode,
		jpeg.Decode,
		func(rr io.Reader) (image.Image, error) {
			img, _, err := image.Decode(rr)
			return img, err
		},
	}
	for _, d := range decoders {
		r.Seek(0, 0)
		if img, err := d(r); err == nil {
			return img
		}
	}
	am.logger.Error("Failed to decode image")
	return nil
}

type Animation struct {
	frames        []*ebiten.Image
	frameDuration float64
	currentFrame  int
	elapsedTime   float64
	loop          bool
	completed     bool
}

func (a *Animation) UpdateAnimation(dt float64) {
	if a.completed && !a.loop || dt <= 0 || len(a.frames) == 0 {
		return
	}
	a.elapsedTime += dt
	for a.elapsedTime >= a.frameDuration {
		a.elapsedTime -= a.frameDuration
		a.currentFrame++
		if a.currentFrame >= len(a.frames) {
			if a.loop {
				a.currentFrame = 0
			} else {
				a.currentFrame = len(a.frames) - 1
				a.completed = true
				return
			}
		}
	}
}

func (a *Animation) DrawAnimation(screen *ebiten.Image, x, y int, scale ...float64) {
	if len(a.frames) == 0 {
		return
	}
	s := 1.0
	if len(scale) > 0 {
		s = scale[0]
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(float64(x), float64(y))
	screen.DrawImage(a.frames[a.currentFrame], op)
}

func (a *Animation) Reset() {
	a.currentFrame = 0
	a.elapsedTime = 0
	a.completed = false
}

func (a *Animation) IsFinished() bool {
	return a.completed
}
