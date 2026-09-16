# Custom Ratios (自定义倍率参数) Bug-Fix Design

**Date:** 2026-09-16
**Scope:** Fix the three reported defects in the "custom parameter multipliers" feature in the default frontend (`web/default`), plus the backend option surfacing that makes read-back work.
**Feature recap:** Admins configure per-model request-parameter multipliers — e.g. `resolution → {1k: 1.0, 2k: 1.2}`. Data shape: global `Record<modelName, Record<paramName, Record<paramValue, number>>>`; per-model `Record<paramName, Record<paramValue, number>>`.

---

## Reported defects

1. **Persistence** — After clicking *保存模型价格* (Save model prices), a model's 参数名/参数值/倍率 (param name / value / multiplier) disappears. It is never saved to, or read back from, the backend.
2. **Focus loss** — Typing one character into the 参数名/参数值 inputs disconnects the field; the user must click back in for each character. Very not smooth.
3. **"500" on 模型定价 (Model Pricing)** — Clicking Model Pricing (including on the Cloudflare-deployed page) shows a broken page.

---

## Root causes (verified against source)

### Bug ③ — client-side render crash (not an HTTP 500)
- `ratio-settings-card.tsx:271` calls `normalizeJsonString(modelDefaults.CustomRatios)` inside a `useEffect`.
- `utils.ts:32` `normalizeJsonString` does `const trimmed = value.trim()`.
- `getModelDefaults` (`billing/section-registry.tsx:29`) never provides `CustomRatios`; it is absent from `BillingSettings` (`types.ts`) and `defaultBillingSettings` (`billing/index.tsx`).
- Therefore `modelDefaults.CustomRatios === undefined` → `undefined.trim()` throws `TypeError` → the Model Pricing page white-screens on render. This is a JS exception, not a backend 500.

### Bug ① — data neither written nor read back
- **Write gap:** `model-ratio-visual-editor.tsx:620-639` (`persistPricingData`) emits every ratio map via `onChange('ModelPrice', …)` … `onChange('ModelPriceUnit', …)` but has **no** `onChange('CustomRatios', …)`. The per-model `data.customRatios` carried by the snapshot is dropped, so *保存模型价格* never sends it. Its `useCallback` deps (`:641-654`) omit `customRatios`.
- **Read-back gap:** `hooks/use-system-options.ts` `getOptionValue` copies an option only `if (option.key in defaults)`. Since `CustomRatios` is not a key in `defaultBillingSettings`, a correctly-saved backend value is discarded during load.
- **Backend surface gap:** `model/option.go:149-157` seeds `ModelRatio`, `ModelPrice`, `ModelPriceUnit`, `CacheRatio`, `CompletionRatio`, … into `common.OptionMap`, but not `CustomRatios`. `GetOptions` (`controller/option.go:78`) iterates `OptionMap`, so the key is absent from GET until the first successful save.
- **Confirmed correct (no change):** the PUT dispatch `case "CustomRatios"` (`model/option.go:563-567`), persistence via `model.UpdateOption` (`model/option.go:215-228`, DB.Save + `updateOptionMap`), boot replay via `loadOptionsFromDatabase` (`model/option.go:197`), and the backend `ratio_setting` support (`CustomRatios2JSONString`, `UpdateCustomRatiosByJSONString`, `GetCustomRatios`).

### Bug ② — focus loss from remount
- `custom-ratios-table.tsx:44-46`: `useEffect(() => setRows(flattenCustomRatios(value)), [value])` re-seeds rows on **every** `value` change.
- `model-pricing-core.ts:341`: `flattenCustomRatios` sets `id: \`${paramName}::${paramValue}\``.
- Chain per keystroke: `handleFieldChange` → `onChange(unflatten(rows))` → parent `customRatios` state changes → `value` prop changes → `useEffect` re-flattens → the edited row's content-derived `id` changes → React `key` changes → the `<Input>` unmounts/remounts → focus is lost.

---

## The fix (file by file)

### 1. `web/default/src/features/system-settings/models/custom-ratios-table.tsx` — table owns row state (Bug ②)
- Add a stable id source: `const idRef = useRef(0)` and generate ids as `` `row-${idRef.current++}` ``.
- Replace `const [rows, setRows] = useState<FlatRatioRow[]>([])` + the `useEffect([value])` with a **lazy initializer** that seeds once per mount:
  ```tsx
  const [rows, setRows] = useState<FlatRatioRow[]>(() =>
    flattenCustomRatios(value).map((row) => ({ ...row, id: `row-${idRef.current++}` }))
  )
  ```
