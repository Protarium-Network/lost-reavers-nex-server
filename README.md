# Lost Reavers — NEX server

A replacement game server for **Lost Reavers** (Wii U, Bandai Namco /
ACQUIRE, JPN beta 2015-09, JPN release 2015-12, worldwide 2016-04), whose
official servers shut down 2019-05-30.

It speaks NEX — Nintendo's build of the Quazal Rendez-Vous middleware — over
PRUDP, built on [Pretendo Network](https://github.com/PretendoNetwork)'s
`nex-go`, `nex-protocols-go` and `nex-protocols-common-go`, the same
libraries this project's Sonic & All-Stars Racing Transformed and Mario
Tennis: Ultra Smash servers use.

---

## Game constants

| | |
|---|---|
| Title IDs | `000500001018D900` (JPN), `00050000101A4800` (USA), `00050000101B9900` (EUR) |
| Game server ID | `0x1018D900` |
| Access key | **`625f8dcc`** — confirmed live against a real console, see [docs/ACCESS_KEY.md](docs/ACCESS_KEY.md) |
| NEX version | **3.10.0** — confirmed, see below |

Sources: title IDs and NEX version are triple/cross-confirmed from
independent sources: Pretendo's
[nex-viewer](https://github.com/PretendoNetwork/nex-viewer) `titleids.txt`,
the game's own executable strings, and kinnay's
[wiiu.json](https://kinnay.github.io/data/wiiu.json) library fingerprint
database, which also confirms DataStore/Ranking/MatchMaking/Messaging are
genuinely linked (and Utility isn't) — see
[docs/PROTOCOLS.md](docs/PROTOCOLS.md). The access key was absent from every
public source, including that same fingerprint database (it records library
*versions*, not credentials) — recovered instead from a real console's own
captured packet; see [docs/ACCESS_KEY.md](docs/ACCESS_KEY.md) for the full
method.

All three regions share one game server ID, so they share one matchmaking
pool.

---

## How Lost Reavers' online play works

Lost Reavers is a 1-4 player co-op dungeon raid: players pick from four
classes (Sayuri, Dwayne, Victoria, ShadowStalker), team up in parties of up
to four, and clear multi-room maps for loot ("Relics") against waves of
enemies. From the protocol evidence in the executable (no capture exists —
see [docs/PROTOCOLS.md](docs/PROTOCOLS.md)):

- **Matchmaking** puts up to four players in a party (`MatchMaking` +
  `MatchmakeExtension`), and the game additionally links a **persistent
  community/lobby** layer (`CreateCommunity`, `PersistentGathering`) not
  present in simpler titles like Ultra Smash — plausibly a raid-hub or
  friends-list-style lobby.
- **DataStore** is used extensively — far more than a typical Wii U title —
  almost certainly for the treasure/inventory/equipment persistence the
  game's own class names (`USnowTreasureSet`, `USnowWarehouse`,
  `AttachEquipment`) point at.
- **Ranking** backs leaderboards.
- **Messaging** is linked and registered for real (unlike Ranking on Ultra
  Smash, which is defensive-only).

```
        console                       this server
           │
           │  ── LoginEx ─────────────▶  authentication server  :60000
           │  ◀─ Kerberos ticket +
           │     secure server address
           │
           │  ── ticket, RegisterEx ──▶  secure server          :60001
           │  ── AutoMatchmake /
           │     CreateCommunity ─────▶
           │  ◀─ gathering + up to 3 other consoles' station URLs
           │
           │  ── NAT probes ──────────▶
           │  ── DataStore: post/get
           │     treasure objects ────▶  (S3-compatible bucket)
           ▼
    party of up to 4, peer to peer for the raid itself
```

---

## Protocols implemented

See [docs/PROTOCOLS.md](docs/PROTOCOLS.md) for the full evidence trail behind
this list — every entry here is backed by real symbols found in the
decompressed game executable, not guessed.

### Authentication server (UDP 60000)

| Protocol | ID |
|---|---|
| Ticket Granting | `0x0A` |

### Secure server (UDP 60001)

| Protocol | ID | Notes |
|---|---|---|
| Secure Connection | `0x0B` | Both `Register` and `RegisterEx` enabled — unconfirmed which the title calls |
| NAT Traversal | `0x03` | |
| Match Making | `0x15` | |
| Match Making Ext | `0x32` | |
| Matchmake Extension | `0x6D` | Includes communities (`CreateCommunity`) and a block list |
| DataStore | `0x73` | Full implementation — object bodies in S3, metadata in Postgres |
| Ranking | `0x70` | |
| Messaging | `0x17` | |
| Utility | `0x6E` | **Not confirmed linked** (kinnay's library fingerprint has no `nexut`) — kept registered defensively, same call Ultra Smash's server makes for its own unconfirmed protocols |
| Monitoring / Health | `0x13` / `0x12` | Liveness only |

**Not implemented:** NintendoNotificationEvent (`0x64`) — confirmed linked,
no common-go implementation exists, left to a logged `NotImplemented` rather
than a hand-rolled guess. See docs/PROTOCOLS.md.

**Anything else** gets a logged, loud `NotImplemented` rather than silence —
`nex/trace.go`'s fallback tracer names the exact protocol ID so a genuinely
missing one is a one-line report, not a mystery hang.

---

## Requirements

- **Go 1.25 or newer**
- **PostgreSQL 13 or newer** — matchmaking, unique IDs, leaderboards and
  DataStore metadata. Schemas are created on first boot.
- **An S3-compatible bucket** for DataStore object bodies. `docker compose
  up` runs a local MinIO instance for you; any other S3-compatible provider
  works too via `PN_LR_S3_*`.

---

## Running

### From source

```bash
go build -o build/lost-reavers .
```

```bash
cp .env.example .env
cp settings.example.json settings.json
```

Edit `.env` — **you cannot skip `PN_LR_ACCESS_KEY`**, see
[docs/ACCESS_KEY.md](docs/ACCESS_KEY.md) — then:

```bash
./build/lost-reavers
```

### With Docker

```bash
cp settings.example.json settings.json
```

```bash
PN_LR_SECURE_SERVER_HOST=192.168.1.10 PN_LR_ACCESS_KEY=<confirmed key> docker compose up --build
```

This also starts Postgres and a local MinIO instance for DataStore.

---

## Configuration

Full reference in [.env.example](.env.example). The variables with no safe
default and no fallback:

| Variable | Meaning |
|---|---|
| `PN_LR_SECURE_SERVER_HOST` | Address the console dials — your LAN or public IP/domain |
| `PN_LR_POSTGRES_URI` | e.g. `postgres://lostreavers:lostreavers@localhost/lostreavers?sslmode=disable` |
| `PN_LR_ACCESS_KEY` | **No default.** See docs/ACCESS_KEY.md |
| `PN_LR_S3_ENDPOINT` / `_ACCESS_KEY` / `_SECRET_KEY` | DataStore object storage |

Both PRUDP ports need inbound UDP open on any firewall in between:

```powershell
New-NetFirewallRule -DisplayName "Lost Reavers NEX" -Direction Inbound -Protocol UDP -LocalPort 60000,60001 -Action Allow
```

---

## Accounts

Same shape as this project's other Wii U servers — see
[settings.example.json](settings.example.json) for standalone mode
(`PN_LR_LOCAL_AUTH=1`), or configure `PN_LR_ACCOUNT_GRPC_*` /
`PN_LR_ACCOUNT_SECRET` to use a real account server. Friends
(`PN_LR_FRIENDS_GRPC_*`) are optional but plausibly load-bearing here, unlike
Ultra Smash — see [globals/friends.go](globals/friends.go).

---

## Pointing a console at it

Same as this project's other servers: install
[Inkay](https://github.com/PretendoNetwork/Inkay) in Cemu (or on Aroma for a
retail console) and point it at an account server that will issue a NEX
token for game server ID `1018D900`. See
[docs/ACCESS_KEY.md](docs/ACCESS_KEY.md) for the specific first run, which
doubles as the access-key capture session.

---

## Credits

Built on [Pretendo Network](https://github.com/PretendoNetwork)'s NEX
libraries. Title IDs from Pretendo's
[nex-viewer](https://github.com/PretendoNetwork/nex-viewer). Protocol
evidence from static analysis of the game's own executable — see
[docs/PROTOCOLS.md](docs/PROTOCOLS.md).

This is a clean-room replacement server for a service that no longer exists.
It contains no Nintendo or Bandai Namco code or assets.

## License

GNU Affero General Public License, version 3 or later. See
[LICENSE](LICENSE) and [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
