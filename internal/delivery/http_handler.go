package delivery

import (
	"crm-core/internal/service"
	"encoding/json"
	"net/http"
	"strings"
)

func isValidEmail(email string) bool {
	at := strings.Index(email, "@")
	// Require an "@" that is neither the first nor last character, so
	// things like "@x", "x@", "" are rejected without pulling in a full
	// RFC 5322 validator for a learning project.
	return at > 0 && at < len(email)-1
}

type HTTPHandler struct {
	customerService *service.CustomerService
}

func NewHTTPHandler(cs *service.CustomerService) *HTTPHandler {
	return &HTTPHandler{customerService: cs}
}

func (h *HTTPHandler) CreateCustomer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.Email == "" || !isValidEmail(req.Email) {
		http.Error(w, "a valid email is required", http.StatusBadRequest)
		return
	}

	c, err := h.customerService.RegisterCustomer(req.Name, req.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(c)
}

func (h *HTTPHandler) GetCustomer(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/customers/")
	c, err := h.customerService.GetCustomerProfile(id)
	if err != nil {
		http.Error(w, "Customer not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(c)
}
