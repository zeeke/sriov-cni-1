package main

import (
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)
func main() {
	fmt.Println("Start")
	Acquire("README.md")
	fmt.Println("Acquired")
	time.Sleep(10*time.Second)
	fmt.Println("Finished")
}


// Acquire acquires a lock on a file for the duration of the process. This method
// is reentrant.
func Acquire(path string) error {
	fd, err := unix.Open(path, unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}

	// We don't need to close the fd since we should hold
	// it until the process exits.

	return unix.Flock(fd, unix.LOCK_EX)
}
