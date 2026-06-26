package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
	"github.com/danieljustus/symaira-vibecoder/internal/engine"
	"github.com/danieljustus/symaira-vibecoder/internal/runner"
)

// loadCycle returns the active (default) cycle from disk, materializing the seed
// on first run.
func (s *Server) loadCycle() (*config.Cycle, error) {
	return config.LoadCycle(s.cfg.Defaults.Cycle)
}

// persist saves the cycle and notifies boards to refetch.
func (s *Server) persist(w http.ResponseWriter, c *config.Cycle) bool {
	c.Reindex()
	if err := c.Validate(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return false
	}
	if err := config.SaveCycle(c); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return false
	}
	s.eng.Bus().Publish(engine.Event{Type: "board"})
	return true
}

func (s *Server) getCycle(w http.ResponseWriter, r *http.Request) {
	c, err := s.loadCycle()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, c)
}

// putCycle replaces the whole cycle (the simplest robust edit path: the board
// PUTs its full edited model).
func (s *Server) putCycle(w http.ResponseWriter, r *http.Request) {
	if s.busy() {
		writeErr(w, http.StatusConflict, "a run is in progress")
		return
	}
	var c config.Cycle
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid cycle json: "+err.Error())
		return
	}
	if c.ID == "" {
		c.ID = s.cfg.Defaults.Cycle
	}
	if !s.persist(w, &c) {
		return
	}
	writeOK(w, &c)
}

func (s *Server) addStep(w http.ResponseWriter, r *http.Request) {
	if s.busy() {
		writeErr(w, http.StatusConflict, "a run is in progress")
		return
	}
	var body struct {
		PhaseID string      `json:"phase_id"`
		Step    config.Step `json:"step"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	c, err := s.loadCycle()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, err := c.AddStep(body.PhaseID, body.Step)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.persist(w, c) {
		return
	}
	writeOK(w, map[string]string{"id": id})
}

func (s *Server) deleteStep(w http.ResponseWriter, r *http.Request) {
	if s.busy() {
		writeErr(w, http.StatusConflict, "a run is in progress")
		return
	}
	c, err := s.loadCycle()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !c.DeleteStep(r.PathValue("id")) {
		writeErr(w, http.StatusNotFound, "step not found")
		return
	}
	if !s.persist(w, c) {
		return
	}
	writeOK(w, map[string]bool{"ok": true})
}

func (s *Server) moveStep(w http.ResponseWriter, r *http.Request) {
	if s.busy() {
		writeErr(w, http.StatusConflict, "a run is in progress")
		return
	}
	var body struct {
		ToPhaseID string `json:"to_phase_id"`
		ToIndex   int    `json:"to_index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	c, err := s.loadCycle()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := c.MoveStep(r.PathValue("id"), body.ToPhaseID, body.ToIndex); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.persist(w, c) {
		return
	}
	writeOK(w, map[string]bool{"ok": true})
}

