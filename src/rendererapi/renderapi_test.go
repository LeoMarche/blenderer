package rendererapi

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/LeoMarche/blenderer/src/node"
	"github.com/LeoMarche/blenderer/src/render"
	fifo "github.com/foize/go.fifo"

	"github.com/LeoMarche/blenderer/src/rendererdb"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

const DefaultRemoteAddr = "1.2.3.4"

func copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	buf := make([]byte, 1024)
	for {
		n, err := source.Read(buf)
		if err != nil && err != io.EOF {
			return 0, err
		}
		if n == 0 {
			break
		}

		if _, err := destination.Write(buf[:n]); err != nil {
			return 0, err
		}
	}
	return 0, err
}

func TestGetIP(t *testing.T) {
	assert := assert.New(t)
	r := httptest.NewRequest("GET", "/wouwou", nil)
	r.RemoteAddr = "1.2.3.4"
	ip := getIP(r)
	assert.Equal(ip, "1.2.3.4")
}

func TestAbortJob(t *testing.T) {
	assert := assert.New(t)

	dataTab := []url.Values{{}, {}}
	//Creating request and recorder
	dataTab[0].Set("api_key", "test_api")
	dataTab[0].Set("name", "localhost")
	dataTab[0].Set("id", "test_api")

	dataTab[1].Set("api_key", "test_api")
	dataTab[1].Set("name", "localhost")
	dataTab[1].Set("id", "false_test_api")

	expectedNodeState := []string{"rendering", "rendering"}
	expectedReturnCode := []string{"OK", "error: can't find job"}
	expectedRenderState := []string{"abort", "rendering"}

	for i := 0; i < len(dataTab); i++ {

		//Creating ws for handling
		cg := Configuration{
			Folder:      "",
			DBName:      "",
			Certname:    "",
			UserAPIKeys: []string{"test_api"},
		}

		nd := &node.Node{
			Name:   "localhost",
			IP:     "127.0.0.1",
			APIKey: "test_api",
		}
		nd.SetState("rendering")

		tas := &render.Task{
			Project:         "cube",
			ID:              "test_api",
			Input:           "cube.blend",
			Output:          "cube.blend",
			Frame:           127,
			State:           "rendering",
			RendererName:    "blender",
			RendererVersion: "2.91.0",
		}

		rd := &Render{
			myTask:  tas,
			myNode:  nd,
			Percent: "0",
			Mem:     "0",
		}

		nodesT := new(sync.Map)
		nodesT.Store(nd.Name+"//"+nd.IP, nd)

		rendersT := new(sync.Map)
		newRenderMap := new(sync.Map)
		tmpRenderMap, _ := rendersT.LoadOrStore(tas.ID, newRenderMap)
		tmpRenderMap.(*sync.Map).Store(tas.Frame, rd)

		tasksT := new(sync.Map)
		newMap := new(sync.Map)
		tmpMap, _ := tasksT.LoadOrStore(tas.ID, newMap)
		tmpMap.(*sync.Map).Store(tas.Frame, tas)

		DBT := fifo.NewQueue()

		ws := WorkingSet{
			Config:      cg,
			RenderNodes: nodesT,
			Renders:     rendersT,
			Tasks:       tasksT,
			DBTransacts: DBT,
		}

		tas.SetState("rendering")
		nd.SetState("rendering")

		//Creating request
		r, _ := http.NewRequest(http.MethodPost, "https://127.0.0.1/abortJob", strings.NewReader(dataTab[i].Encode()))
		r.RemoteAddr = "127.0.0.1:1001"
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Add("Content-Length", strconv.Itoa(len(dataTab[i].Encode())))

		w := httptest.NewRecorder()

		//Running handler
		ws.AbortJob(w, r)
		resp := w.Result()
		body, _ := ioutil.ReadAll(resp.Body)
		dt := new(ReturnValue)
		json.Unmarshal(body, dt)

		//Asserts
		testNode, ok := nodesT.Load("localhost" + "//127.0.0.1")
		assert.Equal(true, ok, "Couldn't retrieve the node after posting it in test %d", i)
		assert.Equal("application/json", resp.Header.Get("Content-Type"), "Bad header in test %d", i)
		assert.Equal(expectedNodeState[i], testNode.(*node.Node).State(), "Bad state assigned to node in test %d", i)
		assert.Equal(expectedReturnCode[i], dt.State, "Bad state assigned to task in test %d", i)
		assert.Equal(expectedRenderState[i], tas.State, "Invalid task state after aborting in test %d, i")
	}
}

