package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func (ws *WebServer) setRoutes() {
	// mwMux applies the middleware chain to all registered routes.
	// Bypass middleware by registering route on top level mux: `ws.mux.Handle(...)`.
	mwMux := http.NewServeMux()
	ws.mux.Handle("/", ws.middlewareChain(mwMux))

	// home & team actions
	mwMux.HandleFunc("GET /{$}", ws.home)

	// refresh team automatically
	mwMux.HandleFunc("GET /game", ws.getGame)
	// start a new game, with all players set to 0.
	mwMux.HandleFunc("POST /game/start", ws.startGame)
	// pause a game, subbing off players.
	mwMux.HandleFunc("POST /game/pause", ws.pauseGame)
	// resume a game.
	mwMux.HandleFunc("POST /game/resume", ws.resumeGame)
	// end a game without resetting player statistics.
	mwMux.HandleFunc("POST /game/end", ws.endGame)
	// reset game.
	mwMux.HandleFunc("POST /game/reset", ws.resetGame)

	// players
	mwMux.HandleFunc("GET /players", ws.listPlayers)
	mwMux.HandleFunc("POST /players/{name}/reset", ws.resetPlayer)
	mwMux.HandleFunc("POST /players/{name}/set", ws.setPlayer)
	mwMux.HandleFunc("POST /players/{name}/sub-on", ws.subOnPlayer)
	mwMux.HandleFunc("POST /players/{name}/sub-off", ws.subOffPlayer)

	// static assets
	mwMux.Handle("GET /robots.txt", ws.HandleStaticFiles())
	mwMux.Handle("GET /favicon.ico", ws.HandleStaticFiles())
	mwMux.Handle("GET /static/", http.StripPrefix("/static", ws.HandleStaticFiles()))
}

func (ws *WebServer) home(w http.ResponseWriter, r *http.Request) {
	var poll bool
	switch ws.subber.Game.State() {
	case GameStateInProgress, GameStatePaused:
		poll = true
	default: // GameStateNotStarted, GameStateFinished
	}

	home := home(ws.subber.Game, ws.subber.ListPlayers(), poll)
	tc := layout("Go Subs", "Manage team subs", home)
	ws.renderTemplate(http.StatusOK, tc, w, r)
}

// getGame retrieves the current game.
func (ws *WebServer) getGame(w http.ResponseWriter, r *http.Request) {
	var poll bool
	switch ws.subber.Game.State() {
	case GameStateInProgress, GameStatePaused:
		poll = true
	default: // GameStateNotStarted, GameStateFinished
	}

	tc := game(ws.subber.Game, poll)
	ws.renderTemplate(http.StatusOK, tc, w, r)
}

// startGame starts a new game.
func (ws *WebServer) startGame(w http.ResponseWriter, r *http.Request) {
	ws.subber.StartGame()

	var poll bool
	switch ws.subber.Game.State() {
	case GameStateInProgress, GameStatePaused:
		poll = true
	default: // GameStateNotStarted, GameStateFinished
	}

	tc := home(ws.subber.Game, ws.subber.ListPlayers(), poll)
	ws.renderTemplate(http.StatusOK, tc, w, r)
}

// pauseGame pauses the game, subbing off all players.
func (ws *WebServer) pauseGame(w http.ResponseWriter, r *http.Request) {
	ws.subber.PauseGame()
	tc := home(ws.subber.Game, ws.subber.ListPlayers(), false)
	ws.renderTemplate(http.StatusOK, tc, w, r)
}

// resumeGame resumes the game.
func (ws *WebServer) resumeGame(w http.ResponseWriter, r *http.Request) {
	ws.subber.ResumeGame()
	tc := game(ws.subber.Game, true)
	ws.renderTemplate(http.StatusOK, tc, w, r)
}

// endGame stops the game.
func (ws *WebServer) endGame(w http.ResponseWriter, r *http.Request) {
	ws.subber.EndGame()
	tc := home(ws.subber.Game, ws.subber.ListPlayers(), false)
	ws.renderTemplate(http.StatusOK, tc, w, r)
}

