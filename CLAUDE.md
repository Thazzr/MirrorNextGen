# MirrorNextGen – Projektregeln

Diese Datei ist die **einzige Regelquelle** des Repos (`AGENTS.md` verweist nur hierher). Den Entwicklungsablauf hat MirrorNextGen aus dem Schwesterprojekt Core übernommen und an Go/TypeScript angepasst. Layout und Design des Dashboards stammen aus SmartMirror2, das weiterläuft, bis MirrorNextGen es ersetzt.

## Was das Projekt ist

Ein Smart Mirror für einen **Raspberry Pi 5** mit **Ubuntu 26.04 LTS Server** (ohne Desktop). Er läuft vollständig in Docker und zeigt das Dashboard im Vollbild auf einem WQHD-Monitor im Hochformat (1440×2560 CSS-Pixel, `devicePixelRatio` 1). Auf dem Host ist nur Docker installiert; gestartet wird von der Konsole mit `docker compose up -d`.

```text
Pi 5 · Ubuntu 26.04 Server · Host: nur Docker
├─ mirror  Go-Binary: API + eingebettete React-App, :8080, Volume /data
├─ kiosk   Debian trixie-slim + Cage + Chromium + kioskd (Go-Agent)
│          öffnet http://mirror:8080, setzt Drehung und Monitor an/aus per wlr-randr
└─ camera  go2rtc, nur im internen Docker-Netz
```

## Workflow

- Keine Dev-Server oder Hintergrundprozesse starten, die nicht für die Aufgabe nötig sind (API-Tests, Screenshots, Integrationstests). Selbst gestartete Prozesse am Ende wieder beenden.
- Vor Änderungen an der Architektur oder größeren Umbauten erst planen.
- Für Änderungen die Edit-Werkzeuge verwenden. Codeblöcke im Chat nur für Beispiele, immer mit Sprache im Info-String.
- Nach jeder abgeschlossenen Aufgabe die passenden Prüfungen laufen lassen, bevor die Antwort fertig ist.
- Am Ende jeder Aufgabe die nächsten sinnvollen Schritte vorschlagen.

### Arbeitsteilung: planen und kontrollieren – umsetzen

- **Die Hauptsitzung plant und kontrolliert, sie schreibt keinen Produktcode.** Sie klärt die Anforderung, zerlegt sie in Arbeitspakete und prüft das Ergebnis. Sie schreibt die Commit-Nachrichten, setzt Tags und pflegt `README.md` und `CLAUDE.md`. Code und Tests gehen als Paket an einen Subagenten, auch kleine Korrekturen aus dem Review.
- **Umsetzen: Subagent `implementer`** (Sonnet, `.claude/agents/implementer.md`). Ein Aufruf bekommt genau ein Paket mit
  - Ziel,
  - betroffenen Dateien bzw. Bereichen,
  - Akzeptanzkriterien,
  - den zu schreibenden Tests,
  - den Prüfbefehlen.

  Produktentscheidungen trifft die Hauptsitzung vorher (mit dem Nutzer). Ein Paket enthält keine offene Frage, die der Subagent raten müsste.
- **Parallel nur, was keine Dateien teilt.** `api/openapi.yaml` und die daraus erzeugten Dateien sind eine gemeinsame Engstelle: Pakete, die sie berühren, laufen nacheinander. Parallele Pakete laufen mit `isolation: "worktree"` (dort fehlt `node_modules`, also zuerst `npm ci`).
- **Kontrollieren, bevor committet wird.** Die Hauptsitzung
  1. liest den Diff (`git diff`) gegen Paket und Hausregeln;
  2. lässt den Subagenten `mutation-checker` (Sonnet, `.claude/agents/mutation-checker.md`) die neuen Tests mit gezielten Gegenproben prüfen;
  3. führt `npm run verify` aus;
  4. sieht sichtbare Änderungen im Browser an – per Playwright-Screenshot bei 1440×2560, für das Admin-UI zusätzlich bei Handybreite.

  Befunde gehen als neues Paket zurück an den `implementer`. Committet wird erst, wenn Diff, Gegenproben und Gate stimmen.
- **Subagenten** committen, taggen und pushen nicht. Sie starten keine weiteren Subagenten und berichten nach dem festen Schema ihrer Definition. Der Bericht ist Material, kein Beweis: Was er behauptet, prüft die Hauptsitzung selbst nach.
- Neue Dateien in `.claude/agents/` lädt Claude Code erst in einer neuen Sitzung. In einer Sitzung, die sie noch nicht kennt, startet die Hauptsitzung einen allgemeinen Agenten mit dem Modell Sonnet und lässt ihn die Rollendatei lesen und befolgen.

### Nach JEDER Phase (obligatorisch, in dieser Reihenfolge)

