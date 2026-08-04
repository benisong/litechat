# JSON Complex Character Card Parser Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Add a new JSON complex character-card parser with embedded main/sub worldbook entries, hidden scheduler-only entries, injection controls, and story initialization support while leaving the legacy parser and legacy chat path unchanged until the new path is proven and later merged.

**Architecture:** Introduce interfaces at the parsing/normalization boundary. Keep `LegacyCharacterCardParser` behavior unchanged and add `JsonCharacterCardParser` as a parallel implementation. Both produce a normalized internal card model; the new initializer converts worldbook plot/scheduler entries into a schedulable worldbook, Manifest, and StoryState. During dual-track operation, format/version selection is explicit and failures are visible; no silent fallback from new JSON to legacy parsing.

**Tech Stack:** Go, Gin, SQLite, existing React/Vite frontend, existing worldbook and Story Runtime stores, Go table-driven tests, frontend JSON fixture tests/build.

---

## Design Contract

### New JSON envelope

```json
{
  "card_version": "1.0",
  "character": {
    "name": "重生之玄幻之旅",
    "pov": "second",
    "description": "角色身份，不放完整系统规则",
    "personality": "角色行为和说话风格",
    "scenario": "当前初始场景",
    "first_message": "第一条角色消息"
  },
  "worldbook": {
    "id": "rebirth-fantasy-main",
    "name": "重生之玄幻之旅·世界书",
    "version": "1.0",
    "global_enabled": true,
    "main_entries": [],
    "sub_entries": []
  },
  "tags": ["修仙", "复杂剧情", "动态世界"]
}
```

### Worldbook entry controls

Every new entry supports:

- `id`, `title`, `content`
- `keys`, `secondary_keys`
- `enabled`: entry switch
- `constant`: always active regardless of keyword matching
- `user_visible`: whether normal UI/API exposes it
- `scheduler_enabled`: whether initialization/scheduler may use it
- `priority`, `order`
- `injection_position`: existing numeric contract (`0` relative end, `1` absolute position)
- `injection_depth`
- `scan_depth`
- `case_sensitive`
- `role`
- optional `activation.requires` / `activation.excludes`

Worldbook-level `global_enabled` gates the entire embedded book. Existing worldbook entry fields remain compatible with the current `WorldBookEntry` model; new visibility/scheduler metadata is added only to the new parser model first.

### Visibility rules

- UI/API normal character/worldbook views expose only `global_enabled && enabled && user_visible` entries.
- Initialization can read all enabled entries in the JSON envelope.
- Scheduler compilation uses enabled entries with `scheduler_enabled=true` plus public plot/rule entries explicitly marked for scheduling.
- Hidden scheduler entries must never be displayed by the normal worldbook editor or sent back through ordinary user-facing character-card APIs.
- `effects` and executable state changes are not trusted from the scheduler model; Go validation/commit remains authoritative.

---

## Phase 0: Freeze and inventory the legacy path

### Task 0.1: Identify the legacy parser boundary

**Files:**
- Inspect `internal/service/character_card_service.go`
- Inspect character-card handlers and store methods under `internal/api/` and `internal/store/`
- Inspect `web/src/store/index.js` character import methods

**Steps:**
1. Record the current legacy input shapes and output model.
2. Record all callers and tests.
3. Do not change behavior or rename old fields.

**Validation:** `go test ./... -run '^$'` and existing character-card tests pass before new code starts.

**Commit:** `chore: document legacy character card boundary`

### Task 0.2: Add characterization fixtures for legacy behavior

**Files:**
- Test file adjacent to the existing character-card service tests
- Add fixture under `internal/service/testdata/` only if the project already uses testdata fixtures

**Steps:**
1. Capture a representative legacy card.
2. Assert the existing parser output exactly.
3. Assert malformed legacy input keeps its current error behavior.

**Validation:** Run only the characterization tests and confirm they pass before changing implementation.

**Commit:** `test: characterize legacy character card parsing`

---

## Phase 1: Define new interfaces and neutral models

### Task 1.1: Add normalized parser interfaces

**Files:**
- Create `internal/service/character_card_parser.go`
- Test `internal/service/character_card_parser_test.go`

**Interfaces:**

```go
type CharacterCardParser interface {
    Parse(ctx context.Context, input []byte) (*ParsedCharacterCard, error)
    Format() string
}

type WorldBookParser interface {
    ParseWorldBook(input json.RawMessage) (*ParsedWorldBook, error)
}

type CharacterCardParserRegistry interface {
    Resolve(format string, version string) (CharacterCardParser, error)
}
```

