package render

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetIndividualTasks(t *testing.T) {

	assert := assert.New(t)
	task0 := Task{
		Project:         "test_p",
		ID:              "test_id",
		Input:           "test_input",
		Output:          "test_output",
		Frame:           0,
		State:           "waiting",
		RendererName:    "test_renderer",
		RendererVersion: "test_version",
	}
	task1 := Task{
		Project:         "test_p",
		ID:              "test_id",
		Input:           "test_input",
		Output:          "test_output",
		Frame:           1,
		State:           "waiting",
		RendererName:    "test_renderer",
		RendererVersion: "test_version",
	}

	expected := []Task{task0, task1}

	vt := VideoTask{
		Project:         "test_p",
		ID:              "test_id",
		Input:           "test_input",
		Output:          "test_output",
		FrameStart:      0,
		FrameStop:       1,
		State:           "waiting",
		RendererName:    "test_renderer",
		RendererVersion: "test_version",
	}

	t0 := vt.GetIndividualTasks()
	assert.Equal(len(t0), 2, "Not good number of images")

	frames := []Task{*t0[0], *t0[1]}
	assert.Equal(frames, expected, "Bad Tasks from VideoTask")
}

func TestMatchRenderer(t *testing.T) {
	assert := assert.New(t)

	r0 := Renderer{
		Executable: "./blender",
		Name:       "blender",
		Version:    "2.83",
	}

	r1 := Renderer{
		Executable: "./blender",
		Name:       "blender",
		Version:    "2.82",
	}

	r := []Renderer{r0, r1}

	task0 := Task{
		Project:         "test_p",
		ID:              "test_id",
		Input:           "test_input",
		Output:          "test_output",
		Frame:           0,
		State:           "waiting",
		RendererName:    "blender",
		RendererVersion: "2.82",
	}

	rt := task0.MatchRenderer(r)

	assert.Equal(&r1, rt.Renderer, "Bad renderer for project")
	assert.Equal(&task0, rt.Task, "Task not returned in rendererTask")
}

func TestLaunchrender(t *testing.T) {
	assert := assert.New(t)

	r0 := Renderer{
		Executable: filepath.Join("..", "..", "testdata", "render_tests", "blender-2.91.0-linux64", "blender"),
		Name:       "blender",
		Version:    "2.91.0",
	}

	os.MkdirAll("tmp", os.ModePerm)
	defer os.RemoveAll("tmp")

	src, err := os.Open(filepath.Join("..", "..", "testdata", "render_tests", "cube.blend"))
	assert.Equal(nil, err, "Returned an error while copying render file")
	destination, err := os.Create(filepath.Join("tmp", "cube.blend"))
	assert.Equal(nil, err, "Returned an error while copying render file")
	_, err = io.Copy(destination, src)
	assert.Equal(nil, err, "Returned an error while copying render file")

	task0 := Task{
		Project:         "test_p",
		ID:              "test_id",
		Input:           filepath.Join("tmp", "cube.blend"),
		Output:          filepath.Join("tmp", "cube.blend"),
		Frame:           1,
		State:           "rendering",
		RendererName:    "blender",
		RendererVersion: "2.91.0",
	}

	rt0 := RendererTask{
		Task:     &task0,
		Renderer: &r0,
	}

	_, err = rt0.LaunchRender()

	assert.Equal(nil, err, "Returned an error while launchin rendering")
	assert.FileExists(filepath.Join("tmp", "cube.blend0001.png"))
}

