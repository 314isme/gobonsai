package gobonsai

import (
	"bytes"
	"encoding/gob"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/314isme/gonekko"

	"github.com/hajimehoshi/ebiten/v2"
)

type Component interface{}

type ColliderComponent struct {
	Group       string
	CrossShape  bool
	OffsetX     float64
	OffsetY     float64
	Width       float64
	Height      float64
	HorizWidth  float64
	HorizHeight float64
	VertWidth   float64
	VertHeight  float64
}

type PositionComponent struct {
	X float64
	Y float64
}

type SizeComponent struct {
	Width        float64
	Height       float64
	TopOffset    float64
	BottomOffset float64
	LeftOffset   float64
	RightOffset  float64
}

type VelocityComponent struct {
	X float64
	Y float64
}

type Entity uint32

type System interface {
	Init()
	Update(deltaTime float64, args ...interface{})
	Draw(screen *ebiten.Image, args ...interface{})
}

type ECSManager struct {
	lastEntityID   uint32
	entities       sync.Map
	indentToEntity sync.Map
	components     sync.Map
	systems        map[string]System
	logger         *Logger
}

func NewECSManager() *ECSManager {
	registerGob()
	return &ECSManager{
		logger: NewLogger("bonsai:ecs"),
	}
}

func registerGob() {
	gob.Register(gonekko.MessageType{})
	gob.Register(&PositionComponent{})
	gob.Register(&VelocityComponent{})
	gob.Register(&ColliderComponent{})
	gob.Register(&SizeComponent{})
}

func (em *ECSManager) AddEntity(ident ...string) Entity {
	id := Entity(atomic.AddUint32(&em.lastEntityID, 1))
	em.entities.Store(id, true)
	em.components.Store(id, &sync.Map{})
	if len(ident) > 0 {
		em.indentToEntity.Store(ident[0], id)
		em.logger.Debug("Created entity:", id, "with network ID:", ident[0])
	} else {
		em.logger.Debug("Created entity:", id)
	}
	return id
}

func (em *ECSManager) RemoveEntity(e Entity) {
	em.indentToEntity.Range(func(k, v interface{}) bool {
		if v.(Entity) == e {
			em.indentToEntity.Delete(k)
			return false
		}
		return true
	})
	em.entities.Delete(e)
	em.components.Delete(e)
	em.logger.Debug("Removed entity:", e)
}

func (em *ECSManager) RemoveEntities() {
	em.entities.Range(func(k, v interface{}) bool {
		if k.(Entity).GetComponent("persistent") == nil {
			em.RemoveEntity(k.(Entity))
		}
		return true
	})
}

func (em *ECSManager) GetEntityByIdent(ident string) (Entity, bool) {
	v, ok := em.indentToEntity.Load(ident)
	if !ok {
		return 0, false
	}
	return v.(Entity), true
}

func (em *ECSManager) GetEntityIdent(e Entity) (string, bool) {
	var ident string
	em.indentToEntity.Range(func(k, v interface{}) bool {
		if v.(Entity) == e {
			ident = k.(string)
			return false
		}
		return true
	})
	if ident == "" {
		return "", false
	}
	return ident, true
}

func (em *ECSManager) GetEntityByID(id Entity) (Entity, bool) {
	_, ok := em.entities.Load(id)
	return id, ok
}

func (e Entity) AddComponent(name string, comp Component) {
	if _, exists := Mecs.entities.Load(e); !exists {
		Mecs.logger.Warn("Entity does not exist:", e)
		return
	}
	v, _ := Mecs.components.Load(e)
	v.(*sync.Map).Store(name, comp)
	Mecs.logger.Trace("Added component:", name, "to entity:", e)
}

func (e Entity) AddComponents(comps map[string]Component) {
	for n, c := range comps {
		e.AddComponent(n, c)
	}
}

func (e Entity) SetComponent(name string, comp Component) {
	v, exists := Mecs.components.Load(e)
	if !exists {
		return
	}
	v.(*sync.Map).Store(name, comp)
}

func (e Entity) GetComponent(name string) Component {
	v, exists := Mecs.components.Load(e)
	if !exists {
		return nil
	}
	comp, _ := v.(*sync.Map).Load(name)
	return comp
}

func (e Entity) RemoveComponent(name string) {
	v, exists := Mecs.components.Load(e)
	if !exists {
		return
	}
	v.(*sync.Map).Delete(name)
}

func (e Entity) RemoveComponents(names ...string) {
	for _, n := range names {
		e.RemoveComponent(n)
	}
}

func (e Entity) HasComponent(name string) bool {
	v, exists := Mecs.components.Load(e)
	if !exists {
		return false
	}
	_, exists = v.(*sync.Map).Load(name)
	return exists
}

func (em *ECSManager) AddSystem(name string, system System) {
	if em.systems == nil {
		em.systems = make(map[string]System)
	}
	em.systems[name] = system
	em.logger.Debug("Added system:", name)
}

func (em *ECSManager) AddSystems(names []string, systems []System) {
	for i, name := range names {
		em.AddSystem(name, systems[i])
	}
}

func (em *ECSManager) InitSystems() {
	for _, system := range em.systems {
		system.Init()
	}
}

func (em *ECSManager) UpdateEntities(deltaTime float64) {
	for _, entity := range em.GetEntitiesWithComponents("update") {
		if update := entity.GetComponent("update"); update != nil {
			update.(func(float64, Entity))(deltaTime, entity)
		}
	}
}

