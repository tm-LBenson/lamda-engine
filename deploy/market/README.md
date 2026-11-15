# Shared LamdaMarket

Build from the Engine directory with:

```
docker build -f deploy/market/Dockerfile -t lamdaui-market:local .
```

Alternatively, `Tools/prepare-market-host.py` packages only this server’s source
dependencies and embedded assets into `.build/market-host/source.tar.gz`, with a
checksum manifest. No repository publication is needed.

Configure `MARKET_HOST`, `MARKET_OWNER_EMAIL`, `CF_ACCESS_ISSUER` (the HTTPS team
origin) and `CF_ACCESS_AUDIENCE` (the application AUD tag). These are identifiers,
not session tokens. `MARKET_CLIENT` defaults to `forever`, `MARKET_DATA` to `/data`
and `MARKET_LISTEN` to `:8080`. Missing auth configuration prevents startup.
Mount persistent storage at `/data`, owned by UID/GID 10001. Keep Forever and
Retail in separate deployments and data directories.

Put the whole hostname behind a Cloudflare Access Allow policy containing only
invited email addresses, with One-time PIN as its login method. Route its tunnel
to the app’s loopback host port. A second Access application may bypass the
exact `/api/collector/import` path for collectors: the application itself verifies
Ed25519 signatures, timestamps and nonces and accepts public observations only.
Never bypass the rest of `/api` or the whole hostname.

`/healthz` returns a liveness result without private information. Market pages
and data require a valid Access JWT even if the origin is reached directly.
The origin must receive the original `Host` header. POST controls require the
matching HTTPS Origin and the application’s JSON request header.

Enable collectors through the authenticated owner’s Research settings. A local
Engine generates and retains its own signing key; only its public key is copied
to the hosted site. Disabling a collector rejects further uploads. Friend
collectors can be authorized later without sharing private account databases.

Back up `shared.sqlite` and all `profiles/*.sqlite` using SQLite’s online backup
API, or stop this app before copying databases. Store backups outside `/data`.
The companion’s local database and all other Coolify services remain separate.

After restoring an older shared database backup, replay the collector history:
turn sharing off and save an empty site address in the local Engine, then enter
the hosted address and enable it again. Changing the destination resets the
outgoing cursor while keeping the collector key; event replay is idempotent.
Restore the private profiles as well when recovering account lists and ledgers.
The installed personal deployment uses SQLite-aware daily backups at 04:17 UTC,
outside its live data folder, with no automatic deletion.