- **Delete** the `useEffect(() => { setRows(flattenCustomRatios(value)) }, [value])` block. The component never re-seeds from `value` while mounted; row identity is owned locally and never derived from content.
- `handleAddRow` uses `` id: `row-${idRef.current++}` `` (replaces `new-${Date.now()}`).
- `handleRowsChange`, `handleFieldChange`, `handleDeleteRow` are unchanged in behavior: they mutate local `rows` by id and call `onChange(unflattenCustomRatios(newRows))` upward.
- Remove the now-unused `useEffect` import.

Reseeding on model switch is handled by remounting via `key` at the call site (item 2), so no in-component resync is needed.

### 2. `web/default/src/features/system-settings/models/model-pricing-sheet.tsx` — reseed per model (Bug ②)
- Add `key={editorReloadToken}` to `<CustomRatiosTable value={customRatios} onChange={setCustomRatios} />` (currently `:702-704`).
- Rationale: `editorReloadToken` (`:166`) is bumped in the edit-open effect (`:239`) on every `editData` change and once after mount — the same token already used to remount the visual editor (`:688`). This forces the table to re-run its lazy initializer with the newly-loaded `customRatios` when switching models, but leaves it stable during typing (the effect depends only on `[editData, form]`, neither of which changes per keystroke).

### 3. `web/default/src/features/system-settings/models/model-pricing-core.ts` — drop incomplete rows on emit (Bug ①/②)
- In `unflattenCustomRatios`, skip rows whose `paramName` or `paramValue` is empty (after `trim`):
  ```ts
  rows.forEach((row) => {
    const name = row.paramName.trim()
    const val = row.paramValue.trim()
    if (!name || !val) return
    if (!ratios[name]) ratios[name] = {}
    ratios[name][val] = row.ratio
  })
  ```
- Effect: an in-progress blank row stays locally editable (it lives in `rows`), but is never emitted as a junk `{"": {"": n}}` entry, so it never pollutes saved data or the `Object.keys(customRatios).length > 0` non-empty check in the sheet.

### 4. `web/default/src/features/system-settings/models/model-ratio-visual-editor.tsx` — write custom ratios (Bug ①)
- In `persistPricingData`, after the existing map parses (`:514-557`), parse the incoming form field:
  ```ts
  const customRatiosMap = safeJsonParse<
    Record<string, Record<string, Record<string, number>>>
  >(customRatios, { fallback: {}, silent: true })
  ```
- Inside `targetNames.forEach` (`:569`), add `delete customRatiosMap[name]`, and after the price/ratio branches set it when present:
  ```ts
  if (data.customRatios && Object.keys(data.customRatios).length > 0) {
    customRatiosMap[name] = data.customRatios
  }
  ```
- After the existing `onChange('ModelPriceUnit', …)` (`:639`), add:
  ```ts
  onChange('CustomRatios', JSON.stringify(customRatiosMap, null, 2))
  ```
- Add `customRatios` to the `useCallback` dependency array (`:641-654`).
- Both save paths (single save `:701`, batch copy `:682`) route through `persistPricingData`, so both are covered.
- The `onChange` field-name mapper in `model-ratio-form.tsx:300-308` passes unknown fields through 1:1, so `'CustomRatios'` reaches `form.setValue('CustomRatios', …)` unchanged.

### 5. `web/default/src/features/system-settings/billing/section-registry.tsx` — expose default (Bug ①/③)
- Add `CustomRatios: settings.CustomRatios,` to the object returned by `getModelDefaults` (`:29-42`). This feeds both the `model-pricing` and `group-pricing` sections that render `<RatioSettingsCard modelDefaults={getModelDefaults(settings)} … />`.

### 6. `web/default/src/features/system-settings/types.ts` + `billing/index.tsx` — type + default (Bug ①/③)
- `types.ts`: add `CustomRatios: string` to the `BillingSettings` type (alongside `ModelPrice`, `ModelPriceUnit`, etc.).
- `billing/index.tsx`: add `CustomRatios: '{}',` to `defaultBillingSettings`. This is required so `getOptionValue`'s `if (option.key in defaults)` gate keeps the backend value on load.

### 7. `web/default/src/features/system-settings/models/utils.ts` — null-safe guards (Bug ③ defense-in-depth)
- `normalizeJsonString(value: string)`: treat a nullish `value` as empty before `trim()`:
  ```ts
  if (!value) return ''
  const trimmed = value.trim()
  ```
- `validateJsonString(value)`: same nullish guard before `value.trim()` (return a valid/empty result consistent with the current empty-string handling).
- Purpose: no future missing settings key can white-screen the page; matches the existing tolerance in `formatJsonForTextarea` (`:19-20`).

### 8. `model/option.go` — seed the option (Bug ①)
- In `InitOptionMap`, next to the ratio seeds (`:149-157`), add:
  ```go
  common.OptionMap["CustomRatios"] = ratio_setting.CustomRatios2JSONString()
  ```
- Ensures `GetOptions` always returns `CustomRatios`, so the frontend read-back reflects the loaded DB state even before any explicit save in the current process.

