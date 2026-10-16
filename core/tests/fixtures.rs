//! Тесты ядра на общих фикстурах `shared/fixtures`.

use std::fs;
use std::path::PathBuf;

use vpn_core::{build_core_config, parse_subscription, pick_channel};

fn shared(rel: &str) -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("..")
        .join("shared")
        .join(rel)
}

fn expected(rel: &str) -> serde_json::Value {
    let text = fs::read_to_string(shared(rel)).expect("read fixture");
    let mut v: serde_json::Value = serde_json::from_str(&text).expect("parse fixture");
    if let Some(obj) = v.as_object_mut() {
        obj.remove("_comment");
    }
    v
}

#[test]
fn parses_two_channels() {
    let links = fs::read_to_string(shared("fixtures/sample-links.txt")).unwrap();
    let chans = parse_subscription(&links).unwrap();
    assert_eq!(chans.len(), 2);

    assert_eq!(chans[0].transport, "xhttp");
    assert_eq!(chans[0].port, 443);
    assert_eq!(chans[0].reality.server_name, "www.github.com");
    assert_eq!(chans[0].reality.fingerprint, "firefox");
    let x = chans[0].xhttp.as_ref().unwrap();
    assert_eq!(x.mode, "stream-one");
    assert_eq!(x.path, "/api/v1/updates");

    assert_eq!(chans[1].transport, "tcp");
    assert_eq!(chans[1].port, 8443);
    assert_eq!(chans[1].flow.as_deref(), Some("xtls-rprx-vision"));
    assert!(chans[1].xhttp.is_none());
}

#[test]
fn core_config_matches_fixtures() {
    let links = fs::read_to_string(shared("fixtures/sample-links.txt")).unwrap();
    let chans = parse_subscription(&links).unwrap();

    let xhttp = pick_channel(&chans, "xhttp").unwrap();
    assert_eq!(build_core_config(&xhttp, 10808, 10809), expected("fixtures/expected/xhttp.json"));

    let vision = pick_channel(&chans, "vision").unwrap();
    assert_eq!(build_core_config(&vision, 10808, 10809), expected("fixtures/expected/vision.json"));
}

#[test]
fn rejects_bad_input() {
    assert!(parse_subscription("").is_err());
    assert!(vpn_core::parse_link("https://example.com").is_err());
    assert!(vpn_core::parse_link("vless://uuid-only").is_err());
}
