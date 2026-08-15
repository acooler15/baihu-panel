package main

import (
	"os"

	"github.com/engigu/baihu-panel/cmd"
)

func main() {
	cmd.InitHandlers()
	cmd.Execute(os.Args)
}
