package database

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"log"
)

func Connect(user string, password string, database string, instance string) *sql.DB {
	dsn := fmt.Sprintf("%s:%s@%s/%s", user, password, instance, database)
	db, err := sql.Open("mysql", dsn)

	if err != nil {
		log.Fatalf("Could not open db: %v", err)
	}

	db.SetMaxIdleConns(1)
	db.SetMaxOpenConns(1)

	return db
}
