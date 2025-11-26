package manager

import (
	"context"
	"encoding/json"

	extism "github.com/extism/go-sdk"
)

type logRequest struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

type broadcastRequest struct {
	Message string `json:"message"`
}

type sendMessageRequest struct {
	PlayerUUID string `json:"player_uuid"`
	Message    string `json:"message"`
}

type getPlayerRequest struct {
	PlayerUUID string `json:"player_uuid"`
}

type playerResponse struct {
	UUID      string   `json:"uuid"`
	Name      string   `json:"name"`
	WorldName string   `json:"world_name"`
	Position  position `json:"position"`
	Error     string   `json:"error,omitempty"`
}

type position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type blockPos struct {
	X int `json:"x"`
	Y int `json:"y"`
	Z int `json:"z"`
}

type playersResponse struct {
	Players []playerResponse `json:"players"`
}

type teleportRequest struct {
	PlayerUUID string   `json:"player_uuid"`
	Position   position `json:"position"`
	WorldName  string   `json:"world_name"`
}

type kickRequest struct {
	PlayerUUID string `json:"player_uuid"`
	Reason     string `json:"reason"`
}

type setHealthRequest struct {
	PlayerUUID string  `json:"player_uuid"`
	Health     float32 `json:"health"`
}

type setGamemodeRequest struct {
	PlayerUUID string `json:"player_uuid"`
	Gamemode   int32  `json:"gamemode"`
}

type getBlockRequest struct {
	WorldName string   `json:"world_name"`
	Position  blockPos `json:"position"`
}

type blockResponse struct {
	BlockType  string            `json:"block_type"`
	Position   blockPos          `json:"position"`
	Properties map[string]string `json:"properties"`
	Error      string            `json:"error,omitempty"`
}

type setBlockRequest struct {
	WorldName  string            `json:"world_name"`
	Position   blockPos          `json:"position"`
	BlockType  string            `json:"block_type"`
	Properties map[string]string `json:"properties"`
}

func (m *Manager) createHostFunctions() []extism.HostFunction {
	return []extism.HostFunction{
		m.hostLog(),
		m.hostBroadcast(),
		m.hostSendMessage(),
		m.hostGetPlayer(),
		m.hostGetOnlinePlayers(),
		m.hostTeleportPlayer(),
		m.hostKickPlayer(),
		m.hostSetPlayerHealth(),
		m.hostSetPlayerGamemode(),
		m.hostGetBlock(),
		m.hostSetBlock(),
	}
}

func (m *Manager) hostLog() extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"host_log",
		func(_ context.Context, p *extism.CurrentPlugin, stack []uint64) {
			data, err := p.ReadBytes(stack[0])
			if err != nil {
				return
			}

			var req logRequest
			if err := json.Unmarshal(data, &req); err != nil {
				return
			}

			switch req.Level {
			case "debug":
				m.logger.Debug(req.Message)
			case "warn":
				m.logger.Warn(req.Message)
			case "error":
				m.logger.Error(req.Message)
			default:
				m.logger.Info(req.Message)
			}
		},
		[]extism.ValueType{extism.ValueTypeI64},
		[]extism.ValueType{},
	)
}

func (m *Manager) hostBroadcast() extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"host_broadcast",
		func(_ context.Context, p *extism.CurrentPlugin, stack []uint64) {
			data, err := p.ReadBytes(stack[0])
			if err != nil {
				stack[0] = 0
				return
			}

			var req broadcastRequest
			if err := json.Unmarshal(data, &req); err != nil {
				stack[0] = 0
				return
			}

			if m.serverAPI != nil {
				m.serverAPI.BroadcastMessage(req.Message)
			}
			stack[0] = 1
		},
		[]extism.ValueType{extism.ValueTypeI64},
		[]extism.ValueType{extism.ValueTypeI64},
	)
}

