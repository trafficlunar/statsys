package main

func main() {
	LoadConfig()
	InitializeDatabase()
	defer CloseDatabase()
	StartCheckingStatuses()
	StartHttpServer()
}
