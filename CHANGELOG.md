# Changelog

Alle nennenswerten Änderungen an diesem Projekt werden in dieser Datei dokumentiert.

Das Format basiert auf [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) und
[Conventional Commits](https://www.conventionalcommits.org/de/v1.0.0/). Diese Datei wird
automatisch mit [git-cliff](https://git-cliff.org) aus den Commit-Messages generiert –
**nicht von Hand bearbeiten**.

## [0.7.2] - 2026-11-14

### Changed

- Rtspx:// für UniFi Protect in README und Projektregeln

### Fixed

- **camera**: UniFi-Protect-Streams über rtspx:// abspielen

## [0.7.1] - 2026-11-14

### Fixed

- **kiosk**: Kein Mauszeiger und keine Übersetzungsleiste am Spiegel
- **dashboard**: Spiegel übernimmt Änderungen ohne Neustart

## [0.7.0] - 2026-11-14

### Added

- **admin**: Logbuch und Update-Knopf

### Changed

- Logbuch und Update-Knopf in README, INSTALL und Projektregeln

## [0.6.0] - 2026-11-14

### Added

- **display**: Nachtruhe schaltet den Monitor nach Zeitplan aus und an
- **camera**: Kamera-Modul mit go2rtc im internen Netz

### Changed

- **web**: Upload-Tests nutzen Nodes FormData und File
- **dashboard**: Nachtruhe rendert nur die schwarze Fläche
- Kamera und Nachtruhe in README und Projektregeln
- **web**: MockApi liest Multipart auch unter undici 7
- Node-Versionen und jsdom-Globals in den Testregeln

### Fixed

- **camera**: Kein Warn-Log ohne eingerichtete Kamera
- **admin**: Neutraler Hinweis bei gespeicherten Geheimnissen

## [0.5.0] - 2026-11-14

### Added

- **calendar**: Kalender-Einstellungen, iCal-Abruf, Vorschau und Symbol-Bibliothek
- **deploy**: Host-Dienst für Updates per Knopf
- **bring**: Bring!-Einkaufsliste als Modul
- **calendar**: Kalender- und Schicht-Widgets am Spiegel
- **calendar**: Kalender-Verwaltung mit Regeln, Vorschau und Symbol-Bibliothek

### Changed

- README und Regeln auf den Stand nach Phase 5 bringen

### Fixed

- **test**: WLAN-Test hing von der Map-Reihenfolge ab

## [0.4.0] - 2026-11-14

### Added

- **weather**: Wetter- und WLAN-Modul im Server
- **layout**: Modulverzeichnis und freie Anordnung im Server
- **dashboard**: Regionen und Module Uhr, Wetter und WLAN im Dashboard
- **calendar**: Kalender-Kern mit korrekter Expansion, Regeln und Schichten
- **admin**: Admin-Seiten Wetter, WLAN-QR-Code und Anordnung

### Changed

- Ausnahme für den WLAN-QR-Code und Bedienung der Anordnung präzisieren
- README auf den Stand nach Phase 4 bringen

## [0.3.0] - 2026-11-14

### Added

- **kiosk**: Kiosk-Container mit Cage, Chromium und Agent kioskd
- **kiosk**: Dashboard dreht sich bei Bedarf selbst, Kiosk-Image auf arm64-Runner

### Changed

- README für Verwaltung und Kiosk nachziehen, Changelog bis v0.2.0
- Interne Module und freie Anordnung festlegen
- README auf den Stand nach Phase 3 bringen

## [0.2.0] - 2026-11-14

### Added

- **auth**: Admin-Zugang mit Ersteinrichtung, Login und Sitzungen
- **settings**: Einstellungsspeicher mit Anzeige-Abschnitt und Kiosk-Status
- **admin**: Admin-Oberfläche mit Anmeldung, Übersicht und Anzeige-Seite

### Changed

- Umgang mit verweigertem Tag-Push festhalten
- Gestaltung des Admin-Bereichs festlegen
- MIT-Lizenz und Commit-Identität festlegen
- Kalender als Neuentwurf festlegen

## [0.1.0] - 2026-11-14

### Added

- **server**: Go-Server mit OpenAPI-Vertrag und eingebetteter Oberfläche
- **web**: React-Oberfläche mit Uhr und Diagnose-Testbild

### Changed

- Entwicklungsprozess und Projektregeln anlegen
- Admin-Bereich als vollständigen Neuentwurf festlegen
- Prüfskript, Container-Image und CI
- README und Gate-Regeln für Phase 1 nachziehen


