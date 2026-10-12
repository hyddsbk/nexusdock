package httpx

import (
	"errors"
	"net/http"

	"github.com/uvwt/nexusdock/internal/agentdock"
	"github.com/uvwt/nexusdock/internal/core"
)

func (s *Server) agentDockTrustCheckpointPublish(w http.ResponseWriter, r *http.Request) {
	if s.agentDock == nil || !s.access.configured() {
		writeError(w, http.StatusServiceUnavailable, "AGENTDOCK_TRUST_CHECKPOINT_UNAVAILABLE", "AgentDock trust checkpoint service is unavailable")
		return
	}
	nodeID, err := s.access.authenticateDeviceToken(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		switch core.ErrorCodeOf(err) {
		case core.CodeAuthRequired, core.CodeInvalidToken, core.CodeTokenRevoked:
			writeError(w, http.StatusUnauthorized, "INVALID_DEVICE_TOKEN", "AgentDock Device Token is invalid")
		default:
			writeError(w, http.StatusInternalServerError, "AGENTDOCK_DEVICE_AUTH_FAILED", "Unable to authenticate AgentDock Device Token")
		}
		return
	}
	var input agentdock.AuditTrustCheckpointPublishInput
	if !decodeJSON(w, r, &input) {
		return
	}
	record, err := s.agentDock.PublishAuditTrustCheckpoint(r.Context(), nodeID, input)
	if err != nil {
		writeAgentDockTrustCheckpointError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "trust_checkpoint": record})
}

func (s *Server) agentDockTrustCheckpointGet(w http.ResponseWriter, r *http.Request) {
	if s.agentDock == nil {
		writeError(w, http.StatusServiceUnavailable, "AGENTDOCK_TRUST_CHECKPOINT_UNAVAILABLE", "AgentDock trust checkpoint service is unavailable")
		return
	}
	record, err := s.agentDock.AuditTrustCheckpoint(r.Context(), r.PathValue("nodeID"))
	if err != nil {
		if errors.Is(err, agentdock.ErrTrustCheckpointNotFound) {
			writeError(w, http.StatusNotFound, "AGENTDOCK_TRUST_CHECKPOINT_NOT_FOUND", "AgentDock trust checkpoint was not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "AGENTDOCK_TRUST_CHECKPOINT_READ_FAILED", "Unable to read AgentDock trust checkpoint")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "trust_checkpoint": record})
}

func writeAgentDockTrustCheckpointError(w http.ResponseWriter, err error) {
	var validation agentdock.ValidationError
	switch {
	case errors.As(err, &validation):
		writeError(w, http.StatusBadRequest, "AGENTDOCK_TRUST_CHECKPOINT_INVALID", validation.Error())
	case errors.Is(err, agentdock.ErrNodeNotFound):
		writeError(w, http.StatusNotFound, "AGENTDOCK_NODE_NOT_FOUND", "AgentDock node was not found")
	case errors.Is(err, agentdock.ErrNodeDisabled):
		writeError(w, http.StatusForbidden, "AGENTDOCK_NODE_DISABLED", "AgentDock node is disabled")
	case errors.Is(err, agentdock.ErrTrustCheckpointRollback):
		writeError(w, http.StatusConflict, "AGENTDOCK_TRUST_CHECKPOINT_ROLLBACK", "Trust checkpoint would roll back the latest known revision")
	case errors.Is(err, agentdock.ErrTrustCheckpointEquivocation):
		writeError(w, http.StatusConflict, "AGENTDOCK_TRUST_CHECKPOINT_EQUIVOCATION", "Trust checkpoint conflicts with the registry already recorded for this revision")
	case errors.Is(err, agentdock.ErrTrustRootChanged):
		writeError(w, http.StatusConflict, "AGENTDOCK_TRUST_ROOT_CHANGED", "Trust root changes require explicit rebootstrap")
	default:
		writeError(w, http.StatusInternalServerError, "AGENTDOCK_TRUST_CHECKPOINT_WRITE_FAILED", "Unable to store AgentDock trust checkpoint")
	}
}
