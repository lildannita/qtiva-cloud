package manager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/lildannita/qtiva-cloud/internal/authx"
	"github.com/lildannita/qtiva-cloud/internal/commonx"
	"github.com/lildannita/qtiva-cloud/internal/dbx"
	"github.com/lildannita/qtiva-cloud/internal/idgen"
)

// Создаёт администратора, при необходимости заменяет существующего
func RegisterSeedUserCommand(root *cobra.Command) {
	var (
		email    string
		password string
		replace  bool
	)

	cmd := &cobra.Command{
		Use:   "seed-admin --email <email> --password <password> [--replace]",
		Short: "Создать первого администратора через CLI",
		RunE: func(cmd *cobra.Command, args []string) error {
			commonx.LoadDotenvIfDev()

			email = strings.TrimSpace(email)
			if email == "" {
				return errors.New("seed-admin: не задан --email")
			}
			if _, err := mail.ParseAddress(email); err != nil {
				return fmt.Errorf("seed-admin: некорректный email: %w", err)
			}

			password = strings.TrimSpace(password)
			if len(password) < 12 {
				return errors.New("seed-admin: пароль слишком короткий (минимум 12 символов)")
			}
			if len(password) > 256 {
				return errors.New("seed-admin: пароль слишком длинный (максимум 256 символов)")
			}

			db, pingTimeout, err := dbx.OpenFromEnv()
			if err != nil {
				return err
			}
			defer db.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			if err := dbx.Ping(ctx, db, pingTimeout); err != nil {
				return err
			}

			var (
				oldAdminID    string
				oldAdminEmail string
				newAdminID    string
				clientID      string
			)

			err = dbx.WithTx(ctx, db, func(tx *sql.Tx) error {
				// Находим активного админа, если есть
				err := tx.QueryRowContext(ctx,
					`SELECT id, email FROM users WHERE role='admin' AND deleted_at IS NULL LIMIT 1`,
				).Scan(&oldAdminID, &oldAdminEmail)

				if err != nil && !errors.Is(err, sql.ErrNoRows) {
					return fmt.Errorf("seed-admin: select admin: %w", err)
				}

				if err == nil && !replace {
					return errors.New("seed-admin: admin уже существует, используй --replace чтобы заменить")
				}

				// Если replace=true и админ есть
				if err == nil && replace {
					// Тот же email → просто обновляем пароль
					if strings.EqualFold(strings.TrimSpace(oldAdminEmail), email) {
						hash, e := authx.HashPassword(password)
						if e != nil {
							return e
						}

						if _, e := tx.ExecContext(ctx,
							`UPDATE users SET password_hash=$1, updated_at=now() WHERE id=$2`,
							hash, oldAdminID,
						); e != nil {
							return fmt.Errorf("seed-admin: update admin password: %w", e)
						}

						newAdminID = oldAdminID
						_ = tx.QueryRowContext(ctx,
							`SELECT client_id FROM users WHERE id=$1`,
							oldAdminID,
						).Scan(&clientID)

						return nil
					}

					// Email другой → помечаем старого удалённым
					if _, err := tx.ExecContext(ctx,
						`UPDATE users SET deleted_at=now(), updated_at=now() WHERE id=$1`,
						oldAdminID,
					); err != nil {
						return fmt.Errorf("seed-admin: delete old admin: %w", err)
					}
				}

				// Берём первый client, либо создаём базовый
				e := tx.QueryRowContext(ctx,
					`SELECT id FROM clients ORDER BY created_at ASC LIMIT 1`,
				).Scan(&clientID)

				if e != nil {
					if !errors.Is(e, sql.ErrNoRows) {
						return fmt.Errorf("seed-admin: select client: %w", e)
					}
					newClientID, e := idgen.New("cl")
					if e != nil {
						return e
					}
					if _, e := tx.ExecContext(ctx,
						`INSERT INTO clients (id, name, network_allowed, concurrency_limit)
						 VALUES ($1,$2,$3,$4)`,
						newClientID, "System", false, 1,
					); e != nil {
						return fmt.Errorf("seed-admin: insert client: %w", e)
					}
					clientID = newClientID
				}

				// Создаём нового админа
				id, e := idgen.New("usr")
				if e != nil {
					return e
				}
				hash, e := authx.HashPassword(password)
				if e != nil {
					return e
				}

				if _, e := tx.ExecContext(ctx,
					`INSERT INTO users (id, client_id, email, password_hash, role)
					 VALUES ($1,$2,$3,$4,$5)`,
					id, clientID, email, hash, "admin",
				); e != nil {
					return fmt.Errorf("seed-admin: insert admin: %w", e)
				}

				newAdminID = id
				return nil
			})
			if err != nil {
				return err
			}

			// Сообщения
			if replace && oldAdminID != "" && newAdminID == oldAdminID {
				cmd.Printf(
					"admin пароль обновлён: user_id=%s client_id=%s email=%s\n",
					newAdminID, clientID, email,
				)
				return nil
			}

			if oldAdminID != "" && replace {
				cmd.Printf(
					"admin заменён: old=%s new=%s client_id=%s email=%s\n",
					oldAdminID, newAdminID, clientID, email,
				)
				return nil
			}

			cmd.Printf(
				"admin создан: user_id=%s client_id=%s email=%s\n",
				newAdminID, clientID, email,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "Email администратора")
	cmd.Flags().StringVar(&password, "password", "", "Пароль администратора")
	cmd.Flags().BoolVar(&replace, "replace", false, "Заменить существующего администратора")

	_ = cmd.MarkFlagRequired("email")
	_ = cmd.MarkFlagRequired("password")

	root.AddCommand(cmd)
}
