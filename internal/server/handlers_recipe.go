package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/danieljustus/symaira-vibecoder/internal/recipe"
)

// recipeRun handles POST /api/recipe/run. It accepts a RecipeRequest, runs the
// recipe synchronously through the recipe service, and returns a RecipeResult.
func (s *Server) recipeRun(w http.ResponseWriter, r *http.Request) {
	if !s.requireRunnable(w, r) {
		return
	}

	var req recipe.RecipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	svc := recipe.NewService(s.eng.Runner())
	result, err := svc.Run(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, recipe.ErrInvalidSchema),
			errors.Is(err, recipe.ErrMissingPrompt),
			errors.Is(err, recipe.ErrMissingWorkspace):
			writeErr(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, recipe.ErrBackendUnavailable):
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"error": err.Error(),
			})
		default:
			writeErr(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeOK(w, result)
}
