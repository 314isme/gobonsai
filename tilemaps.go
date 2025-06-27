package gobonsai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type Layer struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Data    []int    `json:"data"`
	Objects []Object `json:"objects"`
	Width   int      `json:"width"`
	Height  int      `json:"height"`
	Visible bool     `json:"visible"`
	Opacity float64  `json:"opacity"`
	Layers  []Layer  `json:"layers"`
}

type Object struct {
	ID       int        `json:"id"`
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	X        float64    `json:"x"`
	Y        float64    `json:"y"`
	Width    float64    `json:"width"`
	Height   float64    `json:"height"`
	RawProps []Property `json:"properties"`
	GID      int        `json:"gid,omitempty"`
}

type Property struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

type AnimationFrame struct {
	TileID   int `json:"tileid"`
	Duration int `json:"duration"`
}

type TilesetTile struct {
	ID         int              `json:"id"`
	Properties []Property       `json:"properties"`
	Animation  []AnimationFrame `json:"animation"`
}

type AnimationInfo struct {
	Frames       []AnimationFrame
	CurrentFrame int
	ElapsedTime  float64
	BaseGID      int
}

type Tile struct {
	ID            int
	Properties    map[string]interface{}
	AnimationInfo *AnimationInfo
}

type Tileset struct {
	FirstGID    int           `json:"firstgid"`
	Name        string        `json:"name"`
	Image       string        `json:"image"`
	TileWidth   int           `json:"tilewidth"`
	TileHeight  int           `json:"tileheight"`
	ImageWidth  int           `json:"imagewidth"`
	ImageHeight int           `json:"imageheight"`
	Tiles       []TilesetTile `json:"tiles"`
}

type EmbeddedTileset struct {
	FirstGID    int           `json:"firstgid"`
	Source      string        `json:"source,omitempty"`
	Name        string        `json:"name"`
	Image       string        `json:"image"`
	TileWidth   int           `json:"tilewidth"`
	TileHeight  int           `json:"tileheight"`
	ImageWidth  int           `json:"imagewidth"`
	ImageHeight int           `json:"imageheight"`
	Tiles       []TilesetTile `json:"tiles"`
}

type Map struct {
	Width      int               `json:"width"`
	Height     int               `json:"height"`
	TileWidth  int               `json:"tilewidth"`
	TileHeight int               `json:"tileheight"`
	Layers     []Layer           `json:"layers"`
	Tilesets   []EmbeddedTileset `json:"tilesets"`
}

type TileMap struct {
	Map           *Map
	Tilesets      []*Tileset
	TilesetImages map[int]*ebiten.Image
	Tiles         map[int]*Tile
	CachedTiles   map[int]*ebiten.Image
	Objects       map[int]Object
}

type TilemapsManager struct {
	tileMaps       map[string]*TileMap
	currentTileMap string
	logger         *Logger
	mu             sync.Mutex
	callbacks      map[string]func(Object)
	scale          float64
}

func NewTilemapsManager() *TilemapsManager {
	return &TilemapsManager{
		tileMaps:  make(map[string]*TileMap),
		logger:    NewLogger("bonsai:tilemap"),
		callbacks: make(map[string]func(Object)),
	}
}

func (tmm *TilemapsManager) AddCallback(name string, callback func(Object)) {
	tmm.mu.Lock()
	defer tmm.mu.Unlock()
	tmm.callbacks[name] = callback
}

func (tmm *TilemapsManager) AddTilemaps(tilemaps map[string]string) {
	for name, path := range tilemaps {
		if err := tmm.AddTilemap(name, path); err != nil {
			tmm.logger.Error("failed to add tilemap: ", name)
		}
	}
}

func (tmm *TilemapsManager) GetTilemaps() map[string]*TileMap {
	tmm.mu.Lock()
	defer tmm.mu.Unlock()
	return tmm.tileMaps
}

func (obj Object) GetProperty(name string) interface{} {
	for _, prop := range obj.RawProps {
		if prop.Name == name {
			return prop.Value
		}
	}
	return nil
}