// resetGame stops the game.
func (ws *WebServer) resetGame(w http.ResponseWriter, r *http.Request) {
	ws.subber.ResetGame()
	tc := home(ws.subber.Game, ws.subber.ListPlayers(), false)
	ws.renderTemplate(http.StatusOK, tc, w, r)
}

// listPlayers returns all player statistics.
func (ws *WebServer) listPlayers(w http.ResponseWriter, r *http.Request) {
	var poll bool
	switch ws.subber.Game.State() {
	case GameStateInProgress, GameStatePaused:
		poll = true
	default: // GameStateNotStarted, GameStateFinished
	}

	tc := playerStatistics(ws.subber.ListPlayers(), poll)
	ws.renderTemplate(http.StatusOK, tc, w, r)
}

// resetPlayer play count and duration to zero.
func (ws *WebServer) resetPlayer(w http.ResponseWriter, r *http.Request) {
	ws.logger.Info("form data", "path", r.URL.EscapedPath(), "data", r.Form.Encode())

	if err := r.ParseForm(); err != nil {
		ws.respondError(http.StatusBadRequest, fmt.Errorf("parsing form: %v", err), w, r)

		return
	}

	names := r.Form["playerName"]
	if len(names) == 0 {
		ws.respondError(http.StatusBadRequest, errors.New("player names not provided"), w, r)

		return
	}

	for _, name := range names {
		ws.subber.PlayerReset(name)
	}

	var poll bool
	switch ws.subber.Game.State() {
	case GameStateInProgress, GameStatePaused:
		poll = true
	default: // GameStateNotStarted, GameStateFinished
	}

	tc := playerStatistics(ws.subber.ListPlayers(), poll)
	ws.renderTemplate(http.StatusOK, tc, w, r)
}

// setPlayer to specified play count and duration.
func (ws *WebServer) setPlayer(w http.ResponseWriter, r *http.Request) {
	ws.logger.Info("form data", "path", r.URL.EscapedPath(), "data", r.Form.Encode())

	if err := r.ParseForm(); err != nil {
		ws.respondError(http.StatusBadRequest, fmt.Errorf("parsing form: %v", err), w, r)

		return
	}

	names := r.Form["playerName"]
	playCounts := r.Form["playCount"]
	playDurations := r.Form["playDuration"]

	if len(names) != len(playCounts) && len(names) != len(playDurations) {
		ws.respondError(http.StatusBadRequest, errors.New("all player values not provided"), w, r)

		return
	}

	for idx, name := range names {
		count, err := strconv.Atoi(playCounts[idx])
		if err != nil {
			ws.respondError(
				http.StatusBadRequest,
				fmt.Errorf("parsing play count, name: %s error: %v", name, err),
				w, r,
			)

			return
		}

		duration, err := time.ParseDuration(playDurations[idx])
		if err != nil {
			ws.respondError(
				http.StatusBadRequest,
				fmt.Errorf("parsing duration, name: %s error: %v", name, err),
				w, r,
			)

			return
		}

		ws.subber.PlayerSet(
			names[idx],
			count,
			duration,
		)
	}

	var poll bool
	switch ws.subber.Game.State() {
	case GameStateInProgress, GameStatePaused:
		poll = true
	default: // GameStateNotStarted, GameStateFinished
	}

	tc := playerStatistics(ws.subber.ListPlayers(), poll)
	ws.renderTemplate(http.StatusOK, tc, w, r)
}

// subOnPlayer increasing play count and resuming play duration timer.
func (ws *WebServer) subOnPlayer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	if name == "" {
		ws.respondError(http.StatusBadRequest, errors.New("player name not provided"), w, r)

		return
	}

	ws.subber.PlayerSubOn(name)
	tc := subButton(name, true)
	ws.renderTemplate(http.StatusOK, tc, w, r)
}

// subOffPlayer pausing play duration timer.
func (ws *WebServer) subOffPlayer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	if name == "" {
		ws.respondError(http.StatusBadRequest, errors.New("player name not provided"), w, r)

		return
	}

	ws.subber.PlayerSubOff(name)
	tc := subButton(name, false)
	ws.renderTemplate(http.StatusOK, tc, w, r)
}
