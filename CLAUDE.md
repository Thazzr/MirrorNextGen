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
2. **Commit mit Conventional Commit** – Autor und Committer jedes Commits ist **`Thazzr <22547926+Thazzr@users.noreply.github.com>`** (Nutzerentscheidung 2026-11-14: kein Klarname, keine private oder Firmen-E-Mail in der Historie; die lokale Git-Konfiguration des Repos ist so gesetzt, in einem frischen Klon vor dem ersten Commit `git config user.name Thazzr` und `git config user.email 22547926+Thazzr@users.noreply.github.com`). Ein per cherry-pick übernommener Commit bekommt danach `git commit --amend --no-edit --reset-author`. Format `type(scope): beschreibung`, Betreff auf Deutsch. Der Betreff wird zur Changelog-Zeile, deshalb **immer einen Body** mit Ursache bzw. Anlass, Lösung und Prüfung. Typ → Sektion: `feat`→Added, `fix`→Fixed, `perf`/`refactor`/`docs`/`test`/`chore`/`build`/`style`→Changed, `revert`→Removed, `fix(sec…)`/`sec…`→Security. `ci` erscheint nicht im Changelog.
3. **Versions-Tag setzen + CHANGELOG.md erzeugen (nie von Hand editieren)** – Bei einer eigenständig abgeschlossenen Phase setzt `node tools/changelog.mjs --tag v0.X.0 --message "<Kurzbeschreibung>"` einen annotierten Tag auf HEAD und erzeugt `CHANGELOG.md` neu. Bei einem Zwischenschritt genügt `node tools/changelog.mjs` ohne Tag. Die neue `CHANGELOG.md` wird als eigener Commit `docs(changelog): update` committet (erscheint selbst nicht im Changelog).
4. **Pushen** – den Branch und den neuen Tag pushen. Der Cloud-Container ist flüchtig, lokale Tags gingen sonst verloren (bewusste Abweichung von Core). Verweigert die Umgebung den Tag-Push (die Cloud-Sitzung erlaubt nur Pushes auf ihren Arbeits-Branch, ein Tag-Push endet mit HTTP 403), wird das nicht umgangen: Die Hauptsitzung nennt dem Nutzer Tag, Commit und Tag-Nachricht, damit er ihn selbst setzt (`git tag -a vX.Y.Z <commit> -m "<nachricht>"` und `git push origin vX.Y.Z`).
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
- **Einzige Ausnahme: der Kamera-Stream** `GET /api/v1/camera/stream`. Er ist ein WebSocket-Proxy (`httputil.ReverseProxy`) und kein Strict-Server-Endpunkt. Er steht im Vertrag (101/404/426/502), ist per `exclude-operation-ids` von der Erzeugung ausgenommen und wird in `api.Register` von Hand registriert, bewusst öffentlich wie das Dashboard. Er leitet nur zur festen Quelle `mirror-camera` weiter, nichts aus der Anfrage des Browsers bestimmt die Quelle. Der Player `web/src/vendor/go2rtc/video-rtc.js` (MIT, unverändert bis auf den Herkunftskopf) öffnet ihn direkt.
- **Keine Datenbank.**
  - Persistenter Zustand liegt unter `MIRROR_DATA_DIR` (Container: `/data`, ein Docker-Volume).
  - Einstellungen liegen in `settings.json` mit `schemaVersion`.
  - Schreiben immer atomar: temporäre Datei im selben Verzeichnis, dann rename.
- **Geheimnisse verlassen den Server nie.**
  - API-Keys, Passwörter und Tokens werden nie an den Client ausgeliefert. Die Oberfläche bekommt nur „ist gesetzt“ (`hasX: true`), ein leer gesendetes Geheimnis behält den gespeicherten Wert.
  - Geheimnisse erscheinen nicht im Log.
  - Einzige bewusste Ausnahme: das Gast-WLAN-Passwort steckt im QR-Code des WLAN-Moduls, denn ihn anzuzeigen ist der Zweck des Moduls. Der öffentliche Endpunkt liefert nur die QR-Matrix, nie das Passwort als Text.