func (tmm *TilemapsManager) AddTilemap(name, path string) error {
	tmm.mu.Lock()
	defer tmm.mu.Unlock()
	jsonPath := tmm.normalizeJSONPath(path)
	data, err := tmm.loadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("failed to load tilemap: %w", err)
	}
	var m Map
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("failed to parse map json: %w", err)
	}
	tileMap := &TileMap{
		Map:           &m,
		Tilesets:      []*Tileset{},
		TilesetImages: make(map[int]*ebiten.Image),
		Tiles:         make(map[int]*Tile),
		CachedTiles:   make(map[int]*ebiten.Image),
		Objects:       make(map[int]Object),
	}
	baseDir := filepath.Dir(jsonPath)
	for _, emb := range m.Tilesets {
		if emb.Source != "" {
			tilesetPath := tmm.normalizeJSONPath(filepath.Join(baseDir, emb.Source))
			tsData, err := tmm.loadFile(tilesetPath)
			if err != nil {
				return fmt.Errorf("failed to load tileset json (%s): %w", emb.Source, err)
			}
			var ts Tileset
			if err := json.Unmarshal(tsData, &ts); err != nil {
				return fmt.Errorf("failed to parse tileset json (%s): %w", emb.Source, err)
			}
			ts.FirstGID = emb.FirstGID
			tileMap.Tilesets = append(tileMap.Tilesets, &ts)
			imagePath := tmm.normalizeAssetPath(ts.Image)
			imgData, err := tmm.loadFile(imagePath)
			if err != nil {
				return fmt.Errorf("failed to load tileset image: %w", err)
			}
			img, _, err := ebitenutil.NewImageFromReader(bytes.NewReader(imgData))
			if err != nil {
				return fmt.Errorf("failed to decode tileset image: %w", err)
			}
			tileMap.TilesetImages[ts.FirstGID] = img
			tmm.cacheTiles(ts, img, tileMap)
		} else {
			ts := &Tileset{
				FirstGID:    emb.FirstGID,
				Name:        emb.Name,
				Image:       emb.Image,
				TileWidth:   emb.TileWidth,
				TileHeight:  emb.TileHeight,
				ImageWidth:  emb.ImageWidth,
				ImageHeight: emb.ImageHeight,
				Tiles:       emb.Tiles,
			}
			tileMap.Tilesets = append(tileMap.Tilesets, ts)
			imagePath := tmm.normalizeAssetPath(ts.Image)
			imgData, err := tmm.loadFile(imagePath)
			if err != nil {
				return fmt.Errorf("failed to load tileset image: %w", err)
			}
			img, _, err := ebitenutil.NewImageFromReader(bytes.NewReader(imgData))
			if err != nil {
				return fmt.Errorf("failed to decode tileset image: %w", err)
			}
			tileMap.TilesetImages[ts.FirstGID] = img
			tmm.cacheTiles(*ts, img, tileMap)
		}
	}
	tmm.tileMaps[name] = tileMap
	return nil
}

func (tmm *TilemapsManager) loadFile(path string) ([]byte, error) {
	if data := Membeds.GetFile(path); data != nil {
		return data, nil
	}
	return os.ReadFile(path)
}

func (tmm *TilemapsManager) normalizeJSONPath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.TrimPrefix(path, "./")
	if !strings.HasPrefix(path, "json/") {
		parts := strings.Split(path, "/")
		newParts := []string{}
		for _, part := range parts {
			if part == ".." && len(newParts) > 1 {
				newParts = newParts[:len(newParts)-1]
			} else if part != "." && part != ".." {
				newParts = append(newParts, part)
			}
		}
		path = "json/" + strings.Join(newParts, "/")
	}
	path = strings.TrimSuffix(path, "/")
	tmm.logger.Info("Resolved JSON path:", path)
	return path
}

