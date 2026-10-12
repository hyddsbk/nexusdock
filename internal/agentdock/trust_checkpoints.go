package agentdock

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	AuditTrustRegistryContract       = "audit-trust-registry/v1"
	AuditTrustCheckpointContract     = "audit-trust-checkpoint/v1"
	AuditTrustCheckpointStatement    = "audit-trust-checkpoint-statement/v1"
	AuditTrustAlgorithm              = "ed25519"
	auditTrustMaxCheckpointFreshness = 24 * time.Hour
)

var (
	ErrTrustCheckpointNotFound     = errors.New("AgentDock audit trust checkpoint not found")
	ErrTrustCheckpointRollback     = errors.New("AgentDock audit trust checkpoint revision rollback")
	ErrTrustCheckpointEquivocation = errors.New("AgentDock audit trust checkpoint equivocation")
	ErrTrustRootChanged            = errors.New("AgentDock audit trust root changed")
)

type AuditTrustSubject struct {
	NodeID   string `json:"node_id"`
	DeviceID string `json:"device_id"`
}

type AuditTrustKey struct {
	KeyID     string `json:"key_id"`
	Algorithm string `json:"algorithm"`
	PublicKey string `json:"public_key"`
	Status    string `json:"status"`
	ValidFrom string `json:"valid_from"`
	RetiredAt string `json:"retired_at,omitempty"`
	RevokedAt string `json:"revoked_at,omitempty"`
}

type AuditTrustRegistry struct {
	Contract      string            `json:"contract"`
	Revision      int               `json:"revision"`
	Subject       AuditTrustSubject `json:"subject"`
	RootKeyID     string            `json:"root_key_id"`
	RootPublicKey string            `json:"root_public_key"`
	Keys          []AuditTrustKey   `json:"keys"`
	UpdatedAt     string            `json:"updated_at"`
	Signature     string            `json:"signature"`
}

type AuditTrustCheckpointStatementV1 struct {
	Contract         string `json:"contract"`
	NodeID           string `json:"node_id"`
	DeviceID         string `json:"device_id"`
	RootKeyID        string `json:"root_key_id"`
	RegistryRevision int    `json:"registry_revision"`
	RegistrySHA256   string `json:"registry_sha256"`
	IssuedAt         string `json:"issued_at"`
	FreshUntil       string `json:"fresh_until"`
}

type AuditTrustCheckpoint struct {
	Contract  string                          `json:"contract"`
	Algorithm string                          `json:"algorithm"`
	Statement AuditTrustCheckpointStatementV1 `json:"statement"`
	Signature string                          `json:"signature"`
}

type AuditTrustCheckpointPublishInput struct {
	Checkpoint AuditTrustCheckpoint `json:"checkpoint"`
	Registry   AuditTrustRegistry   `json:"registry"`
}