1. **README.md prüfen und ggf. aktualisieren** – wenn die Phase Struktur, Setup, Architektur, Prüfbefehle, Deployment oder Abhängigkeiten ändert.
2. **Commit mit Conventional Commit** – Format `type(scope): beschreibung`, Betreff auf Deutsch. Der Betreff wird zur Changelog-Zeile, deshalb **immer einen Body** mit Ursache bzw. Anlass, Lösung und Prüfung. Typ → Sektion: `feat`→Added, `fix`→Fixed, `perf`/`refactor`/`docs`/`test`/`chore`/`build`/`style`→Changed, `revert`→Removed, `fix(sec…)`/`sec…`→Security. `ci` erscheint nicht im Changelog.
3. **Versions-Tag setzen + CHANGELOG.md erzeugen (nie von Hand editieren)** – Bei einer eigenständig abgeschlossenen Phase setzt `node tools/changelog.mjs --tag v0.X.0 --message "<Kurzbeschreibung>"` einen annotierten Tag auf HEAD und erzeugt `CHANGELOG.md` neu. Bei einem Zwischenschritt genügt `node tools/changelog.mjs` ohne Tag. Die neue `CHANGELOG.md` wird als eigener Commit `docs(changelog): update` committet (erscheint selbst nicht im Changelog).
4. **Pushen** – den Branch und den neuen Tag pushen. Der Cloud-Container ist flüchtig, lokale Tags gingen sonst verloren (bewusste Abweichung von Core).
5. **Nächste Schritte vorschlagen.**
6. **Keine offenen Änderungen mitschleifen** – vor Beginn der nächsten Phase ist `git status` sauber.

## Versioning

- SemVer `0.MINOR.PATCH`, solange das Projekt vor 1.0 ist. MAJOR bleibt `0`, bis der Nutzer MirrorNextGen ausdrücklich als Ersatz für SmartMirror2 freigibt.
- MINOR-Bump, wenn die Phase mindestens einen `feat`- oder Breaking-Commit enthält, sonst PATCH.
- Der Changelog wird aus der gesamten Historie erzeugt. `cliff.toml` gruppiert ihn nach Tags; Commits ohne Conventional-Format fallen heraus.

## Projektstand

- Frühe Entwicklung, **noch nicht im Einsatz**. Es gibt keine Bestandsdaten, die erhalten werden müssten: Das Einstellungsformat darf sich brechend ändern (dann `schemaVersion` erhöhen und alte Dateien verwerfen, keine Migration).
- SmartMirror2 bleibt Vorlage für Layout und Design, aber kein Code wird übernommen, ohne ihn zu verstehen. Bekannte Schwächen dort, die hier **nicht** wiederholt werden:
  - Serientermine wurden nicht expandiert;
  - die Display-Steuerung führte frei eingegebene Shell-Befehle aus;
  - Updates bauten die App auf dem Pi neu.
- HomeConnect und Roborock gehören nicht zum Umfang.

## Architektur-Regeln

- **Repo-Layout:**

  | Pfad | Inhalt |
  |---|---|
  | `api/openapi.yaml` | API-Vertrag |
  | `server/` | Go-Modul, `cmd/<binary>` und `internal/<fachbereich>` |
  | `web/` | React-App |
  | `kiosk/` | Kiosk-Image |
  | `deploy/` | Compose und Installationsanleitung |
  | `tools/` | Entwicklerwerkzeuge |

- **API spec-first.** `api/openapi.yaml` (OpenAPI 3.0.3) ist die einzige Quelle für Server- und Client-Typen. Server-Code erzeugt `oapi-codegen` (strict server auf `net/http`, versioniert über die `tool`-Direktive in `server/go.mod`). Den Client erzeugen `openapi-typescript` und `openapi-fetch`. Erzeugte Dateien werden committet und nie von Hand geändert. Das Gate erkennt, wenn sie nicht zum Vertrag passen. Ablauf für jeden neuen oder geänderten Endpunkt:
  1. Vertrag in `api/openapi.yaml` ändern;
  2. `npm run generate` (Go-Server-Code und TS-Typen);
  3. Handler in Go implementieren und testen;
  4. Feature-API-Funktion im Frontend über den erzeugten Client schreiben.
- **Jeder Frontend-API-Aufruf läuft über den erzeugten Client** (`web/src/shared/api/client.ts`). Rohes `fetch` auf `/api/*` ist verboten.
- **Keine Datenbank.**
  - Persistenter Zustand liegt unter `MIRROR_DATA_DIR` (Container: `/data`, ein Docker-Volume).
  - Einstellungen liegen in `settings.json` mit `schemaVersion`.
  - Schreiben immer atomar: temporäre Datei im selben Verzeichnis, dann rename.