func (tmm *TilemapsManager) normalizeAssetPath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	for strings.HasPrefix(path, "../") || strings.HasPrefix(path, "..\\") {
		path = path[3:]
	}
	if !strings.HasPrefix(path, "assets/") {
		path = "assets/" + path
	}
	path = filepath.Clean(path)
	tmm.logger.Info("Resolved Asset path:", path)
	return path
}

func (tmm *TilemapsManager) cacheTiles(ts Tileset, img *ebiten.Image, tileMap *TileMap) {
	totalTiles := (ts.ImageWidth / ts.TileWidth) * (ts.ImageHeight / ts.TileHeight)
	tpr := ts.ImageWidth / ts.TileWidth
	for gid := ts.FirstGID; gid < ts.FirstGID+totalTiles; gid++ {
		tid := gid - ts.FirstGID
		sx := (tid % tpr) * ts.TileWidth
		sy := (tid / tpr) * ts.TileHeight
		tileMap.CachedTiles[gid] = img.SubImage(image.Rect(sx, sy, sx+ts.TileWidth, sy+ts.TileHeight)).(*ebiten.Image)
	}
	for _, tsTile := range ts.Tiles {
		if len(tsTile.Animation) > 0 {
			gid := ts.FirstGID + tsTile.ID
			tileMap.Tiles[gid] = &Tile{
				ID:         gid,
				Properties: make(map[string]interface{}),
				AnimationInfo: &AnimationInfo{
					Frames:       tsTile.Animation,
					CurrentFrame: 0,
					ElapsedTime:  0,
					BaseGID:      ts.FirstGID,
				},
			}
		}
	}
}

func (tmm *TilemapsManager) SetTilemap(name string, scale float64) error {
	Mecs.RemoveEntities()
	tmm.mu.Lock()
	defer tmm.mu.Unlock()
	if _, ok := tmm.tileMaps[name]; !ok {
		return fmt.Errorf("tilemap %s not found", name)
	}
	tmm.currentTileMap = name
	tmm.scale = scale
	tm := tmm.tileMaps[name]
	tm.Objects = make(map[int]Object)
	for _, tsLayer := range tm.Map.Layers {
		if tsLayer.Name == "entities" {
			for _, obj := range tsLayer.Objects {
				scaledObj := Object{
					ID:       obj.ID,
					Name:     obj.Name,
					Type:     obj.Type,
					X:        obj.X * scale,
					Y:        obj.Y * scale,
					Width:    obj.Width * scale,
					Height:   obj.Height * scale,
					RawProps: obj.RawProps,
					GID:      obj.GID,
				}
				tm.Objects[obj.ID] = scaledObj
			}
			for _, obj := range tm.Objects {
				if callback, ok := tmm.callbacks[obj.Type]; ok {
					callback(obj)
				} else if obj.Type != "" {
					tmm.logger.Error("callback not found: ", obj.Type)
				}
			}
		}
		if tsLayer.Name == "collider" {
			for _, obj := range tsLayer.Objects {
				Mphysics.AddCollider(obj.X*scale, obj.Y*scale, obj.Width*scale, obj.Height*scale)
			}
		}
	}
	return nil
}

func (tmm *TilemapsManager) GetObject(id float64) Object {
	tmm.mu.Lock()
	defer tmm.mu.Unlock()
	if obj, exists := tmm.tileMaps[tmm.currentTileMap].Objects[int(id)]; exists {
		return obj
	}
	return Object{}
}

func (tmm *TilemapsManager) GetTilemap() string {
	return tmm.currentTileMap
}

func (tmm *TilemapsManager) UpdateTilemap(delta float64) {
	tmm.mu.Lock()
	defer tmm.mu.Unlock()
	if tmm.currentTileMap == "" {
		return
	}
	tm := tmm.tileMaps[tmm.currentTileMap]
	for _, tile := range tm.Tiles {
		if tile.AnimationInfo == nil {
			continue
		}
		tile.AnimationInfo.ElapsedTime += delta * 1000
		frame := tile.AnimationInfo.Frames[tile.AnimationInfo.CurrentFrame]
		if tile.AnimationInfo.ElapsedTime >= float64(frame.Duration) {
			tile.AnimationInfo.ElapsedTime -= float64(frame.Duration)
			tile.AnimationInfo.CurrentFrame = (tile.AnimationInfo.CurrentFrame + 1) % len(tile.AnimationInfo.Frames)
		}
	}
}

