# Changelog

Alle nennenswerten Änderungen an diesem Projekt werden in dieser Datei dokumentiert.

Das Format basiert auf [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) und
[Conventional Commits](https://www.conventionalcommits.org/de/v1.0.0/). Diese Datei wird
automatisch mit [git-cliff](https://git-cliff.org) aus den Commit-Messages generiert –
**nicht von Hand bearbeiten**.

## [Unreleased]

### Added

- **kiosk**: Kiosk-Container mit Cage, Chromium und Agent kioskd

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


