package main

import "statsys/internal"

func main() {
	internal.ParseFlags()
	internal.LoadConfig()
	internal.InitDatabase()
	defer internal.CloseDatabase()
	internal.StartCheckingStatuses()
	internal.StartHttpServer()
}
