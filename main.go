package main

//go:generate sqlboiler crdb --add-soft-deletes

import "go.infratographer.com/permissions-api/cmd"

func main() {
	cmd.Execute()
}
