package main

import (
	"fmt"
	migrations "main/migration"
	"os"
)

func main() {
	urlExample := fmt.Sprintf("postgres://%s:%s@db:5432", os.Getenv("DB_USER"), os.Getenv("DB_NAME"))

	err := migrations.Migrate(urlExample, migrations.LogModeError)
	if err != nil {
		fmt.Println(err)
	}

	start()
}