func (em *ECSManager) SortEntities(entities []Entity) []Entity {
	sort.Slice(entities, func(i, j int) bool {
		return entities[i] < entities[j]
	})
	return entities
}

func (em *ECSManager) UpdateSystems(deltaTime float64, exclude ...string) {
	for name, system := range em.systems {
		if containsSystem(exclude, name) {
			continue
		}
		system.Update(deltaTime)
	}
}

func (em *ECSManager) DrawSystems(screen *ebiten.Image, exclude ...string) {
	for name, system := range em.systems {
		if containsSystem(exclude, name) {
			continue
		}
		system.Draw(screen)
	}
}

func (em *ECSManager) UpdateSystem(name string, deltaTime float64, args ...interface{}) {
	if system, ok := em.systems[name]; ok {
		system.Update(deltaTime, args...)
	}
}

func (em *ECSManager) DrawSystem(name string, screen *ebiten.Image, args ...interface{}) {
	if system, ok := em.systems[name]; ok {
		system.Draw(screen, args...)
	}
}

func containsSystem(exclude []string, systemName string) bool {
	for _, name := range exclude {
		if name == systemName {
			return true
		}
	}
	return false
}

func (em *ECSManager) DrawEntities(screen *ebiten.Image) {
	for _, entity := range em.GetEntitiesWithComponents("draw") {
		if draw := entity.GetComponent("draw"); draw != nil {
			draw.(func(*ebiten.Image, Entity))(screen, entity)
		}
	}
}

func (em *ECSManager) GetEntitiesWithComponents(names ...string) []Entity {
	var result []Entity
	em.entities.Range(func(k, v interface{}) bool {
		e := k.(Entity)
		compVal, _ := em.components.Load(e)
		if compVal == nil {
			return true
		}
		cm := compVal.(*sync.Map)
		for _, n := range names {
			if _, ok := cm.Load(n); !ok {
				return true
			}
		}
		result = append(result, e)
		return true
	})
	return result
}

func (em *ECSManager) ToSerializable() *SerializableECSManager {
	entities := make(map[Entity]bool)
	em.entities.Range(func(k, v interface{}) bool {
		entities[k.(Entity)] = v.(bool)
		return true
	})

	components := make(map[Entity]map[string]Component)
	em.components.Range(func(k, v interface{}) bool {
		e := k.(Entity)
		cMap := make(map[string]Component)
		v.(*sync.Map).Range(func(ck, cv interface{}) bool {
			cMap[ck.(string)] = cv
			return true
		})
		components[e] = cMap
		return true
	})

	identMap := make(map[uint32]Entity)
	em.indentToEntity.Range(func(k, v interface{}) bool {
		identMap[k.(uint32)] = v.(Entity)
		return true
	})

	return &SerializableECSManager{
		LastEntityID: em.lastEntityID,
		Entities:     entities,
		Components:   components,
		identMap:     identMap,
	}
}

func (em *ECSManager) FromSerializable(data *SerializableECSManager) {
	em.lastEntityID = data.LastEntityID
	for e, active := range data.Entities {
		em.entities.Store(e, active)
		cm := &sync.Map{}
		if comps, ok := data.Components[e]; ok {
			for n, c := range comps {
				cm.Store(n, c)
			}
		}
		em.components.Store(e, cm)
	}
	for ident, e := range data.identMap {
		em.indentToEntity.Store(ident, e)
	}
}

func (em *ECSManager) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := gob.NewEncoder(buf).Encode(em.ToSerializable())
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (em *ECSManager) Decode(data []byte) error {
	buf := bytes.NewBuffer(data)
	sData := &SerializableECSManager{}
	if err := gob.NewDecoder(buf).Decode(sData); err != nil {
		return err
	}
	em.FromSerializable(sData)
	return nil
}

type SerializableEntity struct {
	ID         Entity
	Ident      string
	Components map[string]Component
}

func (em *ECSManager) EncodeEntity(e Entity) ([]byte, error) {
	v, _ := em.components.Load(e)
	cm := v.(*sync.Map)
	comps := make(map[string]Component)
	cm.Range(func(k, v interface{}) bool {
		comps[k.(string)] = v
		return true
	})

	var ident string
	em.indentToEntity.Range(func(k, val interface{}) bool {
		if val.(Entity) == e {
			ident = k.(string)
			return false
		}
		return true
	})

	entityData := SerializableEntity{ID: e, Ident: ident, Components: comps}
	buf := new(bytes.Buffer)
	err := gob.NewEncoder(buf).Encode(entityData)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (em *ECSManager) DecodeEntity(data []byte) (Entity, error) {
	buf := bytes.NewBuffer(data)
	var entityData SerializableEntity
	if err := gob.NewDecoder(buf).Decode(&entityData); err != nil {
		return 0, err
	}
	var target Entity
	if entityData.Ident != "" {
		if ex, ok := em.GetEntityByIdent(entityData.Ident); ok {
			target = ex
		} else {
			target = em.AddEntity(entityData.Ident)
		}
	} else {
		target = em.AddEntity()
	}
	for name, comp := range entityData.Components {
		target.AddComponent(name, comp)
	}
	return target, nil
}

type SerializableECSManager struct {
	LastEntityID uint32
	Entities     map[Entity]bool
	Components   map[Entity]map[string]Component
	identMap     map[uint32]Entity
}

func (e SerializableEntity) SetComponent(name string, comp Component) {
	v, exists := Mecs.components.Load(e.ID)
	if !exists {
		return
	}
	v.(*sync.Map).Store(name, comp)
}
