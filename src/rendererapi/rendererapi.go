package rendererapi

import (
	"database/sql"
	"log"
	"net/http"
	"sync"

	"github.com/LeoMarche/blenderer/src/node"
	"github.com/LeoMarche/blenderer/src/render"
)

//WorkingSet contains variables for main to work
type WorkingSet struct {
	Db             *sql.DB
	Uploading      []*render.Task
	Waiting        []*render.Task
	Completed      []*render.Task
	RenderNodes    []*node.Node
	Renders        []*Render
	Config         Configuration
	UploadingMutex sync.Mutex
	WaitingMutex   sync.Mutex
	CompletedMutex sync.Mutex
	RendersMutex   sync.Mutex
}

type returnvalue struct {
	State string
}

func getIP(r *http.Request) string {
	forwarded := r.Header.Get("X-FORWARDED-FOR")
	if forwarded != "" {
		return forwarded
	}
	return r.RemoteAddr
}

func isIn(s string, t []string) int {

	for i := 0; i < len(t); i++ {
		if t[i] == s {
			return i
		}
	}

	return -1
}

func isInInterface(t *[]interface{}, te interface{}) bool {
	for i := 0; i < len(*t); i++ {
		if (*t)[i] == te {
			return true
		}
	}

	return false
}

//Configuration is the main configuration
type Configuration struct {
	Folder      string
	DBName      string
	Certname    string
	UserAPIKeys []string
}

//Upload allows client to upload file
type Upload struct {
	Token   string
	Project string
	State   string
}

//Render is the base descriptor of a render
type Render struct {
	myTask  *render.Task
	myNode  *node.Node
	Percent string
	Mem     string
}

//GetState returns the state of the myTask of the Render Object
func (r *Render) GetState() string {
	return r.myTask.State
}

//This function updates states in database using state of objects
func (r *Render) updateDatabase(db *sql.DB) {

	reqTask := "UPDATE projects SET state = ? WHERE project = ? AND id = ? AND frame = ?"
	reqNode := "UPDATE compute_nodes SET state = ? WHERE name = ? AND ip = ?"

	tx, e := db.Begin()
	if e != nil {
		log.Fatal(e.Error())
	}
	statement, err := db.Prepare(reqTask)
	if err != nil {
		tx.Rollback()
		log.Fatal(err.Error())
	}
	statement.Exec(r.myTask.State, r.myTask.Project, r.myTask.ID, r.myTask.Frame)

	statement, err = db.Prepare(reqNode)
	if err != nil {
		tx.Rollback()
		log.Fatal(err.Error())
	}
	statement.Exec(r.myNode.State(), r.myNode.Name, r.myNode.IP)
	tx.Commit()
}

//Updates a project in database
func updateProjectinDB(db *sql.DB, rt *render.Task) {
	reqTask := "UPDATE projects SET state = ? WHERE project = ? AND id = ? AND frame = ?"
	tx, e := db.Begin()
	if e != nil {
		log.Fatal(e.Error())
	}
	statement, err := db.Prepare(reqTask)
	if err != nil {
		tx.Rollback()
		log.Fatal(err.Error())
	}
	statement.Exec(rt.State, rt.Project, rt.ID, rt.Frame)
	tx.Commit()
}

//Insert a list of projects in database
func insertProjectsInDB(db *sql.DB, it []*render.Task) {
	rq := "INSERT INTO projects VALUES(?,?,?,?,?,?,?,?,?)"
	tx, e := db.Begin()
	if e != nil {
		log.Fatal(e.Error())
	}
	statement, err := db.Prepare(rq)
	for i := 0; i < len(it); i++ {
		statement.Exec(
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
			log.Fatal(err.Error())
		}
	}
	err = tx.Commit()
}
