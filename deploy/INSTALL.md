# MirrorNextGen auf dem Raspberry Pi 5 installieren

> **Hinweis:** Diese Anleitung wurde ohne Zugriff auf die Hardware geschrieben.
> Jeder Schritt, vor allem die Anzeige (Abschnitte 2, 7 und 8), muss am Gerät
> bestätigt werden. Wenn etwas abweicht, gilt, was du am Gerät siehst.

Der Pi lädt nur fertige Images und baut nichts selbst. Auf dem Host ist nur Docker installiert, alles andere läuft in den Containern `mirror` (Server und Oberfläche) und `kiosk` (Bildschirmanzeige).

## 1. Voraussetzungen

- Raspberry Pi 5 mit Netzteil und Speicherkarte (oder SSD)
- WQHD-Monitor (2560 × 1440), per HDMI am Pi, im Hochformat aufgestellt
- Netzwerk (LAN oder WLAN) mit Internetzugang
- Ein zweiter Rechner oder das Handy für die Verwaltung

Schritte:

1. Schreibe **Ubuntu Server 26.04 LTS (64 Bit, arm64)** mit dem Raspberry Pi Imager auf die Karte. Lege im Imager Hostname, Benutzer und SSH-Zugang gleich fest.
2. Starte den Pi und melde dich an (direkt oder per SSH).
3. Bring das System auf den neuesten Stand:

```bash
sudo apt update && sudo apt full-upgrade -y
sudo reboot
```

## 2. Anzeige vorbereiten

Der Kiosk-Container nutzt den Grafiktreiber des Hosts (KMS). Prüfe, dass er aktiv ist:

```bash
ls /dev/dri
```

Du solltest `card0` (oder `card1`) und `renderD128` sehen. Fehlt `/dev/dri`, geht die Anzeige nicht. Prüfe dann, ob in `/boot/firmware/config.txt` die Zeile `dtoverlay=vc4-kms-v3d` steht (Standard bei Ubuntu für den Pi).

Damit die Textkonsole nicht über dem Dashboard erscheint:

```bash
# Login-Prompt auf tty1 abschalten
sudo systemctl disable --now getty@tty1
# Konsolen-Cursor ausblenden und Bildschirmschoner der Konsole abschalten
sudo cp /boot/firmware/cmdline.txt /boot/firmware/cmdline.txt.bak
sudo sed -i '1 s/$/ consoleblank=0 vt.global_cursor_default=0/' /boot/firmware/cmdline.txt
cat /boot/firmware/cmdline.txt
sudo reboot
```

`cmdline.txt` muss **eine einzige Zeile** bleiben. Prüfe das mit dem `cat`.

## 3. Docker installieren

```bash
sudo apt install -y docker.io docker-compose-v2
sudo usermod -aG docker $USER
```

Melde dich ab und wieder an (oder starte neu), damit die Gruppe gilt. Dann:

```bash
docker version
docker compose version
```

## 4. Zugriff auf die Images (GHCR)

Die Images liegen öffentlich in der GitHub Container Registry, `docker compose pull` braucht keine Anmeldung.

Nur wenn die Pakete privat sind (z. B. in einem Fork), melde dich vorher an:

1. Erzeuge auf GitHub ein **Personal Access Token (classic)** mit der Berechtigung `read:packages` (Einstellungen → Developer settings → Personal access tokens).
2. Melde dich am Pi an der Registry an. Das Token gibst du erst auf Nachfrage ein, nicht in den Befehl:

```bash
docker login ghcr.io -u <github-benutzername>
```

## 5. Deploy-Dateien holen

Du brauchst nur `compose.yaml` und `.env.example` aus dem Ordner `deploy/` des Repositorys:

```bash
mkdir -p ~/mirror && cd ~/mirror
# Beide Dateien aus deploy/ des Repositorys hierher kopieren, z. B. per scp vom Rechner:
#   scp deploy/compose.yaml deploy/.env.example <benutzer>@<pi>:~/mirror/
cp .env.example .env
nano .env
```

In `.env` kannst du Zeitzone (`TZ`), Port (`MIRROR_PORT`) und die Kiosk-Optionen anpassen. Die Standardwerte genügen für den ersten Start.

## 6. Starten

```bash
cd ~/mirror
docker compose pull
docker compose up -d
docker compose logs -f kiosk
```