func TestErrorNode(t *testing.T) {
	assert := assert.New(t)

	dataTab := []url.Values{{}, {}}
	//Creating request and recorder
	dataTab[0].Set("api_key", "test_api")
	dataTab[0].Set("name", "localhost")

	dataTab[1].Set("api_key", "test_api")
	dataTab[1].Set("name", "localhost2")

	expectedNodeState := []string{"error", "rendering"}
	expectedReturnCode := []string{"Done", "Couldn't find matching node"}
	expectedRenderState := []string{"waiting", "rendering"}
	expectedRenderDeleted := []bool{true, false}

	for i := 0; i < len(dataTab); i++ {

		//Creating ws for handling
		cg := Configuration{
			Folder:      "",
			DBName:      "",
			Certname:    "",
			UserAPIKeys: []string{"test_api"},
		}

		nd := &node.Node{
			Name:   "localhost",
			IP:     "127.0.0.1",
			APIKey: "test_api",
		}
		nd.SetState("rendering")

		tas := &render.Task{
			Project:         "cube",
			ID:              "test_api",
			Input:           "cube.blend",
			Output:          "cube.blend",
			Frame:           127,
			State:           "rendering",
			RendererName:    "blender",
			RendererVersion: "2.91.0",
		}

		rd := &Render{
			myTask:  tas,
			myNode:  nd,
			Percent: "0",
			Mem:     "0",
		}

		nodesT := new(sync.Map)
		nodesT.Store(nd.Name+"//"+nd.IP, nd)

		rendersT := new(sync.Map)
		newRenderMap := new(sync.Map)
		tmpRenderMap, _ := rendersT.LoadOrStore(tas.ID, newRenderMap)
		tmpRenderMap.(*sync.Map).Store(tas.Frame, rd)

		tasksT := new(sync.Map)
		newMap := new(sync.Map)
		tmpMap, _ := tasksT.LoadOrStore(tas.ID, newMap)
		tmpMap.(*sync.Map).Store(tas.Frame, tas)

		DBT := fifo.NewQueue()

		ws := WorkingSet{
			Config:      cg,
			RenderNodes: nodesT,
			Renders:     rendersT,
			Tasks:       tasksT,
			DBTransacts: DBT,
		}

		//Creating request
		r, _ := http.NewRequest(http.MethodPost, "https://127.0.0.1/errorNode", strings.NewReader(dataTab[i].Encode()))
		r.RemoteAddr = "127.0.0.1:1001"
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Add("Content-Length", strconv.Itoa(len(dataTab[i].Encode())))

		w := httptest.NewRecorder()

		//Running handler
		ws.ErrorNode(w, r)
		resp := w.Result()
		body, _ := ioutil.ReadAll(resp.Body)
		dt := new(ReturnValue)
		json.Unmarshal(body, dt)

		//Asserts
		testNode, ok := nodesT.Load("localhost" + "//127.0.0.1")
		assert.Equal(true, ok, "Couldn't retrieve the node after posting it in test %d", i)
		assert.Equal("application/json", resp.Header.Get("Content-Type"), "Bad header in test %d", i)
		assert.Equal(expectedNodeState[i], testNode.(*node.Node).State(), "Bad state assigned to node in test %d", i)
		assert.Equal(expectedReturnCode[i], dt.State, "Bad state assigned to task in test %d", i)
		assert.Equal(expectedRenderState[i], tas.State, "Bad state assigner to task when putting node in error in test %d", i)
		tmpRd, ok := rendersT.Load(tas.ID)
		if expectedRenderDeleted[i] {
			if ok {
				_, ok2 := tmpRd.(*sync.Map).Load(tas.Frame)
				assert.Equal(false, ok2, "Render should have been deleted but is not in test %d", i)
			}
		} else {
			assert.Equal(true, ok, "Render has been deleted and it shouldn't have in test %d", i)
			if ok {
				_, ok2 := tmpRd.(*sync.Map).Load(tas.Frame)
				assert.Equal(true, ok2, "Render has been deleted and it shouldn't have in test %d", i)
			}
		}
	}
}

