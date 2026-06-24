# symvibe — SwiftUI Multiplatform Client · Umsetzungsplan

> Ziel: Ein nativer Client für **iOS / iPadOS / macOS** aus *einer* SwiftUI-Codebase,
> mit dem man die `symvibe`-Engine fernsteuert. Verbindung iPhone → Mac auf **zwei**
> Wegen: **(a)** lokal per **QR-Code-Pairing** (self-hosted, ohne Cloud) und
> **(b)** remote über einen **Symaira-Account** (Relay).

---

## 0. Grundprinzip (bitte zuerst lesen)

1. **Der Go-Core bleibt das Backend.** Swift ersetzt **nur** die GUI-Schicht
   (heute `web/dist/index.html`). `cmd/symvibe`, `internal/{engine,runner,config,server}`
   und die CLI bleiben unverändert das Herz.
2. **Die Engine läuft niemals auf iOS.** `opencode`, `git`, `gh`, das Repo und
   das Dateisystem brauchen eine echte Maschine. Darum ist die iPhone-App
   **immer** ein Remote-Client für einen Mac/Server, auf dem `symvibe serve` läuft.
3. **Die Architektur ist schon Client/Server.** Es existieren REST (`/api/*`) +
   ein SSE-Stream (`/events`). Der Swift-Client ist „nur" eine UI über HTTP. Die
   eigentliche Arbeit ist deshalb: **(A)** den Server netzwerk-/auth-fähig machen,
   **(B)** den Account-/Relay-Weg bauen, **(C)** die SwiftUI-App.
4. **Das Web-Board bleibt erhalten** als Desktop-/Browser-Fallback. Der Swift-Client
   ist additiv, kein Ersatz.

---

## 1. Zielarchitektur

```
   ┌─────────────────────────┐     ┌─────────────────────────┐
   │  iPhone / iPad           │     │  Mac (SwiftUI App)       │
   │  SwiftUI App (Client)    │     │  Client  +  optional:    │
   │                          │     │  startet symvibe lokal   │
   └───────────┬──────────────┘     └───────────┬──────────────┘
               │                                  │
   ┌───────────┴──────────────────────────────────┴──────────────┐
   │           gemeinsamer Networking-Layer (Swift Package)        │
   │   APIClient (REST, async/await)  ·  SSEClient (URLSession)    │
   │   ConnectionStore  ·  Keychain  ·  TLS-Pinning                │
   └───────────┬───────────────────────────────────┬──────────────┘
               │ (a) LAN / QR                        │ (b) Symaira-Account
   ┌───────────▼─────────────┐         ┌─────────────▼──────────────┐
   │  direkt: https://mac:    │        │  Symaira Relay / Rendezvous │
   │  4317  (cert pinned)     │        │  (NAT-Traversal, Auth)      │
   └───────────┬─────────────┘         └─────────────┬──────────────┘
               └───────────────┬───────────────────┘
                               ▼
   ┌──────────────────────────────────────────────────────────────┐
   │  symvibe serve  (Go-Binary, unverändertes Herz)               │
   │  internal/server  →  internal/engine  →  internal/runner →     │
   │  opencode / git / gh   auf dem echten Dateisystem             │
   └──────────────────────────────────────────────────────────────┘
```

**Zwei Verbindungsmodi, gleicher API-Vertrag:** Egal ob LAN-QR oder Relay — der
Client spricht am Ende dieselben Endpoints. Nur Transport + Auth-Bootstrap
unterscheiden sich.

---

## Arbeitspaket A — Go-Server härten *(Voraussetzung für jeden Remote-Client)*

Alles im bestehenden Repo. Ohne A funktioniert weder QR noch Account.

### A1 · Netzwerk-Bind konfigurierbar machen
- Heute bindet [`serve.go`](../cmd/symvibe/serve.go) via `net.Listen("tcp", addr)`
  an `cfg.Server.Host` (Default `127.0.0.1`).
- Neu: expliziter, bewusster „Remote-Modus". Reines `--host 0.0.0.0` ist zu
  stumpf (siehe Sicherheit). Vorschlag: neuer Flag/Config-Wert
  `server.access = "loopback" | "lan" | "relay"`, der gleichzeitig Bind **und**
  Auth-Pflicht steuert. `lan`/`relay` ⇒ Auth zwingend, sonst Start-Fehler.
