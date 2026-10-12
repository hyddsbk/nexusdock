package agentdock

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type trustFixtureKeys struct {
	rootPublic  ed25519.PublicKey
	rootPrivate ed25519.PrivateKey
	signPublic  ed25519.PublicKey
}

func newTrustFixtureKeys(t *testing.T) trustFixtureKeys {
	t.Helper()
	rootPublic, rootPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return trustFixtureKeys{rootPublic: rootPublic, rootPrivate: rootPrivate, signPublic: signPublic}
}

func makeTrustCheckpointFixture(t *testing.T, node Node, keys trustFixtureKeys, revision int, registryUpdatedAt, issuedAt time.Time, freshness time.Duration) AuditTrustCheckpointPublishInput {
	t.Helper()
	registry := AuditTrustRegistry{
		Contract:      AuditTrustRegistryContract,
		Revision:      revision,
		Subject:       AuditTrustSubject{NodeID: node.ID, DeviceID: node.DeviceID},
		RootKeyID:     auditTrustKeyID("audit-root-sha256", keys.rootPublic),
		RootPublicKey: base64.RawStdEncoding.EncodeToString(keys.rootPublic),
		Keys: []AuditTrustKey{{
			KeyID: auditTrustKeyID("audit-key-sha256", keys.signPublic), Algorithm: AuditTrustAlgorithm,
			PublicKey: base64.RawStdEncoding.EncodeToString(keys.signPublic), Status: "active",
			ValidFrom: registryUpdatedAt.UTC().Format(time.RFC3339Nano),
		}},
		UpdatedAt: registryUpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	payload, err := auditTrustRegistrySigningBytes(registry)
	if err != nil {
		t.Fatal(err)
	}
	registry.Signature = base64.RawStdEncoding.EncodeToString(ed25519.Sign(keys.rootPrivate, payload))
	registrySHA, err := auditTrustRegistrySHA256(registry)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := AuditTrustCheckpoint{
		Contract:  AuditTrustCheckpointContract,
		Algorithm: AuditTrustAlgorithm,
		Statement: AuditTrustCheckpointStatementV1{
			Contract: AuditTrustCheckpointStatement,
			NodeID:   node.ID, DeviceID: node.DeviceID, RootKeyID: registry.RootKeyID,
			RegistryRevision: revision, RegistrySHA256: registrySHA,
			IssuedAt:   issuedAt.UTC().Format(time.RFC3339Nano),
			FreshUntil: issuedAt.Add(freshness).UTC().Format(time.RFC3339Nano),
		},
	}
	checkpointPayload, err := json.Marshal(checkpoint.Statement)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint.Signature = base64.RawStdEncoding.EncodeToString(ed25519.Sign(keys.rootPrivate, checkpointPayload))
	return AuditTrustCheckpointPublishInput{Checkpoint: checkpoint, Registry: registry}
}

func pairTrustCheckpointNode(t *testing.T, store *Store) Node {
	t.Helper()
	pairing, err := store.CreatePairingCode(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	node, err := store.Pair(t.Context(), PairInput{Code: pairing.Code, DeviceID: "device_trust_checkpoint_123", Name: "TrustDock"})
	if err != nil {
		t.Fatal(err)
	}
	return node
}

func TestAuditTrustCheckpointPublishReadAndRefresh(t *testing.T) {
	store, _ := newTestStore(t)
	base := time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)
	store.now = func() time.Time { return base.Add(30 * time.Minute) }
	node := pairTrustCheckpointNode(t, store)
	keys := newTrustFixtureKeys(t)

	first := makeTrustCheckpointFixture(t, node, keys, 1, base, base.Add(time.Minute), 4*time.Hour)
	record, err := store.PublishAuditTrustCheckpoint(t.Context(), node.ID, first)
	if err != nil {
		t.Fatal(err)
	}
	if record.RegistryRevision != 1 || record.RootKeyID != first.Registry.RootKeyID || record.RegistrySHA256 != first.Checkpoint.Statement.RegistrySHA256 {
		t.Fatalf("record = %#v", record)
	}

	refreshed := makeTrustCheckpointFixture(t, node, keys, 1, base, base.Add(2*time.Minute), 5*time.Hour)
	record, err = store.PublishAuditTrustCheckpoint(t.Context(), node.ID, refreshed)
	if err != nil {
		t.Fatal(err)
	}
	if !record.IssuedAt.Equal(base.Add(2*time.Minute)) || !record.FreshUntil.Equal(base.Add(5*time.Hour+2*time.Minute)) {
		t.Fatalf("refresh record = %#v", record)
	}
}

func TestAuditTrustCheckpointRejectsRollbackEquivocationAndRootChange(t *testing.T) {
	store, _ := newTestStore(t)
	base := time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)
	store.now = func() time.Time { return base.Add(time.Hour) }
	node := pairTrustCheckpointNode(t, store)
	keys := newTrustFixtureKeys(t)

	revision2 := makeTrustCheckpointFixture(t, node, keys, 2, base.Add(2*time.Minute), base.Add(3*time.Minute), 4*time.Hour)
	if _, err := store.PublishAuditTrustCheckpoint(t.Context(), node.ID, revision2); err != nil {
		t.Fatal(err)
	}

	revision1 := makeTrustCheckpointFixture(t, node, keys, 1, base, base.Add(4*time.Minute), 4*time.Hour)
	if _, err := store.PublishAuditTrustCheckpoint(t.Context(), node.ID, revision1); !errors.Is(err, ErrTrustCheckpointRollback) {
		t.Fatalf("rollback error = %v", err)
	}

	equivocation := makeTrustCheckpointFixture(t, node, keys, 2, base.Add(5*time.Minute), base.Add(6*time.Minute), 4*time.Hour)
	if _, err := store.PublishAuditTrustCheckpoint(t.Context(), node.ID, equivocation); !errors.Is(err, ErrTrustCheckpointEquivocation) {
		t.Fatalf("equivocation error = %v", err)
	}

	otherRoot := newTrustFixtureKeys(t)
	rootChange := makeTrustCheckpointFixture(t, node, otherRoot, 3, base.Add(7*time.Minute), base.Add(8*time.Minute), 4*time.Hour)
	if _, err := store.PublishAuditTrustCheckpoint(t.Context(), node.ID, rootChange); !errors.Is(err, ErrTrustRootChanged) {
		t.Fatalf("root change error = %v", err)
	}
}