func TestGetAllRenders(t *testing.T) {
	assert := assert.New(t)

	dataTab := []url.Values{{}, {}}
	//Creating request and recorder
	dataTab[0].Set("api_key", "test_api")
	dataTab[1].Set("api_key", "wrong_test_api")

	expectedReturn0 := []TaskToSend{{
		Project:   "cube",
		ID:        "test_api",
		Percent:   5,
		Nb:        1,
		StartTime: "",
	}}
	expectedReturn1 := []TaskToSend{{}}

	expectedReturn := [][]TaskToSend{expectedReturn0, expectedReturn1}
	expectedCode := []int{200, 404}

	for i := 0; i < len(dataTab); i++ {

		//Creating ws for handling
		cg := Configuration{
			Folder:      "",
			DBName:      "",
			Certname:    "",
			UserAPIKeys: []string{"test_api"},
		}

		nd := &node.Node{
			Name:   "localhost",
			IP:     "127.0.0.1",
			APIKey: "test_api",
		}
		nd.SetState("down")

		tas := &render.Task{
			Project:         "cube",
			ID:              "test_api",
			Input:           "cube.blend",
			Output:          "cube.blend",
			Frame:           127,
			State:           "rendering",
			RendererName:    "blender",
			RendererVersion: "2.91.0",
		}

		rd := &Render{
			myTask:  tas,
			myNode:  nd,
			Percent: "5.0",
			Mem:     "150",
		}

		nodesT := new(sync.Map)
		nodesT.Store(nd.Name+"//"+nd.IP, nd)

		rendersT := new(sync.Map)
		newRendersMap := new(sync.Map)
		tmpRendersMap, _ := rendersT.LoadOrStore(tas.ID, newRendersMap)
		tmpRendersMap.(*sync.Map).Store(tas.Frame, rd)

		tasksT := new(sync.Map)
		newMap := new(sync.Map)
		tmpMap, _ := tasksT.LoadOrStore(tas.ID, newMap)
		tmpMap.(*sync.Map).Store(tas.Frame, tas)

		DBT := fifo.NewQueue()

		ws := WorkingSet{
			Config:      cg,
			RenderNodes: nodesT,
			Renders:     rendersT,
			Tasks:       tasksT,
			DBTransacts: DBT,
		}

		//Creating request
		r, _ := http.NewRequest(http.MethodPost, "https://127.0.0.1/getAllRenderTasks", strings.NewReader(dataTab[i].Encode()))
		r.RemoteAddr = "127.0.0.1:1001"
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Add("Content-Length", strconv.Itoa(len(dataTab[i].Encode())))

		w := httptest.NewRecorder()

		//Running handler
		ws.GetAllRenderTasks(w, r)
		resp := w.Result()
		body, _ := ioutil.ReadAll(resp.Body)
		dt := new([]TaskToSend)
		json.Unmarshal(body, dt)

		//Asserts
		assert.Equal(expectedCode[i], resp.StatusCode, "Bad HTML response code, %d instead of %d", resp.StatusCode, expectedCode[i])
		if expectedCode[i] == 200 {
			assert.Equal(expectedReturn[i], *dt, "Bad return in test %d", i)
		}
	}
}

