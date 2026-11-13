# SGD and ZPL commands used by Zebra Config

Sources, checked November 2026:

- [ZPL Programming Guide](https://docs.zebra.com/us/en/printers/software/zpl-pg.html), SGD command pages and the host-status commands
- [ZD411D configuration cross-reference](https://docs.zebra.com/us/en/printers/desktop/zd411d-direct-thermal-link-os-printer/zpl-configuration/configuration-setting-to-command-cross-reference.html)

The main screen only `getvar`s and `setvar`s the names in this table. The console is a separate denylist (see the README). A name from an old note is listed in the last column when the guide uses a different one.

SGD framing, from the guide's getvar and setvar pages:

```
! U1 getvar "name"\r\n
! U1 setvar "name" "value"\r\n
! U1 do "name" "value"\r\n
```

getvar replies are quoted and are not terminated. This program reads until the closing quote or a deadline.

| Screen label | Command | Allowed values in the guide | Notes |
| --- | --- | --- | --- |
| Model (read-only) | `device.product_name` | Printer-defined | Used with `~HI` to tell a ZD410/ZD411 from anything else. |
| Firmware (read-only) | `appl.name` | Printer-defined | Guide: "the printer's firmware version." |
| Resolution (read-only) | `head.resolution.in_dpi` | `"200"`, `"300"`, `"600"` | The guide does not list `203`. The ZD411 user guide calls the same head 203 dpi (8 dots/mm). This tool treats 200 and 203, and `~HI` dots/mm `8`, as the supported head. |
| Friendly name | `device.friendly_name` | A short name. Guide default is the logic-board serial. | |
| Command language | `device.languages` | `epl`, `epl_zpl`, `zpl`, `hybrid_xml_zpl`, plus virtual-device values this tool does not offer | Guide: setting `zpl` reads back as `hybrid_xml_zpl`. Default in the guide is `epl_zpl`. |
| Media type | `ezpl.media_type` | `continuous`, `gap/notch`, `mark`. Default `gap/notch`. | Same idea as `^MN`. The name `media.type` is not the documented command, so it is not sent. |
| Print mode | `media.printmode` | setvar codes `T` `P` `R` `A` `C` `D` `F` `L` `U` `K` `V` `S` | Same idea as `^MM`. The guide's example sets `"T"` and the getvar result is `"tear off"`. The screen shows the words and sends the code. |
| Print method | `ezpl.print_method` | `thermal trans`, `direct thermal`. Guide default `thermal trans`. | Same idea as `^MT`. |
| Media sensor | `device.sensor_select` | `reflective`, `transmissive` | Same idea as `^JS`. The ZD411 report's default line says AUTO (`^MNA`). AUTO is not in the SGD value list, so it is shown if the printer reports it and is not offered as a new choice. The name `sensor.select` is not the documented command. |
| When the printer turns on | `ezpl.power_up_action` | `calibrate`, `feed`, `length`, `no motion`, `short cal`. Guide default `calibrate`. | Same idea as `^MF`. The ZD411 report says MEDIA POWER UP default is NO MOTION. |
| When the printhead is closed | `ezpl.head_close_action` | Same list. Guide default `calibrate`. | The ZD411 report says HEAD CLOSE default is FEED. |
| Darkness | `print.tone` | `0.0` to `30.0`. Guide default `4.0`. | Same idea as `~SD` / `^MD`. The ZD411 report says DARKNESS default `10.0`. |
| Print speed | `media.speed` | `2` to `12` inches per second, or `up` / `down`. Guide default `2`. | Same idea as `^PR`. The ZD411 report lists 6 ips as the 203 dpi maximum (one ZD411 introduction also says 8 ips). |
| Print width | `ezpl.print_width` | Dots, up to the printhead width. | Same idea as `^PW`. ZD411 report: 448 dots at 203 dpi, 640 at 300 dpi. This tool rejects a write above 448 because it only writes to 203 dpi ZD410/ZD411 printers. |
| Label length | `zpl.label_length` | `1` to `32000` dots. | Same idea as `^LL`. ZD411 report default `1225`, and it says the value is updated while printing. |
| Tear-off position | `ezpl.tear_off` | `-120` to `120` | Same idea as `~TA`. The guide's default line repeats the range, which is not a single default. The ZD411 report says TEAR OFF `+000`. The name `media.tear_off` is not the documented command. |
| Wired addressing | `internal_wired.ip.protocol` | `bootp`, `dhcp`, `rarp`, `glean`, `permanent`, `all`. Guide default `all`. | This is the wired print-server command. `ip.dhcp.enable` (`on`/`off`) and a generic `ip.protocol` exist on other branches. They are not sent, so a Wi-Fi setting is not changed by mistake. The ZT411 user guide names the wired command `internal_wired.ip.protocol`. |
| Wired IP address | `internal_wired.ip.addr` | Any IPv4 address. Guide default `0.0.0.0`. | The guide says a set address applies when the protocol is `permanent` and the print server is reset. |
| Wired subnet mask | `internal_wired.ip.netmask` | A subnet mask. Guide default `255.255.255.0`. | Ignored unless the protocol is `permanent`. |
| Wired gateway | `internal_wired.ip.gateway` | Any IPv4 address. | Same permanent-address rule as the IP address. |
| Raw printing port | `internal_wired.ip.port` | `1`–`65535`, excluding ports used by other services. The page names `21`, `23`, `80`, and `515`. Default `9100`. | |
| Keep printer always on | `power.energy_star.enable` | `on`, `off` | [power.energy_star.enable](https://docs.zebra.com/us/en/printers/software/zpl-pg/c-sgd-commands-from-a-to-d/r-sgd-power-energy-star-enable.html). Checked on the screen sends `off`. The desktop command index does not list `power.sleep.enable`; that name is used on mobile printers and is not sent. |
| Sleep after (seconds) | `power.energy_star.timeout` | `180` to `65535` seconds. Default `180`. | [power.energy_star.timeout](https://docs.zebra.com/us/en/printers/software/zpl-pg/c-sgd-printer-commands/r-sgd-power-energy-star-timeout.html). |
| Wired DHCP client ID | `internal_wired.ip.dhcp.cid_enable` | `on`, `off`. Default `off`. | [cid_enable](https://docs.zebra.com/us/en/printers/software/zpl-pg/r-sgd-wireless-sgd-wireless-commands/r-sgd-wired-internal-wired-ip-dhcp-cid-enable.html). Option 61. |
| Wired client ID type | `internal_wired.ip.dhcp.cid_type` | `0` text, `1` MAC address, `2` hex | [cid_type](https://docs.zebra.com/us/en/printers/software/zpl-pg/r-sgd-wireless-sgd-wireless-commands/r-sgd-wired-internal-wired-ip-dhcp-cid-type.html). Prefix and suffix are used for `0` and `2`. |
| Wired client ID prefix | `internal_wired.ip.dhcp.cid_prefix` | Up to 10 characters for text, or 20 for hex | [cid_prefix](https://docs.zebra.com/us/en/printers/software/zpl-pg/r-sgd-wireless-sgd-wireless-commands/r-sgd-wired-internal-wired-ip-dhcp-cid-prefix.html). |
| Wired client ID suffix | `internal_wired.ip.dhcp.cid_suffix` | Up to 60 characters for text, or 120 for hex | [cid_suffix](https://docs.zebra.com/us/en/printers/software/zpl-pg/r-sgd-wireless-sgd-wireless-commands/r-sgd-wired-internal-wired-ip-dhcp-cid-suffix.html). |
| Wi-Fi DHCP client ID | `wlan.ip.dhcp.cid_enable` | `on`, `off` | Same values as the wired command. The wireless page is [wlan.ip.dhcp.cid_prefix](https://docs.zebra.com/us/en/printers/software/zpl-pg/r-sgd-wireless-sgd-wireless-commands/r-sgd-wireless-wlan-ip-dhcp-cid-prefix.html) and [wlan.ip.dhcp.cid_type](https://docs.zebra.com/us/en/printers/software/zpl-pg/r-sgd-wireless-sgd-wireless-commands/r-sgd-wireless-wlan-ip-dhcp-cid-type.html). `ip.dhcp.cid_*` is the older wireless name and is not also written. |
| Wi-Fi client ID type | `wlan.ip.dhcp.cid_type` | `0`, `1`, `2` | Same meaning as the wired type. |
| Wi-Fi client ID prefix | `wlan.ip.dhcp.cid_prefix` | Up to 10 characters for text, or 20 for hex | |
| Wi-Fi client ID suffix | `wlan.ip.dhcp.cid_suffix` | Up to 60 characters for text, or 120 for hex | |
| Send wired DHCP host name | `internal_wired.ip.dhcp.option12` | `on`, `off`. The wireless page's default is `on`. | [ip.dhcp.option12](https://docs.zebra.com/us/en/printers/software/zpl-pg/r-sgd-wireless-sgd-wireless-commands/r-sgd-wireless-ip-dhcp-option12.html) documents the values. The wired command is `internal_wired.ip.dhcp.option12`. |
| Wired DHCP host name | `internal_wired.ip.dhcp.option12_format` | A string. `<device.friendly_name>` uses the friendly name. | [option12_value](https://docs.zebra.com/us/en/printers/software/zpl-pg/r-sgd-wireless-sgd-wireless-commands/r-sgd-wired-internal-wired-ip-dhcp-option12-value.html) shows this format. |
| Wired DHCP host name in use | `internal_wired.ip.dhcp.option12_value` | Read only | The printer fills this in. This tool does not set it. |
| Send Wi-Fi DHCP host name | `ip.dhcp.option12` | `on`, `off`. Default `on`. | Wireless discovery packet. |
| Wi-Fi DHCP host name | `ip.dhcp.option12_format` | A string | [ip.dhcp.option12_format](https://docs.zebra.com/us/en/printers/software/zpl-pg/r-sgd-wireless-sgd-wireless-commands/r-sgd-wireless-ip-dhcp-option12-format.html). |
| Whole client ID | `internal_wired.ip.dhcp.cid_all` | Up to 60 text characters, or 120 hex | Not shown as its own field. The guide says setting it clears the prefix and puts the whole ID in the suffix. |

## Actions

| Button | Command | Guide |
| --- | --- | --- |
| Calibrate media | `~JC` | Set Media Sensor Calibration. In continuous mode only the sensors are calibrated. No reply is documented. |
| Print configuration label | `~WC` | Prints the configuration label when the printer is idle. It is a settings report, not a stored format. Allowed. |
| Reset to last saved settings | `^XA^JUR^XZ` | `^JU` parameter `R`, recall last saved settings. |
| Save after a change | `^XA^JUS^XZ` | `^JU` parameter `S`, save current settings. The guide says factory and network reloads are lost at power-off if they are not saved. |
| Reset printer settings to factory | `^XA^JUF^XZ` | `^JU` parameter `F`, reload factory settings (not network). |
| Reset network settings to factory | `^XA^JUN^XZ` | `^JU` parameter `N`, reload factory network settings. |
| Restart printer | `! U1 setvar "device.reset" ""` | Soft reset. The wired-address page says the print server must be reset before a new permanent address takes effect. |

`^JUA` (factory settings and factory network together) is implemented in the simulator and rejected nowhere in the console if someone types it, but there is no button for it. `device.restore_defaults` is documented only for the `ip`, `wlan`, and `internal_wired` branches. The factory button uses `^JUF`, which covers printer settings rather than only that SGD branch.

## Host queries

| Query | What the guide says comes back |
| --- | --- |
| `~HI` | `model,version,dots-per-mm,memory[,options]`. Dots/mm: 6, 8, 12, or 24. 8 dots/mm is the 203 dpi head. Real replies are usually wrapped in STX and ETX. |
| `~HS` | Three STX…ETX LF strings (interface, paper, pause, head, ribbon, print mode, and so on). The printer sends nothing if it is out of paper, out of ribbon, the head is open, or the rewinder is full. |
| `^HH` | Configuration label returned to the host. Settings text. Allowed in the console. Not read by the main screen. |

## Not sent, on purpose

These read or dump stored formats, graphics, captured data, or the file list. The console rejects them. The simulator does not invent a file list if one is asked anyway.

`file.dir`, `file.type`, other `file.*`, `capture.channel*`, `allcv`, `^HF`, `^HG`, `^HY`, `^WD`, `^DF`, `^XF`, `^XG`, `~HD`, `~DY`, `~DG`, `~DB`, and label field data such as `^FD`.
