# NETGEAR management interfaces

Reference notes on how NETGEAR hardware can be driven programmatically.
NETGEAR ships no official API, no vendor Terraform provider, and no vendor Ansible collection, so every option below is either an undocumented interface or a community tool.

## Management surfaces

| Surface | Availability | Suitability as a provider backend |
| --- | --- | --- |
| FASTPATH CLI over telnet, port 60000 | Smart switches (GS7xxT, GS3xx), off by default | Best available option, full config coverage |
| Cisco-style CLI over SSH | Fully managed lines only (M4250/M4300/M4350) | Good, but not present on smart switches |
| REST API | M4300 and newer managed switches only | Ideal where present, absent on the hardware in scope |
| Web UI form posts | All smart switches | Works, brittle across firmware revisions |
| SNMP | Most smart switches, v1/v2c/v3 | Read-friendly, painful for writes |
| Config file export and import | All smart switches | Whole-config only, no partial applies or diffs |

## The FASTPATH CLI

Smart switches run Broadcom FASTPATH firmware and expose its CLI over telnet on **port 60000**.
NETGEAR does not document or support this, but the command set largely matches the published CLI reference manuals for the fully managed lines.

The CLI is **disabled by default**.
Enable it in the web UI under `Maintenance > Troubleshooting > Remote Diagnostics`.
Until that is set, connections to port 60000 are refused while ICMP still answers, which is the usual first symptom.

Login flow differs by firmware generation, per [oxidized's model notes](https://github.com/ytti/oxidized/blob/master/docs/Model-Notes/Netgear.md):

- **Newer models (GS724Tv4, and similar):** a `User:` prompt, then the web UI password, then `enable` with **no** enable password prompt.
  The shell prompt is the model name, for example `(GS724Tv4) #`.
- **Older models (GS748Tv4, and similar):** the username prompt is embedded in an `Applying Interface configuration, please wait ...` banner, and `enable` prompts for a password that expects an empty string.
  The shell prompt is the generic `(Broadcom FASTPATH Switching) #`.

Once in, `terminal length 0` disables pagination and `show running-config` dumps the whole configuration.
Changes are applied live and must be written to NVRAM separately to survive a reboot.

Two properties matter for a provider design.
The CLI is a real, stable, structured interface rather than HTML, which makes read and write both tractable.
But it is telnet with no transport security, and the enable step is effectively unauthenticated, so it should be treated as a management-VLAN-only interface and the provider should be explicit about that in its docs.

## Prior art

- [ntgrrc](https://github.com/nitram509/ntgrrc): Go CLI for the GS30x/GS31xEP plus switches.
  Drives the web UI, covers ports and PoE, supports JSON output.
  Useful as a reference for the web UI protocol, but it does not cover the larger smart switches.
- [mnalis/netgear-switch-config](https://github.com/mnalis/netgear-switch-config): Perl and Expect tooling that applies a declarative VLAN and port config file to GS724T-series switches over the FASTPATH CLI.
  Unmaintained since 2020 and hardcoded for the older prompt style, but it is the closest existing thing to declarative config for this hardware and its config file format is worth studying.
- [oxidized](https://github.com/ytti/oxidized): config backup daemon that already speaks the NETGEAR FASTPATH CLI and commits `show running-config` output to git.
  Complements rather than competes with a provider: oxidized reads, the provider would write.
- [ardje/backupswitch](https://github.com/ardje/backupswitch): shell and curl script that re-enables the telnet CLI by posting to `telnetSmart.html`, for cases where the setting does not persist.
  Also a working example of the web UI login and form-post flow.
- [pynetgear](https://github.com/MatMaul/pynetgear): SOAP client for consumer routers, as used by Home Assistant.
  Query and reboot only, not a configuration surface.

## Implications for this provider

The FASTPATH CLI is the only interface with enough coverage to back real resources on smart switches, so an Expect-style client over telnet is the pragmatic starting point.
Model detection matters: the prompt string and the enable behavior both vary by firmware generation, so the client should detect which of the two login flows applies rather than assuming one.
Because the interface is undocumented and off by default, the provider should fail with a clear, actionable error when port 60000 refuses a connection, pointing at the Remote Diagnostics setting.

For hardware where the CLI is unavailable, the realistic fallbacks are SNMP for reads and config export and import for whole-device state, neither of which maps cleanly onto Terraform's resource model.

## Reference hardware

`GS724Tv4`, 24 gigabit copper ports plus 2 dedicated SFP ports, no serial console port.
It runs the newer-style FASTPATH login flow.

## Verified against the reference hardware

Confirmed on a `GS724Tv4` running firmware `6.3.1.19`, boot `B1.0.0.4`.

Two spellings name the same port, and commands accept either:

| Surface | Physical port | LAG |
| --- | --- | --- |
| `show running-config`, `show port` | `g7` | `lag 1` |
| `show interfaces status` | `0/7` | `3/1` |

The provider normalizes everything it reads to the slot form, so `g7` becomes `0/7` and `lag 1` becomes `3/1`.
`show interfaces status g7` echoes `0/7`, which is how the two are known to be the same port.

Port status needs `show port`, not `show interfaces status`.
Only `show port` carries an Admin Mode column; `show interfaces status` reports the link state alone.

Config and interface modes nest the prompt: `(GS724Tv4) (Config)#`, then `(GS724Tv4) (Interface 0/7)#`.
Prompt matching has to consume every parenthesized group or the outer name leaks into command output.

A rejected command answers in one of two ways, both of which the client treats as an error:

```
% Invalid input detected at '^' marker.
An invalid interface range has been used for this function.
```

The config writes `vlan participation auto 1` for the default VLAN.
Only `include` counts as membership, so an `auto` line is deliberately ignored.

LAG interfaces exist whether or not they are configured: `show port all` lists `lag 1` through `lag 26`, and `show port-channel all` gives each a default name of `ch1` and up.
