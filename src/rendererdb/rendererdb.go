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
func LoadDatabase(dbName string) (*sql.DB, error) {

	var dB *sql.DB

	if _, err := os.Stat(dbName); err == nil {
		dB, err = sql.Open("sqlite3", dbName)
		if err != nil {
			return nil, err
		}
	} else {
		file, err := os.Create(dbName)
		if err != nil {
			return nil, err
		}
		file.Close()
		dB, err = sql.Open("sqlite3", dbName)
		if err != nil {
			return nil, err
		}
		createTable(dB)
	}

	return dB, nil
}

//LoadNodeFromDB loads nodes from a sqlite database
func LoadNodeFromDB(db *sql.DB, t *[]*node.Node) error {
	row, err := db.Query("SELECT * FROM compute_nodes")

	if err != nil {
		return err
	}

	defer row.Close()

	for row.Next() { // Iterate and fetch the records from result cursor
		var na, ip, apiKey, st string

		err = row.Scan(&na, &ip, &apiKey, &st)

		if err != nil {
			return err
		}

		*t = append(*t, &node.Node{
			Name:   na,
			IP:     ip,
			APIKey: apiKey,
		})
		(*t)[len(*t)-1].SetState(st)
	}
	return nil
}

//LoadTasksFromDB loads tasks from sqlite db
func LoadTasksFromDB(db *sql.DB, t *[]*render.Task) error {
	row, err := db.Query("SELECT * FROM projects")

	if err != nil {
		return err
	}

	defer row.Close()

	for row.Next() {

		var pr, id, in, ou, st, rN, rV, sT string
		var fr int

		err = row.Scan(&pr, &id, &in, &ou, &fr, &st, &rN, &rV, &sT)

		if err != nil {
			return err
		}

		if st != "failed" && st != "completed" {
			*t = append(*t, &render.Task{
				Project:         pr,
				ID:              id,
				Input:           in,
				Output:          ou,
				Frame:           fr,
				State:           st,
				RendererName:    rN,
				RendererVersion: rV,
				StartTime:       sT,
			})
		}
	}

	return nil
}

//Updates a project in database
func UpdateTaskInDB(db *sql.DB, rt *render.Task) error {
	tx, err := db.Begin()

	if err != nil {
		return err
	}

	reqTask := "UPDATE projects SET state = ? WHERE project = ? AND id = ? AND frame = ?"
	statement, err := db.Prepare(reqTask)

	if err != nil {
		tx.Rollback()
		return nil
	}
	_, err = statement.Exec(rt.State, rt.Project, rt.ID, rt.Frame)

	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func UpdateNodeInDB(db *sql.DB, nd *node.Node) error {
	tx, err := db.Begin()

	if err != nil {
		tx.Rollback()
		return err
	}

	reqNode := "UPDATE compute_nodes SET state = ? WHERE name = ? AND ip = ?"
	statement, err := db.Prepare(reqNode)
	if err != nil {
		return err
	}

	_, err = statement.Exec(nd.State(), nd.Name, nd.IP)

	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

//Insert a list of projects in database
func InsertProjectsInDB(db *sql.DB, it []*render.Task) error {
	tx, err := db.Begin()

	if err != nil {
		return err
	}

	rq := "INSERT INTO projects VALUES(?,?,?,?,?,?,?,?,?)"
	statement, err := db.Prepare(rq)

	if err != nil {
		return err
	}

	for i := 0; i < len(it); i++ {
		_, err = statement.Exec(
			it[i].Project,
			it[i].ID,
			it[i].Input,
			it[i].Output,
			it[i].Frame,
			it[i].State,
			it[i].RendererName,
			it[i].RendererVersion,
			it[i].StartTime)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

func InsertNodeInDB(db *sql.DB, n *node.Node) error {
	tx, err := db.Begin()

	if err != nil {
		return err
	}

	rq := "INSERT INTO compute_nodes VALUES(?,?,?,?)"
	statement, err := db.Prepare(rq)

	if err != nil {
		tx.Rollback()
		return err
	}
	_, err = statement.Exec(n.Name, n.IP, n.APIKey, n.State())

	if err != nil {
		tx.Rollback()
		return nil
	}

	return tx.Commit()
}