func TestGetJob(t *testing.T) {

	assert := assert.New(t)

	dataTab := []url.Values{{}}

	os.MkdirAll("../../testdata/rendererapi_tests/getJob", os.ModePerm)
	copy("../../testdata/rendererapi_tests/testGetJob.sql", "../../testdata/rendererapi_tests/getJob/testGetJob.sql")

	//Creating request and recorder
	dataTab[0].Set("api_key", "test_api")
	dataTab[0].Set("name", "localhost")

	expectedNodeState := []string{"rendering"}
	expectedFrameState := []string{"rendering"}

	for i := 0; i < len(dataTab); i++ {

		//Creating ws for handling
		cg := Configuration{
			Folder:      "",
			DBName:      "../../testdata/rendererapi_tests/getJob/testGetJob.sql",
			Certname:    "",
			UserAPIKeys: []string{"test_api"},
		}

		db, _ := rendererdb.LoadDatabase(cg.DBName)

		nd := &node.Node{
			Name:   "localhost",
			IP:     "127.0.0.1",
			APIKey: "test_api",
		}
		nd.SetState("available")

		tas := &render.Task{
			Project:         "cube",
			ID:              "test_api",
			Input:           "cube.blend",
			Output:          "cube.blend",
			Frame:           127,
			State:           "waiting",
			RendererName:    "blender",
			RendererVersion: "2.91.0",
		}

		nodesT := new(sync.Map)
		nodesT.Store(nd.Name+"//"+nd.IP, nd)

		rendersT := new(sync.Map)

		tasksT := new(sync.Map)
		newMap := new(sync.Map)
		tmpMap, _ := tasksT.LoadOrStore(tas.ID, newMap)
		tmpMap.(*sync.Map).Store(tas.Frame, tas)

		DBT := fifo.NewQueue()

		ws := WorkingSet{
			Db:          db,
			Config:      cg,
			RenderNodes: nodesT,
			Renders:     rendersT,
			Tasks:       tasksT,
			DBTransacts: DBT,
		}

		//Creating request
		r, _ := http.NewRequest(http.MethodPost, "https://127.0.0.1/getJob", strings.NewReader(dataTab[i].Encode()))
		r.RemoteAddr = "127.0.0.1:1001"
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Add("Content-Length", strconv.Itoa(len(dataTab[i].Encode())))

		w := httptest.NewRecorder()

		//Running handler
		ws.GetJob(w, r)
		resp := w.Result()
		body, _ := ioutil.ReadAll(resp.Body)
		dt := new(render.Task)
		json.Unmarshal(body, dt)

		//Asserts
		assert.Equal("application/json", resp.Header.Get("Content-Type"), "Bad header in test %d", i)
		assert.Equal(tas.ID, dt.ID, "Bad task returned")
		assert.Equal(tas.Frame, dt.Frame, "Bad task returned")
		assert.Equal(expectedNodeState[i], nd.State(), "Bad state assigned to node")
		assert.Equal(expectedFrameState[i], tas.State, "Bad state assigned to task")
	}

	os.RemoveAll("../../testdata/rendererapi_tests/getJob")
}

func TestPostJob(t *testing.T) {
	assert := assert.New(t)

	dataTab := []url.Values{{}}

	//Creating request and recorder
	//The request must be a post with api_key, project, input, output, frameStart, frameStop, rendererName, rendererVersion, startTime
	dataTab[0].Set("api_key", "test_api")
	dataTab[0].Set("name", "localhost")
	dataTab[0].Set("project", "cube")
	dataTab[0].Set("input", "cube.blend")
	dataTab[0].Set("output", "cube.blend")
	dataTab[0].Set("frameStart", "127")
	dataTab[0].Set("frameStop", "128")
	dataTab[0].Set("rendererName", "blender")
	dataTab[0].Set("rendererVersion", "2.91.0")
	dataTab[0].Set("startTime", "123")

	expectedReturnCode := []int{200}
	expectedUploadState := []string{"ready"}

	for i := 0; i < len(dataTab); i++ {

		//Creating ws for handling
		cg := Configuration{
			Folder:      "",
			DBName:      "../../testdata/rendererapi_tests/updateJob/testUpdateJob.sql",
			Certname:    "",
			UserAPIKeys: []string{"test_api"},
		}

		db, _ := rendererdb.LoadDatabase(cg.DBName)

		nd := &node.Node{
			Name:   "localhost",
			IP:     "127.0.0.1",
			APIKey: "test_api",
		}
		nd.SetState("rendering")

		nodesT := new(sync.Map)
		nodesT.Store(nd.Name+"//"+nd.IP, nd)

		rendersT := new(sync.Map)

		tasksT := new(sync.Map)

		DBT := fifo.NewQueue()
		ws := WorkingSet{
			Db:          db,
			Config:      cg,
			RenderNodes: nodesT,
			Renders:     rendersT,
			Tasks:       tasksT,
			DBTransacts: DBT,
		}

		//Creating request
		r, _ := http.NewRequest(http.MethodPost, "https://127.0.0.1/postJob", strings.NewReader(dataTab[i].Encode()))
		r.RemoteAddr = "127.0.0.1:1001"
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Add("Content-Length", strconv.Itoa(len(dataTab[i].Encode())))

		w := httptest.NewRecorder()

		//Running handler
		ws.PostJob(w, r)
		resp := w.Result()
		body, _ := ioutil.ReadAll(resp.Body)
		dt := new(Upload)
		json.Unmarshal(body, dt)

		//Asserts
		assert.Equal(expectedReturnCode[i], resp.StatusCode, "Bad status code in test %d : %d instead of %d", i, resp.StatusCode, expectedReturnCode[i])
		if expectedReturnCode[i] == 200 {
			assert.Equal(expectedUploadState[i], dt.State, "Bad state of upload in test %d", i)
		}
	}
}

