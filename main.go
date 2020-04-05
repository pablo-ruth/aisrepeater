package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"sync"
)

func main() {

	// List of readers + associated mutex
	var readers []chan string
	var readersMutex sync.RWMutex

	// NMEA reading from ACM tty
	go func() {

		// Open NMEA ACM tty
		file, err := os.Open("/dev/ttyACM0")
		if err != nil {
			fmt.Printf("failed to open ACM tty: %s", err)
			os.Exit(1)
		}
		defer file.Close()

		// Read tty line by line
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {

			// NMEA message
			msg := fmt.Sprintf("%s\r\n", scanner.Text())

			// Write message to all registered channels
			readersMutex.Lock()
			for _, reader := range readers {
				select {
				case reader <- msg:
				default:
					// If chan is full, drop oldest message
					// and append the new one
					<-reader
					reader <- msg
				}
			}
			readersMutex.Unlock()
		}
	}()

	// Start TCP server on port 2000
	l, err := net.Listen("tcp", ":2000")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer l.Close()

	// Create a new goroutine for each TCP connection
	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Printf("failed to accept incoming tcp connection: %s\n", err)
			continue
		}

		fmt.Printf("Serving %s\n", c.RemoteAddr().String())

		go func() {
			defer c.Close()

			// Create connection channel
			ch := make(chan string, 5)
			defer func() {
				// Remove chan from readers list and close it
				readersMutex.Lock()
				for i, reader := range readers {
					if reader == ch {
						readers[i] = readers[len(readers)-1]
						readers = readers[:len(readers)-1]
					}
				}
				readersMutex.Unlock()
				close(ch)
			}()

			// Register channel to readers list
			readersMutex.Lock()
			readers = append(readers, ch)
			readersMutex.Unlock()

			// Write each message received from channel
			for {
				msg := <-ch

				_, err := c.Write([]byte(msg))
				if err != nil {
					fmt.Printf("failed to write bytes: %s\n", err)
					break
				}
			}
		}()
	}
}