func TestAuditTrustCheckpointRejectsTamperExpiryAndOtherNode(t *testing.T) {
	store, _ := newTestStore(t)
	base := time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)
	store.now = func() time.Time { return base.Add(2 * time.Hour) }
	node := pairTrustCheckpointNode(t, store)
	keys := newTrustFixtureKeys(t)

	expired := makeTrustCheckpointFixture(t, node, keys, 1, base, base.Add(time.Minute), time.Hour)
	if _, err := store.PublishAuditTrustCheckpoint(t.Context(), node.ID, expired); err == nil {
		t.Fatal("expired trust checkpoint was accepted")
	}

	valid := makeTrustCheckpointFixture(t, node, keys, 1, base, base.Add(90*time.Minute), 4*time.Hour)
	valid.Checkpoint.Statement.RegistrySHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
	if _, err := store.PublishAuditTrustCheckpoint(t.Context(), node.ID, valid); err == nil {
		t.Fatal("tampered trust checkpoint was accepted")
	}

	otherPairing, err := store.CreatePairingCode(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.Pair(t.Context(), PairInput{Code: otherPairing.Code, DeviceID: "device_trust_checkpoint_456", Name: "OtherDock"})
	if err != nil {
		t.Fatal(err)
	}
	bound := makeTrustCheckpointFixture(t, node, keys, 1, base, base.Add(90*time.Minute), 4*time.Hour)
	if _, err := store.PublishAuditTrustCheckpoint(t.Context(), other.ID, bound); err == nil {
		t.Fatal("checkpoint bound to another node was accepted")
	}
}