func TestPostNode(t *testing.T) {
	assert := assert.New(t)

	dataTab := []url.Values{{}, {}}
	//Creating request and recorder
	dataTab[0].Set("api_key", "test_api")
	dataTab[0].Set("name", "localhost")

	dataTab[1].Set("api_key", "test_api")
	dataTab[1].Set("name", "localhost2")

	expectedNodeState := []string{"available", "available"}
	expectedReturnCode := []string{"Exists", "Added"}
	nodeName := []string{"localhost", "localhost2"}
	expectedDBOP := []int{rendererdb.UPDATENODE, rendererdb.INSERTNODE}

	for i := 0; i < len(dataTab); i++ {

		//Creating ws for handling
		cg := Configuration{
			Folder:      "",
			DBName:      "",
			Certname:    "",
			UserAPIKeys: []string{"test_api"},
		}

		nd := &node.Node{
			Name:   "localhost",
			IP:     "127.0.0.1",
			APIKey: "test_api",
		}
		nd.SetState("down")

		tas := &render.Task{
			Project:         "cube",
			ID:              "test_api",
			Input:           "cube.blend",
			Output:          "cube.blend",
			Frame:           127,
			State:           "waiting",
			RendererName:    "blender",
			RendererVersion: "2.91.0",
		}

		nodesT := new(sync.Map)
		nodesT.Store(nd.Name+"//"+nd.IP, nd)

		rendersT := new(sync.Map)

		tasksT := new(sync.Map)
		newMap := new(sync.Map)
		tmpMap, _ := tasksT.LoadOrStore(tas.ID, newMap)
		tmpMap.(*sync.Map).Store(tas.Frame, tas)

		DBT := fifo.NewQueue()

		ws := WorkingSet{
			Config:      cg,
			RenderNodes: nodesT,
			Renders:     rendersT,
			Tasks:       tasksT,
			DBTransacts: DBT,
		}

		//Creating request
		r, _ := http.NewRequest(http.MethodPost, "https://127.0.0.1/postNode", strings.NewReader(dataTab[i].Encode()))
		r.RemoteAddr = "127.0.0.1:1001"
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Add("Content-Length", strconv.Itoa(len(dataTab[i].Encode())))

		w := httptest.NewRecorder()

		//Running handler
		ws.PostNode(w, r)
		resp := w.Result()
		body, _ := ioutil.ReadAll(resp.Body)
		dt := new(ReturnValue)
		json.Unmarshal(body, dt)

		//Asserts
		testNode, ok := nodesT.Load(nodeName[i] + "//127.0.0.1")
		assert.Equal(true, ok, "Couldn't retrieve the node after posting it in test %d", i)
		assert.Equal("application/json", resp.Header.Get("Content-Type"), "Bad header in test %d", i)
		assert.Equal(expectedNodeState[i], testNode.(*node.Node).State(), "Bad state assigned to node in test %d", i)
		assert.Equal(expectedReturnCode[i], dt.State, "Bad state assigned to task in test %d", i)
		currentTrans := ws.DBTransacts.Next()
		assert.NotEqual(nil, currentTrans, "Couldn't retrieve the DBTransaction associated withe postNode, test n°%d", i)
		assert.Equal(expectedDBOP[i], currentTrans.(rendererdb.DBTransact).OP, "The DBTransaction created by test %d isn't correct", i)
		assert.Equal(testNode.(*node.Node), currentTrans.(rendererdb.DBTransact).Argument, "Bad argument passed to the DBtransaction in test %d", i)
	}
}