**Requirements:**
- No legacy parser changes.
- New interfaces return neutral models, not database models.
- Errors identify format/version/schema problems.

**Validation:** Interface compile test and registry unknown-format test.

**Commit:** `feat: define character card parser interfaces`

### Task 1.2: Define new neutral card/worldbook models

**Files:**
- Create `internal/service/character_card_schema.go`
- Test `internal/service/character_card_schema_test.go`

**Models:**
- `ParsedCharacterCard`
- `ParsedCharacter`
- `ParsedWorldBook`
- `ParsedWorldBookEntry`
- `ParsedEntryActivation`
- `NormalizedCharacterCard`

**Requirements:**
- Preserve unknown extension fields in a controlled `Extensions map[string]json.RawMessage` if needed.
- Keep `user_visible`, `scheduler_enabled`, `global_enabled`, injection fields separate from legacy `WorldBookEntry` until adapter design is complete.
- Normalize `pov: second` and reject unsupported POV values explicitly.

**Validation:** Table tests for defaults, required fields, invalid enums, and unknown optional fields.

**Commit:** `feat: add neutral complex card and worldbook models`

---

## Phase 2: Implement the new JSON parser independently

### Task 2.1: Add JSON schema decoding and top-level validation

**Files:**
- Create `internal/service/json_character_card_parser.go`
- Test `internal/service/json_character_card_parser_test.go`

**Steps:**
1. Decode the JSON envelope.
2. Require `card_version`, `character`, and `worldbook`.
3. Validate `character.name`, `pov`, and required textual fields.
4. Validate `worldbook.global_enabled` and entry arrays.
5. Reject duplicate entry IDs.
6. Reject malformed `injection_position`, negative depth, and invalid roles.

**Validation:** RED/GREEN table tests for valid card, missing fields, duplicate IDs, invalid depth, and unsupported version.

**Commit:** `feat: parse versioned json character cards`

### Task 2.2: Parse main/sub entries and visibility/scheduler metadata

**Files:**
- Modify `internal/service/json_character_card_parser.go`
- Test `internal/service/json_character_card_parser_test.go`

**Requirements:**
- Preserve main/sub grouping.
- Preserve all injection controls.
- Apply safe defaults:
  - `enabled=true`
  - `global_enabled=true` at book level unless explicitly false
  - `injection_position=0`
  - `injection_depth=4`
  - `scan_depth=0`
  - `role=system`
- Do not infer `scheduler_enabled=true` from a tag alone; require the explicit boolean.
- Permit tags such as `scheduler`, `hidden`, `lore`, but treat booleans as authoritative.

**Validation:** Assert parsed values exactly match input and defaults are deterministic.

**Commit:** `feat: parse worldbook entry controls`

### Task 2.3: Add parser adapter output for existing character creation

**Files:**
- Create `internal/service/character_card_adapter.go`
- Test `internal/service/character_card_adapter_test.go`

**Requirements:**
- Convert only the visible character fields to the existing character model.
- Do not inject hidden scheduler entries into `description`, `personality`, or `scenario`.
- Produce a separate normalized worldbook payload for the new initializer.
- Preserve the card/worldbook version and content hash inputs.

**Validation:** Assert the legacy character model does not contain hidden worldbook content.

**Commit:** `feat: adapt json cards without coupling worldbook into character text`

---

## Phase 3: Build the schedulable-worldbook initializer

### Task 3.1: Define worldbook compilation interfaces

**Files:**
- Create `internal/service/schedulable_worldbook.go`
- Test `internal/service/schedulable_worldbook_test.go`

**Interfaces:**

```go
type SchedulableWorldBookBuilder interface {
    Build(ctx context.Context, card *NormalizedCharacterCard) (*SchedulableWorldBook, error)
}

type WorldBookVisibilityFilter interface {
    ForUser(book *ParsedWorldBook) *ParsedWorldBook
    ForScheduler(book *ParsedWorldBook) *ParsedWorldBook
}
```

**Output sections:**
- public entries
- scheduler-only entries
- state candidates
- observation rules
- event candidates
- unresolved entries requiring compiler review

**Validation:** Hidden entries appear only in scheduler output; disabled/global-disabled entries are excluded.

**Commit:** `feat: define schedulable worldbook builder interfaces`