func (m *Manager) hostSendMessage() extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"host_send_message",
		func(_ context.Context, p *extism.CurrentPlugin, stack []uint64) {
			data, err := p.ReadBytes(stack[0])
			if err != nil {
				stack[0] = 0
				return
			}

			var req sendMessageRequest
			if err := json.Unmarshal(data, &req); err != nil {
				stack[0] = 0
				return
			}

			if m.serverAPI == nil {
				stack[0] = 0
				return
			}

			player, ok := m.serverAPI.GetPlayer(req.PlayerUUID)
			if !ok {
				stack[0] = 0
				return
			}

			player.SendMessage(req.Message)
			stack[0] = 1
		},
		[]extism.ValueType{extism.ValueTypeI64},
		[]extism.ValueType{extism.ValueTypeI64},
	)
}

func (m *Manager) hostGetPlayer() extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"host_get_player",
		func(_ context.Context, p *extism.CurrentPlugin, stack []uint64) {
			data, err := p.ReadBytes(stack[0])
			if err != nil {
				stack[0] = writeError(p, "failed to read input")
				return
			}

			var req getPlayerRequest
			if err := json.Unmarshal(data, &req); err != nil {
				stack[0] = writeError(p, "failed to parse request")
				return
			}

			if m.serverAPI == nil {
				stack[0] = writeError(p, "server API not available")
				return
			}

			player, ok := m.serverAPI.GetPlayer(req.PlayerUUID)
			if !ok {
				stack[0] = writeError(p, "player not found")
				return
			}

			x, y, z := player.Position()
			var worldName string
			if w := player.World(); w != nil {
				worldName = w.Name()
			}

			stack[0] = writeJSON(p, playerResponse{
				UUID:      player.UUID(),
				Name:      player.Name(),
				WorldName: worldName,
				Position:  position{X: x, Y: y, Z: z},
			})
		},
		[]extism.ValueType{extism.ValueTypeI64},
		[]extism.ValueType{extism.ValueTypeI64},
	)
}

func (m *Manager) hostGetOnlinePlayers() extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"host_get_online_players",
		func(_ context.Context, p *extism.CurrentPlugin, stack []uint64) {
			if m.serverAPI == nil {
				stack[0] = writeError(p, "server API not available")
				return
			}

			players := m.serverAPI.GetAllPlayers()
			resp := playersResponse{Players: make([]playerResponse, 0, len(players))}

			for _, pl := range players {
				x, y, z := pl.Position()
				var worldName string
				if w := pl.World(); w != nil {
					worldName = w.Name()
				}
				resp.Players = append(resp.Players, playerResponse{
					UUID:      pl.UUID(),
					Name:      pl.Name(),
					WorldName: worldName,
					Position:  position{X: x, Y: y, Z: z},
				})
			}

			stack[0] = writeJSON(p, resp)
		},
		[]extism.ValueType{},
		[]extism.ValueType{extism.ValueTypeI64},
	)
}

func (m *Manager) hostTeleportPlayer() extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"host_teleport_player",
		func(_ context.Context, p *extism.CurrentPlugin, stack []uint64) {
			data, err := p.ReadBytes(stack[0])
			if err != nil {
				stack[0] = 0
				return
			}

			var req teleportRequest
			if err := json.Unmarshal(data, &req); err != nil {
				stack[0] = 0
				return
			}

			if m.serverAPI == nil {
				stack[0] = 0
				return
			}

			player, ok := m.serverAPI.GetPlayer(req.PlayerUUID)
			if !ok {
				stack[0] = 0
				return
			}

			if err := player.Teleport(req.Position.X, req.Position.Y, req.Position.Z, req.WorldName); err != nil {
				stack[0] = 0
				return
			}
			stack[0] = 1
		},
		[]extism.ValueType{extism.ValueTypeI64},
		[]extism.ValueType{extism.ValueTypeI64},
	)
}

func (m *Manager) hostKickPlayer() extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"host_kick_player",
		func(_ context.Context, p *extism.CurrentPlugin, stack []uint64) {
			data, err := p.ReadBytes(stack[0])
			if err != nil {
				stack[0] = 0
				return
			}

			var req kickRequest
			if err := json.Unmarshal(data, &req); err != nil {
				stack[0] = 0
				return
			}

			if m.serverAPI == nil {
				stack[0] = 0
				return
			}

			player, ok := m.serverAPI.GetPlayer(req.PlayerUUID)
			if !ok {
				stack[0] = 0
				return
			}

			player.Kick(req.Reason)
			stack[0] = 1
		},
		[]extism.ValueType{extism.ValueTypeI64},
		[]extism.ValueType{extism.ValueTypeI64},
	)
}

