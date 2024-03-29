package blocks

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	httputil "github.com/soapboxsocial/soapbox/pkg/http"
)

type Endpoint struct {
	backend *Backend
}

func NewEndpoint(backend *Backend) *Endpoint {
	return &Endpoint{
		backend: backend,
	}
}

func (e *Endpoint) Router() *mux.Router {
	r := mux.NewRouter()

	r.HandleFunc("/", e.unblock).Methods("DELETE")
	r.HandleFunc("/create", e.block).Methods("POST")

	return r
}

func (e *Endpoint) unblock(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		httputil.JsonError(w, http.StatusBadRequest, httputil.ErrorCodeInvalidRequestBody, "")
		return
	}

	id, err := strconv.Atoi(r.Form.Get("id"))
	if err != nil {
		httputil.JsonError(w, http.StatusBadRequest, httputil.ErrorCodeInvalidRequestBody, "invalid id")
		return
	}

	userID, ok := httputil.GetUserIDFromContext(r.Context())
	if !ok {
		httputil.JsonError(w, http.StatusInternalServerError, httputil.ErrorCodeInvalidRequestBody, "invalid id")
		return
	}

	err = e.backend.UnblockUser(userID, id)
	if err != nil {
		httputil.JsonError(w, http.StatusInternalServerError, httputil.ErrorCodeInvalidRequestBody, "failed to unblock")
		return
	}
}

func (e *Endpoint) block(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		httputil.JsonError(w, http.StatusBadRequest, httputil.ErrorCodeInvalidRequestBody, "")
		return
	}

	id, err := strconv.Atoi(r.Form.Get("id"))
	if err != nil {
		httputil.JsonError(w, http.StatusBadRequest, httputil.ErrorCodeInvalidRequestBody, "invalid id")
		return
	}

	userID, ok := httputil.GetUserIDFromContext(r.Context())
	if !ok {
		httputil.JsonError(w, http.StatusInternalServerError, httputil.ErrorCodeInvalidRequestBody, "invalid id")
		return
	}

	err = e.backend.BlockUser(userID, id)
	if err != nil {
		httputil.JsonError(w, http.StatusInternalServerError, httputil.ErrorCodeInvalidRequestBody, "failed to block")
		return
	}

	httputil.JsonSuccess(w)
}