- **Keine frei konfigurierbaren Befehle.** Der Server führt keine Shell-Befehle aus Einstellungen aus. Monitor an/aus, Drehung und Updates sind feste, im Code benannte Funktionen. Prozesse werden nur mit fester Argumentliste gestartet (`exec.Command(name, args...)`), nie über eine Shell.
- **Updates per Knopf laufen auf dem Host, nicht im Container.** Der Server schreibt nur `update-request` atomar in `MIRROR_CONTROL_DIR` und liest `update-status.json` (`internal/update`); `deploy/host/mirror-update.sh` (systemd-`.path`-Unit) führt `docker compose pull` und `up -d` aus. Der `mirror`-Container bekommt dafür weder Docker-Socket noch Rechte.
- **Drehung und Monitor-Schaltung gehören dem Kiosk-Agenten.** `kioskd` liest den Sollzustand vom Server (`GET /api/v1/kiosk/state`, mit `rotation` und `powerOn`) und setzt ihn per `wlr-randr` am Compositor. Der `mirror`-Container braucht dafür keine Rechte.
  - Die Nachtruhe berechnet der Server (`internal/display`, reine Funktionen mit `now`, Aus-Zeit inklusive, Ein-Zeit exklusive).
  - `kioskd` schaltet mit `--off`/`--on`, setzt nach `--on` die Drehung neu und bleibt beim zuletzt geschalteten Ausgang, auch wenn er ausgeschaltet als `enabled: false` erscheint.
  - Das Dashboard rendert bei `powerOn: false` nur eine schwarze Fläche (Rückfall für Monitore, die das Abschalten ignorieren, und keine Arbeit in der Nacht).
- **Interne Dienste bleiben intern.** go2rtc und andere Hilfsdienste werden nicht ins LAN veröffentlicht. Was der Browser davon braucht, reicht `mirror` gezielt durch.
  - go2rtcs API hat keine Anmeldung und kann Befehle ausführen (`exec:`-Quellen). Der Server registriert deshalb nur `rtsp://`-, `rtsps://`- und `rtspx://`-Adressen (RTSPS ohne Zertifikatsprüfung, für UniFi Protect) unter einem festen Namen. Den Parameter `enableSrtp` aus UniFi-Links nimmt er vor der Übergabe heraus, gespeichert bleibt die Adresse unverändert.
  - go2rtcs eigener RTSP-Server ist in `deploy/compose.yaml` abgeschaltet.
  - Die Stream-Adresse enthält meist Zugangsdaten: Sie ist ein Geheimnis und erscheint in keinem Fehler und keinem Log.
- **Zeitzone:** `TZ` wird einmal in `deploy/.env` gesetzt (Standard `Europe/Berlin`) und gilt für alle Container. Das Go-Binary bettet `time/tzdata` ein, damit es auch im distroless-Image funktioniert.

## Module und Anordnung

Nutzerentscheidung 2026-11-14: Der Spiegel besteht aus **internen Modulen**, und der Nutzer **ordnet sie frei an**. Ein Plugin-System mit Fremdcode zur Laufzeit gibt es bewusst nicht.

- **Module:** Uhr (`clock`), Wetter (`weather`), Kalender-Bereiche (`calendar:<bereichs-id>`, eine Instanz je Bereich), Schichten (`calendar:<bereichs-id>` vom Typ Schichten), WLAN (`wifi`), Bring! (`bring`) und Kamera (`camera`). Ein Modul bringt mit:
  - einen Einstellungsabschnitt in `settings.json`;
  - eigene, typisierte Endpunkte im Vertrag: Einstellungen (geschützt) und Dashboard-Daten (öffentlich, ohne Geheimnisse);
  - ein Go-Paket unter `internal/<modul>`;
  - im Web einen Ordner `web/src/features/modules/<modul>/` mit
    - `Widget.tsx` (Dashboard, ohne Mantine),
    - `admin/` (Detailseite und Kachel-Status, nur im Admin-Chunk).
- **Registrierung:** Zwei getrennte Registries, damit Mantine nie ins Dashboard gelangt:
  - `features/modules/widgets.ts` ordnet Schlüssel → Widget zu; Module mit Instanzen (`calendar:<id>`) stehen dort als Präfix, `resolveWidget()` löst beides auf (exakte Schlüssel über `Object.hasOwn`, Präfixe nur mit nicht leerer ID);
  - `features/admin/modules.ts` ordnet Schlüssel → Admin-Seite, Kachel und Status zu.

  Im Go-Server kennt ein Modulverzeichnis (`internal/modules`) die gültigen Schlüssel. Die Anordnung wird dagegen validiert.