func TestSetAvailable(t *testing.T) {
	assert := assert.New(t)

	dataTab := []url.Values{{}, {}}
	//Creating request and recorder
	dataTab[0].Set("api_key", "test_api")
	dataTab[0].Set("name", "localhost")

	dataTab[1].Set("api_key", "test_api")
	dataTab[1].Set("name", "localhost2")

	expectReturnCode := []int{200, 200}
	expectNodeStatus := []string{"available", "down"}
	expectedReturn := []string{"OK", "Can't find node"}

	for i := 0; i < len(dataTab); i++ {

		//Creating ws for handling
		cg := Configuration{
			Folder:      "",
			DBName:      "",
			Certname:    "",
			UserAPIKeys: []string{"test_api"},
		}

		nd := &node.Node{
			Name:   "localhost",
			IP:     "127.0.0.1",
			APIKey: "test_api",
		}
		nd.SetState("down")

		nodesT := new(sync.Map)
		nodesT.Store(nd.Name+"//"+nd.IP, nd)

		rendersT := new(sync.Map)

		tasksT := new(sync.Map)

		DBT := fifo.NewQueue()

		ws := WorkingSet{
			Config:      cg,
			RenderNodes: nodesT,
			Renders:     rendersT,
			Tasks:       tasksT,
			DBTransacts: DBT,
		}

		//Creating request
		r, _ := http.NewRequest(http.MethodPost, "https://127.0.0.1/setAvailable", strings.NewReader(dataTab[i].Encode()))
		r.RemoteAddr = "127.0.0.1:1001"
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Add("Content-Length", strconv.Itoa(len(dataTab[i].Encode())))

		w := httptest.NewRecorder()

		//Running handler
		ws.SetAvailable(w, r)
		resp := w.Result()
		body, _ := ioutil.ReadAll(resp.Body)
		dt := new(ReturnValue)
		json.Unmarshal(body, dt)

		//Asserts
		testNode, ok := nodesT.Load("localhost" + "//127.0.0.1")
		assert.Equal(true, ok, "Couldn't retrieve the node after posting it in test %d", i)
		assert.Equal("application/json", resp.Header.Get("Content-Type"), "Bad header in test %d", i)
		assert.Equal(expectReturnCode[i], resp.StatusCode, "Bad status code : %d instead of %d in test %d", resp.StatusCode, expectReturnCode[i], i)
		assert.Equal(expectNodeStatus[i], testNode.(*node.Node).State(), "Bad node status in tes %d", i)
		assert.Equal(expectedReturn[i], dt.State, "Bad return state %s instead of %s in test %d", dt.State, expectedReturn[i], i)
	}
}

