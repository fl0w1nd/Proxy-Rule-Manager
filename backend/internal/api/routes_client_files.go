package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/store"
)

func (s *Server) registerClientFileRoutes(r chi.Router) {
	// Public listing first (no admin auth required).
	r.Get("/client-files/public", s.handleListPublicClientFiles)

	r.Get("/clients/{clientId}/files", s.adminGuard(s.handleListClientFiles))
	r.Post("/clients/{clientId}/files", s.adminGuard(s.handleCreateClientFile))
	r.Get("/clients/{clientId}/files/{fileId}", s.adminGuard(s.handleGetClientFile))
	r.Put("/clients/{clientId}/files/{fileId}", s.adminGuard(s.handleUpdateClientFile))
	r.Delete("/clients/{clientId}/files/{fileId}", s.adminGuard(s.handleDeleteClientFile))
}

func (s *Server) handleListClientFiles(w http.ResponseWriter, r *http.Request) {
	clientID, _ := url.PathUnescape(chi.URLParam(r, "clientId"))
	files, err := s.Store.ListClientFiles(r.Context(), clientID)
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSONList(w, "files", files)
}

func (s *Server) handleListPublicClientFiles(w http.ResponseWriter, r *http.Request) {
	files, err := s.Store.ListPublicClientFiles(r.Context())
	if err != nil {
		s.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.JSONList(w, "files", files)
}

type clientFilePayload struct {
	ConfigID    string  `json:"configId"`
	DisplayName string  `json:"displayName"`
	Description *string `json:"description"`
	Ext         string  `json:"ext"`
	IsPublic    *bool   `json:"isPublic"`
	Content     *string `json:"content"`
}

func (s *Server) handleCreateClientFile(w http.ResponseWriter, r *http.Request) {
	clientID, _ := url.PathUnescape(chi.URLParam(r, "clientId"))
	var payload clientFilePayload
	if err := s.DecodeJSON(r, &payload); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if payload.ConfigID == "" {
		s.Error(w, http.StatusBadRequest, "configId is required")
		return
	}
	if strings.TrimSpace(payload.DisplayName) == "" {
		s.ErrorWithCode(w, http.StatusBadRequest, "VALIDATION_ERROR", "displayName is required")
		return
	}
	if err := schema.ValidateConfigID(payload.ConfigID); err != nil {
		s.ErrorWithCode(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	ext, err := validateClientFileExt(payload.Ext)
	if err != nil {
		s.ErrorWithCode(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	input := store.ClientFileInput{
		ConfigID:    payload.ConfigID,
		DisplayName: payload.DisplayName,
		Description: payload.Description,
		Ext:         ext,
		IsPublic:    payload.IsPublic != nil && *payload.IsPublic,
	}
	if payload.Content != nil {
		input.Content = *payload.Content
		input.ContentSet = true
	}
	meta, err := s.Store.CreateClientFile(r.Context(), clientID, input)
	if err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{"success": true, "file": meta})
}

func (s *Server) handleGetClientFile(w http.ResponseWriter, r *http.Request) {
	clientID, _ := url.PathUnescape(chi.URLParam(r, "clientId"))
	fileID, _ := url.PathUnescape(chi.URLParam(r, "fileId"))

	meta, err := s.Store.GetClientFileMeta(r.Context(), fileID)
	if err != nil {
		s.Error(w, http.StatusNotFound, "Client file not found")
		return
	}
	// Ownership check: use 404 to avoid leaking existence of other clients' files.
	if meta.ClientID != clientID {
		s.Error(w, http.StatusNotFound, "Client file not found")
		return
	}

	content, err := s.Store.GetClientFileContent(r.Context(), fileID)
	if err != nil {
		// Content file may be missing from disk; return empty string like TS `content ?? ""`.
		content = ""
	}
	s.JSON(w, http.StatusOK, map[string]any{"file": meta, "content": content})
}

func (s *Server) handleUpdateClientFile(w http.ResponseWriter, r *http.Request) {
	clientID, _ := url.PathUnescape(chi.URLParam(r, "clientId"))
	fileID, _ := url.PathUnescape(chi.URLParam(r, "fileId"))

	// Ownership check before applying any updates.
	existing, err := s.Store.GetClientFileMeta(r.Context(), fileID)
	if err != nil {
		s.Error(w, http.StatusNotFound, "Client file not found")
		return
	}
	if existing.ClientID != clientID {
		s.Error(w, http.StatusNotFound, "Client file not found")
		return
	}

	var raw map[string]json.RawMessage
	if err := s.DecodeJSON(r, &raw); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	input := store.ClientFileInput{}
	if v, ok := raw["configId"]; ok {
		var cid string
		_ = json.Unmarshal(v, &cid)
		if cid != "" {
			if err := schema.ValidateConfigID(cid); err != nil {
				s.ErrorWithCode(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
				return
			}
		}
		input.ConfigID = cid
	}
	if v, ok := raw["displayName"]; ok {
		_ = json.Unmarshal(v, &input.DisplayName)
	}
	if v, ok := raw["description"]; ok {
		if string(v) == "null" {
			none := ""
			input.Description = &none
		} else {
			var desc string
			_ = json.Unmarshal(v, &desc)
			input.Description = &desc
		}
	}
	if v, ok := raw["ext"]; ok {
		_ = json.Unmarshal(v, &input.Ext)
		normalized, err := validateClientFileExt(input.Ext)
		if err != nil {
			s.ErrorWithCode(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		input.Ext = normalized
	}
	if v, ok := raw["isPublic"]; ok {
		var b bool
		_ = json.Unmarshal(v, &b)
		input.IsPublic = b
		input.IsPublicSet = true
	}
	if v, ok := raw["content"]; ok {
		var c string
		_ = json.Unmarshal(v, &c)
		input.Content = c
		input.ContentSet = true
	}
	meta, err := s.Store.UpdateClientFile(r.Context(), fileID, input)
	if err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{"success": true, "file": meta})
}

func (s *Server) handleDeleteClientFile(w http.ResponseWriter, r *http.Request) {
	clientID, _ := url.PathUnescape(chi.URLParam(r, "clientId"))
	fileID, _ := url.PathUnescape(chi.URLParam(r, "fileId"))

	// Ownership check: fetch meta first to verify ownership and return 404 on mismatch.
	meta, err := s.Store.GetClientFileMeta(r.Context(), fileID)
	if err != nil {
		s.Error(w, http.StatusNotFound, "Client file not found")
		return
	}
	if meta.ClientID != clientID {
		s.Error(w, http.StatusNotFound, "Client file not found")
		return
	}

	if err := s.Store.DeleteClientFile(r.Context(), fileID); err != nil {
		s.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	s.JSON(w, http.StatusOK, map[string]any{"success": true, "deletedFile": fileID})
}
