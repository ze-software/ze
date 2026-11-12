package packer

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"syscall"
)

func (p *Pack) partitionDevice(o *os.File, path string) error {
	devsize, err := deviceSize(uintptr(o.Fd()))
	if err != nil {
		return err
	}
	log.Printf("device holds %d bytes", devsize)
	if devsize == 0 {
		return fmt.Errorf("path %s does not seem to be a device", path)
	}

	if err := p.Partition(o, devsize); err != nil {
		return err
	}

	return p.RereadPartitions(o)
}

func unixConn(fd uintptr) (*net.UnixConn, error) {
	file := os.NewFile(fd, "")
	if file == nil {
		return nil, fmt.Errorf("invalid socket descriptor %d", fd)
	}
	fc, err := net.FileConn(file)
	closeErr := file.Close()
	if err != nil {
		return nil, errors.Join(err, closeErr)
	}
	if closeErr != nil {
		return nil, errors.Join(closeErr, fc.Close())
	}
	conn, ok := fc.(*net.UnixConn)
	if !ok {
		return nil, errors.Join(fmt.Errorf("descriptor %d is not a Unix socket", fd), fc.Close())
	}
	return conn, nil
}

func (p *Pack) partition(path string) (*os.File, error) {
	if p.Cfg.InternalCompatibilityFlags.SudoOrDefault() == "always" {
		return p.SudoPartition(path)
	}
	o, err := os.Create(path)
	if err != nil {
		pe, ok := err.(*os.PathError)
		if ok && pe.Err == syscall.EACCES && p.Cfg.InternalCompatibilityFlags.SudoOrDefault() == "auto" {
			// permission denied
			log.Printf("Using sudo to gain permission to format %s", path)
			if runtime.GOOS == "linux" {
				log.Printf("If you prefer, cancel and use: sudo setfacl -m u:${USER}:rw %s", path)
			}
			return p.SudoPartition(path)
		}
		if ok && pe.Err == syscall.EROFS {
			log.Printf("%s read-only; check if you have a physical write-protect switch on your SD card?", path)
			return nil, err
		}
		return nil, err
	}
	return o, p.partitionDevice(o, path)
}
