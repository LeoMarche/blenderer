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
	"testing"

	"github.com/LeoMarche/blenderer/src/node"
	"github.com/LeoMarche/blenderer/src/render"

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
	fmt.Println(src)
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

	dataTab[1].Set("api_key", "test_api")
	dataTab[1].Set("id", "test_api")
	dataTab[1].Set("frame", "128")
	dataTab[1].Set("state", "rendering")
	dataTab[1].Set("percent", "10.0")
	dataTab[1].Set("mem", "10.0")

	dataTab[2].Set("api_key", "test_api")
	dataTab[2].Set("id", "test_api")
	dataTab[2].Set("frame", "127")
	dataTab[2].Set("state", "rendered")
	dataTab[2].Set("percent", "100.0")
	dataTab[2].Set("mem", "0.0")

	dataTab[3].Set("api_key", "test_api")
	dataTab[3].Set("id", "test_api")
	dataTab[3].Set("frame", "127")
	dataTab[3].Set("state", "rendered")
	dataTab[3].Set("percent", "100.0")
	dataTab[3].Set("mem", "0.0")

	dataTab[4].Set("api_key", "test_api")
	dataTab[4].Set("frame", "127")
	dataTab[4].Set("state", "rendered")
	dataTab[4].Set("percent", "100.0")
	dataTab[4].Set("mem", "0.0")

	//Creating ws for handling
	cg := Configuration{
		Folder:      "",
		DBName:      "../../testdata/rendererapi_tests/updateJob/testUpdateJob.sql",
		Certname:    "",
		UserAPIKeys: []string{"test_api"},
	}

	db, _ := rendererdb.LoadDatabase(cg.DBName)

	nd := node.Node{
		Name:   "localhost",
		IP:     "127.0.0.1",
		APIKey: "test_api",
	}
	nd.SetState("rendering")

	tas := render.Task{
		Project:         "cube",
		ID:              "test_api",
		Input:           "cube.blend",
		Output:          "cube.blend",
		Frame:           127,
		State:           "rendering",
		RendererName:    "blender",
		RendererVersion: "2.91.0",
	}

	rd := Render{
		myTask:  &tas,
		myNode:  &nd,
		Percent: "0.0",
		Mem:     "0.0",
	}

	rd1 := rd
	rd1.Percent = "1.0"
	rd1.Mem = "2.0"

	ws := WorkingSet{
		Db:          db,
		Config:      cg,
		RenderNodes: []*node.Node{&nd},
		Renders:     []*Render{&rd},
		Uploading:   []*render.Task{},
		Waiting:     []*render.Task{},
		Completed:   []*render.Task{},
	}

	expectedMem := []string{"2.0", "2.0", "0.0", "0.0", "0.0"}
	expectedPercent := []string{"1.0", "1.0", "100.0", "100.0", "100.0"}
	expectedReturn := []ReturnValue{
		{State: "OK"},
		{State: "Error : No matching Renders"},
		{State: "OK"},
		{State: "Error : No matching Renders"},
		{State: "Error : Missing Parameter"}}
	expectedNodeState := []string{"rendering", "rendering", "available", "available", "available"}
	expectedState := []string{"rendering", "rendering", "rendered", "rendered", "rendered"}

	for i := 0; i < len(dataTab); i++ {

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
		assert.Equal(expectedMem[i], ws.Renders[0].Mem, "Bad value for memory in test %d", i)
		assert.Equal(expectedPercent[i], ws.Renders[0].Percent, "Bad value for percents in test %d", i)
		assert.Equal(expectedReturn[i], *dt, "Bad result returned in test %d", i)
		assert.Equal(expectedState[i], ws.Renders[0].myTask.State)
		assert.Equal(expectedNodeState[i], ws.Renders[0].myNode.State())

		if len(ws.Completed) > 0 {
			assert.Equal(1, len(ws.Completed), "Added too much element in completed : %d elements in test %d", len(ws.Completed), i)
			assert.Equal(ws.Completed[0], ws.Renders[0].myTask)
		}
	}

	os.RemoveAll("../../testdata/rendererapi_tests/updateJob")
}

func TestGetJob(t *testing.T) {

	assert := assert.New(t)

	dataTab := []url.Values{{}}

	os.MkdirAll("../../testdata/rendererapi_tests/getJob", os.ModePerm)
	copy("../../testdata/rendererapi_tests/testGetJob.sql", "../../testdata/rendererapi_tests/getJob/testGetJob.sql")

	//Creating request and recorder
	dataTab[0].Set("api_key", "test_api")

	//Creating ws for handling
	cg := Configuration{
		Folder:      "",
		DBName:      "../../testdata/rendererapi_tests/getJob/testGetJob.sql",
		Certname:    "",
		UserAPIKeys: []string{"test_api"},
	}

	db, _ := rendererdb.LoadDatabase(cg.DBName)

	nd := node.Node{
		Name:   "localhost",
		IP:     "127.0.0.1",
		APIKey: "test_api",
	}
	nd.SetState("available")

	tas := render.Task{
		Project:         "cube",
		ID:              "test_api",
		Input:           "cube.blend",
		Output:          "cube.blend",
		Frame:           127,
		State:           "waiting",
		RendererName:    "blender",
		RendererVersion: "2.91.0",
	}

	rd := Render{
		myTask:  &tas,
		myNode:  &nd,
		Percent: "",
		Mem:     "",
	}

	ws := WorkingSet{
		Db:          db,
		Config:      cg,
		RenderNodes: []*node.Node{&nd},
		Renders:     []*Render{},
		Uploading:   []*render.Task{},
		Waiting:     []*render.Task{&tas},
		Completed:   []*render.Task{},
	}

	tas2 := tas
	tas2.State = "rendering"

	expectedReturn := []render.Task{tas2}
	expectedNodeState := []string{"rendering"}

	for i := 0; i < len(dataTab); i++ {

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
		assert.Equal(expectedReturn[i], *dt, "Bad task returned")
		assert.Equal(expectedNodeState[i], ws.RenderNodes[0].State(), "Bad state assigned to node")

		if len(ws.Renders) > 0 {
			assert.Equal(rd, *ws.Renders[0])
		}
	}

	os.RemoveAll("../../testdata/rendererapi_tests/getJob")

}
