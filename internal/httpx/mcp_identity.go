package httpx

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const nexusMCPBaseVersion = "1"

type mcpToolIdentity struct {
	Name         string                  `json:"name"`
	Title        string                  `json:"title,omitempty"`
	Description  string                  `json:"description,omitempty"`
	InputSchema  any                     `json:"inputSchema,omitempty"`
	OutputSchema any                     `json:"outputSchema,omitempty"`
	Annotations  *mcpsdk.ToolAnnotations `json:"annotations,omitempty"`
	Meta         mcpsdk.Meta             `json:"_meta,omitempty"`
}

func contractAwareMCPVersion(baseVersion, contractHash string) string {
	baseVersion = strings.TrimSpace(baseVersion)
	if baseVersion == "" {
		baseVersion = "unknown"
	}
	digest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(contractHash), "sha256:"))
	if len(digest) < 12 {
		return baseVersion
	}
	digest = digest[:12]
	if strings.Contains(baseVersion, "+") {
		return baseVersion + ".tools." + digest
	}
	return baseVersion + "+tools." + digest
}

func mcpToolContractHash(tools []*mcpsdk.Tool) string {
	identities := make([]mcpToolIdentity, 0, len(tools))
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		identities = append(identities, mcpToolIdentity{
			Name: tool.Name, Title: tool.Title, Description: tool.Description,
			InputSchema: tool.InputSchema, OutputSchema: tool.OutputSchema,
			Annotations: tool.Annotations, Meta: mcpIdentityMeta(tool.Meta),
		})
	}
	sort.Slice(identities, func(i, j int) bool { return identities[i].Name < identities[j].Name })
	encoded, err := json.Marshal(identities)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(encoded))
}

func mcpIdentityMeta(meta mcpsdk.Meta) mcpsdk.Meta {
	if len(meta) == 0 {
		return nil
	}
	filtered := make(mcpsdk.Meta, len(meta))
	for key, value := range meta {
		if key == "ui" {
			continue
		}
		filtered[key] = value
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func (s *Server) publicMCPTools() []*mcpsdk.Tool {
	if s == nil {
		return nil
	}
	enabled := s.mcpAppsEnabled()
	tools := append([]*mcpsdk.Tool(nil), nexusToolDefinitionsWithApps(enabled)...)
	if s.publishedToolBridge != nil {
		for _, published := range s.publishedToolBridge.PublishedTools() {
			tools = append(tools, nodeMCPToolWithApps(published.Descriptor, enabled))
		}
	}
	return tools
}

func (s *Server) desiredMCPVersion() string {
	return contractAwareMCPVersion(nexusMCPBaseVersion, mcpToolContractHash(s.publicMCPTools()))
}
