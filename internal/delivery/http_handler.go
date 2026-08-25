package delivery

import (
	"crm-core/internal/service"
	"encoding/json"
	"net/http"
	"strings"
)

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
	json.NewDecoder(r.Body).Decode(&req)

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
