# Existing plant cosmetics migration

The new collection keeps legacy `selected_palette` and `selected_pot` fields during the compatibility window. The backfill adds matching permanent ownership and fills only empty palette/pot equipment slots; it never clears or overwrites an existing equipment choice. All 16 legacy palettes, four basic pots, and 10 special reward pots have catalog entries. Existing special pots therefore remain usable after migration.

1. Take a recoverable backup of the `users` collection before an apply run. Keep the backup until the learner and admin release checks are complete.
2. Deploy the backend with the expanded cosmetic catalog first. The old frontend still uses the legacy fields.
3. Set `MONGO_URI` and `DATABASE_NAME` for the intended environment. Run `go run ./cmd/migrate-plant-cosmetics` to preview the count. This command is dry-run by default.
4. Review the count and target environment, then run `go run ./cmd/migrate-plant-cosmetics --apply`. Run the dry-run again; `planned=0` means the backfill has converged. A partial run can be retried safely.
5. Deploy the frontend and check learners with a basic palette, a special reward pot, and an already equipped cosmetic. Also check that an admin plant edit grants and equips a newly selected pot.

Do not remove the legacy selected fields during this release. Roll back application code first if a release problem appears; the additive ownership/equipment data is compatible with the previous app. If the data itself must be reverted, restore from the pre-apply backup after comparing any cosmetics earned since that backup. Do not blindly remove cosmetic IDs because a learner may have earned the same item independently.
