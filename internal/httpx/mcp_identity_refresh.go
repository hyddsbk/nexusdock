package httpx

func (s *Server) refreshMCPServerIdentity() {
	if s == nil || s.mcp == nil {
		return
	}

	version := s.desiredMCPVersion()
	if version == "" || s.mcp.currentVersion() == version {
		return
	}

	next := newNexusMCPServer(version)
	s.registerCentralToolsOn(next)

	if s.publishedToolBridge != nil {
		enabled := s.mcpAppsEnabled()
		for _, published := range s.publishedToolBridge.PublishedTools() {
			next.AddTool(
				nodeMCPToolWithApps(published.Descriptor, enabled),
				s.nodeToolHandler(published.Descriptor.Name),
			)
		}
	}

	s.mcp.replaceServer(next, version)

	s.mcp.resourcesMu.Lock()
	s.mcp.resources = make(map[string]struct{})
	s.mcp.resourcesMu.Unlock()
}
