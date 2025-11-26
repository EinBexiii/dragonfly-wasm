package host

import (
	"context"
	"sync"

	extism "github.com/extism/go-sdk"
	"go.uber.org/zap"
)

const (
	fnLog              = "host_log"
	fnGetPlayer        = "host_get_player"
	fnGetOnlinePlayers = "host_get_online_players"
	fnSendMessage      = "host_send_message"
	fnBroadcast        = "host_broadcast"
	fnKickPlayer       = "host_kick_player"
	fnTeleportPlayer   = "host_teleport_player"
	fnSetPlayerHealth  = "host_set_player_health"
	fnSetPlayerGamemode = "host_set_player_gamemode"
	fnGiveItem         = "host_give_item"
	fnGetBlock         = "host_get_block"
	fnSetBlock         = "host_set_block"
	fnStorageGet       = "host_storage_get"
	fnStorageSet       = "host_storage_set"
	fnStorageDelete    = "host_storage_delete"
	fnScheduleTask     = "host_schedule_task"
	fnCancelTask       = "host_cancel_task"
)

type ServerAPI interface {
	GetPlayer(uuid string) (PlayerInfo, bool)
	GetPlayerByName(name string) (PlayerInfo, bool)
	GetOnlinePlayers() []PlayerInfo
	SendMessage(playerUUID, message string) error
	BroadcastMessage(message string)
	KickPlayer(uuid, reason string) error
	TeleportPlayer(uuid string, x, y, z float64, world string) error
	SetPlayerHealth(uuid string, health float32) error
	SetPlayerGameMode(uuid string, gameMode int32) error
	GiveItem(uuid, itemType string, count int32, metadata map[string]string) error
	GetBlock(world string, x, y, z int32) (BlockInfo, error)
	SetBlock(world string, x, y, z int32, blockType string, properties map[string]string) error
	StorageGet(pluginID, key string) ([]byte, bool)
	StorageSet(pluginID, key string, value []byte) error
	StorageDelete(pluginID, key string) error
	ScheduleTask(pluginID, taskID string, delayMs int64, data []byte) error
	CancelTask(pluginID, taskID string) error
}

type PlayerInfo struct {
	UUID      string
	Name      string
	XUID      string
	X, Y, Z   float64
	Yaw       float32
	Pitch     float32
	WorldName string
	GameMode  int32
	Health    float32
	MaxHealth float32
}

type BlockInfo struct {
	BlockType  string
	X, Y, Z    int32
	Properties map[string]string
}

type FunctionProvider struct {
	mu     sync.RWMutex
	api    ServerAPI
	logger *zap.Logger
}

func NewFunctionProvider(api ServerAPI, logger *zap.Logger) *FunctionProvider {
	return &FunctionProvider{api: api, logger: logger}
}

type PluginContext struct {
	PluginID string
	Provider *FunctionProvider
}

func (p *FunctionProvider) CreateHostFunctions(pluginID string) []extism.HostFunction {
	ctx := &PluginContext{PluginID: pluginID, Provider: p}
	return []extism.HostFunction{
		p.createLogFunction(ctx),
		p.createGetPlayerFunction(ctx),
		p.createGetOnlinePlayersFunction(ctx),
		p.createSendMessageFunction(ctx),
		p.createBroadcastFunction(ctx),
		p.createKickPlayerFunction(ctx),
		p.createTeleportPlayerFunction(ctx),
		p.createSetPlayerHealthFunction(ctx),
		p.createSetPlayerGameModeFunction(ctx),
		p.createGiveItemFunction(ctx),
		p.createGetBlockFunction(ctx),
		p.createSetBlockFunction(ctx),
		p.createStorageGetFunction(ctx),
		p.createStorageSetFunction(ctx),
		p.createStorageDeleteFunction(ctx),
		p.createScheduleTaskFunction(ctx),
		p.createCancelTaskFunction(ctx),
	}
}

func (p *FunctionProvider) createLogFunction(ctx *PluginContext) extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		fnLog,
		func(_ context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
			level, _ := plugin.ReadString(stack[0])
			message, _ := plugin.ReadString(stack[1])
			field := zap.String("plugin", ctx.PluginID)
			switch level {
			case "debug":
				p.logger.Debug(message, field)
			case "warn":
				p.logger.Warn(message, field)
			case "error":
				p.logger.Error(message, field)
			default:
				p.logger.Info(message, field)
			}
		},
		[]extism.ValueType{extism.ValueTypePTR, extism.ValueTypePTR},
		[]extism.ValueType{},
	)
}

