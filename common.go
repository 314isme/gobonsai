package gobonsai

import (
	"embed"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

var (
	Mlogger     *Logger
	Mscenes     *ScenesManager
	Minputs     *InputsManager
	Mfonts      *FontsManager
	Mecs        *ECSManager
	Membeds     *EmbedManager
	Maudios     *AudiosManager
	Manimations *AnimationsManager
	Mcolliders  *CollisionManager
	Mtilemaps   *TilemapsManager
	Mphysics    *PhysicsSystem
	gtitle      string
	gwidth      int
	gheight     int
	gscale      int
	initOnce    sync.Once
	Debug       bool
)

func Init(title string, width, height, scale int, embeds embed.FS, samplerate int, debug bool, gravity float64) {
	initOnce.Do(func() {
		gtitle, gwidth, gheight, gscale, Debug = title, width, height, scale, debug
		Mlogger = NewLogger("bonsai")
		Membeds = NewEmbedManager(embeds)
		Mscenes = NewScenesManager()
		Minputs = NewInputsManager()
		Mfonts = NewFontsManager()
		Mecs = NewECSManager()
		Mcolliders = NewCollisionManager(Mecs)
		Mtilemaps = NewTilemapsManager()
		Maudios = NewAudiosManager(samplerate)
		Mphysics := NewPhysicsSystem(Mecs)
		Mecs.AddSystem("physics", Mphysics)
		Manimations = NewAnimationsManager()
		Gravity = gravity
	})
}

func HeadlessInit() {
	initOnce.Do(func() {
		Mlogger = NewLogger("bonsai")
		Mecs = NewECSManager()
	})
}

type Loop struct{}

func (l *Loop) Update() error {
	Mscenes.UpdateScenes(1.0 / 60.0)
	Mcolliders.Update()
	return nil
}

func (l *Loop) Draw(screen *ebiten.Image) {
	Mscenes.DrawScenes(screen)
	if Debug {
		Mcolliders.Draw(screen)
	}
}

func (l *Loop) Layout(outsideWidth, outsideHeight int) (int, int) {
	return gwidth, gheight
}

func Run() {
	ebiten.SetWindowTitle(gtitle)
	ebiten.SetWindowSize(gwidth*gscale, gheight*gscale)
	ebiten.SetTPS(60)
	if err := ebiten.RunGame(&Loop{}); err != nil {
		panic(err)
	}
}
