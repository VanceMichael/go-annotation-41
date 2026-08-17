// Command bioctl 是口岸外来物种查验与生物安全处置平台的命令行入口。
package main

import (
	"os"

	"portbio/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args, os.Stdout, os.Stderr))
}
