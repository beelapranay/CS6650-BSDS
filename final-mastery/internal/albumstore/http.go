package albumstore

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

const MaxUploadBytes int64 = 200 * 1024 * 1024

type App struct {
	service       *Service
	publicBaseURL string
}

func NewApp(service *Service, publicBaseURL string) *App {
	return &App{
		service:       service,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
	}
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("panic: %v", recovered)
			writeJSON(w, 500, map[string]any{"error": "internal server error"})
		}
	}()

	path := strings.Trim(r.URL.Path, "/")
	var segments []string
	if path != "" {
		segments = strings.Split(path, "/")
	}

	switch {
	case r.Method == http.MethodGet && len(segments) == 1 && segments[0] == "health":
		writeJSON(w, 200, map[string]any{"status": "ok"})
		return
	case r.Method == http.MethodGet && len(segments) == 1 && segments[0] == "albums":
		albums, err := a.service.ListAlbums()
		if err != nil {
			a.writeError(w, err)
			return
		}
		writeJSON(w, 200, albums)
		return
	case len(segments) == 2 && segments[0] == "albums":
		albumID := segments[1]
		if err := validateUUID(albumID, "album_id"); err != nil {
			a.writeError(w, err)
			return
		}
		if r.Method == http.MethodPut {
			payload, err := readJSONBody(r)
			if err != nil {
				a.writeError(w, err)
				return
			}
			if payload["album_id"] != albumID {
				a.writeError(w, NewAppError(400, map[string]any{"error": "album_id mismatch"}))
				return
			}
			status, body, err := a.service.PutAlbum(payload)
			if err != nil {
				a.writeError(w, err)
				return
			}
			writeJSON(w, status, body)
			return
		}
		if r.Method == http.MethodGet {
			body, err := a.service.GetAlbum(albumID)
			if err != nil {
				a.writeError(w, err)
				return
			}
			writeJSON(w, 200, body)
			return
		}
	case len(segments) == 3 && segments[0] == "albums" && segments[2] == "photos" && r.Method == http.MethodPost:
		albumID := segments[1]
		if err := validateUUID(albumID, "album_id"); err != nil {
			a.writeError(w, err)
			return
		}
		payload, err := a.uploadPhoto(r.Context(), r, albumID)
		if err != nil {
			a.writeError(w, err)
			return
		}
		writeJSON(w, 202, payload)
		return
	case len(segments) == 4 && segments[0] == "albums" && segments[2] == "photos":
		albumID := segments[1]
		photoID := segments[3]
		if err := validateUUID(albumID, "album_id"); err != nil {
			a.writeError(w, err)
			return
		}
		if err := validateUUID(photoID, "photo_id"); err != nil {
			a.writeError(w, err)
			return
		}
		if r.Method == http.MethodGet {
			body, err := a.service.GetPhoto(albumID, photoID, a.baseURL(r))
			if err != nil {
				a.writeError(w, err)
				return
			}
			writeJSON(w, 200, body)
			return
		}
		if r.Method == http.MethodDelete {
			if err := a.service.DeletePhoto(albumID, photoID); err != nil {
				a.writeError(w, err)
				return
			}
			writeJSON(w, 204, nil)
			return
		}
	case len(segments) == 2 && segments[0] == "files" && r.Method == http.MethodGet:
		photoID := segments[1]
		if err := validateUUID(photoID, "photo_id"); err != nil {
			a.writeError(w, err)
			return
		}
		_, meta, reader, err := a.service.ResolveStoredPhoto(photoID)
		if err != nil {
			a.writeError(w, err)
			return
		}
		defer reader.Close()
		w.Header().Set("Content-Type", meta.ContentType)
		w.Header().Set("Content-Length", strconv.FormatInt(meta.ContentLength, 10))
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, reader)
		return
	}

	a.writeError(w, NewAppError(404, map[string]any{"error": "not found"}))
}

func (a *App) Close() error {
	return a.service.Close()
}

func (a *App) uploadPhoto(ctx context.Context, r *http.Request, albumID string) (map[string]any, error) {
	contentLength := strings.TrimSpace(r.Header.Get("Content-Length"))
	if contentLength != "" {
		value, err := strconv.ParseInt(contentLength, 10, 64)
		if err != nil {
			return nil, NewAppError(400, map[string]any{"error": "invalid content length"})
		}
		if value > MaxUploadBytes+(1024*1024) {
			return nil, NewAppError(413, map[string]any{"error": "payload too large"})
		}
	}

	reader, err := r.MultipartReader()
	if err != nil {
		return nil, NewAppError(400, map[string]any{"error": "missing photo"})
	}

	part, err := findPhotoPart(reader)
	if err != nil {
		return nil, err
	}
	defer part.Close()

	filename := part.FileName()
	if filename == "" {
		filename = "upload.bin"
	}
	_ = ctx
	return a.service.UploadPhoto(albumID, part, filename)
}

func findPhotoPart(reader *multipart.Reader) (*multipart.Part, error) {
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return nil, NewAppError(400, map[string]any{"error": "missing photo"})
		}
		if err != nil {
			return nil, NewAppError(400, map[string]any{"error": "missing photo"})
		}
		if part.FormName() == "photo" {
			return part, nil
		}
		_, _ = io.Copy(io.Discard, part)
		_ = part.Close()
	}
}

func readJSONBody(r *http.Request) (map[string]any, error) {
	contentLength := strings.TrimSpace(r.Header.Get("Content-Length"))
	if contentLength != "" {
		if _, err := strconv.Atoi(contentLength); err != nil {
			return nil, NewAppError(400, map[string]any{"error": "invalid content length"})
		}
	}
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return nil, NewAppError(400, map[string]any{"error": "invalid json"})
	}
	return payload, nil
}

func validateUUID(value, field string) error {
	if _, err := uuid.Parse(value); err != nil {
		return NewAppError(400, map[string]any{"error": "invalid " + field})
	}
	return nil
}

func (a *App) baseURL(r *http.Request) string {
	if a.publicBaseURL != "" {
		return a.publicBaseURL
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		scheme = strings.Split(forwarded, ",")[0]
	}
	return scheme + "://" + r.Host
}

func (a *App) writeError(w http.ResponseWriter, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		writeJSON(w, appErr.StatusCode, appErr.Payload)
		return
	}
	log.Printf("request failed err=%v", err)
	writeJSON(w, 500, map[string]any{"error": "internal server error"})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	if status == 204 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	body, err := json.Marshal(payload)
	if err != nil {
		body = []byte(`{"error":"internal server error"}`)
		status = 500
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func MaxUploadBytesFromEnv() int64 {
	if raw := strings.TrimSpace(os.Getenv("MAX_UPLOAD_BYTES")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err == nil && value > 0 {
			return value
		}
	}
	return MaxUploadBytes
}
