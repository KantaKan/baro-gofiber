# Care Energy migration

The application now stores balances and history in `care_energy_balance` and `care_energy_log`.

Set `MONGO_URI` and `DATABASE_NAME`, then inspect the planned migration:

```powershell
go run ./cmd/migrate-care-energy
```

The command stops if an account has conflicting old and new values. Apply only after the dry-run totals look correct:

```powershell
go run ./cmd/migrate-care-energy --apply
```

The command compares account count, total balance, and total history entries before and after the update. It exits with an error if any total changes. Running the same command again is safe.

To roll back for an older application release, preview and then apply the inverse migration:

```powershell
go run ./cmd/migrate-care-energy --rollback
go run ./cmd/migrate-care-energy --rollback --apply
```

Deploy in this order: stop writes, run the forward migration, deploy the new backend, then resume writes. Rollback in reverse order.
