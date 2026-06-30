# Silo SDK — Developer Guide

> Built by Koda Labs. For solo devs who know what they're building.

---

## What Is Silo

Silo is a math-driven data engine. No SQL. No Redis. No query planner. You define your paths, Silo resolves every address by pure math, and you get O(1) reads and writes every time.

You don't talk to a database. You talk to paths.

---

## Install

```bash
npm install silo-sdk
```

---

## Connect

You get one connection string when you create a workspace. That's your only env var.

```bash
# .env
SILO_URL=silo://x7k2m9:koda_wk_xxxxx@kodaworld-silobase.hf.space
```

```javascript
import Silo from 'silo-sdk'

const silo = Silo.connect(process.env.SILO_URL)
```

Done. You're connected.

---

## The Three Methods

Silo has three methods. That's all you need.

```javascript
silo.get(path)
silo.set(path, value)
silo.del(path)
```

---

## Write Data

```javascript
// single field
await silo.set("users/u_600/balance", 34)
await silo.set("users/u_600/city", "Addis Ababa")

// any value type
await silo.set("games/g_42/active", true)
await silo.set("games/g_42/score", 9800)
await silo.set("products/p_01/tags", ["shoes", "sport"])
```

---

## Read Data

```javascript
// single field — O(1) always
const balance = await silo.get("users/u_600/balance")
// returns: 34

// full entity — all fields fetched in parallel
const user = await silo.get("users/u_600")
// returns: { balance: 34, city: "Addis Ababa" }

// nested path
const city = await silo.get("users/u_600/address/city")
// returns: "Nairobi"
```

---

## Delete Data

```javascript
// delete a field
await silo.del("users/u_600/balance")

// delete an entity
await silo.del("users/u_600")
```

---

## Path Rules

Paths are how Silo knows where everything lives. Follow these rules and you'll never have a problem.

| Rule | Good | Bad |
|---|---|---|
| Lowercase only | `users/u_600` | `Users/U_600` |
| No spaces | `user_name` | `user name` |
| No special chars | `balance` | `$balance` |
| Slash separates levels | `users/u_600/balance` | `users.u_600.balance` |
| Your ID, your format | `u_600` or `600` or `uuid` | anything consistent works |

If you send bad paths at registration Silo normalizes them and tells you what it changed. After registration your paths are clean — Silo only deals with normalized paths from that point.

---

## Nested Paths

Silo handles any depth. Nesting is just more path segments.

```javascript
// flat
await silo.set("users/u_600/balance", 34)

// nested
await silo.set("users/u_600/address/city", "Addis Ababa")
await silo.set("users/u_600/address/street", "Bole Road")

// read a nested branch
const address = await silo.get("users/u_600/address")
// returns: { city: "Addis Ababa", street: "Bole Road" }
```

---

## Registration

Before writing data, register your schema. This tells Silo what fields exist under each path segment so composite reads work correctly.

```javascript
await silo.register({
  segment: "users",
  fields: ["balance", "city", "name"]
})

await silo.register({
  segment: "games",
  fields: ["score", "active", "level"]
})
```

Silo returns what it normalized. Check it once and you're done. Registration is idempotent — run it on every app startup, it causes zero damage.

---

## Multiple Workspaces

Two workspaces = two env vars. That is the maximum complexity.

```bash
SILO_USERS=silo://x7k2m9:koda_wk_xxxxx@kodaworld-silobase.hf.space
SILO_GAMES=silo://a1b2c3:koda_wk_yyyyy@kodaworld-silobase.hf.space
```

```javascript
const users = Silo.connect(process.env.SILO_USERS)
const games = Silo.connect(process.env.SILO_GAMES)

await users.get("u_600/balance")
await games.get("g_42/score")
```

---

## Error Handling

Every error has a named reason. No black hole failures.

```javascript
try {
  const balance = await silo.get("users/u_600/balance")
} catch (err) {
  switch (err.code) {
    case "invalid_api_key":     // wrong workspace key
    case "workspace_inactive":  // workspace suspended
    case "rate_limit_exceeded": // slow down
    case "path_not_found":      // nothing at this path yet
    case "checksum_invalid":    // data corruption detected, Silo is recovering
  }
}
```

---

## Response Shape

Every call returns a consistent shape.

```javascript
// success
{ ok: true, value: 34, T: 3 }

// error
{ ok: false, error: "path_not_found", code: 404 }
```

`T` is the state version. You don't need to use it but it's there if you want to track versions.

---

## What Silo Is Not

- Not a SQL database — no queries, no joins, no WHERE clauses
- Not a search engine — you address by path, not by content
- Not a document store — you address fields directly, not whole blobs

If you know your paths ahead of time — and you do, because you built the app — Silo is all you need.

---

*Koda Labs — silo-sdk*