func (tmm *TilemapsManager) DrawTilemap(screen *ebiten.Image, offsetX, offsetY float64, filterOut bool, layers ...string) {
	tmm.mu.Lock()
	defer tmm.mu.Unlock()
	if tmm.currentTileMap == "" {
		return
	}
	tm := tmm.tileMaps[tmm.currentTileMap]
	scale := tmm.scale
	layerMap := make(map[string]bool)
	for _, layer := range layers {
		layerMap[layer] = true
	}
	for _, layer := range tm.Map.Layers {
		if len(layers) == 0 || (filterOut && !layerMap[layer.Name]) || (!filterOut && layerMap[layer.Name]) {
			tmm.drawLayerRecursive(screen, tm, &layer, offsetX, offsetY, scale, filterOut, layerMap)
		}
	}
}

func (tmm *TilemapsManager) drawLayerRecursive(screen *ebiten.Image, tm *TileMap, layer *Layer, offsetX, offsetY, scale float64, filterOut bool, layerMap map[string]bool) {
	if layer.Visible && (len(layerMap) == 0 || (filterOut && !layerMap[layer.Name]) || (!filterOut && layerMap[layer.Name])) {
		if layer.Type == "tilelayer" {
			tmm.drawLayer(screen, tm, layer, offsetX, offsetY, scale)
		} else if layer.Type == "objectgroup" {
			tmm.drawObjects(screen, tm, layer, offsetX, offsetY, scale)
		}
	}
	for _, subLayer := range layer.Layers {
		tmm.drawLayerRecursive(screen, tm, &subLayer, offsetX, offsetY, scale, filterOut, layerMap)
	}
}

func (tmm *TilemapsManager) drawObjects(screen *ebiten.Image, tm *TileMap, layer *Layer, offsetX, offsetY, scale float64) {
	for _, obj := range layer.Objects {
		if obj.GID > 0 {
			currentGID := obj.GID
			if tile, exists := tm.Tiles[obj.GID]; exists && tile.AnimationInfo != nil {
				currentFrame := tile.AnimationInfo.Frames[tile.AnimationInfo.CurrentFrame]
				currentGID = tile.AnimationInfo.BaseGID + currentFrame.TileID
			}
			if img, ok := tm.CachedTiles[currentGID]; ok {
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(scale, scale)
				op.GeoM.Translate(offsetX+obj.X, offsetY+obj.Y-float64(tm.Map.TileHeight)*scale)
				screen.DrawImage(img, op)
			}
		}
	}
}

func (tmm *TilemapsManager) drawLayer(screen *ebiten.Image, tm *TileMap, layer *Layer, offsetX, offsetY, scale float64) {
	tw, th := tm.Map.TileWidth, tm.Map.TileHeight
	for y := 0; y < layer.Height; y++ {
		for x := 0; x < layer.Width; x++ {
			idx := y*layer.Width + x
			if idx >= len(layer.Data) {
				continue
			}
			originalGID := layer.Data[idx]
			if originalGID == 0 {
				continue
			}
			currentGID := originalGID
			if tile, exists := tm.Tiles[originalGID]; exists && tile.AnimationInfo != nil {
				currentFrame := tile.AnimationInfo.Frames[tile.AnimationInfo.CurrentFrame]
				currentGID = tile.AnimationInfo.BaseGID + currentFrame.TileID
			}
			sub, ok := tm.CachedTiles[currentGID]
			if !ok {
				continue
			}
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(scale, scale)
			op.GeoM.Translate(offsetX+float64(x*tw)*scale, offsetY+float64(y*th)*scale)
			op.ColorM.Scale(1, 1, 1, layer.Opacity)
			screen.DrawImage(sub, op)
		}
	}
}