- **Anordnung (`layout` in `settings.json`):** Fünf Regionen des 1440×2560-Dashboards:
  - `topLeft` und `topRight` (die beiden Spalten 45 % | 55 % wie in SmartMirror2);
  - `middle` (volle Breite darunter);
  - `bottomLeft` und `bottomRight` (am unteren Rand verankert).

  Jede Region ist eine geordnete Liste von Modulschlüsseln. Jeder Schlüssel darf höchstens einmal vorkommen. Was in keiner Region steht, ist ausgeschaltet. Die Vorgabe entspricht SmartMirror2:
  - `topLeft`: Uhr, dann die Kalender-Bereiche;
  - `topRight`: Wetter, Kamera;
  - `bottomLeft`: Bring!;
  - `bottomRight`: WLAN.
- **Dashboard:** Es lädt die Anordnung öffentlich (`GET /api/v1/dashboard/layout`) und zeigt je Region die Widgets in Reihenfolge. Ein unbekannter Schlüssel wird übersprungen. Jedes Widget lädt seine Daten selbst und zeigt Fehler an Ort und Stelle.
- **Admin:** Eine Seite „Anordnung“ stellt die Regionen als Spiegel-Skizze dar. Sie fügt Module hinzu und entfernt sie, verschiebt sie zwischen Regionen und ändert die Reihenfolge, per Knöpfen (bedienbar am Handy und per Tastatur; Ziehen kann später dazukommen). Gespeichert wird über die Leiste „Ungespeicherte Änderungen“.
- **Neues Modul anlegen:**
  1. Einstellungsabschnitt mit Validierung;
  2. Endpunkte im Vertrag und `npm run generate`;
  3. Go-Paket und Handler mit Tests;
  4. Schlüssel im Modulverzeichnis;
  5. Web-Ordner mit Widget und Admin-Seite, Einträge in beiden Registries;
  6. Platz in der Vorgabe-Anordnung.

## Go-Regeln (`server/`)

- **Standardbibliothek zuerst.** `net/http` mit `ServeMux`, `log/slog`, `context`, `encoding/json`, `embed`. Eine Abhängigkeit kommt nur dazu, wenn sie echte Arbeit abnimmt (iCal/RRULE, QR, Argon2), und mit fester Version in `go.mod`.
- **Ein Paket pro Fachbereich** unter `internal/`, zum Beispiel `weather` oder `calendar`.
  - Die Fachlogik ist eine reine Funktion ohne I/O, zum Beispiel die Berechnung relativer Zeiten oder die Auswertung des Zeitplans. Sie bekommt die Uhrzeit als Parameter (`now time.Time`) und ruft nie `time.Now()` direkt auf.
  - HTTP-Aufrufe zu Fremddiensten laufen über ein `*http.Client`-Feld mit Timeout, damit Tests einen `httptest.Server` einsetzen können.
  - Handler in `internal/api` sind nur Orchestrierung.
