package api

import (
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
)

func (s *Server) registerClientRoutes(r chi.Router) {
	r.Get("/clients", s.adminGuard(s.handleListClients))
	r.Post("/clients", s.adminGuard(s.handleCreateClient))
	r.Put("/clients/{id}", s.adminGuard(s.handleUpdateClient))
	r.Delete("/clients/{id}", s.adminGuard(s.handleDeleteClient))
}

func (s *Server) handleListClients(w http.ResponseWriter, r *http.Request) {
	clients, err := s.Store.GetClients(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if clients == nil {
		clients = []schema.ClientConfig{}
	}
	s.JSON(w, http.StatusOK, map[string]any{"clients": clients})
}

func (s *Server) handleCreateClient(w http.ResponseWriter, r *http.Request) {
	var payload schema.ClientConfig
	if err := s.DecodeJSON(r, &payload); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if payload.ID == "" || payload.DisplayName == "" {
		s.Error(w, http.StatusBadRequest, "id and displayName are required")
		return
	}
	if err := s.validateClientTransformsAgainstConfig(r, payload.Transforms); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.Store.AddClient(r.Context(), payload); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{"success": true, "client": payload})
}

func (s *Server) handleUpdateClient(w http.ResponseWriter, r *http.Request) {
	id, _ := url.PathUnescape(chi.URLParam(r, "id"))
	var payload schema.ClientConfig
	if err := s.DecodeJSON(r, &payload); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.validateClientTransformsAgainstConfig(r, payload.Transforms); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.Store.UpdateClient(r.Context(), id, payload); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{"success": true})
}

// validateClientTransformsAgainstConfig pulls the persisted transformer
// map so a client save can reject `use` references that point at
// nonexistent JS transformers. Built-in references are validated against
// the in-process registry directly inside validateTransform.
func (s *Server) validateClientTransformsAgainstConfig(r *http.Request, transforms []schema.Transform) error {
	if len(transforms) == 0 {
		return nil
	}
	cfg, err := s.Store.GetConfig(r.Context())
	if err != nil {
		return err
	}
	return validateClientTransforms(transforms, cfg.Transformers)
}

func (s *Server) handleDeleteClient(w http.ResponseWriter, r *http.Request) {
	id, _ := url.PathUnescape(chi.URLParam(r, "id"))
	if err := s.Store.DeleteClient(r.Context(), id); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{"success": true, "deletedClient": id})
}
