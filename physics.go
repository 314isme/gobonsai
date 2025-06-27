package gobonsai

import (
	"image/color"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

var Gravity float64 = 200

type PhysicsSystem struct {
	em *ECSManager
}

func NewPhysicsSystem(em *ECSManager) *PhysicsSystem {
	return &PhysicsSystem{em: em}
}

func (ps *PhysicsSystem) AddCollider(x, y, width, height float64) {
	entity := Mecs.AddEntity()
	entity.AddComponent("position", &PositionComponent{X: x, Y: y})
	entity.AddComponent("size", &SizeComponent{Width: width, Height: height})
	entity.AddComponent("solid", true)
}

func (ps *PhysicsSystem) Init() {}

func (ps *PhysicsSystem) Update(deltaTime float64, args ...interface{}) {
	entities := ps.em.GetEntitiesWithComponents("position", "velocity", "size")

	var wg sync.WaitGroup
	for _, e := range entities {
		wg.Add(1)
		go func(entity Entity) {
			defer wg.Done()
			ps.processEntity(entity, deltaTime)
		}(e)
	}
	wg.Wait()
}

func (ps *PhysicsSystem) processEntity(e Entity, deltaTime float64) {
	pos, ok1 := e.GetComponent("position").(*PositionComponent)
	vel, ok2 := e.GetComponent("velocity").(*VelocityComponent)
	size, ok3 := e.GetComponent("size").(*SizeComponent)
	_, hasExclude := e.GetComponent("solidexclude").(string)

	if !ok1 || !ok2 || !ok3 {
		return
	}

	vel.Y += Gravity * deltaTime

	newX := pos.X + vel.X*deltaTime
	newY := pos.Y + vel.Y*deltaTime

	solidEntities := ps.em.GetEntitiesWithComponents("position", "size", "solid")
	excludeEntities := []Entity{}
	if excludeComp, hasExclude := e.GetComponent("solidexclude").(string); hasExclude && excludeComp != "" {
		excludeEntities = ps.em.GetEntitiesWithComponents(excludeComp)
	}

	for _, other := range solidEntities {
		if other == e || (hasExclude && containsEntity(excludeEntities, other)) {
			continue
		}

		otherPos, ok1 := other.GetComponent("position").(*PositionComponent)
		otherSize, ok2 := other.GetComponent("size").(*SizeComponent)

		if !ok1 || !ok2 {
			continue
		}

		wouldCollideX := isColliding(newX, pos.Y, size, otherPos, otherSize)
		wouldCollideY := isColliding(pos.X, newY, size, otherPos, otherSize)

		if wouldCollideX {
			newX = pos.X
			vel.X = 0
		}

		if wouldCollideY {
			newY = pos.Y
			vel.Y = 0
		}
	}

	pos.X = newX
	pos.Y = newY
}

func containsEntity(entities []Entity, entity Entity) bool {
	for _, e := range entities {
		if e == entity {
			return true
		}
	}
	return false
}

func isColliding(x1, y1 float64, size1 *SizeComponent, pos2 *PositionComponent, size2 *SizeComponent) bool {
	x1Min := x1 - size1.LeftOffset
	x1Max := x1 + size1.Width + size1.RightOffset
	y1Min := y1 - size1.TopOffset
	y1Max := y1 + size1.Height + size1.BottomOffset

	x2Min := pos2.X - size2.LeftOffset
	x2Max := pos2.X + size2.Width + size2.RightOffset
	y2Min := pos2.Y - size2.TopOffset
	y2Max := pos2.Y + size2.Height + size2.BottomOffset

	return x1Min < x2Max && x1Max > x2Min && y1Min < y2Max && y1Max > y2Min
}

func (ps *PhysicsSystem) CheckIfColliding(entity Entity, newx, newy float64) bool {
	pos, ok1 := entity.GetComponent("position").(*PositionComponent)
	size, ok2 := entity.GetComponent("size").(*SizeComponent)

	if !ok1 || !ok2 {
		return false
	}

	solidEntities := Mecs.GetEntitiesWithComponents("position", "size", "solid")

	for _, other := range solidEntities {
		if other == entity {
			continue
		}

		otherPos, ok1 := other.GetComponent("position").(*PositionComponent)
		otherSize, ok2 := other.GetComponent("size").(*SizeComponent)

		if !ok1 || !ok2 {
			continue
		}

		wouldCollideX := isColliding(newx, pos.Y, size, otherPos, otherSize)
		wouldCollideY := isColliding(pos.X, newy, size, otherPos, otherSize)

		if wouldCollideX || wouldCollideY {
			return true
		}
	}

	return false
}

func (ps *PhysicsSystem) Draw(screen *ebiten.Image, args ...interface{}) {
}

func (ps *PhysicsSystem) DrawCollisionBoxes(screen *ebiten.Image) {

	entities := Mecs.GetEntitiesWithComponents("position", "size", "solid")

	for _, e := range entities {
		pos, ok1 := e.GetComponent("position").(*PositionComponent)
		size, ok2 := e.GetComponent("size").(*SizeComponent)

		if !ok1 || !ok2 {
			continue
		}

		left := float32(pos.X - size.LeftOffset)
		top := float32(pos.Y - size.TopOffset)
		right := float32(pos.X + size.Width + size.RightOffset)
		bottom := float32(pos.Y + size.Height + size.BottomOffset)

		vector.StrokeRect(screen, left, top, right-left, bottom-top, 1, color.RGBA{0, 0, 255, 255}, false)
	}
}
