package render

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
)

//Task is the base descriptor of a Render Task
type Task struct {
	Project         string `json:"project"`
	ID              string `json:"id"`
	Input           string `json:"input"`
	Output          string `json:"output"`
	Frame           int    `json:"frame"`
	State           string `json:"state"`
	RendererName    string `json:"rendererName"`
	RendererVersion string `json:"rendererVersion"`
	StartTime       string `json:"startTime"`
	sync.Mutex
}

//VideoTask is the base descriptor for a Video Render Task
type VideoTask struct {
	Project         string `json:"project"`
	ID              string `json:"id"`
	Input           string `json:"input"`
	Output          string `json:"output"`
	FrameStart      int    `json:"frameStart"`
	FrameStop       int    `json:"frameStop"`
	State           string `json:"state"`
	RendererName    string `json:"rendererName"`
	RendererVersion string `json:"rendererVersion"`
	StartTime       string `json:"startTime"`
}

//Renderer is a struct that contains path to blender and version/name of it
type Renderer struct {
	Executable string `json:"executable"`
	Version    string `json:"version"`
	Name       string `json:"name"`
}

//RendererTask describes a Task and its associated Renderer
type RendererTask struct {
	Task     *Task
	Renderer *Renderer
	Log      bytes.Buffer
}

//GetIndividualTasks returns an array of all the tasks of vt frames by frames
func (vt *VideoTask) GetIndividualTasks() []*Task {
	var tasks []*Task
	for i := vt.FrameStart; i <= vt.FrameStop; i++ {
		itask := &Task{
			Project:         vt.Project,
			ID:              vt.ID,
			Input:           vt.Input,
			Output:          vt.Output,
			Frame:           i,
			State:           vt.State,
			RendererName:    vt.RendererName,
			RendererVersion: vt.RendererVersion,
			StartTime:       vt.StartTime,
		}
		tasks = append(tasks, itask)
	}
	return tasks
}

//MatchRenderer tries to match a Task with its renderer and returns a pointer to a RendererTask or nil if not found
func (t *Task) MatchRenderer(rTable []Renderer) *RendererTask {
	for i := 0; i < len(rTable); i++ {
		if rTable[i].Name == t.RendererName && rTable[i].Version == t.RendererVersion {
			return &RendererTask{t, &rTable[i], *new(bytes.Buffer)}
		}
	}

	return nil
}

func (t *Task) SetState(state string) {
	t.Lock()
	t.State = state
	t.Unlock()
}

//LaunchRender launches the render on the renderer
func (rt *RendererTask) LaunchRender() (*exec.Cmd, error) {
	if rt.Renderer.Name == "blender" {
		fmt.Println(rt.Renderer.Executable, rt.Task.Input, rt.Task.Output, rt.Task.Frame)
		cmd := exec.Command(rt.Renderer.Executable, "-b", "-noaudio", rt.Task.Input, "-o", rt.Task.Output+"#####", "-F", "PNG", "-f", strconv.Itoa(rt.Task.Frame))
		cmd.Stdout = &rt.Log
		a := cmd.Start()
		return cmd, a
	}
	return nil, errors.New("no matching renderer")
}

//CheckRender check the state of the render and returns state, percentage, memory use of the render
func (rt *RendererTask) CheckRender() (string, float64, float64) {
	if rt.Renderer.Name == "blender" {

		rN := 0.0
		rD := 10.0
		rM := 0.0
		var err error

		str := rt.Log.String()
		state, _, _, mem, _, renderedNum, renderedDenom := CheckBlenderRender(str)
		if state != "" && renderedNum != "" && renderedDenom != "" {
			rN, err = strconv.ParseFloat(renderedNum, 64)
			if err != nil {
				log.Fatal(err)
			}
			rD, err = strconv.ParseFloat(renderedDenom, 64)
			if err != nil {
				log.Fatal(err)
			}
			rM, err = strconv.ParseFloat(mem, 64)
			if err != nil {
				log.Fatal(err)
			}
		}

		return state, rN / rD, rM
	}

	return "", 0, 0
}

//CheckBlenderRender checks the status of the render and returns stats if currently rendering
func CheckBlenderRender(test string) (string, string, string, string, string, string, string) {
	substRendering := `Time:([\d|:|.]+) \| Remaining:([\d|:|.]+) \| Mem:([\d|:|.]+)M, Peak:([\d|:|.]+)M .* Rendered ([\d]+)/([\d]+) Tiles`
	substRendered := "\\| Finished"
	substQuit := "Blender quit"

	status := "error"
	var time, remaining, mem, peak, renderedNum, renderedDenom string

	rendering := regexp.MustCompile(substRendering)
	rendered := regexp.MustCompile(substRendered)
	quitted := regexp.MustCompile(substQuit)

	if rendering.MatchString(test) {

		status = "rendering"

		tab := rendering.FindAllStringSubmatch(test, -1)
		lastline := tab[len(tab)-1]
		time = lastline[1]
		remaining = lastline[2]
		mem = lastline[3]
		peak = lastline[4]
		renderedNum = lastline[5]
		renderedDenom = lastline[6]
	}

	if rendered.MatchString(test) && quitted.MatchString(test) {
		status = "rendered"
		renderedNum = "1"
		renderedDenom = "1"
		mem = "0"
	}

	return status, time, remaining, mem, peak, renderedNum, renderedDenom
}
