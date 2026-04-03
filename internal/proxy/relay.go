package proxy

import (
	"io"
	"net"
	"sync"
)

type closeWriter interface {
	CloseWrite() error
}

func relayBidirectional(left, right net.Conn, onLeftToRight func(int64), onRightToLeft func(int64)) (leftToRight int64, rightToLeft int64) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		leftToRight, _ = copyMetered(right, left, onLeftToRight)
		if cw, ok := right.(closeWriter); ok {
			_ = cw.CloseWrite()
		}
	}()

	go func() {
		defer wg.Done()
		rightToLeft, _ = copyMetered(left, right, onRightToLeft)
		if cw, ok := left.(closeWriter); ok {
			_ = cw.CloseWrite()
		}
	}()

	wg.Wait()
	return leftToRight, rightToLeft
}

func copyMetered(dst io.Writer, src io.Reader, onBytes func(int64)) (int64, error) {
	buf := make([]byte, 32*1024)
	var total int64
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[:nr])
			if nw > 0 {
				total += int64(nw)
				if onBytes != nil {
					onBytes(int64(nw))
				}
			}
			if ew != nil {
				return total, ew
			}
			if nw != nr {
				return total, io.ErrShortWrite
			}
		}
		if er != nil {
			if er == io.EOF {
				return total, nil
			}
			return total, er
		}
	}
}