Der Kiosk wartet, bis der Server bereit ist, und startet dann die Anzeige. Beenden der Log-Ansicht: `Strg+C` (die Container laufen weiter).

Öffne am Handy `http://<pi>:8080/admin` und lege das Passwort für die Verwaltung fest.

## Kamera (optional)

Der Dienst `camera` (go2rtc) startet mit `docker compose up -d` von selbst und braucht keine Einrichtung. Er hat keinen veröffentlichten Port: Nur `mirror` im internen Docker-Netz erreicht ihn, das Video geht über `http://<pi>:8080` an den Kiosk. Den eigenen RTSP-Server von go2rtc schaltet `compose.yaml` ab, damit die Kamera im Docker-Netz nicht ohne Anmeldung weiterverteilt wird.

1. Admin → **Kamera**: Die Stream-Adresse deiner Kamera eintragen (nur `rtsp://` oder `rtsps://`, z. B. `rtsp://benutzer:passwort@192.168.1.20:554/stream1`), **Kamera anzeigen** einschalten, speichern.
2. Der Spiegel meldet den Stream selbst bei go2rtc an. Die Adresse (mit Zugangsdaten) bleibt auf dem Pi: Sie steht in `settings.json` im Volume `mirror-data` und in der Konfiguration von go2rtc im Volume `camera-config`, aber nie im Browser oder im Log.
3. Das Modul **Kamera** steht in der Standardanordnung oben rechts unter dem Wetter. Auf einer bestehenden Installation fügst du es unter Admin → Anordnung hinzu.

Läuft die Kamera nicht: `docker compose logs camera mirror`. Der Spiegel versucht die Anmeldung bei go2rtc mit wachsenden Abständen erneut, bis der Dienst bereit ist. Veröffentliche Port 1984 nie ins Netz: Die API von go2rtc hat keine Anmeldung und kann Befehle ausführen.

## 7. Abnahme-Checkliste (am Gerät prüfen)

- [ ] Das Dashboard steht im Vollbild auf dem Monitor, ohne Cursor und ohne Rahmen.
- [ ] Admin → Anzeige → **Testbild an**: Es zeigt 1440 × 2560 im Hochformat, der Pfeil „oben“ liegt am **physischen** oberen Rand.
- [ ] Jede Drehung (0°, 90°, 180°, 270°) wird innerhalb von etwa 5 Sekunden übernommen, ohne dass etwas neu startet.
- [ ] Nach `sudo reboot` kommt alles von selbst zurück (Dashboard sichtbar, ohne dass du etwas tippst).
- [ ] `docker compose logs kiosk` zeigt die angewendeten Drehungen (Einträge „applied screen state“ mit `rotation` und `output`).
- [ ] Admin → Anzeige → **Monitor 10 Sekunden ausschalten**: Der Monitor geht innerhalb von etwa 5 Sekunden aus und nach etwa 10 Sekunden wieder an, mit derselben Drehung wie vorher.
- [ ] Nachtruhe mit einer Ausschaltzeit kurz nach jetzt speichern: Der Monitor geht zur eingestellten Minute aus und zur Einschaltzeit wieder an.

Notiere Abweichungen, besonders den Namen des Ausgangs (z. B. `HDMI-A-1`) und welche Drehung zur Aufstellung deines Monitors passt.

## 8. Fehlersuche

**Schwarzer Bildschirm**
- `ls /dev/dri` prüfen (siehe Abschnitt 2).
- Das Grafikgerät festlegen: in `.env` `WLR_DRM_DEVICES=/dev/dri/card1` eintragen (oder `card0`), dann `docker compose up -d`.
- Log ansehen: `docker compose logs kiosk`.

**Grafikprobleme (Flackern, Absturz, schwarzes Fenster)**
- In `.env` `KIOSK_DISABLE_GPU=true` setzen.
- Zusätzlich oder stattdessen `WLR_RENDERER=pixman` (reine Software-Darstellung, langsamer).

**Drehung wird nicht übernommen**
- Im Log nach Fehlern von `wlr-randr` suchen.
- Ausweg: `KIOSK_ROTATION=browser` in `.env`. Dann dreht sich das Dashboard selbst (keine Drehung am Compositor). Der Kiosk-Agent schaltet mit `wlr-randr` nur noch den Monitor für die Nachtruhe aus und an. Das Testbild zeigt dann „Drehung: Browser …“ und die Inhaltsgröße 1440 × 2560.

