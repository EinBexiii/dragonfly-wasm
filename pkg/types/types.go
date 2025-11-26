package types

import (
	"time"

	"github.com/google/uuid"
)

type EventType string

const (
	EventPlayerJoin       EventType = "player.join"
	EventPlayerQuit       EventType = "player.quit"
	EventPlayerChat       EventType = "player.chat"
	EventPlayerMove       EventType = "player.move"
	EventPlayerTeleport   EventType = "player.teleport"
	EventPlayerJump       EventType = "player.jump"
	EventPlayerSprint     EventType = "player.sprint"
	EventPlayerSneak      EventType = "player.sneak"
	EventPlayerRespawn    EventType = "player.respawn"
	EventPlayerDeath      EventType = "player.death"
	EventPlayerHurt       EventType = "player.hurt"
	EventPlayerHeal       EventType = "player.heal"
	EventPlayerAttack     EventType = "player.attack"
	EventPlayerCommand    EventType = "player.command"
	EventPlayerTransfer   EventType = "player.transfer"
	EventPlayerSkinChange EventType = "player.skin_change"

	EventBlockBreak    EventType = "block.break"
	EventBlockPlace    EventType = "block.place"
	EventBlockInteract EventType = "block.interact"
	EventSignEdit      EventType = "block.sign_edit"

	EventItemUse         EventType = "item.use"
	EventItemUseOnBlock  EventType = "item.use_on_block"
	EventItemUseOnEntity EventType = "item.use_on_entity"
	EventItemConsume     EventType = "item.consume"
	EventItemDrop        EventType = "item.drop"
	EventItemPickup      EventType = "item.pickup"

	EventEntitySpawn   EventType = "entity.spawn"
	EventEntityDespawn EventType = "entity.despawn"
)

func (e EventType) String() string { return string(e) }

func (e EventType) IsCancellable() bool {
	switch e {
	case EventPlayerJoin, EventPlayerChat, EventPlayerMove, EventPlayerTeleport,
		EventPlayerHurt, EventPlayerHeal, EventPlayerAttack, EventPlayerCommand,
		EventPlayerTransfer, EventBlockBreak, EventBlockPlace, EventBlockInteract,
		EventItemUse, EventItemUseOnBlock, EventItemUseOnEntity, EventItemConsume,
		EventItemDrop, EventItemPickup, EventSignEdit, EventPlayerDeath:
		return true
	default:
		return false
	}
}

type PluginID string

func (p PluginID) String() string { return string(p) }

type PluginState int

const (
	StateUnloaded PluginState = iota
	StateLoading
	StateLoaded
	StateEnabled
	StateDisabled
	StateError
)

func (s PluginState) String() string {
	names := [...]string{"unloaded", "loading", "loaded", "enabled", "disabled", "error"}
	if int(s) < len(names) {
		return names[s]
	}
	return "unknown"
}

type PluginManifest struct {
	ID                  PluginID           `toml:"id"`
	Name                string             `toml:"name"`
	Version             string             `toml:"version"`
	Authors             []string           `toml:"authors"`
	Description         string             `toml:"description"`
	URL                 string             `toml:"url"`
	License             string             `toml:"license"`
	SubscribedEvents    []EventType        `toml:"subscribed_events"`
	Permissions         []Permission       `toml:"permissions"`
	Dependencies        []PluginDependency `toml:"dependencies"`
	MinDragonflyVersion string             `toml:"min_dragonfly_version"`
	APIVersion          string             `toml:"api_version"`
	Config              map[string]any     `toml:"config"`
}

type Permission struct {
	ID       string `toml:"id"`
	Reason   string `toml:"reason"`
	Required bool   `toml:"required"`
}

type PluginDependency struct {
	PluginID          PluginID `toml:"plugin_id"`
	VersionConstraint string   `toml:"version_constraint"`
	Required          bool     `toml:"required"`
}

type PluginInfo struct {
	Manifest            PluginManifest
	State               PluginState
	LoadedAt            time.Time
	ErrorMessage        string
	MemoryUsage         uint64
	EventsProcessed     uint64
	AvgProcessingTimeUs uint64
}

type EventID string

func NewEventID() EventID        { return EventID(uuid.New().String()) }
func (e EventID) String() string { return string(e) }

type EventContext struct {
	EventID     EventID
	EventType   EventType
	Timestamp   time.Time
	Cancellable bool
	Cancelled   bool
}

type EventResult struct {
	Cancelled        bool
	ModifiedPayload  []byte
	Error            error
	ProcessingTimeUs uint64
}

type LogLevel int

const (
	LogDebug LogLevel = iota
	LogInfo
	LogWarn
	LogError
)

func (l LogLevel) String() string {
	names := [...]string{"debug", "info", "warn", "error"}
	if int(l) < len(names) {
		return names[l]
	}
	return "unknown"
}
