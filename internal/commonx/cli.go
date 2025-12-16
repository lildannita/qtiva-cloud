package commonx

import (
	"github.com/lildannita/qtiva-cloud/internal/buildinfo"
	"github.com/spf13/cobra"
)

func RegisterVersionCommand(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Показать версию и commit сборки",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("version=%s commit=%s\n", buildinfo.Version, buildinfo.Commit)
		},
	})
}
