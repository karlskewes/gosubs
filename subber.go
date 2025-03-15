package main

import (
	"log/slog"
	"sort"
	"sync"
	"time"
)

type Period struct {
	StartTime time.Time
	EndTime   time.Time
}

type Game struct {
	StartTime time.Time
	EndTime   time.Time
	periods   []Period
}

type GameState string

const (
	GameStateNotStarted GameState = "not_started"
	GameStateInProgress GameState = "in_progress"
	GameStatePaused     GameState = "paused"
	GameStateFinished   GameState = "finished"
)

func (gs GameState) String() string {
	return string(gs)
}

// InProgress returns true when the game is currently underway.
func (g Game) State() GameState {
	switch {
	case len(g.periods) == 0:
		return GameStateNotStarted
	case len(g.periods) > 0 && g.periods[len(g.periods)-1].EndTime.IsZero():
		return GameStateInProgress
	case !g.StartTime.IsZero() && !g.EndTime.IsZero():
		return GameStateFinished
	default:
		// case len(g.periods) > 0 &&
		// !g.periods[len(g.periods)-1].EndTime.IsZero():
		return GameStatePaused
	}
}

// CurrentPeriod returns the current period.
func (g Game) CurrentPeriod() Period {
	p := Period{
		StartTime: time.Time{},
		EndTime:   time.Time{},
	}

	if len(g.periods) != 0 {
		idx := len(g.periods) - 1
		p.StartTime = g.periods[idx].StartTime
		p.EndTime = g.periods[idx].EndTime
	}

	return p
}

type Player struct {
	// Name of the player, expected to be unique.
	Name         string
	Number       int
	PlayCount    int
	PlayDuration time.Duration
	Playing      bool
	PlayStarted  time.Time
}

// Subber manages Player stastitcs.
type Subber struct {
	logger *slog.Logger
	Game   Game

	mu      sync.RWMutex
	players map[string]Player // map[name]Player
}

// General

// NewSubber returns a subber ready for the game.
func NewSubber(logger *slog.Logger, players []Player) *Subber {
	ps := make(map[string]Player)

	for _, player := range players {
		ps[player.Name] = player
	}

	return &Subber{
		logger:  logger,
		mu:      sync.RWMutex{},
		players: ps,
	}
}

// StartGame starts the game timer and resets all player statistics.
func (s *Subber) StartGame() {
	p := Period{
		StartTime: time.Now(),
		EndTime:   time.Time{},
	}
	s.Game.periods = append(s.Game.periods, p)

	s.Game.StartTime = time.Now()
	s.Game.EndTime = time.Time{}

	for _, p := range s.players {
		s.PlayerReset(p.Name)
	}
}

// PauseGame pauses the game clock and subs off all players.
func (s *Subber) PauseGame() {
	idx := len(s.Game.periods) - 1
	s.Game.periods[idx].EndTime = time.Now()

	for _, p := range s.players {
		s.PlayerSubOff(p.Name)
	}
}

// ResumeGame resumes the game.
func (s *Subber) ResumeGame() {
	p := Period{
		StartTime: time.Now(),
		EndTime:   time.Time{},
	}
	s.Game.periods = append(s.Game.periods, p)

	for _, p := range s.players {
		s.PlayerSubOff(p.Name)
	}
}

// EndGame stops the game clock and subs off all players. It does not reset statistics.
func (s *Subber) EndGame() {
	idx := len(s.Game.periods) - 1
	s.Game.periods[idx].EndTime = time.Now()
	s.Game.EndTime = time.Now()

	for _, p := range s.players {
		s.PlayerSubOff(p.Name)
	}
}

// ResetGame stops the game clock and resets all player statistics.
func (s *Subber) ResetGame() {
	s.Game.StartTime = time.Time{}
	s.Game.EndTime = time.Time{}
	s.Game.periods = []Period{}

	for _, p := range s.players {
		s.PlayerReset(p.Name)
	}
}

// ListPlayers returns all player statistics.
func (s *Subber) ListPlayers() []Player {
	s.mu.RLock()
	defer s.mu.RUnlock()

	players := make([]Player, 0, len(s.players))
	for _, p := range s.players {
		if p.Playing {
			d := time.Since(p.PlayStarted)
			p.PlayDuration = time.Duration(p.PlayDuration.Nanoseconds() + d.Nanoseconds())
		}

		players = append(players, p)
	}

	sort.Slice(players, func(i, j int) bool {
		return players[i].Name < players[j].Name
	})

	return players
}

// Per Player

// PlayerReset zero's a players game time and play count.
func (s *Subber) PlayerReset(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.players[name]
	if !ok {
		s.logger.Warn("attempt to reset non-existent player", "player", name)

		return
	}

	p.PlayCount = 0
	p.PlayDuration = 0
	p.Playing = false
	p.PlayStarted = time.Time{}
	s.players[name] = p
}

// PlayerSet updates a players game time and play count to the provided values.
func (s *Subber) PlayerSet(name string, playCount int, playDuration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.players[name]
	if !ok {
		return
	}

	p.PlayCount = playCount
	p.PlayDuration = playDuration
	s.players[name] = p
}

// PlayerSubOn a player, increment their play count and starting or resuming play duration timer.
func (s *Subber) PlayerSubOn(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.players[name]
	if !ok {
		s.logger.Warn("attempt to sub on non-existent player", "player", name)

		return
	}

	p.Playing = true
	p.PlayCount++
	p.PlayStarted = time.Now()
	s.players[name] = p
}

// PlayerSubOff a player, pausing play duration timer.
func (s *Subber) PlayerSubOff(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.players[name]
	if !ok {
		s.logger.Warn("attempt to sub off non-existent player", "player", name)

		return
	}

	// calculate time playing
	if !p.PlayStarted.IsZero() {
		d := time.Since(p.PlayStarted)
		p.PlayDuration = time.Duration(p.PlayDuration.Nanoseconds() + d.Nanoseconds())
	}

	p.Playing = false
	p.PlayStarted = time.Time{}
	s.players[name] = p
}