- **Geheimnisse verlassen den Server nie.**
  - API-Keys, Passwörter und Tokens werden nie an den Client ausgeliefert. Die Oberfläche bekommt nur „ist gesetzt“ (`hasX: true`), ein leer gesendetes Geheimnis behält den gespeicherten Wert.
  - Geheimnisse erscheinen nicht im Log.
- **Keine frei konfigurierbaren Befehle.** Der Server führt keine Shell-Befehle aus Einstellungen aus. Monitor an/aus, Drehung und Updates sind feste, im Code benannte Funktionen. Prozesse werden nur mit fester Argumentliste gestartet (`exec.Command(name, args...)`), nie über eine Shell.
- **Drehung und Monitor-Schaltung gehören dem Kiosk-Agenten.** `kioskd` liest den Sollzustand vom Server (`GET /api/v1/kiosk/state`) und setzt ihn per `wlr-randr` am Compositor. Der `mirror`-Container braucht dafür keine Rechte.
- **Interne Dienste bleiben intern.** go2rtc und andere Hilfsdienste werden nicht ins LAN veröffentlicht. Was der Browser davon braucht, reicht `mirror` gezielt durch.
- **Zeitzone:** `TZ` wird einmal in `deploy/.env` gesetzt (Standard `Europe/Berlin`) und gilt für alle Container. Das Go-Binary bettet `time/tzdata` ein, damit es auch im distroless-Image funktioniert.

## Go-Regeln (`server/`)

- **Standardbibliothek zuerst.** `net/http` mit `ServeMux`, `log/slog`, `context`, `encoding/json`, `embed`. Eine Abhängigkeit kommt nur dazu, wenn sie echte Arbeit abnimmt (iCal/RRULE, QR, Argon2), und mit fester Version in `go.mod`.
- **Ein Paket pro Fachbereich** unter `internal/`, zum Beispiel `weather` oder `calendar`.
  - Die Fachlogik ist eine reine Funktion ohne I/O, zum Beispiel die Berechnung relativer Zeiten oder die Auswertung des Zeitplans. Sie bekommt die Uhrzeit als Parameter (`now time.Time`) und ruft nie `time.Now()` direkt auf.
  - HTTP-Aufrufe zu Fremddiensten laufen über ein `*http.Client`-Feld mit Timeout, damit Tests einen `httptest.Server` einsetzen können.
  - Handler in `internal/api` sind nur Orchestrierung.
- **Fehler** werden mit `fmt.Errorf("…: %w", err)` eingewickelt. Erwartbare Fachfehler sind eigene Fehlerwerte oder -typen, keine Panics. Für Fremddienste (Wetter, Kalender, Bring!) gilt: Der Ausfall eines Dienstes darf das Dashboard nicht leeren. Der betroffene Bereich trägt dann eine Fehlermeldung, die anderen bleiben sichtbar.
- **Nebenläufigkeit:** Caches und geteilter Zustand sind mit `sync.Mutex` geschützt. Jede Goroutine hat einen Besitzer und endet mit dem `context`. Tests laufen in der CI mit `-race`.
- **Logging** mit `slog`, strukturiert, auf Englisch, ohne Geheimnisse.
- **Formatierung:** `gofmt`. `go vet` muss sauber sein.

## Frontend-Regeln (`web/`)

- `web/src` ist in `app`, `shared` und `features/<feature>` gegliedert. Wiederverwendbares liegt in `shared`, Feature-Spezifisches im Feature-Ordner.
- **Datenladen nur mit `@tanstack/react-query`**, keine handgebauten `useState`+`useEffect`+fetch-Hooks.
  - **Ein `queryFn` wirft, wenn der Aufruf fehlschlug**: Der erzeugte Client wirft nicht, er liefert `{ data, error }`. Deshalb jedes Ergebnis über `unwrap()` aus `shared/api/client.ts` führen, nie `result.data ?? []` zurückgeben. Sonst wird ein 500 zu einem leeren Erfolg.
  - Jede optimistische Cache-Änderung braucht einen Fehlerpfad (vorherigen Wert zurückschreiben oder invalidieren).
- **Das Dashboard** ist für 1440×2560 CSS-Pixel gebaut und ohne Cursor, Interaktion oder Scrollen.
  - Layout und Stil folgen SmartMirror2 (`SmartMirror2/smartmirror2.client/src/features/dashboard/`): schwarzer Hintergrund, weiße Schrift, IBM Plex Sans (mitgeliefert über `@fontsource`).
  - Sekundärtexte sind hell genug für das teildurchlässige Spiegelglas.
  - Ein ausgefallener Datenbereich zeigt seine Fehlermeldung an Ort und Stelle.
