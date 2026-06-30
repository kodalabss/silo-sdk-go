# SILO_SKILL.md — Silo SDK for AI Agents

> This file is the complete reference for any AI agent working on a project that uses the Silo SDK. Read this once. Follow it exactly. Do not guess. Do not drift.

---

## What Silo Is

Silo is a math-driven data engine by Koda Labs. It is not SQL. It is not Redis. It is not a document store. It uses deterministic path-based addressing. Every piece of data lives at a calculated address. There are no tables. There are no queries. There are no joins.

---

## RULES — Read These First

```
RULE 1:  Never write raw HTTP calls to Silobase. Always use the SDK methods.
RULE 2:  Never hardcode workspace keys. Always read from environment variables.
RULE 3:  Never use db.get() db.set() db.del() — the correct prefix is silo.
RULE 4:  Never write SQL. Silo has no SQL layer.
RULE 5:  Never invent endpoints. Silo has exactly three methods: get, set, del.
RULE 6:  Never store the silo:// URI as a string in code. Always use process.env.
RULE 7:  Always register schema before first write in a new segment.
RULE 8:  Never assume a path exists. Handle path_not_found in every read.
RULE 9:  Paths are always lowercase with underscores. Never spaces or special chars.
RULE 10: Never mix workspaces. Each silo instance connects to exactly one workspace.
```

---

## Setup — Always This Pattern

```javascript
import Silo from 'silo-sdk'

const silo = Silo.connect(process.env.SILO_URL)
```

If the project has multiple workspaces:

```javascript
const users = Silo.connect(process.env.SILO_USERS)
const games = Silo.connect(process.env.SILO_GAMES)
```

Never do this:
```javascript
// WRONG — never hardcode
const silo = Silo.connect("silo://x7k2m9:koda_wk_xxxxx@kodaworld-silobase.hf.space")
```

---

## The Three Methods — Complete Reference

### silo.get(path)

Reads data at a path. Returns the value or a full entity object.

```javascript
// single field
const value = await silo.get("users/u_600/balance")
// returns: 34

// full entity — all fields returned, fetched in parallel internally
const entity = await silo.get("users/u_600")
// returns: { balance: 34, city: "Addis Ababa", name: "Luke" }

// nested path
const city = await silo.get("users/u_600/address/city")
// returns: "Nairobi"
```

### silo.set(path, value)

Writes a value to a path. Accepts any serializable value.

```javascript
await silo.set("users/u_600/balance", 34)
await silo.set("users/u_600/active", true)
await silo.set("users/u_600/name", "Luke")
await silo.set("users/u_600/tags", ["admin", "beta"])
await silo.set("users/u_600/address/city", "Addis Ababa")
```

### silo.del(path)

Deletes a field or entity at a path.

```javascript
// delete one field
await silo.del("users/u_600/balance")

// delete full entity
await silo.del("users/u_600")
```

---

## Path Format — Strict Rules

```
{segment}/{id}/{field}
{segment}/{id}/{nested_segment}/{field}
```

| Rule | Correct | Wrong |
|---|---|---|
| Always lowercase | `users/u_600` | `Users/U_600` |
| Underscores not spaces | `user_name` | `user name` |
| No special characters | `balance` | `$balance` |
| Slash separates levels | `users/u_600/balance` | `users.u_600.balance` |
| No leading slash | `users/u_600` | `/users/u_600` |

---

## Registration — When and How

Register a segment before the first write. Registration is idempotent — safe to run on every app startup.

```javascript
await silo.register({
  segment: "users",
  fields: ["balance", "city", "name", "active"]
})
```

If you add a new field to an existing segment, re-register with the full field list including the new field. Silo handles the diff — no migration needed.

---

## Response Shape

Every method returns a consistent object. Always check `ok` before using `value`.

```javascript
// success
{ ok: true, value: <data>, T: <version_number> }

// error
{ ok: false, error: "<error_code>", code: <http_code> }
```

---

## Error Codes — Handle All Of These

```
path_not_found      →  nothing exists at this path yet. not an exception, handle it.
invalid_api_key     →  wrong workspace key. check environment variable.
workspace_inactive  →  workspace suspended. do not retry.
rate_limit_exceeded →  back off and retry after delay.
checksum_invalid    →  Silo detected corruption and is recovering. retry once.
```

Always use try/catch:

```javascript
try {
  const result = await silo.get("users/u_600/balance")
  if (!result.ok) {
    // handle named error
    console.error(result.error)
    return
  }
  const balance = result.value
} catch (err) {
  // network or unexpected failure
}
```

---

## Common Patterns

### Create a new entity

```javascript
await silo.set("users/u_601/name", "Abel")
await silo.set("users/u_601/city", "Addis Ababa")
await silo.set("users/u_601/balance", 0)
await silo.set("users/u_601/active", true)
```

### Read then update

```javascript
const result = await silo.get("users/u_600/balance")
if (result.ok) {
  const newBalance = result.value + 100
  await silo.set("users/u_600/balance", newBalance)
}
```

### Check if entity exists

```javascript
const result = await silo.get("users/u_600")
if (!result.ok && result.error === "path_not_found") {
  // entity does not exist yet
}
```

### Read multiple entities

```javascript
const ids = ["u_600", "u_601", "u_602"]
const results = await Promise.all(ids.map(id => silo.get(`users/${id}`)))
```

---

## What NOT To Do — Common Agent Mistakes

```javascript
// WRONG — no SQL ever
await silo.query("SELECT * FROM users WHERE balance > 100")

// WRONG — no raw fetch ever
await fetch("https://kodaworld-silobase.hf.space/get", { ... })

// WRONG — wrong prefix
await db.get("users/u_600/balance")

// WRONG — hardcoded key
const silo = Silo.connect("silo://x7k2m9:koda_wk_xxxxx@...")

// WRONG — no leading slash
await silo.get("/users/u_600/balance")

// WRONG — spaces in path
await silo.get("users/u 600/balance")

// WRONG — inventing endpoints
await silo.find("users", { balance: { gt: 100 } })
await silo.query("users", "balance > 100")
```

---

## Environment Variable Reference

```bash
# single workspace
SILO_URL=silo://x7k2m9:koda_wk_xxxxx@kodaworld-silobase.hf.space

# multiple workspaces — name by purpose
SILO_USERS=silo://x7k2m9:koda_wk_xxxxx@kodaworld-silobase.hf.space
SILO_GAMES=silo://a1b2c3:koda_wk_yyyyy@kodaworld-silobase.hf.space
```

Format: `silo://{workspace_id}:{workspace_key}@{host}`

---

## Architecture Notes For Context

- Silo engine resolves every address by math — xxHash64, hierarchical XOR, bit shift
- Silobase is the public gateway — all SDK calls go through it
- Barman handles backups — not relevant to SDK usage
- RAM is never the source of truth — all data is durable on write confirmation
- Writes are state transitions — each field has its own independent version timeline
- Reads follow the state pointer — the write pipeline establishes truth, reads just follow it

---

*Koda Labs — SILO_SKILL.md — AI Agent Reference*
*Do not modify this file without updating the SDK version.*