- Erweiterung von `ServerConfig` in [`config.go`](../internal/config/config.go):
  ```go
  type ServerConfig struct {
      Host        string `toml:"host"`
      Port        int    `toml:"port"`
      OpenBrowser bool   `toml:"open_browser"`
      Access      string `toml:"access"`       // loopback|lan|relay   (NEU)
      TLS         TLSConfig  `toml:"tls"`        // (NEU)
      Auth        AuthConfig `toml:"auth"`       // (NEU)
  }
  ```

### A2 · TLS mit self-signed Cert + Pinning
- Beim ersten Start in `lan`/`relay`-Modus ein self-signed Zert erzeugen und
  unter `~/.local/share/symvibe/tls/{cert.pem,key.pem}` ablegen (analog zur
  Cycle-Persistenz).
- `serve.go`: bei aktivem TLS `httpSrv.ServeTLS(ln, cert, key)` statt `Serve`.
- **SHA-256-Fingerprint** des Zertifikats berechnen → wandert in den QR-Code →
  der Client pinnt genau dieses Zert (kein CA-Vertrauen nötig, robust gegen MITM
  im LAN). In Swift via `URLSessionDelegate.didReceive challenge` + Vergleich des
  Public-Key/Cert-Hashes.

### A3 · Auth-Layer (Device-Token)
- Neues `internal/auth`-Paket + ein **Middleware-Wrapper** um den `ServeMux` aus
  [`server.go`](../internal/server/server.go).
- Schema: `Authorization: Bearer <device-token>`. Token = 256-bit random, pro
  Gerät, serverseitig gehasht gespeichert (Device-Registry, A6).
- **SSE-Sonderfall:** der Swift-Client nutzt `URLSession` (kann Header setzen) —
  also Bearer-Header. Für Browser-Kompatibilität zusätzlich `?token=` auf
  `/events` akzeptieren.
- Loopback-Requests dürfen weiterhin ohne Token durch (lokales Web-Board bleibt
  funktionsfähig) — konfigurierbar.

### A4 · Pairing-Flow + QR-Erzeugung
Neue Endpoints im Router (`routes()` in [`server.go`](../internal/server/server.go)):

| Methode | Pfad | Zweck |
|---|---|---|
| `POST` | `/api/pair/start` | erzeugt Einmal-Code (TTL ~120s), liefert Pairing-Descriptor zurück (nur lokal/aus der App auslösbar) |
| `POST` | `/api/pair/complete` | Client tauscht Einmal-Code gegen langlebiges **Device-Token** |
| `GET`  | `/api/devices` | gekoppelte Geräte auflisten |
| `DELETE` | `/api/devices/{id}` | Gerät widerrufen (Token sperren) |

**QR-Payload** (Custom-Scheme, ein einziger String):
```
symvibe://pair?n=<mac-name>&p=4317
  &h=192.168.1.42&h=mac-mini.local&h=100.x.y.z   (mehrere Host-Kandidaten)
  &fp=<sha256-cert-fingerprint>
  &c=<einmal-pairing-code>
```
- Mehrere `h=` (LAN-IP, mDNS-`.local`, ggf. Tailscale-IP) → Client probiert sie
  der Reihe nach durch (Happy-Eyeballs-artig).
- QR anzeigbar an **drei** Stellen, alle nutzen denselben Flow:
  1. **CLI:** neuer Befehl `symvibe pair` rendert den QR als ASCII/Unicode im
     Terminal (z. B. `github.com/skip2/go-qrcode` → PNG, oder ein TTY-QR-Lib).
  2. **Web-Board:** Button „Mit iPhone verbinden" → ruft `/api/pair/start`,
     zeigt QR (clientseitig generierbar).
  3. **macOS-App:** eigener Pairing-Screen (CoreImage `CIQRCodeGenerator`).

### A5 · mDNS / Bonjour Advertisement *(Komfort, optional)*
- Server bewirbt `_symvibe._tcp` im LAN (z. B. `github.com/grandcat/zeroconf`).
- iPhone kann Macs im LAN dann auch **ohne** QR finden (`NWBrowser`). QR/Token
  bleibt trotzdem nötig fürs Pairing — Bonjour ersetzt nur das Abtippen der IP.

