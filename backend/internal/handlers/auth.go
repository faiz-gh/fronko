package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/faiz-gh/credensync/backend/internal/auth"
	"github.com/faiz-gh/credensync/backend/internal/repository"
)

type AuthHandler struct {
	repo        *repository.Repository
	authService *auth.Service
}

func NewAuthHandler(repo *repository.Repository, authService *auth.Service) *AuthHandler {
	return &AuthHandler{
		repo:        repo,
		authService: authService,
	}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// TODO: implement logic using h.repo and h.authService

	// mock response
	res := LoginResponse{Token: "mock_token"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	// TODO: implement register
	w.WriteHeader(http.StatusCreated)
}
