use extism_pdk::*;
use serde::{Deserialize, Serialize};
use std::cell::RefCell;
use std::collections::HashMap;

thread_local! {
    static STATS: RefCell<HashMap<String, Stats>> = RefCell::new(HashMap::new());
}

const PROTECTED_BLOCKS: &[&str] = &[
    "minecraft:diamond_ore",
    "minecraft:deepslate_diamond_ore",
    "minecraft:ancient_debris",
    "minecraft:spawner",
];

#[derive(Debug, Deserialize)]
struct Player {
    uuid: String,
    name: String,
}

#[derive(Debug, Deserialize)]
struct Position {
    x: i32,
    y: i32,
    z: i32,
}

#[derive(Debug, Deserialize)]
struct Block {
    block_type: String,
    position: Position,
    #[serde(default)]
    properties: HashMap<String, String>,
}

#[derive(Debug, Deserialize)]
struct BlockBreakEvent {
    player: Player,
    block: Block,
    #[serde(default)]
    drops: Vec<ItemStack>,
    #[serde(default)]
    experience: i32,
}

#[derive(Debug, Deserialize)]
struct BlockPlaceEvent {
    player: Player,
    block: Block,
}

#[derive(Debug, Deserialize)]
struct PlayerJoinEvent {
    player: Player,
}

#[derive(Debug, Deserialize)]
struct ItemStack {
    item_type: String,
    count: i32,
}

#[derive(Debug, Serialize, Default)]
struct EventResult {
    cancelled: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    modifications: Option<HashMap<String, String>>,
}

#[derive(Debug, Clone, Default)]
struct Stats {
    broken: u64,
    placed: u64,
    denied: u64,
}

#[derive(Serialize)]
struct LogRequest {
    level: &'static str,
    message: String,
}

#[derive(Serialize)]
struct SendMessageRequest {
    player_uuid: String,
    message: String,
}

#[host_fn]
extern "ExtismHost" {
    fn host_log(data: &[u8]);
    fn host_send_message(data: &[u8]) -> i64;
}

fn log(level: &'static str, msg: String) {
    let req = LogRequest { level, message: msg };
    if let Ok(data) = serde_json::to_vec(&req) {
        unsafe { host_log(&data) }.ok();
    }
}

fn notify(uuid: &str, msg: &str) {
    let req = SendMessageRequest {
        player_uuid: uuid.into(),
        message: msg.into(),
    };
    if let Ok(data) = serde_json::to_vec(&req) {
        unsafe { host_send_message(&data) }.ok();
    }
}

fn get_stats(uuid: &str) -> Stats {
    STATS.with(|s| s.borrow().get(uuid).cloned().unwrap_or_default())
}

fn update_stats<F: FnOnce(&mut Stats)>(uuid: &str, f: F) {
    STATS.with(|s| {
        let mut map = s.borrow_mut();
        let stats = map.entry(uuid.into()).or_default();
        f(stats);
    });
}

fn is_protected(block_type: &str) -> bool {
    let normalized = block_type.to_lowercase();
    PROTECTED_BLOCKS.iter().any(|&b| normalized.contains(b) || normalized.ends_with(b.trim_start_matches("minecraft:")))
}

#[plugin_fn]
pub fn plugin_init() -> FnResult<()> {
    log("info", "block protection initialized".into());
    Ok(())
}

#[plugin_fn]
pub fn on_enable() -> FnResult<()> {
    log("info", "block protection enabled".into());
    Ok(())
}

#[plugin_fn]
pub fn on_disable() -> FnResult<()> {
    log("info", "block protection disabled".into());
    Ok(())
}

#[plugin_fn]
pub fn handle_event(envelope: Vec<u8>) -> FnResult<Vec<u8>> {
    let sep = envelope.iter().position(|&b| b == 0).unwrap_or(envelope.len());
    let event_type = std::str::from_utf8(&envelope[..sep]).unwrap_or("");
    let payload = if sep < envelope.len() { &envelope[sep + 1..] } else { &[] };

    let result = match event_type {
        "block_break" => on_block_break(payload),
        "block_place" => on_block_place(payload),
        "player_join" => on_player_join(payload),
        _ => Ok(EventResult::default()),
    };

    let res = result.unwrap_or_default();
    let mut out = vec![u8::from(res.cancelled)];
    if let Some(ref mods) = res.modifications {
        if let Ok(json) = serde_json::to_vec(mods) {
            out.extend(json);
        }
    }
    Ok(out)
}

fn on_block_break(data: &[u8]) -> Result<EventResult, Error> {
    let ev: BlockBreakEvent = serde_json::from_slice(data)?;
    let pos = &ev.block.position;

    if is_protected(&ev.block.block_type) {
        update_stats(&ev.player.uuid, |s| s.denied += 1);
        notify(
            &ev.player.uuid,
            &format!("§c§lProtected! §r§7{} cannot be mined.", extract_block_name(&ev.block.block_type)),
        );
        log("warn", format!("{} tried to break protected block {} at {},{},{}", ev.player.name, ev.block.block_type, pos.x, pos.y, pos.z));
        return Ok(EventResult { cancelled: true, modifications: None });
    }

    update_stats(&ev.player.uuid, |s| s.broken += 1);
    let stats = get_stats(&ev.player.uuid);

    if stats.broken % 50 == 0 {
        notify(&ev.player.uuid, &format!("§e{} §7blocks broken", stats.broken));
    }

    log("debug", format!("{} broke {} at {},{},{}", ev.player.name, ev.block.block_type, pos.x, pos.y, pos.z));
    Ok(EventResult::default())
}

fn on_block_place(data: &[u8]) -> Result<EventResult, Error> {
    let ev: BlockPlaceEvent = serde_json::from_slice(data)?;
    let pos = &ev.block.position;

    update_stats(&ev.player.uuid, |s| s.placed += 1);
    let stats = get_stats(&ev.player.uuid);

    if stats.placed % 50 == 0 {
        notify(&ev.player.uuid, &format!("§e{} §7blocks placed", stats.placed));
    }

    log("debug", format!("{} placed {} at {},{},{}", ev.player.name, ev.block.block_type, pos.x, pos.y, pos.z));
    Ok(EventResult::default())
}

fn on_player_join(data: &[u8]) -> Result<EventResult, Error> {
    let ev: PlayerJoinEvent = serde_json::from_slice(data)?;
    let stats = get_stats(&ev.player.uuid);

    if stats.broken > 0 || stats.placed > 0 {
        notify(
            &ev.player.uuid,
            &format!("§7Welcome back! §e{} §7broken, §e{} §7placed", stats.broken, stats.placed),
        );
    }

    log("info", format!("{} joined", ev.player.name));
    Ok(EventResult::default())
}

fn extract_block_name(full: &str) -> &str {
    full.split(':').last().unwrap_or(full)
}
