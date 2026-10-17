// По умолчанию скрываем консоль в релизной сборке (приложение с окном).
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod commands;
mod coreproc;
mod proxy;
mod selftest;
mod state;
mod tray;
mod tun;

use tauri::Manager;

fn main() {
    // Самопроверка бэкенда без окна.
    if std::env::args().any(|a| a == "selftest") {
        match selftest::run() {
            Ok(()) => std::process::exit(0),
            Err(e) => {
                eprintln!("selftest FAILED: {e}");
                std::process::exit(1);
            }
        }
    }

    // При панике (сбой приложения) — не оставлять компьютер без интернета.
    let default_hook = std::panic::take_hook();
    std::panic::set_hook(Box::new(move |info| {
        proxy::disable_if_ours(&format!("127.0.0.1:{}", state::HTTP_PORT));
        tun::cleanup_leftovers();
        default_hook(info);
    }));

    tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(|app, _argv, _cwd| {
            if let Some(w) = app.get_webview_window("main") {
                let _ = w.show();
                let _ = w.unminimize();
                let _ = w.set_focus();
            }
        }))
        .plugin(tauri_plugin_updater::Builder::new().build())
        .manage(state::AppState::new())
        .invoke_handler(tauri::generate_handler![
            commands::get_status,
            commands::load_subscription,
            commands::save_subscription,
            commands::parse_channels,
            commands::connect,
            commands::disconnect,
            commands::elevate,
        ])
        .setup(|app| {
            // Убрать «зависшие» настройки от прошлого запуска.
            proxy::clear_stale(&format!("127.0.0.1:{}", state::HTTP_PORT));
            tun::cleanup_leftovers();
            tray::init(app.handle())?;

            // Сторож: восстановление после сна/сбоя ядра.
            commands::watchdog(app.handle().clone());

            // Проверка обновлений при запуске (тихо; при наличии — ставим и перезапускаем).
            let handle = app.handle().clone();
            tauri::async_runtime::spawn(async move {
                use tauri_plugin_updater::UpdaterExt;
                if let Ok(updater) = handle.updater() {
                    if let Ok(Some(update)) = updater.check().await {
                        if update.download_and_install(|_, _| {}, || {}).await.is_ok() {
                            handle.restart();
                        }
                    }
                }
            });
            Ok(())
        })
        .on_window_event(|window, event| {
            // Закрытие окна — не выход: прячем в трей.
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                api.prevent_close();
                let _ = window.hide();
            }
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