### A6 · Device-Registry + Widerruf
- Persistente Liste gekoppelter Geräte unter
  `~/.local/share/symvibe/devices.json`: `{id, name, token_hash, created, last_seen}`.
- Token sind einzeln widerrufbar (`DELETE /api/devices/{id}`) — wichtig, da „Run"
  faktisch Remote-Code-Execution ist.

### A7 · API-Version + Capabilities
- `GET /api/version` (gibt es als CLI schon — als HTTP-Endpoint ergänzen) liefert
  `{api_version, server_version, capabilities:[...]}`. Der Swift-Client prüft die
  API-Version beim Connect und kann sanft degradieren.

**Artefakt A:** Server, der sicher im LAN erreichbar ist, per QR koppelt, Tokens
ausstellt/widerruft und TLS-gepinnt spricht. *Damit ist Modus (a) komplett.*

---

## Arbeitspaket B — Symaira-Account + Relay *(Modus b: remote, außerhalb des LAN)*

Beide Geräte sind oft hinter NAT → es braucht einen Vermittler. Realistisch in
Stufen, nicht alles auf einmal:

### B0 · Pragmatischer Start: Tailscale als „Account-Ersatz"
- Bevor eine eigene Relay-Infra steht: Mac + iPhone in einem **Tailscale**-Tailnet.
  Dann ist Modus (a) (QR/LAN) auch remote nutzbar — die Tailscale-IP (`100.x.y.z`)
  ist einfach einer der `h=`-Hosts im QR. **Null zusätzliche Backend-Arbeit**,
  Ende-zu-Ende-verschlüsselt, NAT-Traversal gelöst.
- Empfehlung: B0 zuerst ausliefern, B1–B3 nur bauen, wenn „echtes" Symaira-SSO
  als Produktmerkmal gewünscht ist.

### B1 · Symaira-Identität
- OAuth/OIDC gegen den Symaira-Account. Swift-App: `ASWebAuthenticationSession`.
- Ergebnis: Account-Token im Keychain.

### B2 · Rendezvous/Relay-Service *(neue Backend-Komponente, separates Repo)*
- Schlanker WebSocket-Relay: Mac (`symvibe serve --access relay`) verbindet sich
  **ausgehend** zum Relay und registriert sich als „Node" unter dem Account.
  iPhone verbindet sich ebenfalls zum Relay, wählt einen Node → der Relay pumpt
  HTTP/SSE-Frames durch den Tunnel. symvibe-seitig nur ein zusätzlicher „Transport"
  vor dem bestehenden `http.Handler`.
- Ende-zu-Ende: das Device-Token (A3) gilt **zusätzlich** im Tunnel; der Relay
  sieht idealerweise nur verschlüsselten/getokenten Verkehr, nicht den Klartext.

### B3 · Node-Auswahl in der App
- Eingeloggte App listet die Macs/Nodes des Accounts (online/offline) und
  verbindet per Tap. Selber API-Vertrag wie LAN → der Board-Code ist identisch.

**Artefakt B:** Aus dem Mobilnetz heraus den Heim-Mac steuern, via Symaira-Login —
ohne im selben WLAN zu sein.

---

## Arbeitspaket C — die SwiftUI-App

