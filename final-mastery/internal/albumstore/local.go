package albumstore

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteAlbumRepository struct {
	db *sql.DB
}

func NewSQLiteAlbumRepository(db *sql.DB) (*SQLiteAlbumRepository, error) {
	repo := &SQLiteAlbumRepository{db: db}
	if err := repo.initialize(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *SQLiteAlbumRepository) initialize() error {
	_, err := r.db.Exec(`
		CREATE TABLE IF NOT EXISTS albums (
			album_id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT NOT NULL,
			owner TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS album_counters (
			album_id TEXT PRIMARY KEY,
			next_seq INTEGER NOT NULL
		);
	`)
	return err
}

func (r *SQLiteAlbumRepository) Upsert(album Album) (bool, error) {
	tx, err := r.db.BeginTx(context.Background(), nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var existing string
	err = tx.QueryRow(`SELECT album_id FROM albums WHERE album_id = ?`, album.AlbumID).Scan(&existing)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	created := errors.Is(err, sql.ErrNoRows)
	if err != nil && !created {
		return false, err
	}

	if created {
		if _, err := tx.Exec(`
			INSERT INTO albums (album_id, title, description, owner, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, album.AlbumID, album.Title, album.Description, album.Owner, now, now); err != nil {
			return false, err
		}
	} else {
		if _, err := tx.Exec(`
			UPDATE albums
			SET title = ?, description = ?, owner = ?, updated_at = ?
			WHERE album_id = ?
		`, album.Title, album.Description, album.Owner, now, album.AlbumID); err != nil {
			return false, err
		}
	}

	if _, err := tx.Exec(`
		INSERT INTO album_counters (album_id, next_seq)
		VALUES (?, 1)
		ON CONFLICT(album_id) DO NOTHING
	`, album.AlbumID); err != nil {
		return false, err
	}

	return created, tx.Commit()
}

func (r *SQLiteAlbumRepository) Get(albumID string) (*Album, error) {
	row := r.db.QueryRow(`SELECT album_id, title, description, owner FROM albums WHERE album_id = ?`, albumID)
	var album Album
	if err := row.Scan(&album.AlbumID, &album.Title, &album.Description, &album.Owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &album, nil
}

func (r *SQLiteAlbumRepository) ListAll() ([]Album, error) {
	rows, err := r.db.Query(`
		SELECT album_id, title, description, owner
		FROM albums
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var albums []Album
	for rows.Next() {
		var album Album
		if err := rows.Scan(&album.AlbumID, &album.Title, &album.Description, &album.Owner); err != nil {
			return nil, err
		}
		albums = append(albums, album)
	}
	return albums, rows.Err()
}

func (r *SQLiteAlbumRepository) Exists(albumID string) (bool, error) {
	row := r.db.QueryRow(`SELECT 1 FROM albums WHERE album_id = ?`, albumID)
	var marker int
	err := row.Scan(&marker)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

type SQLitePhotoRepository struct {
	db             *sql.DB
	sequenceMu     sync.Mutex
	photoLocks     map[string]*sync.Mutex
	photoLocksGuard sync.Mutex
}

func NewSQLitePhotoRepository(db *sql.DB) (*SQLitePhotoRepository, error) {
	repo := &SQLitePhotoRepository{
		db:         db,
		photoLocks: map[string]*sync.Mutex{},
	}
	if err := repo.initialize(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *SQLitePhotoRepository) initialize() error {
	_, err := r.db.Exec(`
		CREATE TABLE IF NOT EXISTS photos (
			photo_id TEXT PRIMARY KEY,
			album_id TEXT NOT NULL,
			seq INTEGER NOT NULL,
			status TEXT NOT NULL,
			original_filename TEXT,
			temp_path TEXT,
			storage_path TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(album_id, seq)
		)
	`)
	return err
}

func (r *SQLitePhotoRepository) AllocateSeq(albumID string) (int64, error) {
	r.sequenceMu.Lock()
	defer r.sequenceMu.Unlock()

	tx, err := r.db.BeginTx(context.Background(), nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var next int64
	err = tx.QueryRow(`SELECT next_seq FROM album_counters WHERE album_id = ?`, albumID).Scan(&next)
	if errors.Is(err, sql.ErrNoRows) {
		next = 1
		if _, err := tx.Exec(`INSERT INTO album_counters (album_id, next_seq) VALUES (?, ?)`, albumID, 2); err != nil {
			return 0, err
		}
	} else if err != nil {
		return 0, err
	} else {
		if _, err := tx.Exec(`UPDATE album_counters SET next_seq = ? WHERE album_id = ?`, next+1, albumID); err != nil {
			return 0, err
		}
	}

	return next, tx.Commit()
}

func (r *SQLitePhotoRepository) CreateProcessingPhoto(photoID, albumID string, seq int64, stagedUpload StagedUpload) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.Exec(`
		INSERT INTO photos (
			photo_id, album_id, seq, status, original_filename, temp_path, storage_path, created_at, updated_at
		) VALUES (?, ?, ?, 'processing', ?, ?, NULL, ?, ?)
	`, photoID, albumID, seq, stagedUpload.OriginalFilename, stagedUpload.TempPath, now, now)
	return err
}

func (r *SQLitePhotoRepository) Get(albumID, photoID string) (*PhotoRecord, error) {
	row := r.db.QueryRow(`
		SELECT photo_id, album_id, seq, status, original_filename, temp_path, storage_path
		FROM photos
		WHERE album_id = ? AND photo_id = ?
	`, albumID, photoID)
	return scanPhoto(row)
}

func (r *SQLitePhotoRepository) GetByPhotoID(photoID string) (*PhotoRecord, error) {
	row := r.db.QueryRow(`
		SELECT photo_id, album_id, seq, status, original_filename, temp_path, storage_path
		FROM photos
		WHERE photo_id = ?
	`, photoID)
	return scanPhoto(row)
}

func (r *SQLitePhotoRepository) MarkCompleted(albumID, photoID, storagePath string) (bool, error) {
	lock := r.photoLock(photoID)
	lock.Lock()
	defer lock.Unlock()

	result, err := r.db.Exec(`
		UPDATE photos
		SET status = 'completed', temp_path = NULL, storage_path = ?, updated_at = ?
		WHERE album_id = ? AND photo_id = ?
	`, storagePath, time.Now().UTC().Format(time.RFC3339Nano), albumID, photoID)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}

func (r *SQLitePhotoRepository) MarkFailed(albumID, photoID string) error {
	lock := r.photoLock(photoID)
	lock.Lock()
	defer lock.Unlock()

	_, err := r.db.Exec(`
		UPDATE photos
		SET status = 'failed', updated_at = ?
		WHERE album_id = ? AND photo_id = ?
	`, time.Now().UTC().Format(time.RFC3339Nano), albumID, photoID)
	return err
}

func (r *SQLitePhotoRepository) Delete(albumID, photoID string) (*PhotoRecord, error) {
	lock := r.photoLock(photoID)
	lock.Lock()
	defer lock.Unlock()

	tx, err := r.db.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRow(`
		SELECT photo_id, album_id, seq, status, original_filename, temp_path, storage_path
		FROM photos
		WHERE album_id = ? AND photo_id = ?
	`, albumID, photoID)
	record, err := scanPhoto(row)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, tx.Commit()
	}
	if _, err := tx.Exec(`DELETE FROM photos WHERE album_id = ? AND photo_id = ?`, albumID, photoID); err != nil {
		return nil, err
	}
	return record, tx.Commit()
}

func (r *SQLitePhotoRepository) photoLock(photoID string) *sync.Mutex {
	r.photoLocksGuard.Lock()
	defer r.photoLocksGuard.Unlock()
	lock, ok := r.photoLocks[photoID]
	if !ok {
		lock = &sync.Mutex{}
		r.photoLocks[photoID] = lock
	}
	return lock
}

func scanPhoto(row interface{ Scan(dest ...any) error }) (*PhotoRecord, error) {
	var record PhotoRecord
	var originalFilename, tempPath, storagePath sql.NullString
	if err := row.Scan(
		&record.PhotoID,
		&record.AlbumID,
		&record.Seq,
		&record.Status,
		&originalFilename,
		&tempPath,
		&storagePath,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	record.OriginalFilename = originalFilename.String
	record.TempPath = tempPath.String
	record.StoragePath = storagePath.String
	return &record, nil
}

type LocalFileStorage struct {
	uploadDir string
	mediaDir  string
}

func NewLocalFileStorage(uploadDir, mediaDir string) (*LocalFileStorage, error) {
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		return nil, err
	}
	return &LocalFileStorage{uploadDir: uploadDir, mediaDir: mediaDir}, nil
}

func (s *LocalFileStorage) StageUpload(file io.Reader, photoID, originalFilename string, maxUploadBytes int64) (StagedUpload, error) {
	tempPath := filepath.Join(s.uploadDir, photoID+".upload")
	handle, err := os.Create(tempPath)
	if err != nil {
		return StagedUpload{}, err
	}
	defer handle.Close()

	limited := io.LimitReader(file, maxUploadBytes+1)
	written, err := io.Copy(handle, limited)
	if err != nil {
		_ = s.DeleteFile(tempPath)
		return StagedUpload{}, err
	}
	if written > maxUploadBytes {
		_ = s.DeleteFile(tempPath)
		return StagedUpload{}, errors.New("payload too large")
	}
	return StagedUpload{OriginalFilename: originalFilename, TempPath: tempPath}, nil
}

func (s *LocalFileStorage) Promote(stagedUpload StagedUpload, photoID string) (string, error) {
	ext := trimExtension(stagedUpload.OriginalFilename)
	finalPath := filepath.Join(s.mediaDir, photoID+ext)
	if err := os.Rename(stagedUpload.TempPath, finalPath); err != nil {
		return "", err
	}
	return finalPath, nil
}

func (s *LocalFileStorage) DeleteFile(path string) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *LocalFileStorage) StatStoredFile(path string) (*StoredFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return &StoredFile{
		Path:          path,
		ContentType:   contentTypeForFile(path),
		ContentLength: info.Size(),
	}, nil
}

func (s *LocalFileStorage) OpenStoredFile(path string) (io.ReadCloser, *StoredFile, error) {
	meta, err := s.StatStoredFile(path)
	if err != nil || meta == nil {
		return nil, nil, err
	}
	handle, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return handle, meta, nil
}

func (s *LocalFileStorage) BuildPublicURL(storagePath, baseURL, photoID string) string {
	return strings.TrimRight(baseURL, "/") + "/files/" + photoID
}

type LocalDispatcher struct {
	jobs chan photoJob
	wg   sync.WaitGroup
}

type photoJob struct {
	albumID string
	photoID string
}

func NewLocalDispatcher(workerCount int, processor func(albumID, photoID string)) *LocalDispatcher {
	if workerCount < 1 {
		workerCount = 1
	}
	dispatcher := &LocalDispatcher{
		jobs: make(chan photoJob, workerCount*8),
	}
	dispatcher.wg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go func() {
			defer dispatcher.wg.Done()
			for job := range dispatcher.jobs {
				processor(job.albumID, job.photoID)
			}
		}()
	}
	return dispatcher
}

func (d *LocalDispatcher) EnqueuePhotoProcessing(albumID, photoID string) error {
	d.jobs <- photoJob{albumID: albumID, photoID: photoID}
	return nil
}

func (d *LocalDispatcher) Close() error {
	close(d.jobs)
	d.wg.Wait()
	return nil
}

func BuildLocalService(dataDir string, maxUploadBytes int64) (*Service, error) {
	if dataDir == "" {
		dataDir = filepath.Join(".", "data")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dataDir, "album_store.sqlite3")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	albumRepo, err := NewSQLiteAlbumRepository(db)
	if err != nil {
		return nil, err
	}
	photoRepo, err := NewSQLitePhotoRepository(db)
	if err != nil {
		return nil, err
	}
	fileStorage, err := NewLocalFileStorage(
		filepath.Join(dataDir, "uploads"),
		filepath.Join(dataDir, "media"),
	)
	if err != nil {
		return nil, err
	}
	var service *Service
	dispatcher := NewLocalDispatcher(envInt("PHOTO_WORKERS", 4), func(albumID, photoID string) {
		service.ProcessPhoto(albumID, photoID)
	})
	service = NewService(albumRepo, photoRepo, fileStorage, dispatcher, maxUploadBytes)
	return service, nil
}

func trimExtension(filename string) string {
	ext := filepath.Ext(filename)
	if len(ext) > 16 {
		return ext[:16]
	}
	return ext
}

func contentTypeForFile(path string) string {
	if contentType := mime.TypeByExtension(filepath.Ext(path)); contentType != "" {
		return contentType
	}
	file, err := os.Open(path)
	if err != nil {
		return "application/octet-stream"
	}
	defer file.Close()
	buffer := make([]byte, 512)
	n, _ := file.Read(buffer)
	return http.DetectContentType(buffer[:n])
}

func envInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