func (p *FunctionProvider) createGetPlayerFunction(ctx *PluginContext) extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		fnGetPlayer,
		func(_ context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
			uuid, _ := plugin.ReadString(stack[0])
			playerInfo, found := p.api.GetPlayer(uuid)
			if !found {
				stack[0] = 0
				return
			}
			offset, _ := plugin.WriteBytes(serializePlayerInfo(playerInfo))
			stack[0] = offset
		},
		[]extism.ValueType{extism.ValueTypePTR},
		[]extism.ValueType{extism.ValueTypePTR},
	)
}

func (p *FunctionProvider) createGetOnlinePlayersFunction(ctx *PluginContext) extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		fnGetOnlinePlayers,
		func(_ context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
			offset, _ := plugin.WriteBytes(serializePlayerInfoList(p.api.GetOnlinePlayers()))
			stack[0] = offset
		},
		[]extism.ValueType{},
		[]extism.ValueType{extism.ValueTypePTR},
	)
}

func (p *FunctionProvider) createSendMessageFunction(ctx *PluginContext) extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		fnSendMessage,
		func(_ context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
			uuid, _ := plugin.ReadString(stack[0])
			message, _ := plugin.ReadString(stack[1])
			stack[0] = boolToUint64(p.api.SendMessage(uuid, message) == nil)
		},
		[]extism.ValueType{extism.ValueTypePTR, extism.ValueTypePTR},
		[]extism.ValueType{extism.ValueTypeI32},
	)
}

func (p *FunctionProvider) createBroadcastFunction(ctx *PluginContext) extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		fnBroadcast,
		func(_ context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
			message, _ := plugin.ReadString(stack[0])
			p.api.BroadcastMessage(message)
		},
		[]extism.ValueType{extism.ValueTypePTR},
		[]extism.ValueType{},
	)
}

func (p *FunctionProvider) createKickPlayerFunction(ctx *PluginContext) extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		fnKickPlayer,
		func(_ context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
			uuid, _ := plugin.ReadString(stack[0])
			reason, _ := plugin.ReadString(stack[1])
			stack[0] = boolToUint64(p.api.KickPlayer(uuid, reason) == nil)
		},
		[]extism.ValueType{extism.ValueTypePTR, extism.ValueTypePTR},
		[]extism.ValueType{extism.ValueTypeI32},
	)
}

func (p *FunctionProvider) createTeleportPlayerFunction(ctx *PluginContext) extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		fnTeleportPlayer,
		func(_ context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
			data, _ := plugin.ReadBytes(stack[0])
			req := deserializeTeleportRequest(data)
			stack[0] = boolToUint64(p.api.TeleportPlayer(req.UUID, req.X, req.Y, req.Z, req.World) == nil)
		},
		[]extism.ValueType{extism.ValueTypePTR},
		[]extism.ValueType{extism.ValueTypeI32},
	)
}

func (p *FunctionProvider) createSetPlayerHealthFunction(ctx *PluginContext) extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		fnSetPlayerHealth,
		func(_ context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
			uuid, _ := plugin.ReadString(stack[0])
			health := float32(stack[1])
			stack[0] = boolToUint64(p.api.SetPlayerHealth(uuid, health) == nil)
		},
		[]extism.ValueType{extism.ValueTypePTR, extism.ValueTypeF32},
		[]extism.ValueType{extism.ValueTypeI32},
	)
}

func (p *FunctionProvider) createSetPlayerGameModeFunction(ctx *PluginContext) extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		fnSetPlayerGamemode,
		func(_ context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
			uuid, _ := plugin.ReadString(stack[0])
			gameMode := int32(stack[1])
			stack[0] = boolToUint64(p.api.SetPlayerGameMode(uuid, gameMode) == nil)
		},
		[]extism.ValueType{extism.ValueTypePTR, extism.ValueTypeI32},
		[]extism.ValueType{extism.ValueTypeI32},
	)
}