### Task 3.2: Implement deterministic filtering before model compilation

**Files:**
- Modify `internal/service/schedulable_worldbook.go`
- Test `internal/service/schedulable_worldbook_test.go`

**Rules:**
- `global_enabled=false` disables all entries.
- `enabled=false` disables one entry.
- `scheduler_enabled=true` includes an entry in scheduler compilation.
- `user_visible=false` excludes an entry from user-facing output.
- Main/sub ordering, priority, injection position, injection depth, and order are retained in normalized output.
- No hidden entry is copied into character description fields.

**Validation:** Matrix tests for all visibility/scheduler combinations.

**Commit:** `feat: filter worldbook entries for user and scheduler tracks`

### Task 3.3: Integrate the existing Manifest compiler behind the new initializer

**Files:**
- Create `internal/service/json_story_initializer.go`
- Reuse but do not alter legacy initializer behavior in `internal/service/story_chat_initializer.go`
- Tests adjacent to Manifest/Story initializer tests

**Requirements:**
- Feed the compiler the plot-outline worldbook plus scheduler-enabled entries.
- Keep user-visible lore available to the primary prompt path.
- Produce Manifest and initial StoryState only after successful validation.
- On failure, do not create an incomplete dynamic chat.
- Store the worldbook version hash based on the normalized JSON source.

**Validation:** Fixture with hidden scheduler entries produces Manifest fields/rules while hidden entries are absent from user-facing output.

**Commit:** `feat: initialize story runtime from json worldbook`

---

## Phase 4: Add explicit dual-track routing

### Task 4.1: Add format detection without changing legacy behavior

**Files:**
- Modify the new parser registry file
- Add a new routing service, e.g. `internal/service/character_card_router.go`
- Test `internal/service/character_card_router_test.go`

**Routing:**
- `card_version` + `character` + `worldbook` → new JSON parser.
- Existing legacy payload shape → legacy parser adapter.
- Ambiguous payload → explicit error; no silent fallback.

**Validation:** Legacy fixture output remains byte/field equivalent; new fixture uses only new parser.

**Commit:** `feat: route legacy and json character cards in parallel`

### Task 4.2: Add a feature flag/configuration switch

**Files:**
- Existing config/model location used for feature flags
- New router tests

**Modes:**
- `legacy_only`
- `dual_track`
- `json_only` only after migration approval

**Default:** `dual_track` in development/test; production initially keeps legacy as default for old payloads.

**Validation:** Explicit mode tests, including disabled new parser and new-parser failure behavior.

**Commit:** `feat: add character card parser track switch`

### Task 4.3: Add API import response separation

**Files:**
- Relevant character-card handler
- `web/src/store/index.js`
- Any import page used by character cards

**Requirements:**
- New JSON import returns card format/version and visible-worldbook summary.
- Hidden scheduler entry count may be returned as metadata, but hidden content must not be returned.
- Legacy response shape remains unchanged for legacy input.

**Validation:** API tests for both tracks and frontend build.

**Commit:** `feat: expose dual-track character card import results`

---

## Phase 5: Build tests before enabling production flow

### Task 5.1: Add fixture coverage

**Files:**
- `internal/service/testdata/complex-card-v1.json`
- `internal/service/testdata/complex-card-invalid.json`
- `internal/service/testdata/complex-card-hidden-scheduler.json`

**Fixtures must cover:**
- visible lore
- hidden scheduler-only entries
- main and sub entries
- global disable
- entry disable
- constant entries
- injection position/depth
- activation conditions
- `pov=second`

**Validation:** Fixture parsing and normalized output snapshot tests.

**Commit:** `test: add complex json card fixtures`

### Task 5.2: Add security/visibility tests

**Tests must prove:**
- Hidden entries do not appear in normal character GET.
- Hidden entries do not appear in normal worldbook editor data.
- Hidden entries do appear in initializer input.
- Scheduler-only content is not copied to character description/personality/scenario.
- User-supplied JSON cannot set arbitrary SQL/code/prompt fields outside the declared schema.

**Commit:** `test: protect hidden scheduler worldbook entries`

### Task 5.3: Add end-to-end tests with a fake compiler/scheduler

**Flow:**

```text
JSON card import
→ worldbook filtering
→ schedulable worldbook
→ Manifest compiler fake
→ StoryState
→ scheduler fake
→ state commit
→ primary prompt
```

