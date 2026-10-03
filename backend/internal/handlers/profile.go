package handlers

import (
	"net/http"

	"github.com/faiz-gh/credensync/backend/internal/repository"
)

type ProfileHandler struct {
	repo *repository.Repository
}

func NewProfileHandler(repo *repository.Repository) *ProfileHandler {
	return &ProfileHandler{
		repo: repo,
	}
}

func (h *ProfileHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	// TODO: implement get profile logic
	w.Write([]byte("Profile: " + id))
}
