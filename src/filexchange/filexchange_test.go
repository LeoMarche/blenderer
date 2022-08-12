package filexchange

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

func TestReceive(t *testing.T) {
	assert := assert.New(t)

	returnStatus := []string{"READY", "ABORT", "ABORT", "READY"}
	destinationFiles := []string{path.Join("tmp", "dummy_received.txt"), path.Join("tmp", "dummy_received_2.txt"), path.Join("tmp", "dummy_received_3.txt"), path.Join("tmp", "dummy_received_4.txt")}
	IDs := []string{"dummy_id", "dummy_id_2", "dummy_id", "dummy_id"}
	srcFiles := []string{"dummy.txt", "dummy.txt", "dummy_2.txt", "dummy3.txt"}

	os.MkdirAll(path.Join("tmp", "dummy_id"), os.FileMode(0777))
	defer os.RemoveAll("tmp")
	_, err := copy(path.Join("..", "..", "testdata", "filexchange_tests", "dummy.txt"), path.Join("tmp", "dummy_id", "dummy.txt"))
	assert.NoError(err)
	_, err = copy(path.Join("..", "..", "testdata", "filexchange_tests", "dummy3.txt"), path.Join("tmp", "dummy_id", "dummy3.txt"))
	assert.NoError(err)

	for i := 0; i < len(returnStatus); i++ {
		b := new(bool)
		*b = false

		go StartListening("tmp", b)

		c, err := net.Dial("tcp", "localhost:9005")
		assert.NoError(err)

		toSend := []byte("RECEIVE " + IDs[i] + " " + srcFiles[i])
		_, err = c.Write(toSend)
		assert.NoError(err)

		dst := destinationFiles[i]
		destination, err := os.Create(dst)
		assert.NoError(err)

		buf := make([]byte, 1024)
		n, err := c.Read(buf)
		assert.NoError(err)
		instr := strings.Split(string(buf[:n]), " ")
		assert.Equal(2, len(instr))

		status := instr[0]
		ln, err := strconv.ParseInt(instr[1], 10, 64)
		assert.NoError(err)

		buf = make([]byte, 1024)
		var nRead int64
		nRead = 0

		assert.Equal(returnStatus[i], status)

		if status == "READY" {
			_, err := c.Write([]byte("GO"))
			assert.NoError(err)

			for nRead != ln {
				n, err := c.Read(buf)
				assert.NoError(err)
				nRead += int64(n)
				destination.Write(buf[:n])
			}

			assert.Equal(ln, nRead)
			_, err = c.Write([]byte("SUCCESS"))
			assert.NoError(err)

			f1, err1 := ioutil.ReadFile(path.Join("tmp", IDs[i], srcFiles[i]))
			assert.NoError(err1)
			f2, err2 := ioutil.ReadFile(destinationFiles[i])
			assert.NoError(err2)

			assert.Equal(true, bytes.Equal(f1, f2))
		}
		*b = true
	}
}

func TestSend(t *testing.T) {
	assert := assert.New(t)

	returnStatus := []string{"READY"}
	destinationFiles := []string{path.Join("tmp", "dummy_id", "dummy.txt")}
	IDs := []string{"dummy_id"}
	srcFiles := []string{"dummy.txt"}

	os.MkdirAll("tmp", os.FileMode(0777))
	defer os.RemoveAll("tmp")

	_, err := copy(path.Join("..", "..", "testdata", "filexchange_tests", "dummy.txt"), path.Join("tmp", "dummy.txt"))
	assert.NoError(err)

	for i := 0; i < len(returnStatus); i++ {
		b := new(bool)
		*b = false

		go StartListening("tmp", b)

		c, err := net.Dial("tcp", "localhost:9005")
		assert.NoError(err)

		st, err := os.Stat(path.Join("tmp", srcFiles[i]))
		assert.NoError(err)

		toSend := []byte("SEND " + strconv.FormatInt(st.Size(), 10) + " " + IDs[i] + " " + srcFiles[i])
		_, err = c.Write(toSend)
		assert.NoError(err)

		src, err := os.Open(path.Join("tmp", srcFiles[i]))
		assert.NoError(err)

		buf := make([]byte, 1024)
		n, err := c.Read(buf)
		assert.NoError(err)
		instr := string(buf[:n])

		assert.Equal(returnStatus[i], instr)

		buf = make([]byte, 1024)
		var nRead int64
		nRead = 0

		if instr == "READY" {

			for nRead != st.Size() {
				n, err := src.Read(buf)
				assert.NoError(err)
				nRead += int64(n)
				c.Write(buf[:n])
			}

			buf := make([]byte, 1024)
			n, err := c.Read(buf)
			assert.NoError(err)
			fin := string(buf[:n])

			assert.Equal("SUCCESS", fin)

			f1, err1 := ioutil.ReadFile(path.Join("tmp", srcFiles[i]))
			assert.NoError(err1)
			f2, err2 := ioutil.ReadFile(destinationFiles[i])
			assert.NoError(err2)

			assert.Equal(true, bytes.Equal(f1, f2))
		}
		*b = true
	}
}
