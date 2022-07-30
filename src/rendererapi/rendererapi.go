package rendererapi

import (
	"database/sql"
	"net/http"
	"sync"

	"github.com/LeoMarche/blenderer/src/node"
	"github.com/LeoMarche/blenderer/src/render"
	"github.com/LeoMarche/blenderer/src/rendererdb"
	fifo "github.com/foize/go.fifo"
)

//WorkingSet contains variables for main to work
type WorkingSet struct {
	Db          *sql.DB
	Tasks       *sync.Map //Index for this map is ID. It contains map with task, indexes are Frame
	RenderNodes *sync.Map //Index for this map is Name+"//"+IP
	Renders     *sync.Map //Index for this map is ID. It contains maps with Render, indexes are Frame
	Config      Configuration
	DBTransacts *fifo.Queue
	StopDB      *bool
}

type ReturnValue struct {
	State string
}

type TaskToSend struct {
	Project   string
	ID        string
	Percent   float64
	Nb        int
	StartTime string
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
func (r *Render) UpdateDatabase(db *sql.DB) {

	rendererdb.UpdateTaskInDB(db, r.myTask)
	rendererdb.UpdateNodeInDB(db, r.myNode)

}
