package filexchange

import (
	"io/fs"
	"log"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
)

func handleSender(conn net.Conn, l int, id, fileName, filesFolder string) {

	// Make the folder if not exists
	err := os.MkdirAll(path.Join(filesFolder, id), 0777) //TODO: change to a better perm value

	if err != nil {
		conn.Write([]byte("ABORT"))
		return
	}

	// Create the file
	f, err := os.Create(path.Join(filesFolder, id, fileName))
	if err != nil {
		conn.Write([]byte("ABORT"))
		return
	}

	defer f.Close()

	// Receive the file
	total := 0
	var rcvbuffer [1024]byte
	conn.Write([]byte("READY"))

	for total < l {
		n, err := conn.Read(rcvbuffer[:])
		if err != nil {
			conn.Write([]byte("ABORT"))
			return
		}

		total += n

		f.Write(rcvbuffer[:n])
	}
	conn.Write([]byte("SUCCESS"))
}

func handleReceiver(conn net.Conn, id, fileName, filesFolder string) {

	// Checks if file is available and open it
	filepath := path.Join(filesFolder, id, fileName)

	var st fs.FileInfo
	var err error

	if st, err = os.Stat(filepath); err != nil {
		conn.Write([]byte("ABORT"))
		return
	}

	f, err := os.Open(filepath)
	if err != nil {
		conn.Write([]byte("ABORT"))
		return
	}

	// Ready to Send
	conn.Write([]byte("READY " + strconv.FormatInt(st.Size(), 10)))

	var readbuffer [1024]byte
	n, err := conn.Read(readbuffer[:])

	if err != nil || string(readbuffer[:n]) != "GO" {
		conn.Write([]byte("ABORT"))
		return
	}

	// Send to the end of file
	var ferr error = nil
	var sendbuffer [1024]byte

	for ferr == nil && n > 0 {
		n, ferr = f.Read(sendbuffer[:])
		_, err := conn.Write(sendbuffer[:n])
		if err != nil {
			conn.Write([]byte("ABORT"))
			return
		}
	}

	// Wait for success
	n, err = conn.Read(readbuffer[:])
	if err != nil || string(readbuffer[:n]) != "SUCCESS" {
		conn.Write([]byte("ABORT"))
		return
	}

}

func handleClient(conn net.Conn, filesFolder string) {
	var buf [1024]byte
	n, err := conn.Read(buf[0:])
	if err != nil {
		conn.Write([]byte("ABORT"))
		return
	}
	instr := strings.Split(string(buf[0:n]), " ")

	switch instr[0] {
	case "SEND":
		if len(instr) != 4 {
			conn.Write([]byte("ABORT"))
			return
		}
		l, err := strconv.Atoi(instr[1])
		if err != nil {
			conn.Write([]byte("ABORT"))
			return
		}
		handleSender(conn, l, instr[2], instr[3], filesFolder)
	case "RECEIVE":
		if len(instr) != 3 {
			conn.Write([]byte("ABORT"))
			return
		}
		handleReceiver(conn, instr[1], instr[2], filesFolder)
	}
}

func StartListening(filesList *[]string, filesFolder string, mustStop *bool) {
	service := ":9005"
	tcpAddr, err := net.ResolveTCPAddr("tcp4", service)
	if err != nil {
		log.Fatal(err)
	}

	listener, err := net.ListenTCP("rcp", tcpAddr)
	if err != nil {
		log.Fatal(err)
	}

	for !(*mustStop) {
		conn, _ := listener.Accept()
		go func() {
			handleClient(conn, filesFolder)
			conn.Close()
		}()
	}

	listener.Close()
}
