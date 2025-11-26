// Package events provides event conversion utilities.
package events

import (
	"reflect"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

// Converter provides utilities for converting between Dragonfly types and protobuf messages.
type Converter struct{}

// NewConverter creates a new Converter.
func NewConverter() *Converter {
	return &Converter{}
}

// Vector3FromMgl converts an mgl64.Vec3 to a Vector3 proto message fields.
func (c *Converter) Vector3FromMgl(v mgl64.Vec3) (x, y, z float64) {
	return v[0], v[1], v[2]
}

// Vector3ToMgl converts Vector3 fields to an mgl64.Vec3.
func (c *Converter) Vector3ToMgl(x, y, z float64) mgl64.Vec3 {
	return mgl64.Vec3{x, y, z}
}

// BlockPosFromCube converts a cube.Pos to BlockPos fields.
func (c *Converter) BlockPosFromCube(pos cube.Pos) (x, y, z int32) {
	return int32(pos.X()), int32(pos.Y()), int32(pos.Z())
}

// BlockPosToCube converts BlockPos fields to a cube.Pos.
func (c *Converter) BlockPosToCube(x, y, z int32) cube.Pos {
	return cube.Pos{int(x), int(y), int(z)}
}

// PlayerData contains serializable player information.
type PlayerData struct {
	UUID      string
	Name      string
	XUID      string
	Position  mgl64.Vec3
	Yaw       float32
	Pitch     float32
	WorldName string
	GameMode  int32
	Health    float32
	MaxHealth float32
}

// PlayerFromDragonfly extracts serializable data from a Dragonfly player.
func (c *Converter) PlayerFromDragonfly(p *player.Player) PlayerData {
	pos := p.Position()
	rot := p.Rotation()

	var worldName string
	if tx := p.Tx(); tx != nil {
		worldName = tx.World().Name()
	}

	return PlayerData{
		UUID:      p.UUID().String(),
		Name:      p.Name(),
		XUID:      p.XUID(),
		Position:  pos,
		Yaw:       float32(rot.Yaw()),
		Pitch:     float32(rot.Pitch()),
		WorldName: worldName,
		GameMode:  gameModeToInt(p.GameMode()),
		Health:    float32(p.Health()),
		MaxHealth: float32(p.MaxHealth()),
	}
}

// ItemStackData contains serializable item stack information.
type ItemStackData struct {
	ItemType string
	Count    int32
	Metadata map[string]string
}

// ItemStackFromDragonfly extracts serializable data from a Dragonfly item stack.
func (c *Converter) ItemStackFromDragonfly(stack item.Stack) ItemStackData {
	itemType := "air"
	if !stack.Empty() {
		itemType = itemName(stack.Item())
	}

	return ItemStackData{
		ItemType: itemType,
		Count:    int32(stack.Count()),
		Metadata: make(map[string]string),
	}
}

// BlockData contains serializable block information.
type BlockData struct {
	BlockType  string
	Position   cube.Pos
	Properties map[string]string
}

// BlockFromDragonfly extracts serializable data from a Dragonfly block.
func (c *Converter) BlockFromDragonfly(b world.Block, pos cube.Pos) BlockData {
	return BlockData{
		BlockType:  blockName(b),
		Position:   pos,
		Properties: make(map[string]string),
	}
}

// EntityData contains serializable entity information.
type EntityData struct {
	UUID       string
	EntityType string
	Position   mgl64.Vec3
	Yaw        float32
	Pitch      float32
}

// EntityFromDragonfly extracts serializable data from a Dragonfly entity.
func (c *Converter) EntityFromDragonfly(e world.Entity) EntityData {
	pos := e.Position()
	rot := e.Rotation()

	return EntityData{
		UUID:       entityUUID(e),
		EntityType: entityTypeName(e),
		Position:   pos,
		Yaw:        float32(rot.Yaw()),
		Pitch:      float32(rot.Pitch()),
	}
}

// gameModeToInt converts a Dragonfly GameMode to an integer.
func gameModeToInt(gm world.GameMode) int32 {
	if id, ok := world.GameModeID(gm); ok {
		return int32(id)
	}
	return 0
}

// itemName returns the name of an item.
func itemName(i world.Item) string {
	t := reflect.TypeOf(i)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

// blockName returns the name of a block.
func blockName(b world.Block) string {
	t := reflect.TypeOf(b)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

// entityTypeName returns the type name of an entity.
func entityTypeName(e world.Entity) string {
	t := reflect.TypeOf(e)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

// entityUUID returns a string UUID for an entity.
func entityUUID(e world.Entity) string {
	// Check if entity has a UUID method (like Player).
	if u, ok := e.(interface {
		UUID() interface{ String() string }
	}); ok {
		return u.UUID().String()
	}
	// Generate a deterministic ID based on entity handle.
	return ""
}
