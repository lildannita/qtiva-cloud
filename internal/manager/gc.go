package manager

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"time"
)

type GarbageCollector struct {
	db       *sql.DB
	interval time.Duration
	stopCh   chan struct{}
}

// Создаёт новый сборщик мусора
func NewGarbageCollector(db *sql.DB, interval time.Duration) *GarbageCollector {
	return &GarbageCollector{
		db:       db,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Запускает GC в фоновом режиме
func (gc *GarbageCollector) Start(ctx context.Context) {
	go gc.run(ctx)
}

// Останавливает GC
func (gc *GarbageCollector) Stop() {
	close(gc.stopCh)
}

// Запускает GC
func (gc *GarbageCollector) run(ctx context.Context) {
	log.Printf("GC запущен с интервалом %s", gc.interval)

	// Первый запуск сразу
	gc.cleanup(ctx)

	ticker := time.NewTicker(gc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-gc.stopCh:
			return
		case <-ticker.C:
			gc.cleanup(ctx)
		}
	}
}

func (gc *GarbageCollector) cleanup(ctx context.Context) {
	log.Println("GC: начало очистки")

	// Удаляем данные прогонов с истёкшим delete_after
	gc.cleanupExpiredRuns(ctx)
	// Удаляем временные артефакты израсходованных прогонов
	gc.cleanupConsumedArtifacts(ctx)
	// Удаляем задания завершённых прогонов старше 24 часов
	gc.cleanupOldJobs(ctx)

	log.Println("GC: очистка завершена")
}

// Удаляет данные прогонов с истёкшим TTL
func (gc *GarbageCollector) cleanupExpiredRuns(ctx context.Context) {
	// Находим прогоны для удаления
	rows, err := gc.db.QueryContext(ctx, `
		SELECT id, log_path 
		FROM runs 
		WHERE delete_after < now() 
		  AND deleted_at IS NULL
		LIMIT 100
	`)
	if err != nil {
		log.Printf("GC: ошибка получения прогонов: %v", err)
		return
	}
	defer rows.Close()

	var deletedCount int
	for rows.Next() {
		var runID string
		var logPath sql.NullString

		if err := rows.Scan(&runID, &logPath); err != nil {
			log.Printf("GC: ошибка сканирования: %v", err)
			continue
		}

		// Удаляем файл лога
		if logPath.Valid && logPath.String != "" {
			if err := os.Remove(logPath.String); err != nil && !os.IsNotExist(err) {
				log.Printf("GC: не удалось удалить лог %s: %v", logPath.String, err)
			} else {
				// Пытаемся удалить родительскую директорию, если она пуста
				gc.removeEmptyParentDirs(filepath.Dir(logPath.String))
			}
		}

		// Помечаем прогон как удалённый
		_, err := gc.db.ExecContext(ctx, `
			UPDATE runs 
			SET deleted_at = now(),
				log_path = NULL
			WHERE id = $1
		`, runID)
		if err != nil {
			log.Printf("GC: не удалось пометить прогон %s удалённым: %v", runID, err)
			continue
		}

		deletedCount++
	}

	if deletedCount > 0 {
		log.Printf("GC: удалено %d прогонов с истёкшим TTL", deletedCount)
	}
}

// Удаляет временные артефакты
func (gc *GarbageCollector) cleanupConsumedArtifacts(ctx context.Context) {
	// Находим израсходованные артефакты старше 1 часа
	rows, err := gc.db.QueryContext(ctx, `
		SELECT id, stored_path 
		FROM artifacts 
		WHERE consumed_at IS NOT NULL 
		  AND consumed_at < now() - interval '1 hour'
		  AND deleted_at IS NULL
		LIMIT 100
	`)
	if err != nil {
		log.Printf("GC: ошибка получения артефактов: %v", err)
		return
	}
	defer rows.Close()

	var deletedCount int
	for rows.Next() {
		var artifactID, storedPath string

		if err := rows.Scan(&artifactID, &storedPath); err != nil {
			log.Printf("GC: ошибка сканирования артефакта: %v", err)
			continue
		}

		// Удаляем файл артефакта
		if err := os.Remove(storedPath); err != nil && !os.IsNotExist(err) {
			log.Printf("GC: не удалось удалить артефакт %s: %v", storedPath, err)
		} else {
			// Пытаемся удалить родительскую директорию, если она пуста
			gc.removeEmptyParentDirs(filepath.Dir(storedPath))
		}

		// Помечаем артефакт как удалённый
		_, err := gc.db.ExecContext(ctx, `
			UPDATE artifacts 
			SET deleted_at = now()
			WHERE id = $1
		`, artifactID)
		if err != nil {
			log.Printf("GC: не удалось пометить артефакт %s удалённым: %v", artifactID, err)
			continue
		}

		deletedCount++
	}

	if deletedCount > 0 {
		log.Printf("GC: удалено %d временных артефактов", deletedCount)
	}
}

// Рекурсивно удаляет пустые родительские директории
// Останавливается на базовых директориях (/data/runs, /data/artifacts) или при ошибке
func (gc *GarbageCollector) removeEmptyParentDirs(dir string) {
	// Защита от удаления системных директорий
	baseRuns := "/data/runs"
	baseArtifacts := "/data/artifacts"

	for dir != "" && dir != "/" && dir != baseRuns && dir != baseArtifacts {
		// Пытаемся удалить директорию (удалится только если пустая)
		err := os.Remove(dir)
		if err != nil {
			// Директория не пуста или другая ошибка — прекращаем
			break
		}
		log.Printf("GC: удалена пустая директория %s", dir)
		// Поднимаемся на уровень выше
		dir = filepath.Dir(dir)
	}
}

// Удаляет старые завершённые задания
func (gc *GarbageCollector) cleanupOldJobs(ctx context.Context) {
	result, err := gc.db.ExecContext(ctx, `
		DELETE FROM jobs 
		WHERE status IN ('done', 'failed')
		  AND updated_at < now() - interval '24 hours'
	`)
	if err != nil {
		log.Printf("GC: ошибка удаления старых заданий: %v", err)
		return
	}

	if affected, _ := result.RowsAffected(); affected > 0 {
		log.Printf("GC: удалено %d старых заданий", affected)
	}
}
