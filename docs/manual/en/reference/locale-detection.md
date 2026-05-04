# Locale auto-detection

`gh-tasks` resolves the output language (`ja` / `en`) in this order.

## Order

1. **`--lang` flag** (explicit)
   - Both `--lang ja` and `--lang=ja` forms supported
   - Same for `--lang en` / `--lang=en`
2. **`~/.config/ozzylabs/gh-tasks.toml` `lang`** (`ja` / `en`)
3. **`LC_ALL` env**: starts with `ja` (case-insensitive) → `ja`
4. **`LANG` env**: starts with `ja` → `ja`
5. **Fallback** → `en`

`LC_ALL` outranks `LANG` per POSIX. With `LC_ALL=en_US` + `LANG=ja_JP`, `en` is selected.

## Invalid values

Specifying `--lang` with anything other than `ja` / `en` is **silently ignored** and the resolver falls through to env / fallback.

```bash
$ gh tasks add 'foo' --scope=repo --repo=owner/name --lang=fr
# `--lang=fr` is dropped; output uses LANG or defaults to `en`
```

When multiple `--lang` flags appear, the **first occurrence** wins.

## Implementation

`packages/gh-tasks/src/i18n/index.ts` `resolveLocale(argv, env?, config?)`. Both `env` and `config` are injectable so tests stay deterministic. `config` is loaded from `~/.config/ozzylabs/gh-tasks.toml` via `lib/config.ts`'s `loadConfig()`.

## SSOT language vs output language

Messages are currently written with **ja as SSOT** (originally per repo-internal [ADR-0002](../../../adr/0002-i18n-japanese-ssot.md), now Superseded by [ADR-0005](../../../adr/0005-i18n-reader-based-ssot.md) which inverts CLI i18n to en SSOT in a follow-up phase). At runtime `t(locale, key)` looks up the requested locale first, then falls back to `en` if the key is missing, and finally returns the key string itself for debugging when both are missing. The fallback is asymmetric: only the `en` table acts as a backstop, so an `en` key with no Japanese counterpart will leak the English string into `ja` output. Author both translations together.

## Related

- [scope-detection.md](./scope-detection.md): same priority pattern for `--scope`
- [cli.md](./cli.md): `--lang` flag
- [docs/adr/0005-i18n-reader-based-ssot.md](../../../adr/0005-i18n-reader-based-ssot.md): current i18n policy (Superseded ADR-0002)
