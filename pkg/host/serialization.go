package host

import (
	"encoding/json"
)

// TeleportRequest represents a teleport request from a plugin.
type TeleportRequest struct {
	UUID  string  `json:"uuid"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Z     float64 `json:"z"`
	World string  `json:"world"`
}

// GiveItemRequest represents a give item request from a plugin.
type GiveItemRequest struct {
	UUID     string            `json:"uuid"`
	ItemType string            `json:"item_type"`
	Count    int32             `json:"count"`
	Metadata map[string]string `json:"metadata"`
}

// GetBlockRequest represents a get block request from a plugin.
type GetBlockRequest struct {
	World string `json:"world"`
	X     int32  `json:"x"`
	Y     int32  `json:"y"`
	Z     int32  `json:"z"`
}

// SetBlockRequest represents a set block request from a plugin.
type SetBlockRequest struct {
	World      string            `json:"world"`
	X          int32             `json:"x"`
	Y          int32             `json:"y"`
	Z          int32             `json:"z"`
	BlockType  string            `json:"block_type"`
	Properties map[string]string `json:"properties"`
}

// ScheduleTaskRequest represents a schedule task request from a plugin.
type ScheduleTaskRequest struct {
	TaskID  string `json:"task_id"`
	DelayMs int64  `json:"delay_ms"`
	Data    []byte `json:"data"`
}

// serializePlayerInfo serializes PlayerInfo to JSON bytes.
func serializePlayerInfo(p PlayerInfo) []byte {
	data, _ := json.Marshal(p)
	return data
}

// serializePlayerInfoList serializes a list of PlayerInfo to JSON bytes.
func serializePlayerInfoList(players []PlayerInfo) []byte {
	data, _ := json.Marshal(players)
	return data
}

// serializeBlockInfo serializes BlockInfo to JSON bytes.
func serializeBlockInfo(b BlockInfo) []byte {
	data, _ := json.Marshal(b)
	return data
}

// deserializeTeleportRequest deserializes a TeleportRequest from JSON bytes.
func deserializeTeleportRequest(data []byte) TeleportRequest {
	var req TeleportRequest
	_ = json.Unmarshal(data, &req)
	return req
}

// deserializeGiveItemRequest deserializes a GiveItemRequest from JSON bytes.
func deserializeGiveItemRequest(data []byte) GiveItemRequest {
	var req GiveItemRequest
	_ = json.Unmarshal(data, &req)
	return req
}

// deserializeGetBlockRequest deserializes a GetBlockRequest from JSON bytes.
func deserializeGetBlockRequest(data []byte) GetBlockRequest {
	var req GetBlockRequest
	_ = json.Unmarshal(data, &req)
	return req
}

// deserializeSetBlockRequest deserializes a SetBlockRequest from JSON bytes.
func deserializeSetBlockRequest(data []byte) SetBlockRequest {
	var req SetBlockRequest
	_ = json.Unmarshal(data, &req)
	return req
}

// deserializeScheduleTaskRequest deserializes a ScheduleTaskRequest from JSON bytes.
func deserializeScheduleTaskRequest(data []byte) ScheduleTaskRequest {
	var req ScheduleTaskRequest
	_ = json.Unmarshal(data, &req)
	return req
}