### C1 · Projekt-Setup
- **Ein** Multiplatform-App-Target (Xcode „Multiplatform App"), Ziel-OS:
  **iOS/iPadOS 17, macOS 14 (Sonoma)** — ermöglicht das `@Observable`/Observation-
  Framework und moderne SwiftUI-APIs.
- Networking + Modelle als lokales **Swift Package** (`SymvibeKit`), damit Logik
  testbar und plattformneutral ist; das App-Target hängt nur die Views dran.
- Plattform-Unterschiede über `#if os(macOS)` / `#if os(iOS)` minimal halten.

### C2 · Networking-Layer (`SymvibeKit`)
- `APIClient` — `async/await` über `URLSession`, Bearer-Header, JSON `Codable`,
  typisierte Fehler (mappt die `{ "error": "…" }`-Antworten aus
  [`errors.go`](../internal/server/errors.go) und Statuscodes 409/503/400).
- `SSEClient` — liest `URLSession.bytes(for:)` als `AsyncSequence`, parst
  `event:` / `data:`-Zeilen, dekodiert auf das `Event`-Modell, hält die
  Verbindung (Reconnect mit Backoff, 15s-Ping vom Server toleriert).
- `TLSPinningDelegate` — pinnt den Cert-Fingerprint aus dem Connection-Profil.

### C3 · Datenmodelle (`Codable`, 1:1 aus den Go-JSON-Tags gespiegelt)
Quelle der Wahrheit sind die Go-Structs — die Felder exakt übernehmen:

```swift
// aus internal/engine/bus.go  (Event)
struct SymEvent: Codable {
    let type: String          // run_state | step_status | log | error | board
    var runID: String?        // run_id
    var stepID: String?       // step_id
    var status: String?       // pending | in_progress | done | skipped | failed | blocked | needs_review
    var kind: String?
    var line: String?
    var state: String?        // idle | running | paused
    var ts: Int64
}

// aus internal/engine/engine.go  (RunState)
struct RunState: Codable {
    let state: String         // idle | running | paused
    var runID: String?        // run_id
    var currentStep: String?  // current_step
    var cycle: String?
    var mode: String?         // step | cycle
}

// aus internal/config/cycle.go  (Cycle/Phase/Step + StepStatus)
struct Cycle: Codable, Identifiable { var id: String; var name: String
    var description: String; var phases: [Phase]; var schemaVersion: Int }
struct Phase: Codable, Identifiable { var id: String; var name: String
    var order: Int; var steps: [Step] }
struct Step: Codable, Identifiable {
    var id, name, skill, category, agent, promptSuffix: String
    var order: Int; var enabled: Bool
    var status: String        // StepStatus
    var modelOverride: Model?  // model_override
    var autoSkip: AutoSkip?    // auto_skip
    var dependsOn: [String]?   // depends_on
}
```
- `Model` / `CategoryBinding` / `ModelInfo` / `Agent` / `Skill` / `doctorResp`
  analog aus [`config.go`](../internal/config/config.go) und
  [`handlers_meta.go`](../internal/server/handlers_meta.go) spiegeln.
- `JSONDecoder.keyDecodingStrategy = .convertFromSnakeCase` deckt die meisten
  Tags ab; Ausnahmen (z. B. `id`) explizit per `CodingKeys`.

### C4 · State & Persistenz
- `@Observable`-Stores: `ConnectionStore` (Profile + aktive Verbindung),
  `BoardStore` (Cycle + Live-Status, hört auf den `SSEClient`), `RunStore`.
- **Connection-Profile** (mehrere Macs!): `{name, hostCandidates, port, certFP, accountNodeID?}`
  in `UserDefaults`/App-Group; das **Device-Token im Keychain**.
- SSE-Events mutieren den `BoardStore` → die Step-Glyphen flippen live (gleiche
  Semantik wie heute im Web-Board).

### C5 · Onboarding / Verbindung (die zwei Wege)
- **Startscreen:** „Mit Mac verbinden" → zwei Optionen:
  - **QR scannen** (Modus a): iOS `DataScannerViewController` (VisionKit) bzw.
    Kamera; parst `symvibe://pair?…`, probiert `h=`-Kandidaten, pinnt `fp`,
    ruft `POST /api/pair/complete` → Token im Keychain. Optional vorab Bonjour-Liste.
  - **Mit Symaira-Account anmelden** (Modus b): `ASWebAuthenticationSession` →
    Node-Liste → verbinden.
- **macOS** ist meist die Gegenstelle: zeigt den QR an (App-internes
  Pairing-Panel) und/oder startet die Engine (C10). Als reiner Client kann der
  Mac sich genauso per Account verbinden.

### C6 · Board-UI (Kernscreen)
- Phasen als Sektionen, **Step-Cards** mit Status-Glyph
  (`○ ◐ ✓ – ✕ ⦸ !` — Mapping aus dem README), Kategorie-Badge, Skill-Chip.
- Reaktiv auf `BoardStore`; `board`-Event ⇒ Cycle neu laden (`GET /api/cycle`).
- iPhone: vertikale Liste; iPad/macOS: mehrspaltig.

### C7 · Step-Detail / Editieren
- Sheet pro Card: Skill binden (`GET /api/skills`), Kategorie wählen
  (`GET /api/categories`), Model-Override (`GET /api/models`), enable/disable.
- Schreiben über die bestehenden Endpoints: `PUT /api/cycle` (ganzer Cycle —
  einfachster robuster Weg, so macht's das Web-Board) bzw. die granularen
  `POST/DELETE /api/cycle/step…`. **409 bei laufendem Run** sauber abfangen
  (Edits sind während eines Runs gesperrt — siehe `busy()`).
- Drag&Drop-Umsortieren später (`POST /api/cycle/step/{id}/move`).

### C8 · Run-Steuerung + Activity-Log
- Buttons: **Run Cycle** (`POST /api/run`), **Run only this**
  (`POST /api/run/step {step_id}`), **Pause/Resume/Cancel**
  (`POST /api/run/control {action}`). Vor-Check `GET /api/doctor` →
  Run-Buttons ausgrauen, wenn `runnable=false` (graceful degradation).
- Activity-Panel: Stream der `log`/`error`-Events des laufenden Steps.

### C9 · Doctor / Status
- Eigener Screen aus `GET /api/doctor` (`opencode_ok`, `git`, `gh`, `runnable`)
  inkl. opencode-Version/Pfad — spiegelt `symvibe doctor`.

### C10 · macOS: Engine einbetten & starten *(„Ein-Klick-Self-Hosted")*
- macOS-App bündelt das `symvibe`-Binary in `Contents/Resources` und startet es
  als Helfer (`Process` / `NSTask`) mit gewähltem `--dir` (Repo-Picker), zeigt
  Pairing-QR. Beenden der App ⇒ Engine sauber stoppen.
- ⚠️ **Sandbox-Konflikt, wichtige Architekturentscheidung:** Die Engine braucht
  vollen Dateisystem-Zugriff + Subprozesse (opencode/git/gh). Das ist mit der
  **App-Store-Sandbox unvereinbar**. Konsequenz:
  - **iOS-App** → App Store (reiner Remote-Client, sandbox-OK). ✅
  - **macOS-App mit eingebetteter Engine** → **Developer-ID + Notarisierung,
    außerhalb des App Store** (nicht-sandboxed). ✅
  - Alternative, falls App-Store-Mac gewünscht: macOS-App bleibt **Thin Client**,
    `symvibe serve` läuft separat per CLI/`launchd`. Dann C10 entfällt.
  - **Empfehlung (Dual-Track, siehe 2b):** App-Store-**Thin-Client** für alle drei
    Plattformen als Default-Reichweite **plus** ein optionaler Developer-ID-
    **All-in-One**-Build (Client + eingebettete Engine) außerhalb des Store für die
    Ein-Klick-„auf-dem-MacBook-starten"-UX. Eine Codebase, zwei Targets/Entitlements.

### C11 · Native Politur *(nach dem MVP)*
- **APNs Push:** Server pingt bei `failed`/`needs_review`/`done` → Push aufs
  iPhone (braucht eine kleine Push-Weiterleitung; im Relay-Fall naheliegend).
- **App Intents / Shortcuts & Siri:** „Starte den Cycle auf <Mac>".
- **WidgetKit:** aktueller Step + Status auf dem Homescreen.
- **Background refresh** für Status, **Live Activity** für laufende Runs.

---

## 2. Sicherheit (nicht verhandelbar)

- „Run" lässt opencode **echten Code gegen dein Repo** ausführen — die API ist
  faktisch ferngesteuerte Code-Ausführung. Jede Netz-Exposition braucht:
  **Token-Auth (A3) + TLS-Pinning (A2) + bewusster Access-Modus (A1)**.
- `access=lan|relay` ⇒ Auth **erzwingen**, sonst Start verweigern (fail-closed).
- Pairing-Code: einmalig, kurz-lebig; Device-Token jederzeit widerrufbar (A6).
- Relay sieht möglichst keinen Klartext (Token/Pinning gelten im Tunnel).
- SECURITY.md entsprechend erweitern (neuer „Remote-Zugriff"-Abschnitt).

---

## 2b. App-Store-Tauglichkeit

**Kurz: Ja — wenn Client und Engine getrennt ausgeliefert werden.** Das
Hindernis ist nie die UI, sondern die *Engine* (sie startet `opencode`/`git`/`gh`
und greift auf beliebige Repos zu — mit der Sandbox unvereinbar, auch Guideline
2.5.2). Konsequenz für die Distribution:

| Komponente | Auslieferung | Sandbox |
|---|---|---|
| **Client** (iOS/iPadOS/macOS, dieselbe Codebase) | **App Store** ✅ | voll sandboxed, führt lokal nichts aus |
| **Engine** (`symvibe serve`) | `go install` / Homebrew / GitHub-Release / notarisiertes Developer-ID-Helper | außerhalb des Store |
| *(optional)* **All-in-One** (Client + eingebettete Engine, C10) | **Developer-ID, notarisiert, außerhalb Store** | nicht-sandboxed |

- **iOS/iPadOS:** unkritisch — reiner Remote-Client, Präzedenz: SSH-/Remote-Dev-
  Clients (Termius, Blink, Prompt, Working Copy). Führt selbst keinen Code aus.
- **macOS App Store:** nur als **Thin Client** (verbindet zu `localhost:4317`
  bzw. übers Netz). Die Engine installiert der Nutzer separat; der Client kann
  durchs Setup führen (erkennen, ob installiert → Link zu Homebrew/Installer),
  sie unter Sandbox aber nicht selbst starten.

**Review-Checkliste (alle Punkte lösbar):**
- `2.5.2` Code-Ausführung → Client führt nichts aus. ✅
- `4.2` Mindestfunktionalität → native UI + QR + Push + Widgets, kein „Webview". ✅
- **Local Network** → `NSLocalNetworkUsageDescription` für Bonjour/mDNS deklarieren.
- **ATS + self-signed Cert** → `URLSession`-Server-Trust + Cert-Pinning (eigene
  Trust-Evaluation ist erlaubt) statt öffentlicher CA.
- `5.1.1` Account → falls Symaira-Login: In-App-Account-Löschung; **Sign in with
  Apple** nur bei *fremden* Social-Logins nötig, bei eigenem SSO nicht.
- **Export-Compliance** → `ITSAppUsesNonExemptEncryption` deklarieren.
- ⚠️ **Reviewer-Zugang** (häufigster Reject-Grund hier): die App „tut nichts"
  ohne Mac → **Demo-Modus** oder **Demo-Node + Reviewer-Credentials** bereitstellen.

→ Diese App-Store-Trennung ist die **empfohlene** Default-Strategie; C10
(eingebettete Engine) wird damit zum *zusätzlichen* Developer-ID-Build, nicht zur
Voraussetzung.

---

## 2c. Abhängigkeiten & Bootstrap (opencode, git, gh)

Die Engine braucht externe Tools. Erkennung existiert schon: `doctor` /
`/api/doctor` melden `opencode_ok`, `git`, `gh`, `runnable`; ohne opencode läuft
das Board read-only. Auflösungs-Reihenfolge für opencode (aus
[`detect.go`](../internal/runner/detect.go)):
**`SYMVIBE_OPENCODE_BIN` → `PATH` → `~/.opencode/bin/opencode` → nicht gefunden.**

| Dep | Wann nötig | Strategie |
|---|---|---|
| `symvibe` | immer | dein statisches Go-Binary — bündeln / brew / `go install` |
| `opencode` | nur „Run" mit *opencode*-Backend | **optional machen** (siehe unten) |
| `git` | faktisch immer (Sensoren + Arbeit) | macOS Xcode CLT; Install-Prompt falls fehlt |
| `gh` | **optional** — nur GitHub-Sensoren + `gh-*`-Skills | nicht erzwingen, optional anbieten |

**Mehrschichtige Lösung (von „immer" zu „nice-to-have"):**

1. **Erkennen + degradieren (vorhanden) → sichtbar machen.** App liest
   `/api/doctor`, zeigt bei fehlenden Deps konkrete Abhilfe (Befehl kopieren /
   Link). Sandboxed Store-Client installiert nichts selbst, *führt* aber.
2. **Dependency-Killer = `api`-Backend.** `runner.backend = opencode|claudecode|api`
   existiert schon. Ein **Direkt-API-Runner (Anthropic)** braucht **kein lokales
   Agent-Binary**, nur einen API-Key → der „einfach-funktioniert"-Consumer-Pfad.
   opencode bleibt der optionale Power-User-Pfad. *Wichtigste strategische Maßnahme.*
3. **Engine-Install je Distribution:**
   - **App-Store-Thin-Client:** Engine separat — **Homebrew-Tap**
     (`brew install symaira/tap/symvibe`, `depends_on` deklariert) + `curl|sh`-
     Fallback; Onboarding zeigt genau den passenden Befehl bei fehlender Engine.
   - **Developer-ID-All-in-One:** `symvibe` bündeln; opencode **mitbündeln**
     (`SYMVIBE_OPENCODE_BIN` auf Bundle-Pfad) **oder** beim ersten Start opencodes
     offiziellen Installer nach `~/.opencode/bin` laufen lassen (dort sucht symvibe
     schon) mit Fortschrittsanzeige.
4. **Versions-Vertrag.** `--format json` ist die Schnittstelle (opencode 1.17.x).
   `doctor` um **Mindestversions-Check** erweitern; Bündeln pinnt den Vertrag.

⚠️ **Vor dem Bündeln von opencode prüfen:** (a) erlaubt die **Lizenz** die
Weiterverteilung? (b) ist es ein **selbst-enthaltenes Binary** (keine versteckte
Node/Bun-Runtime)? Bei Unklarheit → Variante „Bootstrap-Installer beim 1. Start".

→ **Empfehlung:** `api`-Backend als Standard (null Deps), opencode optional;
Engine per Homebrew-Tap + Bootstrap-Onboarding ausliefern.

---

## 3. Reihenfolge / Meilensteine

| M | Inhalt | Pakete | Größe |
|---|---|---|---|
| **M1** | Server: TLS + Auth + Pairing + `symvibe pair` (QR) | A1–A4, A6, A7 | M–L |
| **M2** | SwiftUI-Skelett: Package, APIClient, SSEClient, Modelle, Connect-via-QR, **read-only Board + Live-Status** | C1–C6, C9 | L |
| **M3** | Editieren + Run-Steuerung + Activity-Log (Feature-Parität zum Web-Board) | C7, C8 | M |
| **M4** | Remote: **B0 (Tailscale)** ausliefern; mDNS-Komfort (A5) | A5, B0 | S |
| **M5** | macOS: Engine einbetten/starten, Repo-Picker, notarisierter Build | C10 | M |
| **M6** | Symaira-Account + Relay (echtes SSO/NAT-Traversal) | B1–B3 | L (eigenes Backend-Repo) |
| **M7** | Native Politur: Push, Shortcuts, Widgets | C11 | M (iterativ) |

> **Schnellster Weg zu „iPhone steuert Mac":** M1 → M2 → M3, remote zunächst über
> Tailscale (M4/B0). Das volle Symaira-Account-Relay (M6) ist die größte
> Einzelkomponente und sinnvoll erst, wenn der lokale Weg steht und sich bewährt.

---

## 4. Offene Entscheidungen

1. **Remote-Transport:** Erst Tailscale (B0, ~0 Backend-Aufwand), oder direkt
   eigenes Symaira-Relay (B2) bauen? → *Empfehlung: erst Tailscale.*
2. **macOS-Distribution:** Nur App-Store-Thin-Client, nur Developer-ID-All-in-One,
   oder **beides** (Dual-Track, siehe 2b)? → *Empfehlung: beides — Store-Client für
   Reichweite, Developer-ID-Build für die Ein-Klick-Engine.*
3. **Symaira-Account:** existiert das Identitäts-/OIDC-Backend schon, oder ist es
   Teil des Scopes? (bestimmt, ob M6 „integrieren" oder „neu bauen" heißt.)
4. **Min-OS:** iOS/macOS 17/14 (modernes SwiftUI) ok, oder ältere Geräte nötig?
```
