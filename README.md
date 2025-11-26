# dragonfly-wasm

WASM plugin runtime for [Dragonfly](https://github.com/df-mc/dragonfly) servers. Plugins run in sandboxed WebAssembly via [Extism](https://extism.org/), so a buggy plugin won't take down your server.

## Getting Started

```bash
go build -o server ./cmd/server
./server
```

Plugins live in `plugins/<name>/`. Each plugin folder needs:
- `plugin.toml` - manifest with metadata and event subscriptions
- `plugin.wasm` - compiled WebAssembly binary

The server scans the plugin directory on startup, validates manifests, resolves dependencies, and loads everything in the right order.

## Writing a Plugin

Plugins can be written in any language that compiles to WASM. Rust with extism-pdk works best.

### Cargo.toml

```toml
[package]
name = "my-plugin"
version = "0.1.0"
edition = "2021"

[lib]
crate-type = ["cdylib"]

[dependencies]
extism-pdk = "1.2"
serde = { version = "1", features = ["derive"] }
serde_json = "1"
```

### src/lib.rs

```rust
use extism_pdk::*;
use serde::Deserialize;

#[derive(Deserialize)]
struct Player {
    uuid: String,
    name: String,
}

#[derive(Deserialize)]
struct Position {
    x: i32,
    y: i32,
    z: i32,
}

#[derive(Deserialize)]
struct Block {
    block_type: String,
    position: Position,
}

#[derive(Deserialize)]
struct BlockBreakEvent {
    player: Player,
    block: Block,
}

#[host_fn]
extern "ExtismHost" {
    fn host_log(data: &[u8]);
    fn host_send_message(data: &[u8]) -> i64;
}

// called once when plugin loads
#[plugin_fn]
pub fn plugin_init() -> FnResult<()> {
    Ok(())
}

// called for every subscribed event
#[plugin_fn]
pub fn handle_event(input: Vec<u8>) -> FnResult<Vec<u8>> {
    // input format: "event_type\0{json_payload}"
    let sep = input.iter().position(|&b| b == 0).unwrap();
    let event_type = std::str::from_utf8(&input[..sep]).unwrap();
    let payload = &input[sep + 1..];

    let cancelled = match event_type {
        "block_break" => {
            let ev: BlockBreakEvent = serde_json::from_slice(payload)?;

            // example: prevent breaking diamond ore
            if ev.block.block_type.contains("diamond_ore") {
                send_message(&ev.player.uuid, "§cYou can't break that!");
                true // cancel the event
            } else {
                false
            }
        }
        _ => false,
    };

    // first byte: 0 = allow, 1 = cancel
    Ok(vec![cancelled as u8])
}

fn send_message(uuid: &str, msg: &str) {
    #[derive(serde::Serialize)]
    struct Req { player_uuid: String, message: String }

    let req = Req { player_uuid: uuid.into(), message: msg.into() };
    if let Ok(data) = serde_json::to_vec(&req) {
        unsafe { host_send_message(&data) }.ok();
    }
}
```

### Build

```bash
cargo build --release --target wasm32-unknown-unknown
cp target/wasm32-unknown-unknown/release/my_plugin.wasm plugin.wasm
```

## Plugin Manifest

```toml
id = "com.yourname.pluginid"
name = "Human Readable Name"
description = "What it does"
entry_point = "plugin.wasm"
license = "MIT"
authors = ["Your Name"]

[version]
major = 1
minor = 0
patch = 0

[api_version]
major = 1
minor = 0
patch = 0

# subscribe to events - can have multiple [[events]] blocks
[[events]]
event = "block_break"
priority = 0              # -200 to 300, lower runs first
ignore_cancelled = false  # skip if already cancelled by another plugin?

[[events]]
event = "player_join"
priority = 100

# resource limits (optional, has defaults)
[limits]
max_memory_mb = 32
max_execution_ms = 50
max_fuel = 500000
```

### Priority

Lower runs first: -200 (lowest) → 0 (normal) → 300 (monitor)

## Events

**Player:** `player_join` `player_quit` `player_chat` `player_move` `player_teleport` `player_jump` `player_sprint` `player_sneak` `player_death` `player_respawn` `player_hurt` `player_heal` `player_attack_entity`

**Block:** `block_break` `block_place` `block_interact`

**Item:** `item_use` `item_use_on_block` `item_use_on_entity` `item_consume` `item_drop` `item_pickup`

**Other:** `entity_spawn` `entity_despawn` `command` `sign_edit` `server_transfer`

## Host Functions

Call these from your plugin to interact with the server. All functions take JSON-encoded bytes and return a status code or JSON response.

### Logging
```
host_log({"level": "info"|"warn"|"error"|"debug", "message": "..."})
```

### Player Management
```
host_get_player({"uuid": "..."}) -> Player
host_get_online_players() -> [Player]
host_send_message({"player_uuid": "...", "message": "..."})
host_broadcast({"message": "..."})
host_kick_player({"uuid": "...", "reason": "..."})
host_teleport_player({"uuid": "...", "x": 0, "y": 64, "z": 0})
host_set_player_health({"uuid": "...", "health": 20})
host_set_player_gamemode({"uuid": "...", "gamemode": "survival"|"creative"|"adventure"|"spectator"})
host_give_item({"uuid": "...", "item_type": "minecraft:diamond", "count": 1})
```

### World
```
host_get_block({"x": 0, "y": 64, "z": 0}) -> Block
host_set_block({"x": 0, "y": 64, "z": 0, "block_type": "minecraft:stone"})
```

### Storage

Persistent key-value storage per plugin. Data survives server restarts.

```
host_storage_set({"key": "...", "value": "..."})
host_storage_get({"key": "..."}) -> {"value": "..."}
host_storage_delete({"key": "..."})
```

### Scheduling
```
host_schedule_task({"delay_ms": 1000, "task_id": "my-task"})
host_cancel_task({"task_id": "my-task"})
```

## Server Configuration

`plugins.toml` in server root:

```toml
plugin_dir = "plugins"
data_dir = "plugin_data"

[default_limits]
max_memory_mb = 64
max_execution_ms = 100

[logging]
level = "info"  # debug, info, warn, error
```

## Example Plugin

Check out `examples/plugins/block-logger/` for a complete example that:
- Protects specific blocks from being broken
- Tracks player statistics (blocks broken/placed)
- Shows periodic notifications
- Welcomes returning players with their stats

## License

MIT