func (p *FunctionProvider) createGiveItemFunction(ctx *PluginContext) extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		fnGiveItem,
		func(_ context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
			data, _ := plugin.ReadBytes(stack[0])
			req := deserializeGiveItemRequest(data)
			stack[0] = boolToUint64(p.api.GiveItem(req.UUID, req.ItemType, req.Count, req.Metadata) == nil)
		},
		[]extism.ValueType{extism.ValueTypePTR},
		[]extism.ValueType{extism.ValueTypeI32},
	)
}

func (p *FunctionProvider) createGetBlockFunction(ctx *PluginContext) extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		fnGetBlock,
		func(_ context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
			data, _ := plugin.ReadBytes(stack[0])
			req := deserializeGetBlockRequest(data)
			block, err := p.api.GetBlock(req.World, req.X, req.Y, req.Z)
			if err != nil {
				stack[0] = 0
				return
			}
			offset, _ := plugin.WriteBytes(serializeBlockInfo(block))
			stack[0] = offset
		},
		[]extism.ValueType{extism.ValueTypePTR},
		[]extism.ValueType{extism.ValueTypePTR},
	)
}

func (p *FunctionProvider) createSetBlockFunction(ctx *PluginContext) extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		fnSetBlock,
		func(_ context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
			data, _ := plugin.ReadBytes(stack[0])
			req := deserializeSetBlockRequest(data)
			stack[0] = boolToUint64(p.api.SetBlock(req.World, req.X, req.Y, req.Z, req.BlockType, req.Properties) == nil)
		},
		[]extism.ValueType{extism.ValueTypePTR},
		[]extism.ValueType{extism.ValueTypeI32},
	)
}

func (p *FunctionProvider) createStorageGetFunction(ctx *PluginContext) extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		fnStorageGet,
		func(_ context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
			key, _ := plugin.ReadString(stack[0])
			value, found := p.api.StorageGet(ctx.PluginID, key)
			if !found {
				stack[0] = 0
				return
			}
			offset, _ := plugin.WriteBytes(value)
			stack[0] = offset
		},
		[]extism.ValueType{extism.ValueTypePTR},
		[]extism.ValueType{extism.ValueTypePTR},
	)
}

func (p *FunctionProvider) createStorageSetFunction(ctx *PluginContext) extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		fnStorageSet,
		func(_ context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
			key, _ := plugin.ReadString(stack[0])
			value, _ := plugin.ReadBytes(stack[1])
			stack[0] = boolToUint64(p.api.StorageSet(ctx.PluginID, key, value) == nil)
		},
		[]extism.ValueType{extism.ValueTypePTR, extism.ValueTypePTR},
		[]extism.ValueType{extism.ValueTypeI32},
	)
}

func (p *FunctionProvider) createStorageDeleteFunction(ctx *PluginContext) extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		fnStorageDelete,
		func(_ context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
			key, _ := plugin.ReadString(stack[0])
			stack[0] = boolToUint64(p.api.StorageDelete(ctx.PluginID, key) == nil)
		},
		[]extism.ValueType{extism.ValueTypePTR},
		[]extism.ValueType{extism.ValueTypeI32},
	)
}

func (p *FunctionProvider) createScheduleTaskFunction(ctx *PluginContext) extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		fnScheduleTask,
		func(_ context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
			data, _ := plugin.ReadBytes(stack[0])
			req := deserializeScheduleTaskRequest(data)
			stack[0] = boolToUint64(p.api.ScheduleTask(ctx.PluginID, req.TaskID, req.DelayMs, req.Data) == nil)
		},
		[]extism.ValueType{extism.ValueTypePTR},
		[]extism.ValueType{extism.ValueTypeI32},
	)
}

func (p *FunctionProvider) createCancelTaskFunction(ctx *PluginContext) extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		fnCancelTask,
		func(_ context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
			taskID, _ := plugin.ReadString(stack[0])
			stack[0] = boolToUint64(p.api.CancelTask(ctx.PluginID, taskID) == nil)
		},
		[]extism.ValueType{extism.ValueTypePTR},
		[]extism.ValueType{extism.ValueTypeI32},
	)
}

func boolToUint64(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}
