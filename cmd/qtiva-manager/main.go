package main

import (
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/lildannita/qtiva-cloud/internal/commonx"
	"github.com/lildannita/qtiva-cloud/internal/manager"
)

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	root := &cobra.Command{
		Use:   "qtiva-manager",
		Short: "qtiva-manager — HTTP API сервис управления прогонами GUI-тестирования",
		// SilenceUsage отключает повторный вывод usage при runtime-ошибках команды
		SilenceUsage: true,
	}
	commonx.RegisterVersionCommand(root)
	manager.RegisterServeCommand(root)
	manager.RegisterSeedUserCommand(root)

	if err := root.Execute(); err != nil {
		// Cobra уже печатает понятную ошибку, нам достаточно code=1
		os.Exit(1)
	}
}
