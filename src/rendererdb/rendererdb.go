package rendererdb

import (
	"database/sql"
	"log"
	"os"

	"github.com/LeoMarche/blenderer/src/node"
	"github.com/LeoMarche/blenderer/src/render"
)

//Creates the two needed tables in sql database
func createTable(db *sql.DB) {

	tx, _ := db.Begin()

	createNodeTableSQL := `CREATE TABLE compute_nodes (	
		"name" TEXT,
		"ip" TEXT,
		"api_key" TEXT,
		"state" TEXT	
	  );`

	createProjectTableSQL := `CREATE TABLE projects (
		"project" TEXT,		
		"id" TEXT,
		"input" TEXT,
		"output" TEXT,
		"frame" integer,
		"state" TEXT,
		"rendererName" TEXT,
		"rendererVersion" TEXT,	
		"startTime" TEXT
	  );`

	statement, err := db.Prepare(createNodeTableSQL)
	if err != nil {
		log.Fatal(err.Error())
	}
	statement.Exec()

	statement, err = db.Prepare(createProjectTableSQL)
	if err != nil {
		log.Fatal(err.Error())
	}
	statement.Exec()
	tx.Commit()
}

//LoadDatabase loads a sqlite database from a file
func LoadDatabase(dbName string) *sql.DB {

	var dB *sql.DB

	if _, err := os.Stat(dbName); err == nil {
		dB, err = sql.Open("sqlite3", dbName)
		if err != nil {
			log.Fatal(err.Error())
		}
	} else {
		file, err := os.Create(dbName)
		if err != nil {
			log.Fatal(err.Error())
		}
		file.Close()
		dB, err = sql.Open("sqlite3", dbName)
		if err != nil {
			log.Fatal(err.Error())
		}
		createTable(dB)
	}

	return dB
}

//LoadNodeFromDB loads nodes from a sqlite database
func LoadNodeFromDB(db *sql.DB, t *[]*node.Node) {
	row, err := db.Query("SELECT * FROM compute_nodes")

	if err != nil {
		log.Fatal(err)
	}
	defer row.Close()
	for row.Next() { // Iterate and fetch the records from result cursor
		var na string
		var ip string
		var apiKey string
		var st string
		row.Scan(&na, &ip, &apiKey, &st)
		n := node.Node{
			Name:   na,
			IP:     ip,
			APIKey: apiKey,
		}

		n.SetState(st)

		*t = append(*t, &n)
	}
}

//LoadTasksFromDB loads tasks from sqlite db
func LoadTasksFromDB(db *sql.DB, t *[]*render.Task) {
	row, err := db.Query("SELECT * FROM projects")

	if err != nil {
		log.Fatal(err)
	}
	defer row.Close()
	for row.Next() {

		var pr, id, in, ou, st, rN, rV, sT string
		var fr int

		row.Scan(&pr, &id, &in, &ou, &fr, &st, &rN, &rV, &sT)
		ta := new(render.Task)

		ta.Project = pr
		ta.ID = id
		ta.Input = in
		ta.Output = ou
		ta.Frame = fr
		ta.State = st
		ta.RendererName = rN
		ta.RendererVersion = rV
		ta.StartTime = sT
		if st != "failed" && st != "completed" {
			*t = append(*t, ta)
		}
	}
}
