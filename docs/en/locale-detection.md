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

Messages are written with **ja as SSOT** (repo-internal [ADR-0002](../adr/0002-i18n-japanese-ssot.md)). When the `en` key is missing, `t()` falls back to the ja value; when the `ja` key is missing but `en` exists, `t('ja', key)` returns the en value. When both are missing, the key string itself is returned (for debugging).

## Related

- [scope-detection.md](./scope-detection.md): same priority pattern for `--scope`
- [cli-reference.md](./cli-reference.md): `--lang` flag
- [docs/adr/0002-i18n-japanese-ssot.md](../adr/0002-i18n-japanese-ssot.md): ja SSOT policy
