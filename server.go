package main

import (
	"bufio"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
)

type Server struct {
	done           chan bool
	commands       chan string
	newConnections chan net.Conn
	directory      string
	executable     string
	listener       net.Listener
	connections    []net.Conn
}

func (s *Server) Run() error {
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {

		}
	}(s.listener)

	cmd := exec.Command("java", "-jar", s.executable, "nogui")
	cmd.Dir = s.directory

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	// Wait for server to quit and notify the server wrapper
	go func(cmd *exec.Cmd, s *Server) {
		err = cmd.Wait()
		if err != nil {
			log.Fatal(err)
		}
		s.done <- true
	}(cmd, s)

	go func(s *Server) {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			text := scanner.Text() + "\n" // Get the current line of text
			s.commands <- text
		}
	}(s)

	// Wait on new socket connections and send them  to the server wrapper
	go func() {
		for {
			conn, err := s.listener.Accept()
			if err != nil {
				log.Print("Here 1")
				log.Print(err)
			}
			s.newConnections <- conn
		}
	}()

	// Write the servers Stdout to all connections
	go func() {
		reader := bufio.NewReader(stdout)

		for {
			str, err := reader.ReadString('\n')
			if err != nil {
				log.Print("Here")
				log.Print(err)
			}
			log.Print("Writing to connections: " + str)

			idx := 0
			for _, conn := range s.connections {
				_, err := conn.Write([]byte(str))
				if err != nil && !errors.Is(err, net.ErrClosed) {
					log.Print(err)
				} else {
					// If a connection doesn't error, or is not closed bring it forward
					s.connections[idx] = conn
					idx++
				}
			}

			// Remove redundant or closed connections at the end of the list
			s.connections = s.connections[:idx]
		}
	}()

	for {
		select {
		case <-s.done:
			log.Print("Server closed")
			return err
		case cmd := <-s.commands:
			_, err := stdin.Write([]byte(cmd))
			if err != nil {
				return err
			}
		case conn := <-s.newConnections:
			// Keep track of the connections to write the Stdout content to them all later
			s.connections = append(s.connections, conn)

			// Read server console commands from the connected clients
			go func(conn net.Conn) {
				reader := bufio.NewReader(conn)
				for {
					command, err := reader.ReadString('\n')
					if err != nil {
						if err == io.EOF {
							log.Print("Console connection closed by client")
						} else {
							log.Printf("Error reading from connection: %v", err)
						}
						break // Exit the loop on EOF or any other error
					}
					s.commands <- command
				}
				// Perform cleanup here if necessary, such as removing the connection from a list
				// You might also want to close the connection on this side if it's not done yet
				conn.Close()
			}(conn)

		}
	}
}

func NewServer(directory string, executable string) (*Server, error) {
	os.Remove("/tmp/server_wrapper.sock")
	socket, err := net.Listen("unix", "/tmp/server_wrapper.sock")
	if err != nil {
		return nil, err
	}

	return &Server{done: make(chan bool), commands: make(chan string), newConnections: make(chan net.Conn), directory: directory, executable: executable, listener: socket, connections: []net.Conn{}}, nil
}
