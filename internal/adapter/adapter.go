package adapter

import (
	"sync"

	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"

	"github.com/EinBexiii/dragonfly-wasm/internal/manager"
)

type Adapter struct {
	srv     *server.Server
	players sync.Map
}

func NewAdapter(srv *server.Server) *Adapter {
	return &Adapter{srv: srv}
}

func (a *Adapter) TrackPlayer(p *player.Player)   { a.players.Store(p.UUID().String(), p) }
func (a *Adapter) UntrackPlayer(p *player.Player) { a.players.Delete(p.UUID().String()) }

func (a *Adapter) GetPlayer(uuid string) (manager.PlayerAPI, bool) {
	p, ok := a.players.Load(uuid)
	if !ok {
		return nil, false
	}
	return &PlayerAdapter{player: p.(*player.Player), adapter: a}, true
}

func (a *Adapter) GetAllPlayers() []manager.PlayerAPI {
	var players []manager.PlayerAPI
	a.players.Range(func(_, value any) bool {
		players = append(players, &PlayerAdapter{player: value.(*player.Player), adapter: a})
		return true
	})
	return players
}

func (a *Adapter) GetWorld(name string) (manager.WorldAPI, bool) {
	for _, w := range []*world.World{a.srv.World(), a.srv.Nether(), a.srv.End()} {
		if w != nil && w.Name() == name {
			return &WorldAdapter{world: w}, true
		}
	}
	return nil, false
}

func (a *Adapter) GetDefaultWorld() manager.WorldAPI {
	return &WorldAdapter{world: a.srv.World()}
}

func (a *Adapter) BroadcastMessage(msg string) {
	a.players.Range(func(_, value any) bool {
		value.(*player.Player).Message(msg)
		return true
	})
}

type PlayerAdapter struct {
	player  *player.Player
	adapter *Adapter
}

func (p *PlayerAdapter) UUID() string           { return p.player.UUID().String() }
func (p *PlayerAdapter) Name() string           { return p.player.Name() }
func (p *PlayerAdapter) SendMessage(msg string) { p.player.Message(msg) }
func (p *PlayerAdapter) Kick(reason string)     { p.player.Disconnect(reason) }

func (p *PlayerAdapter) Teleport(x, y, z float64, _ string) error {
	p.player.Teleport(mgl64.Vec3{x, y, z})
	return nil
}

func (p *PlayerAdapter) SetHealth(health float64) {
	if current := p.player.Health(); health > current {
		p.player.Heal(health-current, healSource{})
	}
}

type healSource struct{}

func (healSource) HealingSource() {}

func (p *PlayerAdapter) SetGameMode(mode int) {
	gm, ok := world.GameModeByID(mode)
	if !ok {
		gm = world.GameModeSurvival
	}
	p.player.SetGameMode(gm)
}

func (p *PlayerAdapter) Position() (x, y, z float64) {
	pos := p.player.Position()
	return pos[0], pos[1], pos[2]
}

func (p *PlayerAdapter) World() manager.WorldAPI {
	if tx := p.player.Tx(); tx != nil {
		return &WorldAdapter{world: tx.World()}
	}
	return nil
}

type WorldAdapter struct {
	world *world.World
}

func (w *WorldAdapter) Name() string { return w.world.Name() }

func (w *WorldAdapter) GetBlock(x, y, z int) (string, map[string]string) {
	var blockType string
	w.world.Exec(func(tx *world.Tx) {
		if b := tx.Block(cube.Pos{x, y, z}); b != nil {
			blockType, _ = b.EncodeBlock()
		} else {
			blockType = "air"
		}
	})
	return blockType, make(map[string]string)
}

func (w *WorldAdapter) SetBlock(x, y, z int, blockType string, _ map[string]string) error {
	block, ok := blockByName(blockType)
	if !ok {
		return nil
	}
	w.world.Exec(func(tx *world.Tx) {
		tx.SetBlock(cube.Pos{x, y, z}, block, nil)
	})
	return nil
}

func blockByName(_ string) (world.Block, bool) { return nil, false }
