# Zebra Config

Zebra Config is a small setup and troubleshooting tool for Zebra ZD410 and ZD411 network label printers at 203 dpi. It is for an IT support manager and the support techs who work with them. It is meant to be faster and simpler than Zebra Setup Utilities or ZebraDesigner for the everyday jobs: see the current settings, change a few, calibrate, or print a configuration label.

You double-click one program. It opens a page in the browser on this computer only. You type the printer's IP address. The program talks to that printer on port 9100 and to nothing else. There is no installer, no admin password, no account, and no internet service.

It does not scan the network. It does not read label designs, stored graphics, the printer's file list, or the contents of a print job.

## Download and run on Windows

1. On the [releases page](https://github.com/manovi-code/zebra-config/releases), download `zebra-config-windows-amd64.exe`.
2. If you want to check the file, compare its SHA-256 with `SHA256SUMS` from the same release.
3. Double-click the program. A window stays open while it runs. Closing that window quits Zebra Config.
4. Your browser should open a page that starts with `http://127.0.0.1:`. If it does not, copy the address from the window into the browser.
5. Windows SmartScreen often blocks a new, unsigned program. Choose **More info**, then **Run anyway**.

The program does not need administrator rights.

## Download and run on macOS

1. On the [releases page](https://github.com/manovi-code/zebra-config/releases), download `Zebra-Config-macOS.zip`. This is the file to double-click. It works on both Intel and Apple silicon Macs.
2. Double-click the zip. macOS opens it into an app named **Zebra Config**.
3. The first time, Gatekeeper blocks an app that was downloaded. Right-click **Zebra Config**, choose **Open**, then **Open** again. On newer macOS, if that choice is missing: **System Settings > Privacy & Security**, find the blocked app, and choose **Open Anyway**.
4. A browser page opens at `http://127.0.0.1:`. No installer and no administrator password.
5. When you are finished, click **Quit** on that page. That stops the program. Closing the browser tab does not stop it. The app does not stay in the Dock. If the page is already closed, open Activity Monitor, select **Zebra Config**, and quit it.

If double-click still does nothing, open Terminal, `cd` to the folder that contains the app, and run:

```
xattr -d com.apple.quarantine "Zebra Config.app"
chmod +x "Zebra Config.app/Contents/MacOS/zebra-config"
open "Zebra Config.app"
```

The same release also has raw programs for people who want a terminal: `zebra-config-macos-apple-silicon`, `zebra-config-macos-intel`, and `zebra-config-macos-universal`. Those are not the double-click download. For one of them, run `chmod +x` and `xattr -d com.apple.quarantine` on the file name, then start it with `./`. A terminal window stays open. Closing that window quits the tool.

## Use the page

Type the printer IP and choose **Connect**. Port 9100 is correct unless someone changed the printer's raw port.

The page shows Printer, Media, Print, Sensor, and Network. Change the boxes you need, then **Review changes**. Nothing is sent until you confirm the before/after list. If a setting did not stick, the page says so after it re-reads the printer.

IP address, subnet mask, gateway, port, and addressing method need a second confirmation, because the printer can disappear from the network. Factory reset and restart use that same extra confirmation. Import leaves those network settings out unless you check the box.

**Export** downloads a JSON file of the settings plus the model, firmware, and resolution. It does not include label designs.

Recent printers are remembered on this computer as an IP, a name, and a model. That list is the only thing the tool stores.

**Quit** stops Zebra Config. On Windows, closing the program window also stops it. Closing the browser tab does not.

**Power** has one switch, **Keep printer always on**. Checked means Energy Star sleep is off. The line under it says "Always on" or "Sleeps after N min".

**Network** includes the DHCP client ID (prefix, suffix, type, and on/off) for the wired port and for Wi-Fi, plus the DHCP host name (option 12). Those are network changes: the printer may get a different address. Confirm the extra warning before they are sent.

**Replace a failed printer** saves a replacement file from the old printer, if it still responds. The new printer's name starts as that exact name, and you can edit it in the review. The tool then asks, in its own confirm screen, whether to rename the old printer to that name plus `_`. It does not rename anything without that confirm. If the file is an older saved export and the old printer is not connected, the rename is skipped and the page says so. The old IP address is not copied unless you check that box, because two printers cannot share an address.

When the old printer's DHCP client ID is a text suffix and that ID is turned on, the same confirm offers to add `_` to the suffix, and that box starts checked. The guide uses the suffix only for text (`cid_type` 0) and hex (`cid_type` 2). `_` is not a hex character, and a MAC client ID (`cid_type` 1) does not send the suffix, so those two cases are explained and left unchanged.

## Run the simulator

The simulator is a pretend ZD411 for practice and for tests. It is not a real printer. From a checkout of this repository:

```
go run ./cmd/zdsim -port 9101
```

Leave that running. Start the tool with `go run ./cmd/zebra-config` and connect to `127.0.0.1`, port `9101`.

Other examples:

```
go run ./cmd/zdsim -port 9101 -model ZD621 -dpi 300
go run ./cmd/zdsim -port 9101 -hang
go run ./cmd/zdsim -port 9101 -delay 5s
go run ./cmd/zdsim -port 9101 -refuse
```

`-model ZD621 -dpi 300` is an unsupported printer: the page stays read-only. `-hang` accepts the connection and never answers, which shows the timeout message. `-refuse` does not listen, which shows "connection refused".

## Safety rules

- Only a ZD410 or ZD411 at 203 dpi can be changed. Anything else is read-only. The resolution value `200` is accepted as well, because the programming guide lists 200 dpi for the 8 dots/mm head that the ZD411 user guide calls 203 dpi.
- Edits are not sent as you type. You review a before/after list, then confirm.
- Network changes, factory reset, network reset, and restart ask a second time, with a warning that the printer may drop off the network.
- The console sends one command at a time. A command that changes the printer asks first. A command that reads stored labels, graphics, or the file list is refused and is not sent.
- The tool binds its own page to `127.0.0.1` only.

The command names, allowed values, and guide links are in [docs/sgd-variables.md](docs/sgd-variables.md).

## Privacy

The employer that asked for this tool is a HIPAA covered entity. Zebra Config is built so it cannot pull protected health information off a printer.

- It reads and writes configuration settings only. The main screen uses an allowlist of SGD names from Zebra's programming guide.
- It does not read, store, log, or export label formats, stored graphics, the file directory, or print-job contents. The console denies those commands (`file.dir`, `^HF`, `^HG`, `~DY`, `^WD`, `^FD`, and the others listed in the command table) before anything is written to the printer.
- Console text and printer replies are shown on the page and then forgotten. They are not written to a file.
- The recent-printers file stores an IP address, a friendly name, and a model. Nothing else.
- There is no cloud relay, telemetry, account, or update check. The only outbound connection is the printer IP you type. Host names are rejected so the tool does not do a DNS lookup.

This is a support tool for printer settings. It is not a system for storing or viewing patient information.

## Not yet verified on real hardware

This version was checked against the simulator and against Zebra's published guides. It has not been run on a real ZD410 or ZD411. These are the spots most likely to differ by firmware:

- `head.resolution.in_dpi` may come back as `200`, `203`, or something else on a 203 dpi printer. `~HI` dots-per-mm `8` is the published 203 dpi value.
- `print.tone` default is `4.0` in the programming guide and `10.0` on the ZD411 configuration report.
- `media.speed` default is `2` in the programming guide. The ZD411 report lists 6 ips (and one ZD411 introduction says 8 ips) as the 203 dpi maximum.
- `ezpl.power_up_action` and `ezpl.head_close_action` default to `calibrate` in the programming guide. The ZD411 report says power-up is NO MOTION and head-close is FEED.
- `ezpl.tear_off` default is not a single number in the programming guide (the page repeats the range). The ZD411 report says `+000`. getvar may return `0`, `+0`, or `+000`.
- `media.printmode` is set with a letter (`T`) and may be read back as `tear off`, `TEAR OFF`, or the letter.
- `device.languages` set to `zpl` is documented to read back as `hybrid_xml_zpl`.
- Wired address settings are sent as `internal_wired.ip.*`, which is the documented wired print-server branch. Some firmware also mirrors `ip.addr` / `ip.dhcp.enable`. This tool does not write that other branch.
- A new permanent IP may not apply until `device.reset`. The guide says so. Timing on a ZD410/ZD411 was not checked.
- SGD `setvar` on Link-OS printers often sticks across a restart. The `^JU` page also says unsaved changes are lost at power-off. The **Save on the printer** box sends `^JUS`. Which behavior a given firmware uses was not checked.
- `~JC` and `~WC` may take longer than this tool waits, and they may or may not return bytes. The buttons treat silence as normal.
- `~HS` is documented to stay silent when the printer is out of paper, out of ribbon, or the head is open. A timeout there is shown as "no status reply", not as a failed connection.
- `device.sensor_select` may report `auto` even though the SGD page only lists `reflective` and `transmissive`.
- Quote framing: replies are parsed as `"value"` with no terminator. A value that itself contains a quote is not handled, and the tool refuses to send one.
- Whether a DHCP client ID change (`internal_wired.ip.dhcp.cid_*` or `wlan.ip.dhcp.cid_*`) takes effect before `device.reset`. The guide does not say a reset is required for the client ID. A new permanent IP address does require one.
- `power.sleep.enable` and `power.sleep.timeout` are documented for mobile printers. They are not in the desktop SGD command index, so this tool does not send them. ZD410/ZD411 sleep is sent as `power.energy_star.enable` and `power.energy_star.timeout`. Whether a particular ZD410 firmware also honors `power.sleep.*` was not checked.
- `ip.dhcp.cid_*` exists as well as `wlan.ip.dhcp.cid_*`. This tool writes the `wlan.ip.dhcp.cid_*` names from the wireless pages, not both.
- DHCP option 12 on Wi-Fi is `ip.dhcp.option12`. The wired names are `internal_wired.ip.dhcp.option12`, `option12_format`, and read-only `option12_value`. A ZD410 without Wi-Fi may answer `?` for the Wi-Fi names.

## Build from source

Go 1.22 or newer. No cgo.

```
go test ./...
go build -o zebra-config ./cmd/zebra-config
go build -o zdsim ./cmd/zdsim
```

A tag named `v*` builds the Windows binary, the raw Mac binaries, `Zebra-Config-macOS.zip` (a double-click app), and a Linux binary, then publishes them as a GitHub release. The Mac app is packed on a Mac so the program inside stays executable. See `.github/workflows/ci.yml`.
