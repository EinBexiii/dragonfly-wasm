package handler

import (
	"context"
	"encoding/json"
	"time"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"go.uber.org/zap"

	"github.com/EinBexiii/dragonfly-wasm/pkg/events"
	"github.com/EinBexiii/dragonfly-wasm/pkg/plugin"
)

type PlayerHandler struct {
	player.NopHandler
	dispatcher *events.Dispatcher
	logger     *zap.Logger
	ctx        context.Context
}

func NewPlayerHandler(dispatcher *events.Dispatcher, logger *zap.Logger) *PlayerHandler {
	return &PlayerHandler{
		dispatcher: dispatcher,
		logger:     logger.Named("player-handler"),
		ctx:        context.Background(),
	}
}

type jsonMessage struct{ data []byte }

func (m *jsonMessage) Reset()         {}
func (m *jsonMessage) String() string { return string(m.data) }
func (m *jsonMessage) ProtoMessage()  {}

func playerToMap(p *player.Player) map[string]any {
	pos, rot := p.Position(), p.Rotation()
	var worldName string
	if tx := p.Tx(); tx != nil {
		worldName = tx.World().Name()
	}
	return map[string]any{
		"uuid":       p.UUID().String(),
		"name":       p.Name(),
		"xuid":       p.XUID(),
		"position":   map[string]float64{"x": pos.X(), "y": pos.Y(), "z": pos.Z()},
		"yaw":        rot.Yaw(),
		"pitch":      rot.Pitch(),
		"world_name": worldName,
		"health":     p.Health(),
		"max_health": p.MaxHealth(),
	}
}

func vec3ToMap(v mgl64.Vec3) map[string]float64 {
	return map[string]float64{"x": v.X(), "y": v.Y(), "z": v.Z()}
}

func blockPosToMap(pos cube.Pos) map[string]int {
	return map[string]int{"x": pos.X(), "y": pos.Y(), "z": pos.Z()}
}

func itemToMap(stack item.Stack) map[string]any {
	itemType := "air"
	if !stack.Empty() {
		itemType = itemTypeName(stack.Item())
	}
	return map[string]any{"item_type": itemType, "count": stack.Count()}
}

func itemTypeName(i world.Item) string {
	if i == nil {
		return "air"
	}
	if enc, ok := i.(world.NBTer); ok {
		if name, ok := enc.EncodeNBT()["name"].(string); ok {
			return name
		}
	}
	return "unknown"
}

func blockToMap(b world.Block, pos cube.Pos) map[string]any {
	return map[string]any{"block_type": blockTypeName(b), "position": blockPosToMap(pos)}
}

func blockTypeName(b world.Block) string {
	if b == nil {
		return "air"
	}
	name, _ := b.EncodeBlock()
	return name
}

func entityToMap(e world.Entity) map[string]any {
	pos, rot := e.Position(), e.Rotation()
	data := map[string]any{
		"entity_type": "entity",
		"position":    vec3ToMap(pos),
		"yaw":         rot.Yaw(),
		"pitch":       rot.Pitch(),
	}
	if p, ok := e.(*player.Player); ok {
		data["uuid"] = p.UUID().String()
		data["entity_type"] = "player"
	}
	return data
}

func (h *PlayerHandler) dispatchEvent(eventType plugin.EventType, data map[string]any) (bool, map[string]string) {
	if !h.dispatcher.HasSubscribers(eventType) {
		return false, nil
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		h.logger.Error("marshal event data", zap.Error(err))
		return false, nil
	}

	result, err := h.dispatcher.Dispatch(h.ctx, eventType, &jsonMessage{data: jsonData})
	if err != nil {
		h.logger.Error("dispatch event", zap.String("event", string(eventType)), zap.Error(err))
		return false, nil
	}

	if result != nil {
		return result.Cancelled, result.Modifications
	}
	return false, nil
}

func (h *PlayerHandler) HandleChat(ctx *player.Context, message *string) {
	cancelled, mods := h.dispatchEvent(plugin.EventPlayerChat, map[string]any{
		"player":  playerToMap(ctx.Val()),
		"message": *message,
	})
	if cancelled {
		ctx.Cancel()
	}
	if newMsg, ok := mods["message"]; ok {
		*message = newMsg
	}
}

func (h *PlayerHandler) HandleMove(ctx *player.Context, newPos mgl64.Vec3, newRot cube.Rotation) {
	cancelled, _ := h.dispatchEvent(plugin.EventPlayerMove, map[string]any{
		"new_position": vec3ToMap(newPos),
		"new_yaw":      newRot.Yaw(),
		"new_pitch":    newRot.Pitch(),
	})
	if cancelled {
		ctx.Cancel()
	}
}

func (h *PlayerHandler) HandleTeleport(ctx *player.Context, pos mgl64.Vec3) {
	if cancelled, _ := h.dispatchEvent(plugin.EventPlayerTeleport, map[string]any{"to": vec3ToMap(pos)}); cancelled {
		ctx.Cancel()
	}
}

func (h *PlayerHandler) HandleJump(p *player.Player) {
	h.dispatchEvent(plugin.EventPlayerJump, map[string]any{"player": playerToMap(p)})
}

func (h *PlayerHandler) HandleToggleSprint(ctx *player.Context, after bool) {
	if cancelled, _ := h.dispatchEvent(plugin.EventPlayerSprint, map[string]any{
		"player": playerToMap(ctx.Val()), "sprinting": after,
	}); cancelled {
		ctx.Cancel()
	}
}

