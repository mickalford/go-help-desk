package server

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mickalford/opsmuster/backend/internal/domain/client"
)

// ── Admin client CRUD ─────────────────────────────────────────────────────────

func (s *Server) handleListClients(w http.ResponseWriter, r *http.Request) {
	clients, err := s.clients.List(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	JSON(w, http.StatusOK, clients)
}

func (s *Server) handleCreateClient(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name   string `json:"name"`
		Domain string `json:"domain"`
		Notes  string `json:"notes"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	c, err := s.clients.Create(r.Context(), body.Name, body.Domain, body.Notes)
	if err != nil {
		if errors.Is(err, client.ErrDomainInUse) {
			Error(w, http.StatusConflict, "domain_in_use", "a client with that domain already exists")
			return
		}
		Error(w, http.StatusBadRequest, "create_failed", err.Error())
		return
	}
	JSON(w, http.StatusCreated, c)
}

func (s *Server) handleGetClient(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid_id", "invalid client id")
		return
	}
	c, err := s.clients.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			Error(w, http.StatusNotFound, "not_found", "client not found")
			return
		}
		handleError(w, err)
		return
	}
	JSON(w, http.StatusOK, c)
}

func (s *Server) handleUpdateClient(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid_id", "invalid client id")
		return
	}
	var body struct {
		Name   string `json:"name"`
		Domain string `json:"domain"`
		Notes  string `json:"notes"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	c, err := s.clients.Update(r.Context(), id, body.Name, body.Domain, body.Notes)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			Error(w, http.StatusNotFound, "not_found", "client not found")
			return
		}
		if errors.Is(err, client.ErrDomainInUse) {
			Error(w, http.StatusConflict, "domain_in_use", "a client with that domain already exists")
			return
		}
		handleError(w, err)
		return
	}
	JSON(w, http.StatusOK, c)
}

func (s *Server) handleDeleteClient(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid_id", "invalid client id")
		return
	}
	if err := s.clients.Delete(r.Context(), id); err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleResolveClientByDomain looks up or creates a client by email domain.
// Used by the email-bridge to auto-register clients on first contact.
// POST /api/v1/admin/clients/resolve  {"domain":"acme.com","name":"Acme Corp"}
func (s *Server) handleResolveClientByDomain(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Domain string `json:"domain"`
		Name   string `json:"name"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	c, err := s.clients.ResolveByDomain(r.Context(), body.Domain, body.Name)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			Error(w, http.StatusNotFound, "not_found", "no client found for that domain and no name provided to create one")
			return
		}
		handleError(w, err)
		return
	}
	JSON(w, http.StatusOK, c)
}
