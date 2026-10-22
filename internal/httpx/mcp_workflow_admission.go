package httpx

import (
	"context"
	"errors"
	"fmt"
	"strings"

	protocol "github.com/uvwt/agentdock-protocol"
	"github.com/uvwt/nexusdock/internal/agentdock"
	"github.com/uvwt/nexusdock/internal/runtimecontrol"
)

const admissionWorkflowToolName = "workflow_template_manage"

func (s *Server) callAdmissionBoundWorkflowMatch(ctx context.Context, input workflowTemplateManageInput) (map[string]any, error) {
	nodeID := strings.TrimSpace(input.NodeID)
	admissionRef := strings.TrimSpace(input.AdmissionRef)
	if nodeID == "" {
		return nil, errors.New("node_id is required for workflow match")
	}
	if admissionRef == "" {
		return nil, errors.New("admission_ref is required for workflow match")
	}
	if s.agentDock == nil || s.agentDockHub == nil {
		return nil, errors.New("AgentDock node runtime is unavailable")
	}
	node, err := s.agentDock.Get(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if !node.Enabled {
		return nil, fmt.Errorf("AgentDock node %s is disabled", nodeID)
	}
	descriptor, err := s.admissionWorkflowDescriptor(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if err := validateAdmissionWorkflowDescriptor(descriptor); err != nil {
		return nil, fmt.Errorf("AgentDock node %s workflow contract is not Admission-capable: %w", nodeID, err)
	}

	arguments := map[string]any{
		"action":        "match",
		"admission_ref": admissionRef,
	}
	if value := strings.TrimSpace(input.Goal); value != "" {
		arguments["goal"] = value
	}
	if value := strings.TrimSpace(input.Device); value != "" {
		arguments["device"] = value
	}
	if value := strings.TrimSpace(input.Type); value != "" {
		arguments["type"] = value
	}

	if s.toolGate != nil {
		release, _, gateErr := s.toolGate.Acquire(ctx, runtimecontrol.Request{
			NodeID:       nodeID,
			ResourceKeys: nodeToolResourceKeys(nodeID, "", admissionWorkflowToolName, arguments),
		})
		if gateErr != nil {
			return nil, fmt.Errorf("Runtime tool queue: %w", gateErr)
		}
		defer release()
	}

	remote, err := s.agentDockHub.Invoke(ctx, nodeID, protocol.OperationToolCall, map[string]any{
		"tool":      admissionWorkflowToolName,
		"arguments": arguments,
	})
	if err != nil {
		return nil, err
	}
	return decodeAdmissionWorkflowMatchResult(remote, admissionRef)
}

func (s *Server) admissionWorkflowDescriptor(ctx context.Context, nodeID string) (agentdock.ToolDescriptor, error) {
	descriptors, err := s.agentDock.ToolDescriptors(ctx, nodeID)
	if err != nil {
		return agentdock.ToolDescriptor{}, err
	}
	for _, descriptor := range descriptors {
		if descriptor.Name == admissionWorkflowToolName {
			return descriptor, nil
		}
	}
	return agentdock.ToolDescriptor{}, fmt.Errorf("AgentDock node %s does not provide %s", nodeID, admissionWorkflowToolName)
}

func validateAdmissionWorkflowDescriptor(descriptor agentdock.ToolDescriptor) error {
	inputProps, _ := descriptor.InputSchema["properties"].(map[string]any)
	if inputProps == nil {
		return errors.New("input properties are missing")
	}
	if !schemaStringProperty(inputProps["admission_ref"]) {
		return errors.New("input admission_ref string property is missing")
	}
	action, _ := inputProps["action"].(map[string]any)
	if action == nil || !schemaEnumContains(action["enum"], "match") {
		return errors.New("action enum does not contain match")
	}
	outputProps, _ := descriptor.OutputSchema["properties"].(map[string]any)
	if outputProps == nil {
		return errors.New("output properties are missing")
	}
	for _, name := range []string{"admission_ref", "admission_digest", "workflow_match_digest"} {
		if !schemaStringProperty(outputProps[name]) {
			return fmt.Errorf("output %s string property is missing", name)
		}
	}
	return nil
}

func schemaStringProperty(value any) bool {
	property, ok := value.(map[string]any)
	return ok && property["type"] == "string"
}

func schemaEnumContains(value any, expected string) bool {
	switch values := value.(type) {
	case []string:
		for _, value := range values {
			if value == expected {
				return true
			}
		}
	case []any:
		for _, value := range values {
			if value == expected {
				return true
			}
		}
	}
	return false
}

func decodeAdmissionWorkflowMatchResult(result map[string]any, admissionRef string) (map[string]any, error) {
	if result == nil {
		return nil, errors.New("workflow match returned an empty AgentDock result")
	}
	structured, _ := result["structuredContent"].(map[string]any)
	if isError, _ := result["isError"].(bool); isError {
		message := "AgentDock workflow match failed"
		if structured != nil {
			if value, ok := structured["error"].(string); ok && strings.TrimSpace(value) != "" {
				message = value
			}
		}
		return nil, errors.New(message)
	}
	if structured == nil {
		return nil, errors.New("workflow match result is missing structuredContent")
	}
	if got, _ := structured["admission_ref"].(string); strings.TrimSpace(got) != admissionRef {
		return nil, fmt.Errorf("workflow match admission_ref = %q, want %q", got, admissionRef)
	}
	for _, name := range []string{"admission_digest", "workflow_match_digest"} {
		value, _ := structured[name].(string)
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("workflow match result is missing %s", name)
		}
	}
	return structured, nil
}
