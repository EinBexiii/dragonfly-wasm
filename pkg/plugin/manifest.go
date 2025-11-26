package plugin

import (
	"cmp"
	"errors"
	"fmt"
	"regexp"
)

type Version struct {
	Major int `toml:"major" json:"major"`
	Minor int `toml:"minor" json:"minor"`
	Patch int `toml:"patch" json:"patch"`
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v Version) Compare(other Version) int {
	if c := cmp.Compare(v.Major, other.Major); c != 0 {
		return c
	}
	if c := cmp.Compare(v.Minor, other.Minor); c != 0 {
		return c
	}
	return cmp.Compare(v.Patch, other.Patch)
}

type EventType string

const (
	EventPlayerJoin         EventType = "player_join"
	EventPlayerQuit         EventType = "player_quit"
	EventPlayerChat         EventType = "player_chat"
	EventPlayerMove         EventType = "player_move"
	EventPlayerTeleport     EventType = "player_teleport"
	EventPlayerJump         EventType = "player_jump"
	EventPlayerSprint       EventType = "player_sprint"
	EventPlayerSneak        EventType = "player_sneak"
	EventPlayerRespawn      EventType = "player_respawn"
	EventPlayerDeath        EventType = "player_death"
	EventPlayerHurt         EventType = "player_hurt"
	EventPlayerHeal         EventType = "player_heal"
	EventPlayerAttackEntity EventType = "player_attack_entity"

	EventBlockBreak    EventType = "block_break"
	EventBlockPlace    EventType = "block_place"
	EventBlockInteract EventType = "block_interact"

	EventItemUse         EventType = "item_use"
	EventItemUseOnBlock  EventType = "item_use_on_block"
	EventItemUseOnEntity EventType = "item_use_on_entity"
	EventItemConsume     EventType = "item_consume"
	EventItemDrop        EventType = "item_drop"
	EventItemPickup      EventType = "item_pickup"

	EventEntitySpawn    EventType = "entity_spawn"
	EventEntityDespawn  EventType = "entity_despawn"
	EventCommand        EventType = "command"
	EventSignEdit       EventType = "sign_edit"
	EventServerTransfer EventType = "server_transfer"
)

type Priority int

const (
	PriorityLowest  Priority = -200
	PriorityLow     Priority = -100
	PriorityNormal  Priority = 0
	PriorityHigh    Priority = 100
	PriorityHighest Priority = 200
	PriorityMonitor Priority = 300
)

type EventSubscription struct {
	Event           EventType `toml:"event" json:"event"`
	Priority        Priority  `toml:"priority" json:"priority"`
	IgnoreCancelled bool      `toml:"ignore_cancelled" json:"ignore_cancelled"`
}

type Dependency struct {
	ID       string  `toml:"id" json:"id"`
	Version  Version `toml:"version" json:"version"`
	Optional bool    `toml:"optional" json:"optional"`
}

type ResourceLimits struct {
	MaxMemoryMB    int64  `toml:"max_memory_mb" json:"max_memory_mb"`
	MaxExecutionMs int64  `toml:"max_execution_ms" json:"max_execution_ms"`
	MaxFuel        uint64 `toml:"max_fuel" json:"max_fuel"`
}

func DefaultResourceLimits() ResourceLimits {
	return ResourceLimits{
		MaxMemoryMB:    64,
		MaxExecutionMs: 100,
		MaxFuel:        1_000_000,
	}
}

type Manifest struct {
	ID          string   `toml:"id" json:"id"`
	Name        string   `toml:"name" json:"name"`
	Version     Version  `toml:"version" json:"version"`
	Description string   `toml:"description" json:"description"`
	Authors     []string `toml:"authors" json:"authors"`
	Website     string   `toml:"website" json:"website"`
	License     string   `toml:"license" json:"license"`

	APIVersion Version `toml:"api_version" json:"api_version"`
	EntryPoint string  `toml:"entry_point" json:"entry_point"`

	Events       []EventSubscription `toml:"events" json:"events"`
	Dependencies []Dependency        `toml:"dependencies" json:"dependencies"`
	Limits       ResourceLimits      `toml:"limits" json:"limits"`
	LoadBefore   []string            `toml:"load_before" json:"load_before"`
	LoadAfter    []string            `toml:"load_after" json:"load_after"`
}

var pluginIDRegex = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

func (m *Manifest) Validate() error {
	if m.ID == "" {
		return errors.New("plugin ID is required")
	}
	if !pluginIDRegex.MatchString(m.ID) {
		return fmt.Errorf("invalid plugin ID %q: must be lowercase with dots", m.ID)
	}
	if m.Name == "" {
		return errors.New("plugin name is required")
	}
	if m.EntryPoint == "" {
		return errors.New("entry point is required")
	}
	return nil
}

func (m *Manifest) SubscribedTo(event EventType) bool {
	for _, e := range m.Events {
		if e.Event == event {
			return true
		}
	}
	return false
}

func (m *Manifest) GetEventPriority(event EventType) Priority {
	for _, e := range m.Events {
		if e.Event == event {
			return e.Priority
		}
	}
	return PriorityNormal
}
