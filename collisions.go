package gobonsai

import (
	"fmt"
	"image/color"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type CollisionHandler struct {
	Enter func(entityA, entityB Entity)
	Stay  func(entityA, entityB Entity)
	Leave func(entityA, entityB Entity)
}

type CollisionManager struct {
	handlers           map[string]map[string]CollisionHandler
	previousCollisions sync.Map
	ecs                *ECSManager
	mu                 sync.RWMutex
}

func NewCollisionManager(ecs *ECSManager) *CollisionManager {
	return &CollisionManager{
		handlers: make(map[string]map[string]CollisionHandler),
		ecs:      ecs,
	}
}

func (cm *CollisionManager) AddResolve(groupA, groupB string, handler CollisionHandler) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.handlers[groupA] == nil {
		cm.handlers[groupA] = make(map[string]CollisionHandler)
	}

	cm.handlers[groupA][groupB] = handler
}

func (cm *CollisionManager) Update() {
	currentCollisions := sync.Map{}
	processed := sync.Map{}

	entities := cm.ecs.GetEntitiesWithComponents("collider", "position", "size")

	for _, e1 := range entities {
		c1, p1, s1 := cm.getComponents(e1)
		if c1 == nil || p1 == nil || s1 == nil {
			continue
		}

		for _, e2 := range entities {
			if e1 == e2 {
				continue
			}

			c2, p2, s2 := cm.getComponents(e2)
			if c2 == nil || p2 == nil || s2 == nil {
				continue
			}

			handler, ok := cm.getHandler(c1.Group, c2.Group)
			if !ok {
				continue
			}

			key := cm.collisionKey(e1, e2)
			if _, alreadyProcessed := processed.Load(key); alreadyProcessed {
				continue
			}
			processed.Store(key, true)

			if cm.checkCollision(p1, c1, p2, c2) {
				currentCollisions.Store(key, true)

				_, wasColliding := cm.previousCollisions.Load(key)

				if !wasColliding {
					if handler.Enter != nil {
						handler.Enter(e1, e2)
					}
					cm.previousCollisions.Store(key, true)
				} else {
					if handler.Stay != nil {
						handler.Stay(e1, e2)
					}
				}
			}
		}
	}

	cm.previousCollisions.Range(func(k, v interface{}) bool {
		if _, stillColliding := currentCollisions.Load(k); !stillColliding {
			e1, e2 := cm.parseCollisionKey(k.(string))

			c1, _, _ := cm.getComponents(e1)
			c2, _, _ := cm.getComponents(e2)
			if c1 == nil || c2 == nil {
				cm.previousCollisions.Delete(k)
				return true
			}

			handler, ok := cm.getHandler(c1.Group, c2.Group)
			if ok && handler.Leave != nil {
				handler.Leave(e1, e2)
			}

			cm.previousCollisions.Delete(k)
		}
		return true
	})
}

