package devices_test

import (
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/soapboxsocial/soapbox/pkg/devices"
	httputil "github.com/soapboxsocial/soapbox/pkg/http"
)

func TestMain(m *testing.M) {
	log.SetOutput(ioutil.Discard)
	os.Exit(m.Run())
}

func TestDevicesEndpoint_AddDevice(t *testing.T) {
	token := "123"
	session := 123
	reader := strings.NewReader("token=" + token)

	r, err := http.NewRequest("POST", "/add", reader)
	if err != nil {
		t.Fatal(err)
	}

	req := r.WithContext(httputil.WithUserID(r.Context(), session))

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	endpoint := devices.NewEndpoint(devices.NewBackend(db))

	mock.ExpectPrepare("^INSERT (.+)").ExpectExec().
		WithArgs(token, session).
		WillReturnResult(sqlmock.NewResult(1, 1))

	rr := httptest.NewRecorder()
	handler := endpoint.Router()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestDevicesEndpoint_AddDeviceFailsWithoutToken(t *testing.T) {
	req, err := http.NewRequest("POST", "/add", strings.NewReader("foo=bar"))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	endpoint := devices.NewEndpoint(devices.NewBackend(db))

	rr := httptest.NewRecorder()
	handler := endpoint.Router()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}

func TestDevicesEndpoint_AddDeviceWithBackendError(t *testing.T) {
	token := "123"
	session := 123
	reader := strings.NewReader("token=" + token)

	r, err := http.NewRequest("POST", "/add", reader)
	if err != nil {
		t.Fatal(err)
	}

	req := r.WithContext(httputil.WithUserID(r.Context(), session))

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	endpoint := devices.NewEndpoint(devices.NewBackend(db))

	mock.ExpectPrepare("^INSERT (.+)").ExpectExec().
		WithArgs(token, session).
		WillReturnError(errors.New("boom"))

	rr := httptest.NewRecorder()
	handler := endpoint.Router()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusInternalServerError)
	}
}

func TestDevicesEndpoint_AddDeviceWithoutForm(t *testing.T) {
	req, err := http.NewRequest("POST", "/add", nil)
	if err != nil {
		t.Fatal(err)
	}

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	endpoint := devices.NewEndpoint(devices.NewBackend(db))
	rr := httptest.NewRecorder()
	handler := endpoint.Router()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}
