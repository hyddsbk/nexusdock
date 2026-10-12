package core

import "testing"

func TestSchemaV5AddsAuditTrustCheckpoints(t *testing.T) {
	ctx := t.Context()
	db, err := OpenSQLite(ctx, ":memory:", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, migration := range schemaMigrations[:4] {
		for _, statement := range migration.statements {
			if _, err := db.ExecContext(ctx, statement); err != nil {
				t.Fatalf("apply v%d %s: %v", migration.version, migration.name, err)
			}
		}
	}
	if _, err := db.ExecContext(ctx, `PRAGMA user_version = 4`); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, db); err != nil {
		t.Fatal(err)
	}

	var version int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if CurrentSchemaVersion != 5 {
		t.Fatalf("CurrentSchemaVersion = %d, want 5", CurrentSchemaVersion)
	}
	if version != 5 {
		t.Fatalf("schema version = %d, want 5", version)
	}
	var tableCount int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='agentdock_audit_trust_checkpoints'`,
	).Scan(&tableCount); err != nil {
		t.Fatal(err)
	}
	if tableCount != 1 {
		t.Fatalf("audit trust checkpoint table count = %d, want 1", tableCount)
	}
}
