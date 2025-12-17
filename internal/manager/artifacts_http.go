package manager

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/lildannita/qtiva-cloud/internal/artifactsx"
	"github.com/lildannita/qtiva-cloud/internal/commonx"
	"github.com/lildannita/qtiva-cloud/internal/httpx"
	"github.com/lildannita/qtiva-cloud/internal/idgen"
	"github.com/lildannita/qtiva-cloud/internal/jwtx"
)

type ArtifactsAPI struct {
	DB *sql.DB
}

func RegisterArtifactRoutes(mux *http.ServeMux, api ArtifactsAPI, db *sql.DB, jwtCfg jwtx.Config) {
	mux.Handle("/artifacts", httpx.RequireAuth(db, jwtCfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleUploadArtifact(w, r, api)
	})))
}

func handleUploadArtifact(w http.ResponseWriter, r *http.Request, api ArtifactsAPI) {
	// Пользователь из контекста
	u, ok := httpx.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется авторизация")
		return
	}

	// Конфиг из env
	dataDir, err := commonx.RequireString("QTIVA_DATA_DIR")
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	maxBytes, err := commonx.RequireInt64("QTIVA_ARTIFACT_MAX_BYTES")
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	maxTimeout, err := commonx.RequireInt("QTIVA_MAX_TIMEOUT_SEC")
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Проверка Content-Type на multipart
	ct := r.Header.Get("Content-Type")
	mediatype, params, err := mime.ParseMediaType(ct)
	if err != nil || !strings.HasPrefix(mediatype, "multipart/") {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Ожидается multipart/form-data")
		return
	}

	mr := multipart.NewReader(r.Body, params["boundary"])

	// Ищем part с именем file
	var filePart *multipart.Part
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		if p.FormName() == "file" {
			filePart = p
			break
		}
		_ = p.Close()
	}
	if filePart == nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Не найдено поле file")
		return
	}
	defer filePart.Close()

	artifactID, err := idgen.New("art")
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось создать artifact_id")
		return
	}

	storedPath := filepath.Join(dataDir, "tmp", "artifacts", artifactID, "app.tar.gz")

	saveRes, err := artifactsx.SaveMultipartFile(storedPath, filePart, maxBytes)
	if err != nil {
		if err == artifactsx.ErrTooLarge {
			httpx.WriteError(w, r, http.StatusRequestEntityTooLarge, "ARTIFACT_TOO_LARGE", "Файл слишком большой")
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось сохранить файл")
		return
	}

	manifestBytes, err := artifactsx.ReadManifestFromTarGz(storedPath)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "MANIFEST_INVALID", err.Error())
		return
	}

	m, err := artifactsx.ParseManifestJSON(manifestBytes)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "MANIFEST_INVALID", err.Error())
		return
	}

	if err := artifactsx.ValidateManifest(m, artifactsx.ValidateConfig{MaxTimeoutSec: maxTimeout}); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "MANIFEST_INVALID", err.Error())
		return
	}

	manifestJSON, err := json.Marshal(m)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Не удалось сериализовать manifest")
		return
	}

	// Запись в БД
	_, err = api.DB.ExecContext(r.Context(),
		`INSERT INTO artifacts (id, client_id, user_id, sha256, size_bytes, stored_path, manifest)
		 VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb)`,
		artifactID, u.ClientID, u.UserID, saveRes.SHA256, saveRes.SizeBytes, storedPath, string(manifestJSON),
	)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Sprintf("Не удалось записать в БД: %v", err))
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"artifact_id": artifactID,
		"sha256":      saveRes.SHA256,
		"size_bytes":  saveRes.SizeBytes,
	})
}
