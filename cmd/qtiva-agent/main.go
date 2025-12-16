package main

import (
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/lildannita/qtiva-cloud/internal/agent"
	"github.com/lildannita/qtiva-cloud/internal/commonx"
)

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	root := &cobra.Command{
		Use:   "qtiva-agent",
		Short: "qtiva-agent — служба, выполняющая задания на запуск тестов",
		// SilenceUsage отключает повторный вывод usage при runtime-ошибках команды
		SilenceUsage: true,
	}
	commonx.RegisterVersionCommand(root)
	agent.RegisterServeCommand(root)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
