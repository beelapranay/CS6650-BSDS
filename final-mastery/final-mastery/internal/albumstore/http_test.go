package albumstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthEndpoint(t *testing.T) {
	app := buildTestApp(t)
	defer app.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if body := rec.Body.String(); body != `{"status":"ok"}` {
		t.Fatalf("body=%s", body)
	}
}

func TestAlbumAndPhotoLifecycle(t *testing.T) {
	app := buildTestApp(t)
	defer app.Close()

	albumID := "cce39c35-5064-46e9-9036-8237ff528bb8"
	putBody := map[string]any{
		"album_id":    albumID,
		"title":       "",
		"description": "",
		"owner":       "",
	}
	putJSON(t, app, "/albums/"+albumID, putBody, http.StatusCreated)

	uploadRec := httptest.NewRecorder()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("photo", "sample.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("image-bytes")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/albums/"+albumID+"/photos", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Content-Length", strconvItoa(body.Len()))
	app.ServeHTTP(uploadRec, req)
	if uploadRec.Code != http.StatusAccepted {
		t.Fatalf("upload status=%d body=%s", uploadRec.Code, uploadRec.Body.String())
	}

	var accepted map[string]any
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	photoID := accepted["photo_id"].(string)

	eventually(t, func() bool {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/albums/"+albumID+"/photos/"+photoID, nil)
		app.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			return false
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			return false
		}
		return payload["status"] == "completed"
	})

	deleteReq := httptest.NewRequest(http.MethodDelete, "/albums/"+albumID+"/photos/"+photoID, nil)
	deleteRec := httptest.NewRecorder()
	app.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d", deleteRec.Code)
	}
}

func buildTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	service, err := BuildLocalService(dir, MaxUploadBytes)
	if err != nil {
		t.Fatal(err)
	}
	return NewApp(service, "http://testserver")
}

func putJSON(t *testing.T, app *App, path string, body map[string]any, expected int) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", strconvItoa(len(raw)))
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != expected {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func eventually(t *testing.T, check func() bool) {
	t.Helper()
	for range 40 {
		if check() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("condition not met")
}

func strconvItoa(value int) string {
	return fmt.Sprintf("%d", value)
}