type AuditTrustCheckpointRecord struct {
	NodeID           string               `json:"node_id"`
	DeviceID         string               `json:"device_id"`
	RootKeyID        string               `json:"root_key_id"`
	RegistryRevision int                  `json:"registry_revision"`
	RegistrySHA256   string               `json:"registry_sha256"`
	IssuedAt         time.Time            `json:"issued_at"`
	FreshUntil       time.Time            `json:"fresh_until"`
	Checkpoint       AuditTrustCheckpoint `json:"checkpoint"`
	Registry         AuditTrustRegistry   `json:"registry"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

type auditTrustRegistryPayload struct {
	Contract      string            `json:"contract"`
	Revision      int               `json:"revision"`
	Subject       AuditTrustSubject `json:"subject"`
	RootKeyID     string            `json:"root_key_id"`
	RootPublicKey string            `json:"root_public_key"`
	Keys          []AuditTrustKey   `json:"keys"`
	UpdatedAt     string            `json:"updated_at"`
}

func (s *Store) PublishAuditTrustCheckpoint(ctx context.Context, nodeID string, input AuditTrustCheckpointPublishInput) (AuditTrustCheckpointRecord, error) {
	nodeID = strings.TrimSpace(nodeID)
	node, err := s.Get(ctx, nodeID)
	if err != nil {
		return AuditTrustCheckpointRecord{}, err
	}
	if !node.Enabled {
		return AuditTrustCheckpointRecord{}, ErrNodeDisabled
	}
	if err := validateAuditTrustCheckpoint(node, input, s.now().UTC()); err != nil {
		return AuditTrustCheckpointRecord{}, err
	}

	registrySHA, err := auditTrustRegistrySHA256(input.Registry)
	if err != nil {
		return AuditTrustCheckpointRecord{}, err
	}
	checkpointJSON, err := json.Marshal(input.Checkpoint)
	if err != nil {
		return AuditTrustCheckpointRecord{}, fmt.Errorf("encode AgentDock audit trust checkpoint: %w", err)
	}
	registryJSON, err := json.Marshal(input.Registry)
	if err != nil {
		return AuditTrustCheckpointRecord{}, fmt.Errorf("encode AgentDock audit trust registry: %w", err)
	}
	issuedAt, _ := time.Parse(time.RFC3339Nano, input.Checkpoint.Statement.IssuedAt)
	freshUntil, _ := time.Parse(time.RFC3339Nano, input.Checkpoint.Statement.FreshUntil)
	now := s.now().UTC()

	existing, existingErr := s.AuditTrustCheckpoint(ctx, nodeID)
	switch {
	case existingErr == nil:
		if input.Checkpoint.Statement.RootKeyID != existing.RootKeyID {
			return AuditTrustCheckpointRecord{}, ErrTrustRootChanged
		}
		if input.Checkpoint.Statement.RegistryRevision < existing.RegistryRevision {
			return AuditTrustCheckpointRecord{}, ErrTrustCheckpointRollback
		}
		if input.Checkpoint.Statement.RegistryRevision == existing.RegistryRevision {
			if !strings.EqualFold(registrySHA, existing.RegistrySHA256) {
				return AuditTrustCheckpointRecord{}, ErrTrustCheckpointEquivocation
			}
			if issuedAt.Before(existing.IssuedAt) {
				return AuditTrustCheckpointRecord{}, ErrTrustCheckpointRollback
			}
		}
	case errors.Is(existingErr, ErrTrustCheckpointNotFound):
	default:
		return AuditTrustCheckpointRecord{}, existingErr
	}

	result, err := s.db.ExecContext(ctx, `INSERT INTO agentdock_audit_trust_checkpoints(
		node_id, device_id, root_key_id, registry_revision, registry_sha256,
		issued_at, fresh_until, checkpoint_json, registry_json, updated_at
	) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(node_id) DO UPDATE SET
		device_id = excluded.device_id,
		root_key_id = excluded.root_key_id,
		registry_revision = excluded.registry_revision,
		registry_sha256 = excluded.registry_sha256,
		issued_at = excluded.issued_at,
		fresh_until = excluded.fresh_until,
		checkpoint_json = excluded.checkpoint_json,
		registry_json = excluded.registry_json,
		updated_at = excluded.updated_at
	WHERE agentdock_audit_trust_checkpoints.root_key_id = excluded.root_key_id
	  AND (
		agentdock_audit_trust_checkpoints.registry_revision < excluded.registry_revision
		OR (
			agentdock_audit_trust_checkpoints.registry_revision = excluded.registry_revision
			AND lower(agentdock_audit_trust_checkpoints.registry_sha256) = lower(excluded.registry_sha256)
			AND agentdock_audit_trust_checkpoints.issued_at <= excluded.issued_at
		)
	  )`,
		nodeID, node.DeviceID, input.Checkpoint.Statement.RootKeyID, input.Checkpoint.Statement.RegistryRevision, registrySHA,
		issuedAt.Format(time.RFC3339Nano), freshUntil.Format(time.RFC3339Nano), string(checkpointJSON), string(registryJSON), now.Format(time.RFC3339Nano))
	if err != nil {
		return AuditTrustCheckpointRecord{}, fmt.Errorf("save AgentDock audit trust checkpoint: %w", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		latest, lookupErr := s.AuditTrustCheckpoint(ctx, nodeID)
		if lookupErr != nil {
			return AuditTrustCheckpointRecord{}, fmt.Errorf("save AgentDock audit trust checkpoint lost monotonic race: %w", lookupErr)
		}
		switch {
		case latest.RootKeyID != input.Checkpoint.Statement.RootKeyID:
			return AuditTrustCheckpointRecord{}, ErrTrustRootChanged
		case latest.RegistryRevision > input.Checkpoint.Statement.RegistryRevision:
			return AuditTrustCheckpointRecord{}, ErrTrustCheckpointRollback
		case latest.RegistryRevision == input.Checkpoint.Statement.RegistryRevision && !strings.EqualFold(latest.RegistrySHA256, registrySHA):
			return AuditTrustCheckpointRecord{}, ErrTrustCheckpointEquivocation
		default:
			return AuditTrustCheckpointRecord{}, ErrTrustCheckpointRollback
		}
	}
	return s.AuditTrustCheckpoint(ctx, nodeID)
}

func (s *Store) AuditTrustCheckpoint(ctx context.Context, nodeID string) (AuditTrustCheckpointRecord, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return AuditTrustCheckpointRecord{}, invalid("node id is required")
	}
	var (
		record                               AuditTrustCheckpointRecord
		issuedAt, freshUntil, checkpointJSON string
		registryJSON, updatedAt              string
	)
	err := s.db.QueryRowContext(ctx, `SELECT node_id, device_id, root_key_id, registry_revision, registry_sha256,
		issued_at, fresh_until, checkpoint_json, registry_json, updated_at
		FROM agentdock_audit_trust_checkpoints WHERE node_id = ?`, nodeID).
		Scan(&record.NodeID, &record.DeviceID, &record.RootKeyID, &record.RegistryRevision, &record.RegistrySHA256,
			&issuedAt, &freshUntil, &checkpointJSON, &registryJSON, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AuditTrustCheckpointRecord{}, ErrTrustCheckpointNotFound
	}
	if err != nil {
		return AuditTrustCheckpointRecord{}, fmt.Errorf("read AgentDock audit trust checkpoint: %w", err)
	}
	if record.IssuedAt, err = time.Parse(time.RFC3339Nano, issuedAt); err != nil {
		return AuditTrustCheckpointRecord{}, fmt.Errorf("parse audit trust checkpoint issued_at: %w", err)
	}
	if record.FreshUntil, err = time.Parse(time.RFC3339Nano, freshUntil); err != nil {
		return AuditTrustCheckpointRecord{}, fmt.Errorf("parse audit trust checkpoint fresh_until: %w", err)
	}
	if record.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt); err != nil {
		return AuditTrustCheckpointRecord{}, fmt.Errorf("parse audit trust checkpoint updated_at: %w", err)
	}
	if err := json.Unmarshal([]byte(checkpointJSON), &record.Checkpoint); err != nil {
		return AuditTrustCheckpointRecord{}, fmt.Errorf("decode stored AgentDock audit trust checkpoint: %w", err)
	}
	if err := json.Unmarshal([]byte(registryJSON), &record.Registry); err != nil {
		return AuditTrustCheckpointRecord{}, fmt.Errorf("decode stored AgentDock audit trust registry: %w", err)
	}
	return record, nil
}

func validateAuditTrustCheckpoint(node Node, input AuditTrustCheckpointPublishInput, now time.Time) error {
	registry := input.Registry
	checkpoint := input.Checkpoint
	if registry.Contract != AuditTrustRegistryContract {
		return invalid("unsupported audit trust registry contract")
	}
	if registry.Revision <= 0 || registry.Subject.NodeID != node.ID || registry.Subject.DeviceID != node.DeviceID {
		return invalid("audit trust registry subject does not match authenticated AgentDock node")
	}
	if _, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(registry.UpdatedAt)); err != nil {
		return invalid("audit trust registry updated_at is invalid")
	}
	rootPublic, err := decodeAuditTrustPublicKey(registry.RootPublicKey)
	if err != nil {
		return invalid("audit trust registry root public key is invalid")
	}
	if auditTrustKeyID("audit-root-sha256", rootPublic) != registry.RootKeyID {
		return invalid("audit trust registry root key id does not match its public key")
	}
	payload, err := auditTrustRegistrySigningBytes(registry)
	if err != nil {
		return err
	}
	registrySignature, err := decodeAuditTrustSignature(registry.Signature)
	if err != nil || !ed25519.Verify(rootPublic, payload, registrySignature) {
		return invalid("audit trust registry root signature verification failed")
	}
	if err := validateAuditTrustRegistryKeys(registry.Keys); err != nil {
		return err
	}

	if checkpoint.Contract != AuditTrustCheckpointContract || checkpoint.Algorithm != AuditTrustAlgorithm || checkpoint.Statement.Contract != AuditTrustCheckpointStatement {
		return invalid("unsupported audit trust checkpoint contract")
	}
	if checkpoint.Statement.NodeID != node.ID || checkpoint.Statement.DeviceID != node.DeviceID {
		return invalid("audit trust checkpoint subject does not match authenticated AgentDock node")
	}
	if checkpoint.Statement.RootKeyID != registry.RootKeyID || checkpoint.Statement.RegistryRevision != registry.Revision {
		return invalid("audit trust checkpoint does not match the supplied registry identity/revision")
	}
	registrySHA, err := auditTrustRegistrySHA256(registry)
	if err != nil {
		return err
	}
	if !strings.EqualFold(registrySHA, checkpoint.Statement.RegistrySHA256) {
		return invalid("audit trust checkpoint registry digest does not match the supplied registry")
	}
	issuedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(checkpoint.Statement.IssuedAt))
	if err != nil {
		return invalid("audit trust checkpoint issued_at is invalid")
	}
	freshUntil, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(checkpoint.Statement.FreshUntil))
	if err != nil {
		return invalid("audit trust checkpoint fresh_until is invalid")
	}
	now = now.UTC()
	if issuedAt.After(now.Add(5 * time.Minute)) {
		return invalid("audit trust checkpoint issued_at is unreasonably in the future")
	}
	if !freshUntil.After(issuedAt) || freshUntil.Sub(issuedAt) > auditTrustMaxCheckpointFreshness {
		return invalid("audit trust checkpoint freshness window is invalid")
	}
	if now.After(freshUntil) {
		return invalid("audit trust checkpoint has expired")
	}
	checkpointPayload, err := json.Marshal(checkpoint.Statement)
	if err != nil {
		return fmt.Errorf("encode audit trust checkpoint statement: %w", err)
	}
	checkpointSignature, err := decodeAuditTrustSignature(checkpoint.Signature)
	if err != nil || !ed25519.Verify(rootPublic, checkpointPayload, checkpointSignature) {
		return invalid("audit trust checkpoint root signature verification failed")
	}
	return nil
}

func validateAuditTrustRegistryKeys(keys []AuditTrustKey) error {
	seen := make(map[string]struct{}, len(keys))
	active := 0
	for _, key := range keys {
		if _, exists := seen[key.KeyID]; exists || strings.TrimSpace(key.KeyID) == "" {
			return invalid("audit trust registry contains duplicate or empty signing key ids")
		}
		seen[key.KeyID] = struct{}{}
		if key.Algorithm != AuditTrustAlgorithm {
			return invalid("audit trust registry contains unsupported signing algorithm")
		}
		publicKey, err := decodeAuditTrustPublicKey(key.PublicKey)
		if err != nil || auditTrustKeyID("audit-key-sha256", publicKey) != key.KeyID {
			return invalid("audit trust registry contains invalid signing public key")
		}
		if _, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(key.ValidFrom)); err != nil {
			return invalid("audit trust registry contains invalid key validity")
		}
		switch key.Status {
		case "active":
			active++
			if key.RetiredAt != "" || key.RevokedAt != "" {
				return invalid("active audit trust key has terminal lifecycle timestamps")
			}
		case "retired":
			if _, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(key.RetiredAt)); err != nil || key.RevokedAt != "" {
				return invalid("retired audit trust key lifecycle is invalid")
			}
		case "revoked":
			if _, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(key.RevokedAt)); err != nil {
				return invalid("revoked audit trust key lifecycle is invalid")
			}
		default:
			return invalid("audit trust registry contains unsupported key status")
		}
	}
	if active != 1 {
		return invalid("audit trust registry must contain exactly one active signing key")
	}
	return nil
}

func auditTrustRegistrySigningBytes(registry AuditTrustRegistry) ([]byte, error) {
	keys := append([]AuditTrustKey(nil), registry.Keys...)
	sort.Slice(keys, func(i, j int) bool { return keys[i].KeyID < keys[j].KeyID })
	payload := auditTrustRegistryPayload{
		Contract: registry.Contract, Revision: registry.Revision, Subject: registry.Subject,
		RootKeyID: registry.RootKeyID, RootPublicKey: registry.RootPublicKey,
		Keys: keys, UpdatedAt: registry.UpdatedAt,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode audit trust registry payload: %w", err)
	}
	return data, nil
}

func auditTrustRegistrySHA256(registry AuditTrustRegistry) (string, error) {
	canonical := registry
	canonical.Keys = append([]AuditTrustKey(nil), registry.Keys...)
	sort.Slice(canonical.Keys, func(i, j int) bool { return canonical.Keys[i].KeyID < canonical.Keys[j].KeyID })
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("encode canonical audit trust registry: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func auditTrustKeyID(prefix string, publicKey ed25519.PublicKey) string {
	sum := sha256.Sum256(publicKey)
	return prefix + ":" + hex.EncodeToString(sum[:])
}

func decodeAuditTrustPublicKey(value string) (ed25519.PublicKey, error) {
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("invalid ed25519 public key")
	}
	return ed25519.PublicKey(raw), nil
}

func decodeAuditTrustSignature(value string) ([]byte, error) {
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(raw) != ed25519.SignatureSize {
		return nil, errors.New("invalid ed25519 signature")
	}
	return raw, nil
}