func (m *Manager) hostSetPlayerHealth() extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"host_set_player_health",
		func(_ context.Context, p *extism.CurrentPlugin, stack []uint64) {
			data, err := p.ReadBytes(stack[0])
			if err != nil {
				stack[0] = 0
				return
			}

			var req setHealthRequest
			if err := json.Unmarshal(data, &req); err != nil {
				stack[0] = 0
				return
			}

			if m.serverAPI == nil {
				stack[0] = 0
				return
			}

			player, ok := m.serverAPI.GetPlayer(req.PlayerUUID)
			if !ok {
				stack[0] = 0
				return
			}

			player.SetHealth(float64(req.Health))
			stack[0] = 1
		},
		[]extism.ValueType{extism.ValueTypeI64},
		[]extism.ValueType{extism.ValueTypeI64},
	)
}

func (m *Manager) hostSetPlayerGamemode() extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"host_set_player_gamemode",
		func(_ context.Context, p *extism.CurrentPlugin, stack []uint64) {
			data, err := p.ReadBytes(stack[0])
			if err != nil {
				stack[0] = 0
				return
			}

			var req setGamemodeRequest
			if err := json.Unmarshal(data, &req); err != nil {
				stack[0] = 0
				return
			}

			if m.serverAPI == nil {
				stack[0] = 0
				return
			}

			player, ok := m.serverAPI.GetPlayer(req.PlayerUUID)
			if !ok {
				stack[0] = 0
				return
			}

			player.SetGameMode(int(req.Gamemode))
			stack[0] = 1
		},
		[]extism.ValueType{extism.ValueTypeI64},
		[]extism.ValueType{extism.ValueTypeI64},
	)
}

func (m *Manager) hostGetBlock() extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"host_get_block",
		func(_ context.Context, p *extism.CurrentPlugin, stack []uint64) {
			data, err := p.ReadBytes(stack[0])
			if err != nil {
				stack[0] = writeError(p, "failed to read input")
				return
			}

			var req getBlockRequest
			if err := json.Unmarshal(data, &req); err != nil {
				stack[0] = writeError(p, "failed to parse request")
				return
			}

			if m.serverAPI == nil {
				stack[0] = writeError(p, "server API not available")
				return
			}

			world, ok := m.serverAPI.GetWorld(req.WorldName)
			if !ok {
				world = m.serverAPI.GetDefaultWorld()
			}

			blockType, properties := world.GetBlock(req.Position.X, req.Position.Y, req.Position.Z)

			stack[0] = writeJSON(p, blockResponse{
				BlockType:  blockType,
				Position:   req.Position,
				Properties: properties,
			})
		},
		[]extism.ValueType{extism.ValueTypeI64},
		[]extism.ValueType{extism.ValueTypeI64},
	)
}

func (m *Manager) hostSetBlock() extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"host_set_block",
		func(_ context.Context, p *extism.CurrentPlugin, stack []uint64) {
			data, err := p.ReadBytes(stack[0])
			if err != nil {
				stack[0] = 0
				return
			}

			var req setBlockRequest
			if err := json.Unmarshal(data, &req); err != nil {
				stack[0] = 0
				return
			}

			if m.serverAPI == nil {
				stack[0] = 0
				return
			}

			world, ok := m.serverAPI.GetWorld(req.WorldName)
			if !ok {
				world = m.serverAPI.GetDefaultWorld()
			}

			if err := world.SetBlock(req.Position.X, req.Position.Y, req.Position.Z, req.BlockType, req.Properties); err != nil {
				stack[0] = 0
				return
			}
			stack[0] = 1
		},
		[]extism.ValueType{extism.ValueTypeI64},
		[]extism.ValueType{extism.ValueTypeI64},
	)
}

func writeJSON(p *extism.CurrentPlugin, v any) uint64 {
	data, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	offset, err := p.WriteBytes(data)
	if err != nil {
		return 0
	}
	return offset
}

func writeError(p *extism.CurrentPlugin, msg string) uint64 {
	return writeJSON(p, map[string]string{"error": msg})
}
