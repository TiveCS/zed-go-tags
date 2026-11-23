use std::fs;

use zed_extension_api::{
    self as zed, settings::LspSettings, LanguageServerId, LanguageServerInstallationStatus,
    Result,
};

const SERVER_BINARY: &str = "go-tags-ls";
const GITHUB_REPO: &str = "TiveCS/zed-go-tags";

struct GoTagsExtension {
    /// Path of the downloaded server, once found or installed this session.
    cached_binary_path: Option<String>,
}

impl GoTagsExtension {
    /// Finds the server: a `go-tags-ls` on PATH wins (handy for development),
    /// otherwise the latest GitHub release is downloaded into the extension's
    /// work directory and reused.
    fn server_binary_path(
        &mut self,
        language_server_id: &LanguageServerId,
        worktree: &zed::Worktree,
    ) -> Result<String> {
        if let Some(path) = worktree.which(SERVER_BINARY) {
            return Ok(path);
        }
        if let Some(path) = &self.cached_binary_path {
            if fs::metadata(path).is_ok_and(|m| m.is_file()) {
                return Ok(path.clone());
            }
        }

        zed::set_language_server_installation_status(
            language_server_id,
            &LanguageServerInstallationStatus::CheckingForUpdate,
        );
        let release = zed::latest_github_release(
            GITHUB_REPO,
            zed::GithubReleaseOptions {
                require_assets: true,
                pre_release: false,
            },
        )?;

        let (os, arch) = zed::current_platform();
        let os = match os {
            zed::Os::Mac => "darwin",
            zed::Os::Linux => "linux",
            zed::Os::Windows => "windows",
        };
        let arch = match arch {
            zed::Architecture::Aarch64 => "arm64",
            zed::Architecture::X8664 => "amd64",
            zed::Architecture::X86 => return Err("32-bit x86 is not supported".into()),
        };
        let asset_name = format!("{SERVER_BINARY}-{os}-{arch}.gz");
        let asset = release
            .assets
            .iter()
            .find(|a| a.name == asset_name)
            .ok_or_else(|| format!("no release asset named {asset_name}"))?;

        let version_dir = format!("{SERVER_BINARY}-{}", release.version);
        let exe = if os == "windows" { ".exe" } else { "" };
        let binary_path = format!("{version_dir}/{SERVER_BINARY}{exe}");

        if !fs::metadata(&binary_path).is_ok_and(|m| m.is_file()) {
            zed::set_language_server_installation_status(
                language_server_id,
                &LanguageServerInstallationStatus::Downloading,
            );
            fs::create_dir_all(&version_dir)
                .map_err(|e| format!("failed to create {version_dir}: {e}"))?;
            zed::download_file(
                &asset.download_url,
                &binary_path,
                zed::DownloadedFileType::Gzip,
            )
            .map_err(|e| format!("failed to download {asset_name}: {e}"))?;
            zed::make_file_executable(&binary_path)?;

            // Remove older versions.
            if let Ok(entries) = fs::read_dir(".") {
                for entry in entries.flatten() {
                    let name = entry.file_name();
                    let name = name.to_string_lossy();
                    if name.starts_with(SERVER_BINARY) && name != version_dir {
                        fs::remove_dir_all(entry.path()).ok();
                    }
                }
            }
        }

        zed::set_language_server_installation_status(
            language_server_id,
            &LanguageServerInstallationStatus::None,
        );
        self.cached_binary_path = Some(binary_path.clone());
        Ok(binary_path)
    }
}

impl zed::Extension for GoTagsExtension {
    fn new() -> Self {
        GoTagsExtension {
            cached_binary_path: None,
        }
    }

    fn language_server_command(
        &mut self,
        language_server_id: &LanguageServerId,
        worktree: &zed::Worktree,
    ) -> Result<zed::Command> {
        Ok(zed::Command {
            command: self.server_binary_path(language_server_id, worktree)?,
            args: vec![],
            env: worktree.shell_env(),
        })
    }

    // Forwards `lsp.go-tags-ls.initialization_options` from Zed settings
    // (user settings and the project's .zed/settings.json).
    fn language_server_initialization_options(
        &mut self,
        language_server_id: &LanguageServerId,
        worktree: &zed::Worktree,
    ) -> Result<Option<zed::serde_json::Value>> {
        Ok(LspSettings::for_worktree(language_server_id.as_ref(), worktree)
            .ok()
            .and_then(|settings| settings.initialization_options))
    }
}

zed::register_extension!(GoTagsExtension);