- **Das Admin-UI** wird am Handy und am Rechner benutzt und muss bis 360 px Breite ohne waagerechtes Scrollen funktionieren.
  - Texte auf Deutsch mit „du“.
  - Jede Seite hat genau eine `h1`, Abschnitte sind `h2`.
- **Datum und Uhrzeit**: Anzeige mit `Intl.DateTimeFormat('de-DE', …)`. Auf dem Draht sind Zeitpunkte ISO 8601 mit Offset, reine Daten `yyyy-MM-dd`, Uhrzeiten `HH:mm`.
- TypeScript `strict`. ESLint muss sauber sein.

## Tests

- **Jede neue Regel, jeder Handler und jedes UI-Verhalten bekommt einen Test, der ohne die Änderung fehlschlagen würde.**
- **Go:**
  - Tabellengetriebene Tests mit `testing`.
  - Handler über `httptest`.
  - Fremddienste über `httptest.Server` mit aufgezeichneten Antworten (Fixtures unter `testdata/`).
  - Zeitabhängiges mit festem `now`.
- **Frontend:**
  - Vitest + Testing Library + jsdom.
  - API-Aufrufe in Tests über einen gemockten `fetch` am erzeugten Client, nicht durch Umgehen des Clients.
- **Zeitzonen:** Tests, die von der Zeitzone abhängen, setzen sie ausdrücklich (`time.LoadLocation("Europe/Berlin")` bzw. `TZ` in der Vitest-Konfiguration) und decken die Sommerzeitumstellung ab, wo sie eine Rolle spielt.

## Verifikation

`npm run verify` (= `node tools/verify.mjs`) ist das vollständige Gate, lokal und in der CI. Es läuft unter Windows, Linux und macOS.

- **Stränge:** Zwei Stränge laufen parallel und schreiben je ein eigenes Log. Am Ende gibt das Skript beide Logs aus und eine Tabelle mit Dauer und Ergebnis je Schritt.
  - **Backend:** Drift der erzeugten Go-Dateien, `gofmt`, `go vet`, `go test` (in der CI mit `-race`).
  - **Frontend:** Drift der API-Typen, `tsc`, ESLint, Vitest, Build.
- **Fehler:** Ein fehlgeschlagener Schritt beendet nur seinen Strang. Das Skript endet mit einem Exit-Code ungleich 0, sobald ein Ergebnis weder `ok` noch `skipped` ist.
- **Schalter:**
  - `--sequential` lässt alles nacheinander laufen (zum Eingrenzen);
  - `--only <schritt>` lässt nur einzelne Schritte laufen (ein unbekannter Name endet mit Exit 2, bevor etwas läuft);
  - `--skip-build` lässt den Frontend-Build weg.
- **Das Gate testet sich selbst:** Sein erster Schritt `node --test tools/` prüft die Logik, nach der das Skript über Bestanden oder Fehlgeschlagen entscheidet. Wer diese Logik ändert, ändert den Test mit. Die Umgebungsmarke `VERIFY_SELFTEST_RUNNING` verhindert, dass der Selbsttest das Gate erneut startet.
- **Drift-Prüfung:** Sie erzeugt die Dateien in ein temporäres Verzeichnis und vergleicht sie mit dem Arbeitsbaum. Sie ändert nichts. Behoben wird Drift mit `npm run generate`.

Schnelle Einzelprüfungen:
- Backend (aus `server/`): `go test ./...`, `go vet ./...`.
- Frontend (aus `web/`): `npx tsc -p tsconfig.app.json`, `npm run lint`, `npx vitest run <pfad>`.

Der Docker-Build ist nicht Teil des Gates, er läuft in der CI. Dort baut der Job `image` die Images für `linux/arm64` und `linux/amd64` und pusht sie nach GHCR.

## Deployment

- **Images:** CI baut `ghcr.io/thazzr/mirrornextgen/<dienst>` aus `main`, `claude/**` und Tags `v*`.
  - **Build ohne QEMU:** Das Frontend und das Go-Binary entstehen auf der Build-Plattform (Cross-Compile, `CGO_ENABLED=0`); das Zielimage kopiert nur fertige Dateien.
  - **Image-Tags:** Branch, `sha-…`, die Version bei Tags und `latest` auf `main`.
- **Auf dem Pi:** `deploy/compose.yaml` und `deploy/.env`. Der Pi lädt nur Images (`docker compose pull && docker compose up -d`) und baut nichts selbst.
- **Laufzeit-Image `mirror`:** distroless `static` als `nonroot` (UID 65532). Der Healthcheck läuft über `mirror healthcheck`, weil das Image keine Shell hat.
- **Einzelinstanz:** Caches und Sitzungen liegen im Prozessspeicher. Das ist für einen Spiegel richtig. Eine zweite Instanz ist nicht vorgesehen.
