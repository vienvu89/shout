package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dmulholl/mp3lib"
	"github.com/duythinht/shout/shout"
	"golang.org/x/exp/slog"
)

var (
	ErrWriteTimeout = errors.New("write timeout")
)

const (
	StreamDataTimeout = 200 * time.Millisecond
)

type chunk struct {
	data []byte
	t    time.Duration
}

type Streamer struct {
	_data  chan []byte
	_chunk chan *chunk
	r      *io.PipeReader
	w      *io.PipeWriter
}

func (s *Streamer) NextChunk() *chunk {
	return <-s._chunk
}

func open() *Streamer {

	r, w := io.Pipe()

	return &Streamer{
		_data:  make(chan []byte),
		_chunk: make(chan *chunk),
		r:      r,
		w:      w,
	}
}

func (s *Streamer) Write(data []byte) (int, error) {
	s._data <- data
	return len(data), nil
}

func (s *Streamer) Read(p []byte) (n int, err error) {
	return s.r.Read(p)
}

func (s *Streamer) Stream(_ context.Context) {
	go func() {
		for {
			var data []byte
			t := 0

			// each playback stream 0 frame
			for i := 0; i < 50; i++ {
				frame := mp3lib.NextFrame(s.r)
				if frame == nil {
					continue
				}

				data = append(data, frame.RawBytes...)
				t += int(time.Second) * frame.SampleCount / frame.SamplingRate
			}

			duration := time.Duration(t)

			s._chunk <- &chunk{
				data: data,
				t:    duration,
			}
			time.Sleep(duration)
		}
	}()

	for {
		select {
		case data := <-s.data():
			_, err := s.w.Write(data)
			if err != nil {
				slog.Warn("stream, write mp3", slog.String("error", err.Error()))
			}
		case <-time.After(StreamDataTimeout):
			//slog.Warn("no stream data", slog.Duration("timeout", StreamDataTimeout))
			s._chunk <- &chunk{
				data: nil,
				t:    StreamDataTimeout,
			}
		}
	}
}

func (s *Streamer) data() <-chan []byte {
	return s._data
}

func main() {

	files := []string{
		"1tBlaVjWwbI.mp3",
		"_8vekzCF04Q.mp3",
	}

	_ = files

	s := shout.OpenStreamer()

	go s.Stream(context.Background())

	go func() {
		for {
			chunked := s.NextChunk()
			fmt.Printf("timeout %s\n", chunked.Duration())
		}
	}()

	//time.Sleep(10 * time.Second)

	for _, filename := range files {

		f, err := os.Open("./songs/" + filename)

		if err != nil {
			panic(err)
		}

		io.Copy(s, f)

		time.Sleep(2 * time.Second)
	}
}