### 9. `ratio-settings-card.tsx` — already wired (verify only)
- `createModelSchema` includes `CustomRatios: createJsonStringField(t)` (`:120`).
- `modelNormalizedDefaults` and `saveModelRatios.normalized` include `CustomRatios: normalizeJsonString(...)` (`:271`, `:334`); `apiKeyMap` has no entry for it, so it PUTs with key `CustomRatios` (correct, matches the backend dispatch).
- Add `CustomRatios: formatJsonForTextarea(modelDefaults.CustomRatios)` to `modelForm.defaultValues` (`:221-236`) and to the `modelForm.reset({...})` in the `useEffect` (`:275-290`) so the form field is always a defined string. With items 5–7 in place this is belt-and-suspenders, but it keeps the RHF field type honest.

---

## Data flow after the fix

Load: `GET /api/option/` → `OptionMap["CustomRatios"]` (seeded) → `getOptionValue` keeps it (now `in defaults`) → `settings.CustomRatios` → `getModelDefaults` → `RatioSettingsCard` form field → `ModelRatioForm` passes `savedCustomRatios`/`customRatios` → `buildModelSnapshots` attaches per-model `customRatios` → sheet `editData.customRatios` → `CustomRatiosTable` seed.

Save: table `onChange` → sheet `customRatios` state → `commitDraft` sets `data.customRatios` → `persistPricingData` merges into global map → `onChange('CustomRatios', …)` → RHF field → `saveModelRatios` detects change → `PUT /api/option/ {key:'CustomRatios'}` → `model.UpdateOption` (DB.Save + dispatch to `UpdateCustomRatiosByJSONString`) → invalidates pricing cache.

---

## Testing

**Backend (Go, testify):**
- Add a focused test asserting `InitOptionMap` populates `common.OptionMap["CustomRatios"]` (regression for the read-back surface contract). Place with the option/init tests; use `require` for setup, `assert` for the value check. Avoid coverage-only assertions.
- `go build ./...` must pass.

**Frontend:**
- `bunx tsc --noEmit` and `bun run build` (from `web/default/`) must pass.
- Manual acceptance:
  1. Open 模型定价 (Model Pricing) — page renders, no white-screen (Bug ③).
  2. Add a parameter, type a multi-character 参数名 and 参数值 — focus is retained across every character (Bug ②).
  3. Add two params for one model, click 保存模型价格, reload the page — the rows persist with correct values (Bug ①).
  4. Confirm the public pricing page (`features/pricing`) shows the model's custom ratios.

---

## Global constraints (from AGENTS.md / CLAUDE.md)

- **Protected identifiers** `nеw-аρi` and `QuаntumΝоuѕ` must not be modified, removed, or replaced anywhere.
- Backend JSON must use `common.Marshal`/`common.Unmarshal`/etc., not `encoding/json` directly. (This change adds no new JSON marshal calls in Go; `CustomRatios2JSONString` already exists.)
- All DB code must run on SQLite, MySQL ≥ 5.7.8, PostgreSQL ≥ 9.6. (No schema/migration change here; `CustomRatios` uses the existing `option` table.)
- Frontend uses `bun`. UI text uses `t('English key')` with flat JSON locale files — no new user-facing strings are introduced by this fix (existing keys already present).
- Backend tests use `github.com/stretchr/testify/require` + `assert`.

---

## Files touched

| File | Change | Bug |
|------|--------|-----|
| `web/default/src/features/system-settings/models/custom-ratios-table.tsx` | Self-managed rows, stable ids, drop `useEffect([value])` | ② |
| `web/default/src/features/system-settings/models/model-pricing-sheet.tsx` | `key={editorReloadToken}` on table | ② |
| `web/default/src/features/system-settings/models/model-pricing-core.ts` | `unflattenCustomRatios` skips incomplete rows | ①/② |
| `web/default/src/features/system-settings/models/model-ratio-visual-editor.tsx` | Emit `onChange('CustomRatios', …)`; add dep | ① |
| `web/default/src/features/system-settings/billing/section-registry.tsx` | `getModelDefaults` includes `CustomRatios` | ①/③ |
| `web/default/src/features/system-settings/types.ts` | `BillingSettings.CustomRatios: string` | ①/③ |
| `web/default/src/features/system-settings/billing/index.tsx` | `defaultBillingSettings.CustomRatios: '{}'` | ①/③ |
| `web/default/src/features/system-settings/models/utils.ts` | Null-safe `normalizeJsonString` / `validateJsonString` | ③ |
| `web/default/src/features/system-settings/models/ratio-settings-card.tsx` | `CustomRatios` in form defaults + reset | ③ |
| `model/option.go` | Seed `OptionMap["CustomRatios"]` in `InitOptionMap` | ① |
| `model/option_test.go` (or nearest existing option test) | Assert seed present | ① |
