package database

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"log"
)

func Connect(user string, password string, database string, host string) *sql.DB {
	dsn := fmt.Sprintf("%s:%s@%s/%s?parseTime=true", user, password, host, database)
	db, err := sql.Open("mysql", dsn)

	if err != nil {
		log.Fatalf("Could not open db: %v", err)
	}

	db.SetMaxIdleConns(1)
	db.SetMaxOpenConns(2)

	return db
}
