package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	protocol "github.com/uvwt/agentdock-protocol"
	"github.com/uvwt/nexusdock/internal/agentdock"
)

func admissionWorkflowTestDescriptor() agentdock.ToolDescriptor {
	return agentdock.ToolDescriptor{
		Name: admissionWorkflowToolName,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action":        map[string]any{"type": "string", "enum": []any{"list", "match"}},
				"admission_ref": map[string]any{"type": "string"},
				"goal":          map[string]any{"type": "string"},
			},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action":                map[string]any{"type": "string"},
				"admission_ref":         map[string]any{"type": "string"},
				"admission_digest":      map[string]any{"type": "string"},
				"workflow_match_digest": map[string]any{"type": "string"},
				"recommended":           map[string]any{"type": "string"},
			},
		},
	}
}

func TestAdmissionBoundWorkflowMatchRoutesToSelectedNode(t *testing.T) {
	store := newHTTPTestAgentDockStore(t)
	descriptor := admissionWorkflowTestDescriptor()
	node := pairHTTPTestNode(t, store, "device_workflow_admission", "DockAdmission", "0.8.3", descriptor)
	server := newGatewayTestServer(t, store)
	server.agentDockHub.SetHelloHandler(server.registerNodeTools)
	hub := server.agentDockHub

	connected := make(chan struct{})
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := hub.Accept(w, r, node.ID, ""); err != nil {
			t.Errorf("accept workflow node: %v", err)
			return
		}
		close(connected)
	}))
	t.Cleanup(bridge.Close)

	socket, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(bridge.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = socket.Close() })
	if err := socket.WriteJSON(protocol.Message{
		Type: protocol.MessageNodeHello, ProtocolVersion: agentdock.ConnectionProtocolVersion,
		Hello: &protocol.Hello{
			DeviceID: node.DeviceID, Version: node.Version, ProtocolVersion: agentdock.ConnectionProtocolVersion,
			Capabilities: []string{descriptor.Name},
			Tools:        []protocol.ToolDescriptor{descriptor},
			UIResources:  []protocol.UIResourceCapability{},
		},
	}); err != nil {
		t.Fatal(err)
	}
	var ready protocol.Message
	if err := socket.ReadJSON(&ready); err != nil || ready.Type != protocol.MessageNodeReady {
		t.Fatalf("ready=%#v err=%v", ready, err)
	}
	<-connected

	const admissionRef = "adm_live_contract"
	wantStructured := map[string]any{
		"action":                "match",
		"admission_ref":         admissionRef,
		"admission_digest":      "sha256:admission",
		"workflow_match_digest": "sha256:workflow",
		"recommended":           "plain_task",
	}
	serveDone := make(chan error, 1)
	go func() {
		var invoke protocol.Message
		if err := socket.ReadJSON(&invoke); err != nil {
			serveDone <- err
			return
		}
		if invoke.Operation != protocol.OperationToolCall {
			serveDone <- &unexpectedOperationError{got: invoke.Operation}
			return
		}
		var call struct {
			Tool      string         `json:"tool"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(invoke.Arguments, &call); err != nil {
			serveDone <- err
			return
		}
		wantArguments := map[string]any{
			"action": "match", "admission_ref": admissionRef, "goal": "admission-bound development",
		}
		if call.Tool != admissionWorkflowToolName || !reflect.DeepEqual(call.Arguments, wantArguments) {
			serveDone <- &workspaceContextInvokeError{tool: call.Tool, arguments: call.Arguments}
			return
		}
		result, err := json.Marshal(map[string]any{
			"isError": false, "structuredContent": wantStructured,
			"content": []map[string]any{{"type": "text", "text": "matched"}},
		})
		if err != nil {
			serveDone <- err
			return
		}
		serveDone <- socket.WriteJSON(protocol.Message{
			Type: protocol.MessageToolResult, RequestID: invoke.RequestID, Result: result,
		})
	}()

	got, err := callWorkflowTemplateManageForTest(t, server, t.Context(), map[string]any{
		"action": "match", "node_id": node.ID, "admission_ref": admissionRef,
		"goal": "admission-bound development",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, wantStructured) {
		t.Fatalf("workflow proxy result=%#v want=%#v", got, wantStructured)
	}
	assertCentralToolResultMatchesOutputSchema(t, admissionWorkflowToolName, got)
	if err := <-serveDone; err != nil {
		t.Fatal(err)
	}
}

func TestAdmissionBoundWorkflowMatchRejectsLegacyNodeContract(t *testing.T) {
	descriptor := admissionWorkflowTestDescriptor()
	delete(descriptor.InputSchema["properties"].(map[string]any), "admission_ref")
	if err := validateAdmissionWorkflowDescriptor(descriptor); err == nil || !strings.Contains(err.Error(), "admission_ref") {
		t.Fatalf("legacy workflow descriptor err=%v", err)
	}
}

func TestDecodeAdmissionWorkflowMatchResultRejectsCrossAdmissionReceipt(t *testing.T) {
	_, err := decodeAdmissionWorkflowMatchResult(map[string]any{
		"isError": false,
		"structuredContent": map[string]any{
			"admission_ref": "adm_other", "admission_digest": "sha256:a", "workflow_match_digest": "sha256:w",
		},
	}, "adm_expected")
	if err == nil || !strings.Contains(err.Error(), "want") {
		t.Fatalf("cross-admission workflow result err=%v", err)
	}
}
