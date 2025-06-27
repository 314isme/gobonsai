package gobonsai

import (
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

var sceneWrapperPool = sync.Pool{
	New: func() interface{} {
		return &SceneWrapper{}
	},
}

type Scene interface {
	Init()
	Dispose()
	Enter()
	Leave()
	Update(deltaTime float64)
	Draw(screen *ebiten.Image)
}

type SceneWrapper struct {
	name       string
	scene      Scene
	updateFlag bool
	drawFlag   bool
	initFlag   bool
}

type ScenesManager struct {
	scenewrappers map[string]*SceneWrapper
	scenelist     []*SceneWrapper
	updatelist    []*SceneWrapper
	drawlist      []*SceneWrapper
	logger        *Logger
	poolDirty     bool
}

func NewScenesManager() *ScenesManager {
	return &ScenesManager{
		scenewrappers: make(map[string]*SceneWrapper),
		logger:        NewLogger("bonsai:scene"),
		poolDirty:     true,
	}
}

func (sm *ScenesManager) AddScenes(scenes map[string]Scene) {
	for name, scene := range scenes {
		sm.AddScene(name, scene)
	}
}

func (sm *ScenesManager) AddScene(name string, scene Scene, args ...bool) {
	if scene == nil {
		sm.logger.Warn("AddScene failed. Scene is nil:", name)
		return
	}
	if _, ok := sm.scenewrappers[name]; ok {
		sm.logger.Warn("AddScene failed. Scene already exists:", name)
		return
	}
	update := true
	draw := true
	if len(args) > 0 {
		update = args[0]
	}
	if len(args) > 1 {
		draw = args[1]
	}
	w := sceneWrapperPool.Get().(*SceneWrapper)
	w.name = name
	w.scene = scene
	w.updateFlag = update
	w.drawFlag = draw
	w.initFlag = false
	sm.scenewrappers[name] = w
	sm.logger.Debug("Scene added:", name)
	sm.poolDirty = true
}

func (sm *ScenesManager) GetScene() string {
	if n := len(sm.scenelist); n > 0 {
		return sm.scenelist[n-1].name
	}
	return ""
}

func (sm *ScenesManager) PushScene(name string) {
	w, ok := sm.scenewrappers[name]
	if !ok {
		sm.logger.Warn("Push failed. Scene not found:", name)
		return
	}
	sm.scenelist = append(sm.scenelist, w)
	if !w.initFlag {
		w.scene.Init()
		w.initFlag = true
	}
	w.scene.Enter()
	sm.logger.Debug("Scene pushed:", name)
	sm.poolDirty = true
}

func (sm *ScenesManager) PopScene(name ...string) {
	n := len(sm.scenelist)
	if n == 0 {
		return
	}
	if len(name) > 0 {
		for i := n - 1; i >= 0; i-- {
			if sm.scenelist[i].name == name[0] {
				sm.scenelist[i].scene.Leave()
				sm.scenelist = append(sm.scenelist[:i], sm.scenelist[i+1:]...)
				sm.logger.Debug("Scene popped:", name[0])
				sm.poolDirty = true
				return
			}
		}
	} else {
		w := sm.scenelist[n-1]
		w.scene.Leave()
		sm.scenelist = sm.scenelist[:n-1]
		sm.logger.Debug("Scene popped:", w.name)
		sm.poolDirty = true
	}
}

func (sm *ScenesManager) ToogleScene(name string) {
	if sm.isSceneActive(name) {
		sm.PopScene(name)
	} else {
		sm.PushScene(name)
	}
}

func (sm *ScenesManager) SetSceneFlags(name string, eupdate, edraw bool) {
	w, ok := sm.scenewrappers[name]
	if !ok {
		sm.logger.Warn("SetLogic failed. Scene not found:", name)
		return
	}
	if w.updateFlag != eupdate || w.drawFlag != edraw {
		w.updateFlag = eupdate
		w.drawFlag = edraw
		sm.poolDirty = true
	}
	sm.logger.Debug("Logic set for scene:", name, "Update:", eupdate, "Draw:", edraw)
}

func (sm *ScenesManager) UpdateScenes(deltaTime float64) {
	if sm.poolDirty {
		sm.rebuild()
	}
	for _, w := range sm.updatelist {
		w.scene.Update(deltaTime)
	}
}

func (sm *ScenesManager) DrawScenes(screen *ebiten.Image) {
	if sm.poolDirty {
		sm.rebuild()
	}
	for _, w := range sm.drawlist {
		w.scene.Draw(screen)
	}
}

func (sm *ScenesManager) DisposeScene(name string) {
	w, ok := sm.scenewrappers[name]
	if !ok {
		sm.logger.Warn("Dispose failed. Scene not found:", name)
		return
	}
	for i := len(sm.scenelist) - 1; i >= 0; i-- {
		if sm.scenelist[i] == w {
			sm.scenelist = append(sm.scenelist[:i], sm.scenelist[i+1:]...)
			break
		}
	}
	w.scene.Dispose()
	delete(sm.scenewrappers, name)
	sceneWrapperPool.Put(w)
	sm.logger.Debug("Scene disposed:", name)
	sm.poolDirty = true
}

func (sm *ScenesManager) DisposeScenes(scenes []string) {
	for _, w := range scenes {
		sm.DisposeScene(w)
	}
}

func (sm *ScenesManager) DisposeAllScenes() {
	for _, w := range sm.scenelist {
		w.scene.Dispose()
		sceneWrapperPool.Put(w)
	}
	sm.scenelist = sm.scenelist[:0]
	sm.scenewrappers = make(map[string]*SceneWrapper)
	sm.poolDirty = true
	sm.logger.Debug("All scenes disposed")
}

func (sm *ScenesManager) rebuild() {
	if !sm.poolDirty {
		return
	}
	sm.updatelist = sm.updatelist[:0]
	sm.drawlist = sm.drawlist[:0]
	for _, w := range sm.scenelist {
		if w.updateFlag {
			sm.updatelist = append(sm.updatelist, w)
		}
		if w.drawFlag {
			sm.drawlist = append(sm.drawlist, w)
		}
	}
	sm.poolDirty = false
}

func (sm *ScenesManager) isSceneActive(name string) bool {
	for _, w := range sm.scenelist {
		if w.name == name {
			return true
		}
	}
	return false
}