func (cm *CollisionManager) checkCollision(p1 *PositionComponent, c1 *ColliderComponent,
	p2 *PositionComponent, c2 *ColliderComponent) bool {

	var x1, y1, w1, h1 float64
	var x2, y2, w2, h2 float64

	if c1.CrossShape {
		hx1, hy1, hw1, hh1 := p1.X+c1.OffsetX-c1.HorizWidth/2, p1.Y+c1.OffsetY-c1.HorizHeight/2, c1.HorizWidth, c1.HorizHeight
		vx1, vy1, vw1, vh1 := p1.X+c1.OffsetX-c1.VertWidth/2, p1.Y+c1.OffsetY-c1.VertHeight/2, c1.VertWidth, c1.VertHeight

		if cm.rectOverlap(hx1, hy1, hw1, hh1, p2.X+c2.OffsetX, p2.Y+c2.OffsetY, c2.Width, c2.Height) ||
			cm.rectOverlap(vx1, vy1, vw1, vh1, p2.X+c2.OffsetX, p2.Y+c2.OffsetY, c2.Width, c2.Height) {
			return true
		}
		return false
	} else {
		x1, y1, w1, h1 = p1.X+c1.OffsetX, p1.Y+c1.OffsetY, c1.Width, c1.Height
	}

	if c2.CrossShape {
		hx2, hy2, hw2, hh2 := p2.X+c2.OffsetX-c2.HorizWidth/2, p2.Y+c2.OffsetY-c2.HorizHeight/2, c2.HorizWidth, c2.HorizHeight
		vx2, vy2, vw2, vh2 := p2.X+c2.OffsetX-c2.VertWidth/2, p2.Y+c2.OffsetY-c2.VertHeight/2, c2.VertWidth, c2.VertHeight

		if cm.rectOverlap(hx2, hy2, hw2, hh2, p1.X+c1.OffsetX, p1.Y+c1.OffsetY, c1.Width, c1.Height) ||
			cm.rectOverlap(vx2, vy2, vw2, vh2, p1.X+c1.OffsetX, p1.Y+c1.OffsetY, c1.Width, c1.Height) {
			return true
		}
		return false
	} else {
		x2, y2, w2, h2 = p2.X+c2.OffsetX, p2.Y+c2.OffsetY, c2.Width, c2.Height
	}

	return cm.rectOverlap(x1, y1, w1, h1, x2, y2, w2, h2)
}

func (cm *CollisionManager) rectOverlap(x1, y1, w1, h1, x2, y2, w2, h2 float64) bool {
	return x1 < x2+w2 && x1+w1 > x2 && y1 < y2+h2 && y1+h1 > y2
}

func (cm *CollisionManager) Draw(screen *ebiten.Image) {
	entities := cm.ecs.GetEntitiesWithComponents("collider", "position")

	for _, e := range entities {
		c, p, _ := cm.getComponents(e)
		if c == nil || p == nil {
			continue
		}

		var x, y, w, h float64
		if c.CrossShape {
			hx, hy, hw, hh := p.X+c.OffsetX-c.HorizWidth/2, p.Y+c.OffsetY-c.HorizHeight/2, c.HorizWidth, c.HorizHeight
			vx, vy, vw, vh := p.X+c.OffsetX-c.VertWidth/2, p.Y+c.OffsetY-c.VertHeight/2, c.VertWidth, c.VertHeight

			cm.drawRect(screen, hx, hy, hw, hh, color.RGBA{0, 0, 255, 120})
			cm.drawRect(screen, vx, vy, vw, vh, color.RGBA{0, 0, 255, 120})
		} else {
			x, y, w, h = p.X+c.OffsetX, p.Y+c.OffsetY, c.Width, c.Height
			cm.drawRect(screen, x, y, w, h, color.RGBA{255, 0, 0, 120})
		}
	}
}

func (cm *CollisionManager) drawRect(screen *ebiten.Image, x, y, w, h float64, col color.RGBA) {
	vector.StrokeRect(screen, float32(x), float32(y), float32(w), float32(h), 1, col, false)
}

func (cm *CollisionManager) getHandler(groupA, groupB string) (CollisionHandler, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if handlers, ok := cm.handlers[groupA]; ok {
		if handler, exists := handlers[groupB]; exists {
			return handler, true
		}
	}
	return CollisionHandler{}, false
}

func (cm *CollisionManager) getComponents(e Entity) (*ColliderComponent, *PositionComponent, *SizeComponent) {
	cRaw := e.GetComponent("collider")
	if cRaw == nil {
		return nil, nil, nil
	}
	c, okC := cRaw.(*ColliderComponent)
	if !okC {
		return nil, nil, nil
	}

	p, okP := e.GetComponent("position").(*PositionComponent)
	s, okS := e.GetComponent("size").(*SizeComponent)

	if !okP || !okS {
		return nil, nil, nil
	}
	return c, p, s
}

func (cm *CollisionManager) collisionKey(e1, e2 Entity) string {
	return fmt.Sprintf("%d-%d", e1, e2)
}

func (cm *CollisionManager) parseCollisionKey(key string) (Entity, Entity) {
	var e1, e2 Entity
	fmt.Sscanf(key, "%d-%d", &e1, &e2)
	return e1, e2
}
