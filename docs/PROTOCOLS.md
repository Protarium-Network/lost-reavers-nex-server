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
| **Utility** (`0x6E`) | Implied by generic NEX unique ID use throughout; registered the same way Ultra Smash's server does |
| **Monitoring** (`0x13`) / **Health** (`0x12`) | Every stock NEX 3.x title answers liveness pings; registered defensively, costs nothing |

One protocol is confirmed **linked but intentionally left unimplemented**:

- **NintendoNotificationEvent** (`0x64`) — `NintendoNotificationEventProtocolClient/Server`,
  `_DDL_NintendoNotificationEventGeneral/Profile/KeyValue` are all present
  (friend presence broadcasts, most likely). nex-protocols-common-go has no
  ready-made handler for this protocol family. Rather than hand-roll one
  against unknown wire semantics, it is left unregistered: the console gets a
  loud, logged `NotImplemented` (see `nex/trace.go`'s fallback tracer)
  instead of hanging forever waiting for a reply. If online play is missing
  friend status and nothing else, this protocol is why.