**Assertions:**
- Primary prompt gets public lore and processed dynamic context.
- Scheduler gets scheduler-enabled entries and current state.
- Raw hidden scheduler prompt is not emitted to user.
- Failure leaves no incomplete chat/user message according to current rollback policy.

**Commit:** `test: cover complex json card dual-track flow`

---

## Phase 6: Test2 production validation and staged dual-track enablement

### Task 6.1: Deploy dual-track code without changing legacy defaults

**Validation:**
- `npm --prefix web run build`
- `go test ./...`
- `git diff --check`
- production container health endpoint
- legacy character import smoke test

**Commit:** `chore: enable json card dual-track in production`

### Task 6.2: Run test2 JSON-card E2E

Use a temporary JSON card containing:
- public main worldbook entry
- hidden scheduler-only main entry
- hidden scheduler-only sub entry
- injection controls
- one observable state transition

Verify:
- import succeeds
- UI only shows public entries
- dynamic initialization creates `Manifest.status=ready`
- scheduler succeeds
- state version increments
- primary model receives processed dynamic context
- no credentials or hidden entry content appear in logs/API responses

Do not record test credentials, tokens, or secrets.

### Task 6.3: Compare old/new outputs

For equivalent legacy and JSON cards:
- compare visible character fields
- compare public worldbook output
- compare initial scene behavior
- document intentional differences in dynamic mode

Do not switch all production cards yet.

---

## Phase 7: Final merge into the new implementation

### Task 7.1: Make the new parser the primary implementation

Only after all Phase 6 tests pass:
- Change default routing for versioned JSON cards to `JsonCharacterCardParser`.
- Keep `LegacyCharacterCardAdapter` for historical cards.
- Keep legacy parser source intact behind the adapter.
- Remove duplicate conversion logic, not compatibility behavior.

**Commit:** `refactor: make unified json parser the primary card path`

### Task 7.2: Consolidate shared contracts

Move only proven shared contracts into common packages:
- normalized card model
- worldbook entry controls
- visibility filtering
- parser registry

Do not merge legacy parser internals into the new parser.

**Commit:** `refactor: consolidate character card parser contracts`

### Task 7.3: Final regression and production rollout

Run:

```bash
export PATH=$HOME/go-sdk/go/bin:$PATH
npm --prefix web run build
go test ./...
git diff --check
```

Then:
- test legacy card import
- test JSON card import
- test normal chat
- test dynamic initialization
- test scheduler and primary response order
- verify hidden worldbook entries remain hidden
- deploy only after all checks pass

**Commit:** `chore: complete character card parser migration`

---

## Risks and Decisions

### Risk: Hidden content leaks through APIs

Mitigation: filter at backend serialization/API boundary, not only in React. UI filtering is not a security boundary.

### Risk: New parser accidentally changes legacy behavior

Mitigation: characterization tests, separate adapter, explicit format/version routing, and no edits to legacy parser logic during Phases 1–6.

### Risk: Scheduler receives too little semantic context

Mitigation: scheduler input includes only scheduler-enabled entries plus concise field/rule metadata; effects remain Go-owned. Add fixture tests asserting scheduler prompt contains required rule semantics.

### Risk: Injection controls have different meanings in primary vs scheduler prompts

Mitigation: preserve controls in normalized worldbook entries and define separate consumers. Scheduler compilation must not blindly reuse primary prompt injection order.

### Risk: Partial dynamic initialization

Mitigation: Manifest/StoryState creation remains transactional or compensating; failed initialization cannot create a usable complex chat without a ready Manifest.

### Open decisions before implementation

1. Whether `worldbook.global_enabled` should be persisted in the existing `world_books` table or remain JSON-only until worldbook migration.
2. Whether `user_visible=false` entries should be omitted entirely from normal GET responses or returned only as a count/metadata summary.
3. Whether `injection_position=1` means an absolute message index or a named anchor; retain current numeric behavior first, document it, and avoid redesign during parser work.
4. Whether external user-uploaded worldbooks are merged into `main_entries`/`sub_entries` or retained as referenced source blocks; implement the simpler embedded representation first.

## Completion Criteria

The migration is complete only when:

```text
legacy parser unchanged
new JSON parser independently tested
main/sub worldbook supported
hidden scheduler entries protected
injection controls preserved
worldbook initialization generates schedulable worldbook
Manifest and StoryState created successfully
scheduler and primary model use the intended separated contexts
old and new formats both work
production test2 E2E passes
new parser becomes primary path
legacy adapter remains for compatibility
```
