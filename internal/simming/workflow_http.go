package simming

import (
	"errors"
	"net/http"
)

func (s *Service) workflowHTTP(w http.ResponseWriter, r *http.Request) {
	var value any
	var err error
	switch {
	case r.URL.Path == "/api/sim/workflow" && r.Method == "GET":
		value, err = s.Workspace.Workflow(r.URL.Query().Get("snapshotID"), r.URL.Query().Get("context"))
	case r.URL.Path == "/api/sim/workflow" && r.Method == "POST":
		var request struct {
			SnapshotID string        `json:"snapshotID"`
			Settings   WorkflowDraft `json:"settings"`
		}
		if err = decodeRequest(w, r, &request); err == nil {
			value, err = s.Workspace.SaveWorkflow(request.SnapshotID, request.Settings)
		}
	case r.URL.Path == "/api/sim/workflow/run" && r.Method == "POST":
		var request WorkflowRun
		if err = decodeRequest(w, r, &request); err == nil {
			value, err = s.Workspace.RunWorkflow(request)
		}
	case r.URL.Path == "/api/sim/decisions" && r.Method == "GET":
		value, err = s.Workspace.Decisions(r.URL.Query().Get("snapshotID"), r.URL.Query().Get("context"))
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrWorkflowConflict) {
			status = http.StatusConflict
		}
		failure(w, status, err)
		return
	}
	jsonResponse(w, value)
}
