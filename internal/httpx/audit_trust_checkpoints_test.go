package httpx

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/uvwt/nexusdock/internal/agentdock"
	"github.com/uvwt/nexusdock/internal/core"
)

type httpAuditTrustRegistryPayload struct {
	Contract      string                      `json:"contract"`
	Revision      int                         `json:"revision"`
	Subject       agentdock.AuditTrustSubject `json:"subject"`
	RootKeyID     string                      `json:"root_key_id"`
	RootPublicKey string                      `json:"root_public_key"`
	Keys          []agentdock.AuditTrustKey   `json:"keys"`
	UpdatedAt     string                      `json:"updated_at"`
}

func TestAuditTrustCheckpointRouterEnforcesNodeWriterAndSharedReader(t *testing.T) {
	server := newNodeTestServer(t)
	handler := server.Handler()

	nodeA, tokenA := pairAuditTrustHTTPNode(t, server, "device_trust_http_a", "Trust A")
	_, tokenB := pairAuditTrustHTTPNode(t, server, "device_trust_http_b", "Trust B")
	input := makeAuditTrustHTTPFixture(t, nodeA, time.Now().UTC())
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}

	publish := httptest.NewRequest(http.MethodPut, "/v1/nodes/trust-checkpoint", bytes.NewReader(body))
	publish.Header.Set("Authorization", "Bearer "+tokenA)
	publish.Header.Set("Content-Type", "application/json")
	published := httptest.NewRecorder()
	handler.ServeHTTP(published, publish)
	if published.Code != http.StatusOK || !strings.Contains(published.Body.String(), `"ok":true`) {
		t.Fatalf("publish status=%d body=%s", published.Code, published.Body.String())
	}

	impersonate := httptest.NewRequest(http.MethodPut, "/v1/nodes/trust-checkpoint", bytes.NewReader(body))
	impersonate.Header.Set("Authorization", "Bearer "+tokenB)
	impersonate.Header.Set("Content-Type", "application/json")
	impersonated := httptest.NewRecorder()
	handler.ServeHTTP(impersonated, impersonate)
	if impersonated.Code != http.StatusBadRequest || !strings.Contains(impersonated.Body.String(), "AGENTDOCK_TRUST_CHECKPOINT_INVALID") {
		t.Fatalf("impersonation status=%d body=%s", impersonated.Code, impersonated.Body.String())
	}

	anonymous := httptest.NewRequest(http.MethodGet, "/v1/trust-checkpoints/"+nodeA.ID, nil)
	anonymousResponse := httptest.NewRecorder()
	handler.ServeHTTP(anonymousResponse, anonymous)
	if anonymousResponse.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous read status=%d body=%s", anonymousResponse.Code, anonymousResponse.Body.String())
	}

	read := httptest.NewRequest(http.MethodGet, "/v1/trust-checkpoints/"+nodeA.ID, nil)
	read.Header.Set("Authorization", "Bearer "+tokenB)
	readResponse := httptest.NewRecorder()
	handler.ServeHTTP(readResponse, read)
	if readResponse.Code != http.StatusOK || !strings.Contains(readResponse.Body.String(), nodeA.ID) {
		t.Fatalf("shared read status=%d body=%s", readResponse.Code, readResponse.Body.String())
	}
}

func pairAuditTrustHTTPNode(t *testing.T, server *Server, deviceID, name string) (agentdock.Node, string) {
	t.Helper()
	pairing, err := server.agentDock.CreatePairingCode(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	node, err := server.agentDock.Pair(t.Context(), agentdock.PairInput{Code: pairing.Code, DeviceID: deviceID, Name: name})
	if err != nil {
		t.Fatal(err)
	}
	issued, err := server.access.auth.IssueToken(
		t.Context(),
		core.Actor{Type: core.ActorDevice, ID: node.ID},
		"device_token",
		nil,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	return node, issued.Token
}

func makeAuditTrustHTTPFixture(t *testing.T, node agentdock.Node, now time.Time) agentdock.AuditTrustCheckpointPublishInput {
	t.Helper()
	rootPublic, rootPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyID := func(prefix string, publicKey ed25519.PublicKey) string {
		sum := sha256.Sum256(publicKey)
		return prefix + ":" + hex.EncodeToString(sum[:])
	}
	registry := agentdock.AuditTrustRegistry{
		Contract:      agentdock.AuditTrustRegistryContract,
		Revision:      1,
		Subject:       agentdock.AuditTrustSubject{NodeID: node.ID, DeviceID: node.DeviceID},
		RootKeyID:     keyID("audit-root-sha256", rootPublic),
		RootPublicKey: base64.RawStdEncoding.EncodeToString(rootPublic),
		Keys: []agentdock.AuditTrustKey{{
			KeyID: keyID("audit-key-sha256", signPublic), Algorithm: agentdock.AuditTrustAlgorithm,
			PublicKey: base64.RawStdEncoding.EncodeToString(signPublic), Status: "active",
			ValidFrom: now.Add(-time.Minute).Format(time.RFC3339Nano),
		}},
		UpdatedAt: now.Add(-time.Minute).Format(time.RFC3339Nano),
	}
	registryPayload, err := json.Marshal(httpAuditTrustRegistryPayload{
		Contract: registry.Contract, Revision: registry.Revision, Subject: registry.Subject,
		RootKeyID: registry.RootKeyID, RootPublicKey: registry.RootPublicKey,
		Keys: registry.Keys, UpdatedAt: registry.UpdatedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	registry.Signature = base64.RawStdEncoding.EncodeToString(ed25519.Sign(rootPrivate, registryPayload))
	registryBytes, err := json.Marshal(registry)
	if err != nil {
		t.Fatal(err)
	}
	registrySum := sha256.Sum256(registryBytes)

	checkpoint := agentdock.AuditTrustCheckpoint{
		Contract:  agentdock.AuditTrustCheckpointContract,
		Algorithm: agentdock.AuditTrustAlgorithm,
		Statement: agentdock.AuditTrustCheckpointStatementV1{
			Contract: agentdock.AuditTrustCheckpointStatement,
			NodeID:   node.ID, DeviceID: node.DeviceID, RootKeyID: registry.RootKeyID,
			RegistryRevision: 1, RegistrySHA256: hex.EncodeToString(registrySum[:]),
			IssuedAt: now.Format(time.RFC3339Nano), FreshUntil: now.Add(6 * time.Hour).Format(time.RFC3339Nano),
		},
	}
	checkpointPayload, err := json.Marshal(checkpoint.Statement)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint.Signature = base64.RawStdEncoding.EncodeToString(ed25519.Sign(rootPrivate, checkpointPayload))
	return agentdock.AuditTrustCheckpointPublishInput{Checkpoint: checkpoint, Registry: registry}
}