func TestUpdateJob(t *testing.T) {

	assert := assert.New(t)

	dataTab := []url.Values{{}, {}, {}, {}, {}}

	os.MkdirAll("../../testdata/rendererapi_tests/updateJob", os.ModePerm)
	copy("../../testdata/rendererapi_tests/testUpdateJob.sql", "../../testdata/rendererapi_tests/updateJob/testUpdateJob.sql")

	//Creating request and recorder
	dataTab[0].Set("api_key", "test_api")
	dataTab[0].Set("id", "test_api")
	dataTab[0].Set("frame", "127")
	dataTab[0].Set("state", "rendering")
	dataTab[0].Set("percent", "1.0")
	dataTab[0].Set("mem", "2.0")
	dataTab[0].Set("name", "localhost")

	dataTab[1].Set("api_key", "test_api")
	dataTab[1].Set("id", "test_api")
	dataTab[1].Set("frame", "128")
	dataTab[1].Set("state", "rendering")
	dataTab[1].Set("percent", "10.0")
	dataTab[1].Set("mem", "10.0")
	dataTab[1].Set("name", "localhost")

	dataTab[2].Set("api_key", "test_api")
	dataTab[2].Set("id", "test_api")
	dataTab[2].Set("frame", "127")
	dataTab[2].Set("state", "rendered")
	dataTab[2].Set("percent", "100.0")
	dataTab[2].Set("mem", "0.0")
	dataTab[2].Set("name", "localhost")

	dataTab[3].Set("api_key", "test_api")
	dataTab[3].Set("id", "wrong_test_api")
	dataTab[3].Set("frame", "127")
	dataTab[3].Set("state", "rendered")
	dataTab[3].Set("percent", "100.0")
	dataTab[3].Set("mem", "0.0")
	dataTab[3].Set("name", "localhost")

	dataTab[4].Set("api_key", "test_api")
	dataTab[4].Set("frame", "127")
	dataTab[4].Set("state", "rendered")
	dataTab[4].Set("percent", "100.0")
	dataTab[4].Set("mem", "0.0")
	dataTab[4].Set("name", "localhost")

	expectedMem := []string{"2.0", "0.0", "0.0", "0.0", "0.0"}
	expectedPercent := []string{"1.0", "0.0", "100.0", "0.0", "0.0"}
	expectedReturn := []ReturnValue{
		{State: "OK"},
		{State: "Error : No matching Renders"},
		{State: "OK"},
		{State: "Error : No matching Renders"},
		{State: "Error : Missing Parameter"}}
	expectedNodeState := []string{"rendering", "rendering", "available", "rendering", "rendering"}
	expectedState := []string{"rendering", "rendering", "rendered", "rendering", "rendering"}

	for i := 0; i < len(dataTab); i++ {

		//Creating ws for handling
		cg := Configuration{
			Folder:      "",
			DBName:      "../../testdata/rendererapi_tests/updateJob/testUpdateJob.sql",
			Certname:    "",
			UserAPIKeys: []string{"test_api"},
		}

		db, _ := rendererdb.LoadDatabase(cg.DBName)

		nd := &node.Node{
			Name:   "localhost",
			IP:     "127.0.0.1",
			APIKey: "test_api",
		}
		nd.SetState("rendering")

		tas := &render.Task{
			Project:         "cube",
			ID:              "test_api",
			Input:           "cube.blend",
			Output:          "cube.blend",
			Frame:           127,
			State:           "rendering",
			RendererName:    "blender",
			RendererVersion: "2.91.0",
		}

		rd := &Render{
			myTask:  tas,
			myNode:  nd,
			Percent: "0.0",
			Mem:     "0.0",
		}

		nodesT := new(sync.Map)
		nodesT.Store(nd.Name+"//"+nd.IP, nd)

		rendersT := new(sync.Map)
		newMap := new(sync.Map)
		tmpMap, _ := rendersT.LoadOrStore(tas.ID, newMap)
		tmpMap.(*sync.Map).Store(tas.Frame, rd)

		tasksT := new(sync.Map)
		newMap2 := new(sync.Map)
		tmpMap2, _ := tasksT.LoadOrStore(tas.ID, newMap2)
		tmpMap2.(*sync.Map).Store(tas.Frame, tas)

		DBT := fifo.NewQueue()
		ws := WorkingSet{
			Db:          db,
			Config:      cg,
			RenderNodes: nodesT,
			Renders:     rendersT,
			Tasks:       tasksT,
			DBTransacts: DBT,
		}

		//Creating request
		r, _ := http.NewRequest(http.MethodPost, "https://127.0.0.1/updateJob", strings.NewReader(dataTab[i].Encode()))
		r.RemoteAddr = "127.0.0.1:1001"
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Add("Content-Length", strconv.Itoa(len(dataTab[i].Encode())))

		w := httptest.NewRecorder()

		//Running handler
		ws.UpdateJob(w, r)
		resp := w.Result()
		body, _ := ioutil.ReadAll(resp.Body)
		dt := new(ReturnValue)
		json.Unmarshal(body, dt)

		//Asserts
		assert.Equal("application/json", resp.Header.Get("Content-Type"), "Bad header in test %d", i)
		assert.Equal(expectedMem[i], rd.Mem, "Bad value for memory in test %d", i)
		assert.Equal(expectedPercent[i], rd.Percent, "Bad value for percents in test %d", i)
		assert.Equal(expectedReturn[i], *dt, "Bad result returned in test %d", i)
		assert.Equal(expectedState[i], rd.myTask.State)
		assert.Equal(expectedNodeState[i], rd.myNode.State())
	}

	os.RemoveAll("../../testdata/rendererapi_tests/updateJob")
}
