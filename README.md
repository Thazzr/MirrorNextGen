# MirrorNextGen

Der Nachfolger von SmartMirror2: ein schlanker Smart Mirror für einen **Raspberry Pi 5** mit **Ubuntu 26.04 LTS Server** (ohne Desktop). Alles läuft in Docker. Das Dashboard erscheint im Vollbild auf einem WQHD-Monitor im Hochformat (1440×2560). Die Drehung lässt sich in der App einstellen.

> **Stand: Phase 5 (Kalender, Bring!) abgeschlossen, Phase 6 (Kamera, Nachtruhe) in Arbeit.**
> - **Verwaltung unter `/admin`:** Anordnung, Anzeige, Wetter, WLAN-QR-Code, Kalender (Kalender, Bereiche, Regeln, Vorschau), Symbole, Bring!-Einkaufsliste und Konto.
> - **Dashboard:** Es zeigt die Module Uhr, Wetter, Kalender-Bereiche, Schichten, Bring! und WLAN-QR-Code in der eingestellten Anordnung, im Look von SmartMirror2.
> - **Kiosk-Container:** Er zeigt das Dashboard per Cage und Chromium im Vollbild und dreht es zur Laufzeit. Die Abnahme am Pi steht noch aus (siehe [`deploy/INSTALL.md`](deploy/INSTALL.md)).
> - **Noch offen:** Kamera, Nachtruhe, Logbuch und Update-Knopf. Der Host-Dienst für den Update-Knopf ist schon da (siehe [Roadmap](#roadmap)).

## Architektur

```text
Pi 5 · Ubuntu 26.04 Server · Host: nur Docker (docker compose up -d)
├─ mirror  Go-Binary: API + eingebettete React-App, :8080, Volume /data
├─ kiosk   Debian + Cage + Chromium + kioskd
│          öffnet http://mirror:8080, setzt Drehung und Monitor an/aus
└─ camera  go2rtc, nur im internen Docker-Netz           (ab Phase 6)
```

- **Server (`server/`)**: Go mit der Standardbibliothek (`net/http`, `slog`, `embed`). Das Binary bettet die gebaute React-App ein und liefert sie aus. Unbekannte Client-Routen bekommen `index.html`, unbekannte `/api/`-Pfade einen JSON-404. Gehashte Assets werden ein Jahr gecacht.
- **Oberfläche (`web/`)**: React 19, Vite, TypeScript, React Query. Layout und Design des Dashboards stammen aus SmartMirror2. Der Admin-Bereich wird neu gestaltet.
- **API-Vertrag (`api/openapi.yaml`)**: spec-first. Daraus erzeugt `oapi-codegen` den Go-Server-Code und `openapi-typescript` die Client-Typen. Die Oberfläche spricht die API nur über den erzeugten Client an.
- **Daten**: keine Datenbank. Einstellungen liegen ab Phase 2 als JSON-Datei im Volume `/data`.

| Pfad | Inhalt |
|---|---|
| `api/` | OpenAPI-Vertrag |
| `server/` | Go-Modul: `cmd/mirror` (Server), `cmd/kioskd` (Kiosk-Agent), `internal/{api,auth,bring,calendar,config,fsutil,httpserver,icons,kiosk,modules,settings,weather,web,wifi}` |
| `web/` | React-App (`src/app`, `src/shared`, `src/features/{dashboard,admin,modules}`); je Modul ein Ordner mit Widget und Admin-Seite; die Verwaltung (Mantine) lädt als eigener Chunk |
| `kiosk/` | Dockerfile des Kiosk-Images (Cage, Chromium, kioskd) |
| `deploy/` | `compose.yaml`, `.env.example`, `INSTALL.md` (Installation auf dem Pi) und `host/` (Update-Dienst für den Host) |
| `tools/` | Prüfskript `verify.mjs`, Changelog-Werkzeug `changelog.mjs` |
| `.github/workflows/` | CI: Prüfung und Images für GHCR |

## Entwicklung

Voraussetzungen:
- **Go 1.27**: Mit `GOTOOLCHAIN=auto` (Standard) lädt Go die Version aus `go.mod` selbst nach.
- **Node.js 22 oder neuer**: Die CI nutzt Node 26.
- **git**

Abhängigkeiten der Oberfläche einmalig installieren:

```bash
npm ci --prefix web
```

Server mit gebauter Oberfläche starten:

```bash
npm --prefix web run build      # schreibt nach server/internal/web/dist
cd server
MIRROR_ADDR=127.0.0.1:8080 MIRROR_DATA_DIR=./data go run ./cmd/mirror
```

Danach ist das Dashboard unter <http://127.0.0.1:8080/> erreichbar, das Testbild unter <http://127.0.0.1:8080/?diagnose>. Ohne gebaute Oberfläche antwortet der Server mit 503 und einem Hinweis.

Oberfläche mit Hot Reload entwickeln: den Server wie oben starten, dann `npm --prefix web run dev`. Vite leitet `/api` an `:8080` weiter.

| Umgebungsvariable | Standard | Bedeutung |
|---|---|---|
| `MIRROR_ADDR` | `:8080` | Listen-Adresse (`host:port`) |
| `MIRROR_DATA_DIR` | `/data` | Verzeichnis für Einstellungen und Daten |
| `MIRROR_OPENWEATHER_URL` | `https://api.openweathermap.org/` | Basis-URL von OpenWeather (nur für Tests mit einem nachgebauten Server) |
| `MIRROR_BRING_URL` | `https://api.getbring.com/rest/v2/` | Basis-URL von Bring! (nur für Tests) |
| `MIRROR_CONTROL_DIR` | – | Gemeinsames Verzeichnis mit dem Host-Update-Dienst (im Container `/control`) |

### Verwaltung

Die Verwaltung liegt unter `/admin` und funktioniert am Handy wie am Rechner.
- **Beim ersten Aufruf:** Du legst das Admin-Passwort fest (mindestens 10 Zeichen). Gespeichert wird es als Argon2id-Hash in `<MIRROR_DATA_DIR>/admin.json`.
- **Sitzung:** Eine Anmeldung gilt 8 Stunden ab der letzten Nutzung.
- **Fehlversuche:** Nach 5 falschen Passwörtern in 15 Minuten sperrt der Server weitere Versuche dieser Adresse vorübergehend.
- **Einstellungen:** Sie liegen in `<MIRROR_DATA_DIR>/settings.json`. Eine unlesbare oder fremde Datei wird als `settings.json.discarded-<Zeit>` beiseitegelegt, und der Server startet mit Standardwerten.
- **Passwort vergessen:** Server stoppen, `admin.json` im Datenverzeichnis löschen, neu starten und neu einrichten.
- **Module und Anordnung:** Der Spiegel besteht aus Modulen: Uhr, Wetter, WLAN-QR-Code, je ein Modul pro Kalender-Bereich und Bring!. Die Kamera folgt. Unter „Anordnung“ legst du fest, welches Modul in welcher der fünf Regionen steht: oben links, oben rechts, Mitte, unten links, unten rechts. Was nicht eingeordnet ist, bleibt aus.
- **Wetter:** Es braucht einen OpenWeather-API-Key mit dem Abo „One Call API 3.0“ (1.000 Abrufe am Tag sind kostenlos).
- **Kalender:**
  - Du trägst beliebig viele private iCal-Links ein (Google, iCloud, Outlook) und legst Bereiche an, entweder Terminlisten oder eine Schichtanzeige für den Dienstplan. Neue Bereiche erscheinen oben links.
  - Regeln blenden Termine aus, setzen Symbole oder benennen sie um. Die Vorschau zeigt mit echten Terminen, welche Regel greift.
  - Serientermine, Ausnahmen, ganz- und mehrtägige Termine sowie Zeitzonen werden korrekt berechnet.
  - Die iCal-Adressen bleiben auf dem Server.
- **Symbole:** eigene SVG- oder PNG-Dateien bis 256 KB. SVGs werden auf aktive Inhalte geprüft.
- **Bring!:** Anmeldung mit E-Mail und Passwort deines Bring!-Kontos; die Liste wählst du aus. Am Spiegel erscheinen bis zu 18 Einträge.

### API ändern

1. `api/openapi.yaml` anpassen.
2. `npm run generate` erzeugt den Go-Server-Code und die TypeScript-Typen neu.
3. Den Handler in Go umsetzen und testen.
4. In der Oberfläche über `api` und `unwrap()` aus `web/src/shared/api/client.ts` aufrufen.

Erzeugte Dateien (`*.gen.go`, `schema.gen.ts`) werden committet und nie von Hand geändert.

## Prüfen

```bash
npm run verify
```

Das Gate läuft unter Windows, Linux und in der CI.
- **Selbsttest:** Zuerst prüft es seine eigene Logik (`node --test 'tools/*.test.mjs'`).
- **Zwei Stränge, parallel:**
  - Backend: `go mod tidy -diff`, Drift des erzeugten Go-Codes, `gofmt`, `go vet`, `go test`.
  - Frontend: Drift der API-Typen, `tsc`, ESLint, Vitest, Build.
- **Ergebnis:** Am Ende stehen beide Logs und eine Tabelle mit Dauer und Ergebnis je Schritt.

| Schalter | Wirkung |
|---|---|
| `--only <schritt>[,<schritt>]` | nur diese Schritte (unbekannter Name: Exit 2) |
| `--sequential` | alles nacheinander, zum Eingrenzen |
| `--skip-build` | ohne Frontend-Build |
| `--race` | `go test` mit Race-Detector (CI) |

Einzelprüfungen:
- **Backend** (aus `server/`): `go test ./...`
- **Frontend** (aus `web/`): `npx tsc -p tsconfig.app.json`, `npm run lint`, `npx vitest run`

## Deployment

Die CI baut bei jedem Push auf `main`, `claude/**` und bei Tags `v*` das Image `ghcr.io/thazzr/mirrornextgen/mirror` für `linux/arm64` und `linux/amd64`. Beide Images entstehen per Cross-Compile, ohne QEMU.

| Tag | Quelle |
|---|---|
| Branch-Name | letzter Stand des Branches |
| `sha-…` | ein bestimmter Commit |
| Version | bei Tags `v*` |
| `latest` | `main` |

Auf dem Pi:

```bash
cp deploy/.env.example deploy/.env     # MIRROR_TAG, MIRROR_PORT, TZ anpassen
docker compose -f deploy/compose.yaml --env-file deploy/.env up -d
```

Die Schritt-für-Schritt-Anleitung für den Pi 5 mit Ubuntu 26.04 Server (Anzeige-Vorbereitung, Docker, Start, Abnahme-Checkliste, Fehlersuche) steht in [`deploy/INSTALL.md`](deploy/INSTALL.md). Solange die Images in GHCR privat sind, braucht der Pi einmalig `docker login ghcr.io` mit einem Token mit `read:packages`.

## Roadmap

| Phase | Version | Inhalt |
|---|---|---|
| 1 | 0.1.0 | Grundgerüst: Go-Server, React-Oberfläche, Prüfskript, Image und CI |
| 2 | 0.2.0 | Admin-Zugang und Einstellungen (neu gestaltet), erster Abschnitt „Anzeige“ mit Drehung |
| 3 | 0.3.0 | Kiosk-Container auf dem Pi: Cage + Chromium, Drehung per `wlr-randr`, Installationsanleitung |
| 4 | 0.4.0 | Module und freie Anordnung (fünf Regionen), Uhr, Wetter, WLAN-QR im Layout von SmartMirror2 |
| 5 | 0.5.0 | Kalender (Neuentwurf: Bereiche, Regeln mit Vorschau, Schichtanzeige), Icon-Bibliothek, Bring! |
| 6 | 0.6.0 | Kamera (go2rtc) und Nachtruhe |
| 7 | 0.7.0 | Update-Knopf und Logbuch |

## Mitarbeit

Regeln, Architekturgrenzen und den Ablauf pro Phase beschreibt [`CLAUDE.md`](CLAUDE.md): Planen und Prüfen, Umsetzen durch Subagenten, Gegenproben, Gate, Conventional Commits, SemVer-Tags.

[`CHANGELOG.md`](CHANGELOG.md) wird mit `npm run changelog` aus den Commit-Nachrichten erzeugt und nie von Hand bearbeitet.

## Lizenz

MirrorNextGen steht unter der [MIT-Lizenz](LICENSE). Mitgelieferte Fremdbestandteile behalten ihre eigenen Lizenzen: IBM Plex Sans (SIL Open Font License 1.1), Bootstrap Icons, Tabler Icons und Mantine (MIT). Die Lizenzen der npm- und Go-Abhängigkeiten liegen bei den jeweiligen Paketen.