func (s *Server) duplicateStep(w http.ResponseWriter, r *http.Request) {
	if s.busy() {
		writeErr(w, http.StatusConflict, "a run is in progress")
		return
	}
	c, err := s.loadCycle()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, err := c.DuplicateStep(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	if !s.persist(w, c) {
		return
	}
	writeOK(w, map[string]string{"id": id})
}

func (s *Server) addPhase(w http.ResponseWriter, r *http.Request) {
	if s.busy() {
		writeErr(w, http.StatusConflict, "a run is in progress")
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Name == "" {
		body.Name = "New Phase"
	}
	c, err := s.loadCycle()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	id := c.AddPhase(body.Name)
	if !s.persist(w, c) {
		return
	}
	writeOK(w, map[string]string{"id": id})
}

func (s *Server) deletePhase(w http.ResponseWriter, r *http.Request) {
	if s.busy() {
		writeErr(w, http.StatusConflict, "a run is in progress")
		return
	}
	c, err := s.loadCycle()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !c.DeletePhase(r.PathValue("id")) {
		writeErr(w, http.StatusNotFound, "phase not found")
		return
	}
	if !s.persist(w, c) {
		return
	}
	writeOK(w, map[string]bool{"ok": true})
}

func (s *Server) exportCycle(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		id = s.cfg.Defaults.Cycle
	}
	c, err := config.LoadCycle(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "cycle not found: "+err.Error())
		return
	}
	manifest := config.TemplateManifest{
		ID:          c.ID,
		Name:        c.Name,
		Description: c.Description,
		Version:     "1.0.0",
		Author:      "symvibe",
		Tags:        []string{},
	}
	t := c.ExportTemplate(manifest)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="`+id+`.json"`)
	if err := json.NewEncoder(w).Encode(t); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
	}
}

func (s *Server) importCycle(w http.ResponseWriter, r *http.Request) {
	if s.busy() {
		writeErr(w, http.StatusConflict, "a run is in progress")
		return
	}
	var req struct {
		Template *config.Template  `json:"template"`
		Remap    map[string]string `json:"remap"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Template == nil {
		writeErr(w, http.StatusBadRequest, "missing template field")
		return
	}

	// Build catalog
	skills, _ := config.DiscoverSkills()
	var skillNames []string
	for _, sk := range skills {
		skillNames = append(skillNames, sk.Name)
	}

	var categoryNames []string
	for catName := range s.cfg.Categories {
		categoryNames = append(categoryNames, catName)
	}

	agents, _ := config.DiscoverAgents(s.cfg.Runner.OpencodeBin)
	var agentNames []string
	for _, ag := range agents {
		agentNames = append(agentNames, ag.Name)
	}
	hasDefaultAgent := false
	for _, name := range agentNames {
		if strings.EqualFold(name, s.cfg.Defaults.Agent) {
			hasDefaultAgent = true
			break
		}
	}
	if !hasDefaultAgent && s.cfg.Defaults.Agent != "" {
		agentNames = append(agentNames, s.cfg.Defaults.Agent)
	}

	sensorNames := engine.SensorNames()

	cat := config.Catalog{
		Skills:     skillNames,
		Categories: categoryNames,
		Agents:     agentNames,
		Sensors:    sensorNames,
	}

	// Apply remapping
	req.Template.ApplyRemap(req.Remap)

	// Check requirements
	missing := config.CheckRequirements(req.Template, cat)
	if !missing.Empty() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":     "missing requirements",
			"missing":   missing,
			"available": cat,
		})
		return
	}

	// Import the cycle
	c, err := config.ImportTemplateStruct(req.Template)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	// Persist
	if !s.persist(w, c) {
		return
	}
	writeOK(w, c)
}