func (h *PlayerHandler) HandleToggleSneak(ctx *player.Context, after bool) {
	if cancelled, _ := h.dispatchEvent(plugin.EventPlayerSneak, map[string]any{
		"player": playerToMap(ctx.Val()), "sneaking": after,
	}); cancelled {
		ctx.Cancel()
	}
}

func (h *PlayerHandler) HandleDeath(p *player.Player, src world.DamageSource, keepInv *bool) {
	_, mods := h.dispatchEvent(plugin.EventPlayerDeath, map[string]any{
		"player":         playerToMap(p),
		"damage_source":  damageSourceToString(src),
		"keep_inventory": *keepInv,
	})
	if keep, ok := mods["keep_inventory"]; ok {
		*keepInv = keep == "true"
	}
}

func (h *PlayerHandler) HandleRespawn(p *player.Player, pos *mgl64.Vec3, w **world.World) {
	h.dispatchEvent(plugin.EventPlayerRespawn, map[string]any{
		"player":         playerToMap(p),
		"spawn_position": vec3ToMap(*pos),
	})
}

func (h *PlayerHandler) HandleHurt(ctx *player.Context, damage *float64, immune bool, attackImmunity *time.Duration, src world.DamageSource) {
	eventData := map[string]any{
		"player":        playerToMap(ctx.Val()),
		"damage":        *damage,
		"immune":        immune,
		"damage_source": damageSourceToString(src),
	}
	if attacker, ok := src.(entity.AttackDamageSource); ok && attacker.Attacker != nil {
		eventData["attacker"] = entityToMap(attacker.Attacker)
	}
	if cancelled, _ := h.dispatchEvent(plugin.EventPlayerHurt, eventData); cancelled {
		ctx.Cancel()
	}
}

func (h *PlayerHandler) HandleHeal(ctx *player.Context, health *float64, src world.HealingSource) {
	if cancelled, _ := h.dispatchEvent(plugin.EventPlayerHeal, map[string]any{
		"player":      playerToMap(ctx.Val()),
		"amount":      *health,
		"heal_source": healingSourceToString(src),
	}); cancelled {
		ctx.Cancel()
	}
}

func (h *PlayerHandler) HandleBlockBreak(ctx *player.Context, pos cube.Pos, drops *[]item.Stack, xp *int) {
	dropsData := make([]map[string]any, len(*drops))
	for i, stack := range *drops {
		dropsData[i] = itemToMap(stack)
	}

	var blockType string
	if tx := ctx.Val().Tx(); tx != nil {
		blockType = blockTypeName(tx.Block(pos))
	}

	if cancelled, _ := h.dispatchEvent(plugin.EventBlockBreak, map[string]any{
		"player":     playerToMap(ctx.Val()),
		"block":      map[string]any{"block_type": blockType, "position": blockPosToMap(pos)},
		"drops":      dropsData,
		"experience": *xp,
	}); cancelled {
		ctx.Cancel()
	}
}

func (h *PlayerHandler) HandleBlockPlace(ctx *player.Context, pos cube.Pos, b world.Block) {
	if cancelled, _ := h.dispatchEvent(plugin.EventBlockPlace, map[string]any{
		"player": playerToMap(ctx.Val()),
		"block":  blockToMap(b, pos),
	}); cancelled {
		ctx.Cancel()
	}
}

func (h *PlayerHandler) HandleItemUse(ctx *player.Context) {
	if cancelled, _ := h.dispatchEvent(plugin.EventItemUse, map[string]any{"player": playerToMap(ctx.Val())}); cancelled {
		ctx.Cancel()
	}
}

func (h *PlayerHandler) HandleItemUseOnBlock(ctx *player.Context, pos cube.Pos, face cube.Face, clickPos mgl64.Vec3) {
	if cancelled, _ := h.dispatchEvent(plugin.EventItemUseOnBlock, map[string]any{
		"player":         playerToMap(ctx.Val()),
		"position":       blockPosToMap(pos),
		"face":           int(face),
		"click_position": vec3ToMap(clickPos),
	}); cancelled {
		ctx.Cancel()
	}
}

func (h *PlayerHandler) HandleItemUseOnEntity(ctx *player.Context, e world.Entity) {
	if cancelled, _ := h.dispatchEvent(plugin.EventItemUseOnEntity, map[string]any{
		"player": playerToMap(ctx.Val()),
		"target": entityToMap(e),
	}); cancelled {
		ctx.Cancel()
	}
}

func damageSourceToString(src world.DamageSource) string {
	switch src.(type) {
	case entity.AttackDamageSource:
		return "attack"
	case entity.FallDamageSource:
		return "fall"
	case entity.GlideDamageSource:
		return "glide"
	case entity.VoidDamageSource:
		return "void"
	case entity.SuffocationDamageSource:
		return "suffocation"
	case entity.DrowningDamageSource:
		return "drowning"
	case entity.ExplosionDamageSource:
		return "explosion"
	case entity.LightningDamageSource:
		return "lightning"
	case entity.ProjectileDamageSource:
		return "projectile"
	default:
		return "unknown"
	}
}

func healingSourceToString(src world.HealingSource) string {
	if _, ok := src.(entity.FoodHealingSource); ok {
		return "food"
	}
	return "unknown"
}