- **Fehler** werden mit `fmt.Errorf("…: %w", err)` eingewickelt. Erwartbare Fachfehler sind eigene Fehlerwerte oder -typen, keine Panics. Für Fremddienste (Wetter, Kalender, Bring!) gilt: Der Ausfall eines Dienstes darf das Dashboard nicht leeren. Der betroffene Bereich trägt dann eine Fehlermeldung, die anderen bleiben sichtbar.
- **Nebenläufigkeit:** Caches und geteilter Zustand sind mit `sync.Mutex` geschützt. Jede Goroutine hat einen Besitzer und endet mit dem `context`. Tests laufen in der CI mit `-race`.
- **Logging** mit `slog`, strukturiert, auf Englisch, ohne Geheimnisse. Die letzten 500 Einträge zeigt das Logbuch der Verwaltung (`internal/logbook`). Was geloggt wird, sieht also jeder Admin im Browser.
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
- **Das Admin-UI ist ein vollständiger Neuentwurf** (Nutzerentscheidung 2026-11-14): Aus dem Admin-Bereich von SmartMirror2 werden weder Aufbau noch Gestaltung übernommen, nur der fachliche Umfang der Einstellungen. Festgelegt im Feinplan von Phase 2 (Nutzerentscheidung 2026-11-14):
  - **Bibliothek:** Mantine (`@mantine/core`, `hooks`, `form`, `notifications`), Symbole aus `@tabler/icons-react`. Bootstrap-Icons bleiben dem Dashboard vorbehalten.
  - **Immer dunkel** (`forceColorScheme="dark"`), Schrift IBM Plex Sans wie am Spiegel, Akzentfarbe Gelb (`primaryColor: 'yellow'`, `autoContrast: true`, angelehnt an den Spiegel-Akzent `#f6b73c`).
  - **Aufbau:** Die Übersicht `/admin` zeigt je Bereich eine Kachel mit ihrem Zustand (z. B. „Drehung 90° · Testbild aus“). Die ganze Kachel ist ein Link auf die Detailseite (`/admin/<bereich>`), und jede Detailseite führt mit „← Übersicht“ zurück. Kacheln gibt es nur für fertige Bereiche, keine Platzhalter.
  - **Speichern:** Auf einer Detailseite erscheint mit der ersten Änderung eine Leiste „Ungespeicherte Änderungen“. Sie nennt, was geändert ist, und bietet „Verwerfen“ und „Speichern“. Verlässt man die Seite mit offenen Änderungen, fragt ein Dialog nach (`useBlocker` des Data-Routers, dazu `beforeunload`). Eine Seite hat keinen ausgegrauten Speichern-Knopf. Ein Fehler beim Speichern lässt die Eingaben stehen und zeigt die Feldfehler des Servers an den Feldern.
  - **Mantine erreicht das Dashboard nie:** Der Admin-Bereich wird per `React.lazy` als eigener Chunk geladen, und nur dieser importiert Mantine-CSS und -JS. Mantines globale Stile (Body, Schrift, Zeilenhöhe) würden sonst das Dashboard verändern. Das Gate prüft das nicht; beim Review sieht man es am Build-Output.
- **Kalender: vollständiger Neuentwurf** (Nutzerentscheidung 2026-11-14 – an SmartMirror2 störten fehlende/falsche Termine, die zwei starren Listen und die umständlichen Regeln). Nicht aus SmartMirror2 übernehmen:
  - **Termine korrekt:** Serientermine (RRULE, EXDATE, RDATE, RECURRENCE-ID), ganztägige und mehrtägige Termine sowie Zeitzonen (TZID, Sommerzeit) werden richtig expandiert und angezeigt.
  - **Kalender und Bereiche:** Beliebig viele Kalender (iCal-Links) mit Name und Symbol. Am Spiegel gibt es frei angelegte Bereiche (z. B. „Familie“, „Arbeit“), jedem werden Kalender zugeordnet, und er hat eigene Grenzen für Anzahl und Vorschau-Tage.
  - **Darstellung:** Eine Liste mit relativen Zeitangaben wie bisher („in 2 Std.“, „Morgen 14:30“, „noch 2 Tage“).
  - **Regeln** sind eine geordnete Liste mit klaren Bedingungen (Kalender, Titel enthält/beginnt mit/ist gleich) und Aktionen (ausblenden, Symbol setzen, Titel umbenennen). Im Admin zeigt eine Vorschau mit echten Terminen, welche Regel greift.
  - **Dienstplan als eigene Schichtanzeige:** Ein Bereichstyp „Schichten“ erkennt Schichten an Kürzeln bzw. Stichworten im Titel (z. B. „F“, „Früh“). Die Zuordnung Kürzel → Name, Zeiten und Symbol stellt man im Admin ein. Die Anzeige sieht aus wie die Terminliste: Symbol und Schichtname links, rechts Tag und Zeit („Heute 06:30–19:00“, „Montag 06–14“), eine laufende Schicht hervorgehoben. Standard sind die nächsten Arbeitstage (Anzahl einstellbar, Vorgabe 5, gesucht im Vorschau-Zeitraum des Bereichs). Eine heutige Schicht verschwindet, sobald sie vorbei ist. Freie Tage („frei“) zeigt der Bereich nur, wenn „Freie Tage anzeigen“ an ist (Nutzerentscheidung 2026-11-14 nach der Abnahme am Pi).
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
  - **Node-Version:** Die CI läuft unter Node 26 (undici 7+), lokal oft Node 22 (undici 6). Unter jsdom sind `File`, `Blob` und `FormData` jsdoms Klassen, `Request`/`fetch` aber Nodes. undici 7 lehnt die Mischung ab, undici 6 duldete sie. Deshalb liest `test/mockApi.ts` Multipart-Bodys mit Nodes `File`/`Blob` (`readFormData`), Upload-Tests bauen `FormData` und `File` aus Node, und ein nicht lesbarer Body erscheint als `bodyError` mit Meldung statt als stiller Timeout. Bei Tests rund um Dateien, Streams oder `Request` lokal zusätzlich unter einer Node-Version mit undici 7 laufen lassen, wenn eine vorhanden ist.
