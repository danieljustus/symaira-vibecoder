# symvibe — Vibe Coding Baukasten

[![CI](https://github.com/danieljustus/symaira-vibecoder/actions/workflows/ci.yml/badge.svg)](https://github.com/danieljustus/symaira-vibecoder/actions/workflows/ci.yml)
[![Latest Release](https://img.shields.io/github/v/release/danieljustus/symaira-vibecoder)](https://github.com/danieljustus/symaira-vibecoder/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/danieljustus/symaira-vibecoder)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

> Eine schlanke grafische Oberfläche, die deinen **Cycle** (Cleaning → Code
> Review → Planung → Coden → PR-Check → GH Alerts → Pre-Release → Release)
> autonom durchläuft — angetrieben von **opencode**, mit Modell-Auswahl pro
> Kategorie *und* pro Schritt, Subagents und dezenten Status-Symbolen.

`symvibe` ist Teil des Symaira-Ökosystems: ein einzelnes, CGO-freies Go-Binary,
das ein **Baukasten-Board** auf `127.0.0.1` serviert. Du baust deinen Cycle aus
verschiebbaren Karten zusammen (hinzufügen / entfernen / bearbeiten / per
Drag-&-Drop umsortieren) und sagst dem Tool, es soll loslaufen — der Rest
passiert autonom.

## Architecture

```mermaid
graph LR
    Browser["🌐 Browser<br/><i>embedded board</i>"]
    Server["internal/server<br/><i>net/http · SSE · embed</i>"]
    Engine["internal/engine<br/><i>scheduler · sensors · bus</i>"]
    Config["internal/config<br/><i>Cycle · Resolver · TOML</i>"]
    Runner["internal/runner<br/><i>opencode · api · aider · claudecode · cline · local_api</i>"]
    OpenCode["opencode<br/><i>runtime peer</i>"]

    Browser -- "REST /api/*" --> Server
    Browser -- "GET /events (SSE)" --> Server
    Server -- "Engine API" --> Engine
    Engine -- "config.Resolver" --> Config
    Engine -- "runner.Runner" --> Runner
    Runner -- "os/exec" --> OpenCode
```

## Features

- **Baukasten UX** — drag-and-drop cycle builder with customizable phases and steps
- **opencode integration** — drives opencode headless via `Runner` interface (no fork required)
- **Model bindings per category/step** — assign different AI models to different parts of your cycle
- **Autonomous cycle execution** — run your entire workflow with a single click
- **iOS/macOS client** — native SwiftUI client for monitoring and controlling cycles

```
  symvibe serve  →  http://127.0.0.1:4317
  ┌──────────── Cycle ────────────────────────────────┐
  │ ① Cleaning   ◐ 1.1 Branch   ○ 1.2 Commits          │
  │ ② Code Review ✓ 2.1 Quality  ○ 2.2 Simplify …      │
  │ …  [Karten ziehen · $skill · ▦ kategorie · ▶ run]  │
  └────────────────────────────────────────────────────┘
   ○ pending  ◐ running  ✓ done  – skipped  ✕ failed  ⦸ blocked  ! review
```

## Warum kein Fork von opencode?

`symvibe` **forkt opencode nicht** — es *steuert* es über ein austauschbares
`Runner`-Interface. opencode liefert headless bereits alles, was nötig ist
(`opencode run --format json`, `--agent/--model/--variant`, Sessions, Skills).
Ein Fork würde bedeuten, die gesamte Provider-/Tool-/Session-Maschinerie zu
erben und ewig zu pflegen. So gehört dir die *Vision*-Schicht (Cycle, Baukasten,
Autonomie, Modell-Bindings) zu 100 %, während die Commodity-Agent-Runtime ein
Peer bleibt. Ein Fork kann später jederzeit als weiterer Runner andocken.

## Installation

```bash
# pre-built release (macOS / Linux)
curl -fsSL https://raw.githubusercontent.com/danieljustus/symaira-vibecoder/main/scripts/install.sh | sh

# Homebrew (requires the danieljustus/homebrew-tap repository)
brew install danieljustus/tap/symvibe

# aus dem Quellcode (Go 1.26+)
make build && ./symvibe serve

# oder direkt
go install github.com/danieljustus/symaira-vibecoder/cmd/symvibe@latest
symvibe serve
```

### Runner-Backends

symvibe treibt Coding-Agents über ein austauschbares `Runner`-Interface an.
Welches Backend zum Einsatz kommt, wird über `runner.backend` in
`~/.config/symvibe/config.toml` oder die Env-Variable `SYMVIBE_RUNNER_BACKEND`
gesteuert.

| Backend | Beschreibung | Voraussetzung |
|---------|-------------|---------------|
| `opencode` *(Default)* | Treibt [`opencode`](https://opencode.ai) headless über `opencode run --format json`. Volle Model-/Skill-/Agent-Kontrolle. | `opencode` auf PATH oder `opencode_bin` |
| `api` | Direkte Anthropic-Claude-API — kein opencode nötig. | `api_key` oder `SYMVIBE_ANTHROPIC_API_KEY` |
| `aider` | Treibt [aider](https://aider.chat) CLI headless (`--message`). | `aider` auf PATH oder `aider_bin` |
| `claudecode` | Treibt Claude Code CLI headless (`-p`). | `claude` auf PATH oder `claude_code_bin` |
| `cline` | Treibt [Cline](https://github.com/cline/cline) CLI headless. | `cline` auf PATH oder `cline_bin` |
| `local_api` | Lokaler OpenAI-kompatibler Endpoint (Ollama, LM Studio, MLX). | `local_api_endpoint` (z.B. `http://localhost:11434/v1`) |

Ohne ausführbaren Backend ist das Board read-only.

**Kurzbeispiele:**

```toml
# ~/.config/symvibe/config.toml
[runner]
backend = "opencode"            # oder: api | aider | claudecode | cline | local_api
opencode_bin = ""               # leer = auto-detect auf PATH

# Für api-Backend:
# backend = "api"
# api_key = "sk-ant-..."

# Für aider-Backend:
# backend = "aider"
# aider_bin = "/usr/local/bin/aider"

# Für local_api-Backend (Ollama):
# backend = "local_api"
# local_api_endpoint = "http://localhost:11434/v1"
# local_api_model = "llama3.1"
```

```bash
# Override via Environment
export SYMVIBE_RUNNER_BACKEND=api
export SYMVIBE_ANTHROPIC_API_KEY=sk-ant-...
```

**Voraussetzungen:**

- **git** ist erforderlich; **gh** ist optional (nur für GitHub-Workflows).
- `symvibe doctor` prüft alle konfigurierten Backends und zeigt Install-Hinweise.

## Nutzung

```bash
symvibe serve            # Board starten und im Browser öffnen
symvibe serve --no-open  # ohne Browser
symvibe serve --dir ~/code/mein-repo   # Arbeitsverzeichnis des Cycles
symvibe doctor           # opencode/git/gh prüfen + Modell-IDs gegen `opencode models` abgleichen
symvibe version
```

Im Board:

- **Run Cycle** — läuft autonom ab der aktuellen Position durch (`NextRunnable`).
- **Run only this** (▶ auf einer Karte) — nur diesen Schritt.
- **Pause / Resume / Cancel** — Lauf steuern.
- Karten **bearbeiten**: Skill binden (`$00-sync` …), Kategorie (Modell-Binding)
  wählen, an/aus schalten, löschen, duplizieren, per Drag-&-Drop verschieben.
- Live: das Status-Symbol jeder Karte wechselt in Echtzeit (SSE), das
  Activity-Panel zeigt den Event-Stream des laufenden Schritts.

## Konfiguration

Optional unter `~/.config/symvibe/config.toml` — siehe
[`config/config.example.toml`](config/config.example.toml). Ohne Datei laufen
sinnvolle Defaults (gespiegelt aus `oh-my-openagent.json`).

- **Modell-Registry + Kategorie-Bindings** (`ultrabrain`, `deep`, `quick`,
  `git`, `writing`, `unspecified-*`) mit `temperature`, `variant`,
  `fallback_models`.
- **Pro-Schritt-Override** schlägt die Kategorie (`[phases.steps.model_override]`
  im Cycle).
- Auflösung: **Schritt-Override > Kategorie > Default**; bei Fehler wird die
  `fallback_models`-Kette abgewandert.
- Alle Werte via `SYMVIBE_*` Env überschreibbar (`SYMVIBE_PORT`,
  `SYMVIBE_OPENCODE_BIN`, `SYMVIBE_WORKING_DIR`, …).

Der Cycle (Baukasten) liegt editierbar unter
`~/.local/share/symvibe/cycles/<id>.toml`; der Seed stammt aus
[`config/seed-cycle.toml`](config/seed-cycle.toml) (8 Phasen aus
`docs/Grundidee.csv`).

## Backend-Override

Der Runner-Backend lässt sich auf drei Ebenen konfigurieren — jede höhere
Ebene schlägt die niedrigere:

1. **Global** — `~/.config/symvibe/config.toml` oder `SYMVIBE_RUNNER_BACKEND`
2. **Projektweit** — `.symvibe.toml` im Projektroot (gleiches TOML-Schema)
3. **Pro Schritt** — `backend_override` im Cycle-TOML

```toml
# Projektweiter Override: .symvibe.toml im Repo-Root
[runner]
backend = "aider"       # dieses Projekt nutzt aider statt opencode
```

```toml
# Pro-Schritt-Override im Cycle (cycles/<id>.toml)
[[phases.steps]]
id = "1.1"
name = "Sync"
backend_override = "local_api"   # diesen Schritt über lokales API laufen lassen
```

**Auflösungsreihenfolge:** Schritt-Override > Projekt-Override > Global >
Built-in Default (`opencode`). Bei einem Fehler wird die
`fallback_models`-Kette abgewandert (siehe Konfiguration).

## Autonomie

- **Weiß, wo es ist:** der Scheduler liest den *persistierten* Status jedes
  Schritts; done/skipped werden übersprungen, beim ersten Problem
  (failed/blocked) hält der Lauf an. Ein Absturz mitten im Schritt wird beim
  Resume zurückgesetzt.
- **Weiß, was zu überspringen ist:** ein Schritt kann eine
  `auto_skip = { sensor, when }`-Regel tragen (z. B. `open-issues == 0` →
  Coden-Schritt überspringen). Sensoren sind billige Proben (`git-dirty`,
  `open-issues`, `open-prs`).

## Sicherheit

Loopback-only, unauthentifiziert — nur lokal nutzen. *Run* lässt opencode
echten Code gegen dein Repo ausführen. Details: [SECURITY.md](SECURITY.md).

## Architektur

Go-Core (`cmd/symvibe`, `internal/{config,engine,runner,server,version}`) +
eingebettetes Board (`web/`). Siehe [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## iOS / macOS-Client

Ein SwiftPM-Skelett liegt unter `client/`. `SymvibeKit` enthält REST-Client,
SSE-Parser, TLS-Pinning und `Codable`-Modelle, die 1:1 den Go-Typen folgen.

```bash
cd client
swift build                # macOS
# iOS: in Xcode öffnen oder xcodebuild mit iOS-Simulator-Destination
```

Der Client benötigt iOS 17 / macOS 14.

## Lizenz

Apache-2.0 — siehe [LICENSE](LICENSE).