**Monitor geht zur Nachtruhe nicht aus**
- Admin → Anzeige → **Monitor 10 Sekunden ausschalten**, dann `docker compose logs kiosk` ansehen: Erwartet sind Einträge „applied screen state“ mit `powerOn` false und true.
- Ignoriert der Monitor das Abschalten, bleibt das Bild trotzdem schwarz: Das Dashboard legt dann eine schwarze Fläche über alles. Die Hintergrundbeleuchtung läuft in diesem Fall weiter.
- Die Zeiten gelten in der Zeitzone aus `TZ` in `.env`.

**Meldungen des Servers ansehen**
- Admin → **Logbuch** zeigt die letzten 500 Meldungen des Servers seit dem letzten Start (Kalender-, Wetter- und Bring!-Abrufe, Kamera, Updates). Die Meldungen des Kiosks stehen in `docker compose logs kiosk`.

**Login-Prompt oder Konsolentext sichtbar, Anzeige hängt am Umschalten der Konsole**
- Prüfe, dass `getty@tty1` abgeschaltet ist: `systemctl status getty@tty1`.
- Prüfe die Zeile in `/boot/firmware/cmdline.txt` (siehe Abschnitt 2).
- Starte den Kiosk neu: `docker compose restart kiosk`.

**Aktualisieren**

```bash
cd ~/mirror
docker compose pull
docker compose up -d
```

## 9. Updates per Knopf

> **Hinweis:** Auch dieser Abschnitt wurde ohne Zugriff auf die Hardware geschrieben. Prüfe jeden Schritt am Gerät.

Der Container `mirror` kann sich nicht selbst aktualisieren. Darum gibt es einen kleinen Dienst auf dem Host (systemd). Der Server legt die Datei `control/update-request` ab, der Host-Dienst bemerkt sie und führt `docker compose pull` und `docker compose up -d --remove-orphans` aus. Das Ergebnis steht danach in `control/update-status.json`, die Ausgabe in `control/update.log`.

Einrichten (einmalig, im Ordner mit `compose.yaml`). Kopiere dafür den Ordner `deploy/host/` aus dem Repository neben die `compose.yaml`, z. B. per `scp -r deploy/host <benutzer>@<pi>:~/mirror/`:

```bash
cd ~/mirror
sudo ./host/install.sh
```

Das Skript legt `control/` an (Besitzer UID 65532, der Nutzer im Container), installiert die Units `mirror-update.path` und `mirror-update.service` und aktiviert sie. Du kannst es gefahrlos wiederholen.

Optional zusätzlich ein nächtliches Update (täglich gegen 04:17 Uhr, bis zu 15 Minuten zufällig verschoben):

```bash
sudo ./host/install.sh --with-timer
```

Beobachten:

```bash
systemctl status mirror-update
journalctl -u mirror-update -e
cat control/update-status.json
cat control/update.log
```

Auslösen: In der Verwaltung unter **System → Jetzt aktualisieren**. Die Seite zeigt den Fortschritt, während des Neustarts „Der Spiegel startet neu …“ und danach die neue Version. Von Hand, ohne den Knopf: `touch control/update-request`.

Meldungen auf der Seite „System“:
- **„Updates per Knopf sind auf diesem Spiegel nicht eingerichtet.“** – `MIRROR_CONTROL_DIR` fehlt am Dienst `mirror` (in `compose.yaml` ist es gesetzt).
- **„Der Spiegel kann nicht in das Steuerverzeichnis schreiben.“** – `control/` fehlt oder gehört nicht UID 65532. `sudo ./host/install.sh` ausführen, danach `docker compose up -d`.
- **„Der Update-Dienst hat die Anfrage nicht übernommen.“** – Die Anfrage liegt seit über 2 Minuten unbearbeitet in `control/`. Prüfe `systemctl status mirror-update.path`; ist die Unit nicht aktiv, `sudo ./host/install.sh` ausführen. Ein neuer Klick auf den Knopf ersetzt die alte Anfrage.
- **„Update fehlgeschlagen …“** – darunter stehen die letzten Zeilen von `control/update.log`.

Fallback ohne den Dienst:

```bash
cd ~/mirror
docker compose pull && docker compose up -d
```
