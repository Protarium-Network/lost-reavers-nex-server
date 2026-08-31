# How the protocol list was determined

No public packet capture of Lost Reavers exists (the servers shut down
2019-05-30). Everything below comes from decompressing
`PhantomGame-WiiU-Shipping.rpx` (the game's own executable — "PhantomGame"
appears to be its internal/working title) and searching the recovered
symbols, the same way the game constants were investigated
(see [ACCESS_KEY.md](ACCESS_KEY.md)).

## Decompressing the RPX

Wii U RPX/RPL files are ELF binaries whose sections are individually
zlib-compressed (flagged by an `SHF_RPL_ZLIB` bit, `0x08000000`, on
`sh_flags`; the first 4 bytes of a compressed section are the decompressed
size, big-endian, followed by the zlib stream). A naive `strings` pass over
the raw file only sees the handful of sections that aren't compressed and
turns up almost nothing — this is why an earlier pass of this investigation
found zero `http`, `nintendo`, or `NEX` hits before decompression. After
walking the section table and decompressing every `SHF_RPL_ZLIB` section, the
same scan turns up over 60,000 printable strings, most of them C++ symbol
names (the game ships largely unstripped).

## What the symbols confirm

The executable links `nn::nex` — Nintendo's Wii U build of the Quazal
Rendez-Vous middleware — directly, via the standard Unreal Engine 3
integration Nintendo shipped for third-party Wii U titles (`OnlineWiiUSystem`,
`OnlineWiiUNexSystem`, `SnowHttpSystem` classes, where `Snow` appears to be
this UE3 branch's internal codename). Confirmed present, by protocol:

| Protocol | Evidence |
|---|---|
| **TicketGranting** (`0x0A`) | `TicketGrantingProtocolClient`, `CallLoginEx`, `JobNNIDLogin` |
| **SecureConnection** (`0x0B`) | `SecureConnectionProtocolClient`, `CallRegisterEx`, `CallRequestURLs` |
| **NATTraversal** (`0x03`) | `NATTraversalProtocolClient`, `CallInitiateProbe`, `RVNATRelay` |
| **MatchMaking** (`0x15`) + **MatchMakingExt** (`0x32`) | `MatchMakingProtocolClient`, `CallFindByOwner`, `CallEndParticipation` |
| **MatchmakeExtension** (`0x6D`) | `MatchmakeExtensionProtocolClient`, `CallAutoMatchmake*`, `CallCreateMatchmakeSession*`, `CallJoinMatchmakeSession*`, **and** `CallCreateCommunity` / `PersistentGathering` / `CallFindOfficialCommunity` — a persistent lobby/community layer, plausibly the game's 4-player party hub |
| **DataStore** (`0x73`) | Extensive: `DataStorePreparePostParam`, `DataStoreFileServerClient` (large-object HTTP transfer), `DataStoreRatingInitParamWithSlot`, `UUDKUIDataStore_*`. This is the strongest signal in the whole binary — over 1,600 matching symbols — and is why this server implements DataStore fully rather than skipping it the way the Mario Tennis: Ultra Smash server does |
| **Ranking** (`0x70`) | `RankingProtocolClient`, `CallGetRankingByPIDList` |
| **Messaging** (`0x17`) | `MessagingServiceDDLDeclarations`, `MessagingProtocolClient`, `CallRetrieveMessages` |
| **Monitoring** (`0x13`) / **Health** (`0x12`) | Every stock NEX 3.x title answers liveness pings; registered defensively, costs nothing |

**Utility (`0x6E`) is registered but NOT confirmed** — see the cross-check
below, which settles this one the other way from everything else in the
table.

## Cross-confirmed independently: kinnay's wiiu.json

kinnay's site ([kinnay.github.io](https://kinnay.github.io)) publishes a
second, much larger dataset behind the NEX server list page:
[`data/wiiu.json`](https://kinnay.github.io/data/wiiu.json), a fingerprint
database of every Wii U title kinnay has scanned — which NEX/PIA/etc.
*library versions* are linked into each game's executable. It does **not**
contain access keys (there is no such field in its schema; it's a different
kind of data than the NEX server list page). But for Lost Reavers it does
independently confirm, by matching this project's own RPX byte-for-byte
(`rpxsize: 19016832`, the retail JPN build `WUP-P-APHJ` version 48):

```json
"nex":   [3, 10, 0]
"nexrk": [3, 10, 0]   // Ranking
"nexds": [3, 10, 0]   // DataStore
"nexmm": [3, 10, 0]   // MatchMaking
"nexms": [3, 10, 0]   // Messaging
```

— and the *absence* of `nexut` (Utility) from that same entry. This is the
same kind of signal the Mario Tennis: Ultra Smash server's own README uses to
justify **dropping** DataStore for that title (its library list has no
`nexds`); here it points the other way for four of five protocols and
disqualifies only Utility. Two independent sources (static string analysis
of the executable, and kinnay's separate fingerprint scan of the same
executable) now agree on:

- **NEX version 3.10.0** (was a guessed placeholder before this cross-check)
- DataStore, Ranking, MatchMaking and Messaging genuinely linked
- Utility's linkage is unconfirmed — kept registered anyway, defensively

Title IDs are triple-confirmed the same way: nex-viewer's `titleids.txt`,
strings in the executable itself, and `wiiu.json`'s `aid` field (decimal
`1407375153617152` / `1407375153711104` / `1407375153797376`, which is
`000500001018D900` / `00050000101A4800` / `00050000101B9900` in hex) all
agree exactly.

One protocol is confirmed **linked but intentionally left unimplemented**:

- **NintendoNotificationEvent** (`0x64`) — `NintendoNotificationEventProtocolClient/Server`,
  `_DDL_NintendoNotificationEventGeneral/Profile/KeyValue` are all present
  (friend presence broadcasts, most likely). nex-protocols-common-go has no
  ready-made handler for this protocol family. Rather than hand-roll one
  against unknown wire semantics, it is left unregistered: the console gets a
  loud, logged `NotImplemented` (see `nex/trace.go`'s fallback tracer)
  instead of hanging forever waiting for a reply. If online play is missing
  friend status and nothing else, this protocol is why.