- **Zeitzonen:** Tests, die von der Zeitzone abhängen, setzen sie ausdrücklich (`time.LoadLocation("Europe/Berlin")` bzw. `TZ` in der Vitest-Konfiguration) und decken die Sommerzeitumstellung ab, wo sie eine Rolle spielt.

## Verifikation

`npm run verify` (= `node tools/verify.mjs`) ist das vollständige Gate, lokal und in der CI. Es läuft unter Windows, Linux und macOS.

- **Stränge:** Zwei Stränge laufen parallel und schreiben je ein eigenes Log. Am Ende gibt das Skript beide Logs aus und eine Tabelle mit Dauer und Ergebnis je Schritt.
  - **Backend** (`go-mod-tidy`, `go-generate-drift`, `gofmt`, `go-vet`, `go-test`): `go mod tidy -diff`, Drift des erzeugten Go-Codes, `gofmt -l`, `go vet`, `go test` (in der CI mit `-race`).
  - **Frontend** (`api-types-drift`, `typecheck`, `lint`, `vitest`, `build`): Drift der API-Typen, `tsc -b`, ESLint, Vitest, Vite-Build.
- **Fehler:** Ein fehlgeschlagener Schritt beendet nur seinen Strang, dessen restliche Schritte stehen auf `not-run`. Das Skript endet mit einem Exit-Code ungleich 0, sobald ein Ergebnis weder `ok` noch `skipped` ist.
- **Schalter:**
  - `--sequential` lässt alles nacheinander laufen (zum Eingrenzen);
  - `--only <schritt>[,<schritt>]` lässt nur einzelne Schritte laufen (ein unbekannter Name endet mit Exit 2, bevor etwas läuft);
  - `--skip-build` lässt den Frontend-Build weg;
  - `--race` startet `go test` mit dem Race-Detector (braucht cgo; die CI setzt den Schalter).
- **Das Gate testet sich selbst:** Sein erster Schritt `selftest` (`node --test 'tools/*.test.mjs'`) prüft die Logik, nach der das Skript über Bestanden oder Fehlgeschlagen entscheidet. Schlägt er fehl, starten die Stränge nicht. Wer diese Logik ändert, ändert den Test mit. Die Umgebungsmarke `VERIFY_SELFTEST_RUNNING` verhindert, dass der Selbsttest das Gate erneut startet. (`node --test tools/` ohne Muster behandelt Node 22 als Modulpfad – deshalb das Muster.)
- **Drift-Prüfung:** Sie erzeugt die Dateien in ein temporäres Verzeichnis und vergleicht sie mit dem Arbeitsbaum. Sie ändert nichts. Behoben wird Drift mit `npm run generate`. Achtung: oapi-codegen v2.8.2 ignoriert `-o`, sobald die Config ein `output:` hat. Das Gate erzeugt deshalb über eine temporäre Kopie der Config.
- **Der Gate-Build schreibt nie nach `server/internal/web/dist/`**, sondern in ein temporäres Verzeichnis: Der Backend-Strang kompiliert das eingebettete Verzeichnis gleichzeitig, und ein geleertes Verzeichnis ließe `go vet`/`go test` scheitern. Das Vite-Plugin `keep-dist-gitignore` schreibt die `.gitignore` nur zurück, wenn der Build in genau dieses Verzeichnis geht.

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
