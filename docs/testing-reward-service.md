# Testing the Reward Draw service

The Reward Draw tests are small backend unit tests. They run with `go test ./...` and do not need MongoDB, a server, or environment secrets.

## The mental model

Use Arrange–Act–Assert:

1. Arrange the catalog, owned items, rarity floor, pool, species, and controlled random values.
2. Act by calling `Selector.Select` or `Service.Open` once.
3. Assert the behavior a learner or caller can observe: the chosen rarity, excluded duplicate, explicit pool-complete error, or single persisted reward.

The selector receives a `RandomSource`. Production can use a normal pseudorandom generator. Tests use a tiny sequence or constant fake, so a boundary such as `0.55` always exercises the first Rare outcome. This makes a random feature deterministic without testing private helper functions.

## Fakes and invariants

`fakeDrawRepository` stores draws and ownership in memory. It implements only the repository contract that the service needs. A mutex models the atomic boundary required from the eventual Mongo repository.

The important invariants are:

- one idempotency key resolves to one stored result;
- concurrent opens of the same entitlement grant one item once;
- owned, incompatible, starter, wrong-pool, and below-floor items are never candidates;
- duplicate protection ends with an explicit pool-complete result, not a silent duplicate;
- rarity weights are 55%, 30%, 12%, and 3%, then renormalized after a rarity floor is applied;
- randomness never uses fertilizer, reflection text, or comfort-zone answers.

The service tests prove how it uses an atomic repository contract. They do not prove a future Mongo implementation is atomic. That repository must commit the draw record, ownership update, and consumed entitlement together, with a unique index on the idempotency key. Its integration test should race two commits and verify one stored draw and one inventory change.

## Adding a case

Add another row to a table-driven test when the setup and invariant match existing cases. Give the row a behavior-focused name, provide the smallest catalog and ownership map that demonstrate it, and force random values at the exact boundary you care about. Create a separate test when the behavior needs a different concurrency setup or a different assertion shape.

Run the focused package while editing:

```sh
go test ./internal/service/reward
```

Then run the full backend suite:

```sh
go test ./...
```
