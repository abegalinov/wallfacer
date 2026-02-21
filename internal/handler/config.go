package handler

import (
	"net/http"

	"changkun.de/wallfacer/internal/instructions"
)

// GetConfig returns the server configuration (workspaces, instructions path).
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"workspaces":        h.runner.Workspaces(),
		"instructions_path": instructions.FilePath(h.configDir, h.workspaces),
	})
}