func TestCheckRender(t *testing.T) {
	assert := assert.New(t)

	type testCheck struct {
		State string
		Frac  float64
		Mem   float64
	}

	test := new([3]testCheck)
	ref := new([3]testCheck)
	ref[0] = testCheck{
		State: "error",
		Frac:  0.0,
		Mem:   0.0,
	}
	ref[1] = testCheck{
		State: "rendering",
		Frac:  0.1,
		Mem:   2.94,
	}
	ref[2] = testCheck{
		State: "rendered",
		Frac:  1.0,
		Mem:   0.0,
	}

	r0 := Renderer{
		Executable: filepath.Join("..", "..", "testdata", "render_tests", "blender-2.91.0-linux64", "blender"),
		Name:       "blender",
		Version:    "2.91.0",
	}

	task0 := Task{
		Project:         "test_p",
		ID:              "test_id",
		Input:           filepath.Join("tmp", "cube.blend"),
		Output:          filepath.Join("tmp", "cube.blend"),
		Frame:           1,
		State:           "rendering",
		RendererName:    "blender",
		RendererVersion: "2.91.0",
	}

	rt0 := RendererTask{
		Task:     &task0,
		Renderer: &r0,
	}

	rt0.Log.WriteString(`Blender 2.91.0 (hash 0f45cab862b8 built 2020-11-25 08:51:08)
	Read prefs: /home/leobuntu/.config/blender/2.91/config/userpref.blend
	found bundled python: /home/leobuntu/Documents/go-renderer/testdata/render_tests/blender-2.91.0-linux64/2.91/python
	Read blend: /home/leobuntu/Documents/go-renderer/testdata/render_tests/cube.blend
	Fra:1 Mem:46.88M (Peak 46.90M) | Time:00:00.00 | Mem:0.00M, Peak:0.00M | Scene, View Layer | Synchronizing object | Cube
	Fra:1 Mem:46.88M (Peak 46.90M) | Time:00:00.00 | Mem:0.00M, Peak:0.00M | Scene, View Layer | Initializing`)

	test[0].State, test[0].Frac, test[0].Mem = rt0.CheckRender()

	rt0.Log.WriteString(`Fra:1 Mem:49.99M (Peak 50.25M) | Time:00:00.04 | Remaining:00:00.82 | Mem:2.94M, Peak:3.06M | Scene, View Layer | Rendered 6/510 Tiles
	Fra:1 Mem:49.99M (Peak 50.25M) | Time:00:00.04 | Remaining:00:00.83 | Mem:2.94M, Peak:3.06M | Scene, View Layer | Rendered 7/510 Tiles
	Fra:1 Mem:49.99M (Peak 50.25M) | Time:00:00.04 | Remaining:00:00.84 | Mem:2.94M, Peak:3.06M | Scene, View Layer | Rendered 8/510 Tiles
	Fra:1 Mem:49.99M (Peak 50.25M) | Time:00:00.04 | Remaining:00:00.82 | Mem:2.94M, Peak:3.06M | Scene, View Layer | Rendered 9/510 Tiles
	Fra:1 Mem:49.99M (Peak 50.25M) | Time:00:00.04 | Remaining:00:00.82 | Mem:2.94M, Peak:3.06M | Scene, View Layer | Rendered 10/510 Tiles
	Fra:1 Mem:49.99M (Peak 50.25M) | Time:00:00.05 | Remaining:00:00.79 | Mem:2.94M, Peak:3.06M | Scene, View Layer | Rendered 11/510 Tiles
	Fra:1 Mem:49.99M (Peak 50.25M) | Time:00:00.05 | Remaining:00:00.78 | Mem:2.94M, Peak:3.06M | Scene, View Layer | Rendered 12/510 Tiles
	Fra:1 Mem:49.99M (Peak 50.25M) | Time:00:00.05 | Remaining:00:00.79 | Mem:2.94M, Peak:3.06M | Scene, View Layer | Rendered 13/510 Tiles
	Fra:1 Mem:49.99M (Peak 50.25M) | Time:00:00.05 | Remaining:00:00.78 | Mem:2.94M, Peak:3.06M | Scene, View Layer | Rendered 14/510 Tiles
	Fra:1 Mem:49.99M (Peak 50.25M) | Time:00:00.05 | Remaining:00:00.78 | Mem:2.94M, Peak:3.06M | Scene, View Layer | Rendered 51/510 Tiles`)

	test[1].State, test[1].Frac, test[1].Mem = rt0.CheckRender()

	rt0.Log.WriteString(`Fra:1 Mem:48.58M (Peak 50.25M) | Time:00:01.50 | Mem:1.53M, Peak:3.06M | Scene, View Layer | Finished
	Fra:1 Mem:46.97M (Peak 50.25M) | Time:00:01.50 | Sce: Scene Ve:0 Fa:0 La:0
	Saved: 'cube.blend0001.png'
	 Time: 00:01.90 (Saving: 00:00.40)
	
	
	Blender quit`)

	test[2].State, test[2].Frac, test[2].Mem = rt0.CheckRender()

	assert.Equal(ref, test, "Not returned the good types after checkState")
}
