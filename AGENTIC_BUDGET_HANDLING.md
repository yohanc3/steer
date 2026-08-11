# Agentic Budget Handling

## Goal

Turn a natural-language Telegram `/budget` message into an ordered plan of
independently validated budget actions. The model proposes actions; Go validates,
persists, and executes them. Telegram receives a receipt for every action.

## Action model

The planner may propose `create_budget`, `replace_budget`, `move_allocation`,
`rename_category`, `remove_category`, `show_budget`, `resolve_unmatched`,
`delete_budget`, `clarify`, or `unsupported` actions. Each action has a stable
ID, optional dependencies, and typed JSON arguments.

Safe actions execute in order, each in its own SQLite transaction. A failed
action is recorded without rolling back unrelated actions; dependent actions are
recorded as skipped. Delete and reset actions wait for explicit confirmation.

## Persistence

One JSON-backed budget belongs to each Telegram user. Agent requests and actions
are stored with an idempotency key derived from the Telegram message ID and
action index, preventing webhook redelivery from applying an action twice.

## Safety

Every action preserves budget invariants after its own transaction. The model
cannot issue SQL or write state. Ambiguous supported requests return a
clarification; unsupported requests state the limitation and a supported next
step. Transient or malformed model responses are retried before a safe failure
is returned.

## Verification

Tests cover independent partial success, dependency skips, idempotent retries,
budget transfers that create a new category, confirmation of destructive
actions, and malformed model output without state mutation.