func (s *Server) assistCycle(w http.ResponseWriter, r *http.Request) {
	if s.busy() {
		writeErr(w, http.StatusConflict, "a run is in progress")
		return
	}
	var req struct {
		Cycle       *config.Cycle `json:"cycle"`
		Instruction string        `json:"instruction"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Cycle == nil {
		writeErr(w, http.StatusBadRequest, "missing cycle field")
		return
	}
	if strings.TrimSpace(req.Instruction) == "" {
		writeErr(w, http.StatusBadRequest, "missing instruction field")
		return
	}

	// Build catalog
	skills, _ := config.DiscoverSkills()
	var skillNames []string
	for _, sk := range skills {
		skillNames = append(skillNames, sk.Name)
	}

	var categoryNames []string
	for catName := range s.cfg.Categories {
		categoryNames = append(categoryNames, catName)
	}

	agents, _ := config.DiscoverAgents(s.cfg.Runner.OpencodeBin)
	var agentNames []string
	for _, ag := range agents {
		agentNames = append(agentNames, ag.Name)
	}
	hasDefaultAgent := false
	for _, name := range agentNames {
		if strings.EqualFold(name, s.cfg.Defaults.Agent) {
			hasDefaultAgent = true
			break
		}
	}
	if !hasDefaultAgent && s.cfg.Defaults.Agent != "" {
		agentNames = append(agentNames, s.cfg.Defaults.Agent)
	}

	sensorNames := engine.SensorNames()

	cat := config.Catalog{
		Skills:     skillNames,
		Categories: categoryNames,
		Agents:     agentNames,
		Sensors:    sensorNames,
	}

	// Marshal catalog and cycle to JSON for the prompt
	cycleBytes, err := json.MarshalIndent(req.Cycle, "", "  ")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "marshal cycle: "+err.Error())
		return
	}
	catBytes, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "marshal catalog: "+err.Error())
		return
	}

	prompt := fmt.Sprintf("You are a Baukasten Cycle configuration assistant.\n"+
		"Your task is to modify a given Cycle according to the user's instruction.\n\n"+
		"Available catalog of building blocks (skills, categories, agents, and sensors):\n%s\n\n"+
		"Current Cycle JSON:\n%s\n\n"+
		"Instruction:\n%s\n\n"+
		"Return ONLY the complete modified Cycle JSON. The output must match the Baukasten Cycle schema, "+
		"and its kind MUST be \"symvibe.template\" if returning a template wrapper, or a valid Cycle JSON.\n"+
		"Do not include any prose, explanations, markdown formatting (such as ```json), or additional fields.",
		string(catBytes), string(cycleBytes), req.Instruction)

	// Invoke runner
	runnerInstance := s.eng.Runner()
	if runnerInstance == nil {
		writeErr(w, http.StatusServiceUnavailable, "no coding agent runner configured")
		return
	}

	// We prepare a StepRequest
	stepReq := runner.StepRequest{
		RunID:      "assist",
		StepID:     "assist",
		Message:    prompt,
		WorkingDir: s.cfg.Runner.WorkingDir,
		SkipPerms:  true,
	}

	// Run step and gather the logs
	ctx := r.Context()
	ch, err := runnerInstance.RunStep(ctx, stepReq)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "runner failed to start: "+err.Error())
		return
	}

	var sb strings.Builder
	var doneErr string
	for ev := range ch {
		if ev.Kind == runner.EventDone {
			doneErr = ev.Err
			continue
		}
		if ev.Kind == runner.EventLog {
			sb.WriteString(ev.Text)
			sb.WriteString("\n")
		}
	}

	if doneErr != "" {
		writeErr(w, http.StatusInternalServerError, "assist run failed: "+doneErr)
		return
	}

	// Parse model output as Cycle or Template
	respText := strings.TrimSpace(sb.String())
	// Strip markdown blocks if the model wrapped it in ```json ... ```
	if strings.HasPrefix(respText, "```") {
		if idx := strings.Index(respText, "\n"); idx != -1 {
			respText = respText[idx+1:]
		}
		respText = strings.TrimSuffix(respText, "```")
		respText = strings.TrimSpace(respText)
	}

	// Unmarshal. Try Template first, if it fails try Cycle
	var tpl config.Template
	var c *config.Cycle
	if err := json.Unmarshal([]byte(respText), &tpl); err == nil && tpl.Kind == "symvibe.template" {
		c, err = config.ImportTemplateStruct(&tpl)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid template structure: "+err.Error())
			return
		}
	} else {
		var directCycle config.Cycle
		if err := json.Unmarshal([]byte(respText), &directCycle); err != nil {
			writeErr(w, http.StatusBadRequest, "failed to parse model output as JSON Cycle: "+err.Error()+"\nRaw output:\n"+respText)
			return
		}
		c = &directCycle
	}

	// Validate the imported cycle
	if err := c.Validate(); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid cycle structure: "+err.Error())
		return
	}

	// Run capability check (requirements validation)
	manifest := config.TemplateManifest{
		ID:          c.ID,
		Name:        c.Name,
		Description: c.Description,
		Version:     "1.0.0",
		Author:      "symvibe-assist",
	}
	tplExport := c.ExportTemplate(manifest)
	missing := config.CheckRequirements(tplExport, cat)
	if !missing.Empty() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":     "missing requirements",
			"missing":   missing,
			"available": cat,
		})
		return
	}

	writeOK(w, c)
}
