package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/morpikssolutionsDev/Shusi_latte/api/internal/actor"
	"net/http"
	"strconv"
)

// Handler wires the catalog Service to the TRD §6.1 catalog endpoints.
// Auth and role checks arrive with the auth phase; these handlers are
// staff catalog endpoints and are not the public Client Menu endpoint.
type Handler struct {
	Service Store
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeProblem(w http.ResponseWriter, status int, code, title string) {
	writeJSON(w, status, map[string]any{
		"type":   "about:blank",
		"title":  title,
		"status": status,
		"code":   code,
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return false
	}
	return true
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/catalog", h.getCatalog)
	mux.HandleFunc("GET /api/v1/prep-areas", h.listPrepAreas)
	mux.HandleFunc("POST /api/v1/prep-areas", h.createPrepArea)
	mux.HandleFunc("GET /api/v1/categories", h.listCategories)
	mux.HandleFunc("POST /api/v1/categories", h.createCategory)
	mux.HandleFunc("GET /api/v1/products", h.listProducts)
	mux.HandleFunc("POST /api/v1/products", h.createProduct)
	mux.HandleFunc("POST /api/v1/products/{id}/status", h.updateProductStatus)
	mux.HandleFunc("PUT /api/v1/products/{id}/dietary-tags", h.setProductDietaryTags)
	mux.HandleFunc("POST /api/v1/variants/{id}/sold-out", h.setVariantSoldOut)
	mux.HandleFunc("DELETE /api/v1/variants/{id}/sold-out", h.clearVariantSoldOut)
	mux.HandleFunc("GET /api/v1/dietary-tags", h.listDietaryTags)
	mux.HandleFunc("POST /api/v1/dietary-tags", h.createDietaryTag)
	mux.HandleFunc("POST /api/v1/modifier-groups", h.createModifierGroup)
	mux.HandleFunc("POST /api/v1/modifiers", h.createModifier)
	mux.HandleFunc("PUT /api/v1/modifiers/{id}/dietary-tags", h.setModifierDietaryTags)
	mux.HandleFunc("GET /api/v1/audit-log", h.listAudit)
}

// getCatalog implements TR-PC-05: one endpoint returning the full active
// catalog, with an ETag so clients refetch only when it changes.
func (h *Handler) getCatalog(w http.ResponseWriter, r *http.Request) {
	cat := h.Service.Catalog()
	body, _ := json.Marshal(cat)
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:]) + `"`
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handler) listPrepAreas(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.Service.ListPrepAreas())
}

func (h *Handler) createPrepArea(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	var p PrepArea
	if !decodeJSON(w, r, &p) {
		return
	}
	out, err := h.Service.CreatePrepArea(p)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_prep_area", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *Handler) listCategories(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.Service.ListCategories())
}

func (h *Handler) createCategory(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	var c Category
	if !decodeJSON(w, r, &c) {
		return
	}
	out, err := h.Service.CreateCategory(c)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_category", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *Handler) listProducts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.Service.ListProducts(false))
}

func (h *Handler) createProduct(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	var p Product
	if !decodeJSON(w, r, &p) {
		return
	}
	out, err := h.Service.CreateProduct(p)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_product", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *Handler) updateProductStatus(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	var req struct {
		Status string  `json:"status"`
		Reason *string `json:"reason"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	out, err := h.Service.UpdateProductStatus(r.PathValue("id"), req.Status, req.Reason)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_product_status", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) setProductDietaryTags(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	var req struct {
		DietaryTagIDs []string `json:"dietary_tag_ids"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	out, err := h.Service.SetProductDietaryTags(r.PathValue("id"), req.DietaryTagIDs)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_dietary_tags", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) setVariantSoldOut(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	var req struct {
		By     *string `json:"by"`
		Source string  `json:"source"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	out, err := h.Service.SetVariantSoldOut(r.PathValue("id"), req.By, req.Source)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "variant_not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) clearVariantSoldOut(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	out, err := h.Service.ClearVariantSoldOut(r.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "variant_not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) listDietaryTags(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.Service.ListDietaryTags())
}

func (h *Handler) createDietaryTag(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	var t DietaryTag
	if !decodeJSON(w, r, &t) {
		return
	}
	out, err := h.Service.CreateDietaryTag(t)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_dietary_tag", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *Handler) createModifierGroup(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	var g ModifierGroup
	if !decodeJSON(w, r, &g) {
		return
	}
	out, err := h.Service.CreateModifierGroup(g)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_modifier_group", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *Handler) createModifier(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	var m Modifier
	if !decodeJSON(w, r, &m) {
		return
	}
	out, err := h.Service.CreateModifier(m)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_modifier", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *Handler) setModifierDietaryTags(w http.ResponseWriter, r *http.Request) {
	h = h.withActor(r)
	var req struct {
		DietaryTagIDs []string `json:"dietary_tag_ids"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	out, err := h.Service.SetModifierDietaryTags(r.PathValue("id"), req.DietaryTagIDs)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_dietary_tags", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) listAudit(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := AuditFilter{
		Action:   q.Get("action"),
		Entity:   q.Get("entity"),
		EntityID: q.Get("entity_id"),
		Actor:    q.Get("actor"),
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Offset = n
		}
	}
	if v := q.Get("from_ms"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.FromMS = &n
		}
	}
	if v := q.Get("to_ms"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.ToMS = &n
		}
	}
	entries, total := h.Service.ListAuditFiltered(f)
	writeJSON(w, http.StatusOK, map[string]any{
		"data": entries, "total": total, "limit": f.effectiveLimit(), "offset": f.Offset,
	})
}

// withActor returns a handler copy whose SQLite store records the
// authenticated crew as the audit actor. For other store types it
// returns the handler unchanged.
func (h *Handler) withActor(r *http.Request) *Handler {
	svc, ok := h.Service.(*SQLiteService)
	if !ok {
		return h
	}
	copySvc := *svc
	copySvc.Actor = actor.FromRequest(r.Context())
	return &Handler{Service: &copySvc}
}
