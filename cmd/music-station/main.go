package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/duythinht/shout/utube"
	"github.com/go-chi/chi/v5"

	"github.com/duythinht/shout/station"

	"github.com/duythinht/shout/shout"

	"golang.org/x/exp/slog"
)

func main() {
	ctx := context.Background()
	token := os.Getenv("SLACK_TOKEN")
	address := os.Getenv("SERVER_ADDRESS")

	if address == "" {
		address = "0.0.0.0:8000"
	}

	channelID := "C0UQ8TKLJ"

	s := station.New(token, channelID)

	playlist, err := s.History(ctx)
	qcheck(err)

	queue, err := s.Watch(ctx)
	qcheck(err)

	youtube := utube.New("./songs/")

	w := shout.New()

	defer w.Close()

	mux := chi.NewMux()

	mux.Get("/stream.mp3", w.ServeHTTP)

	mux.Get("/list.txt", func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		fmt.Fprintf(w, "# Songs in Queuing\n\n")
		for _, link := range queue.Links() {
			title, err := youtube.GetSongTitle(link)
			if err != nil {
				continue
			}

			fmt.Fprintf(w, "%s - %s\n", link, title)
		}
	})

	go func() {
		slog.Info("Starting server", slog.String("address", address))
		err := http.ListenAndServe(address, mux)
		qcheck(err)
	}()

	err = s.Welcome(ctx)
	qcheck(err)

	for {

		var link string

		if queue.Size() > 0 {
			// Get the link from queue, and then add it back to playlist for play later (when queue is empty)
			link = queue.Poll()
			playlist.Add(link)
		} else {
			// play suffle if don't have any music in queue
			link = playlist.Shuffle()
		}

		song, err := youtube.GetSong(ctx, link)

		if err != nil {
			// skip this song if get some error
			slog.Warn("get song", slog.String("error", err.Error()))
			continue
		}

		title := song.Video.Title

		slog.Info("Now Playing", slog.String("link", link), slog.String("title", title))

		err = s.SetNowPlaying(song.Video.Title)

		qcheck(err)

		err = w.StreamAll(song)

		if err != nil && !errors.Is(err, io.EOF) {
			qcheck(err)
		}

		song.Close()
	}
}

func qcheck(err error) {
	if err != nil {
		slog.Error("station", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
