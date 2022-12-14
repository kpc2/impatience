package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type SaveData struct {
	Stock struct {
		Limit int
		Loop  int
		Pos   int
		Stack []string
	}
	Tableau struct {
		Stacks   [][]string
		Facedown []int
	}
	Foundations map[string][]string
}

func LoadFile(path string) (*SaveData, error) {
	save := new(SaveData)

	// Open file.
	file, openerr := os.Open(path)
	if openerr != nil {
		return nil, openerr
	}
	defer file.Close()

	// Read file into memory.
	contents, readerr := io.ReadAll(file)
	if readerr != nil {
		return nil, readerr
	}

	// Check if format is JSON or TOML.
	var unmarsherr error
	ext := filepath.Ext(path)
	switch {
	case ext == ".json":
		unmarsherr = json.Unmarshal(contents, save)
	case ext == ".toml":
		unmarsherr = toml.Unmarshal(contents, save)
	}
	if unmarsherr != nil {
		return nil, unmarsherr
	}

	return save, nil
}

// Load game from file
// TODO: Add unit tests.
func (game *Game) Import(save *SaveData) error {
	r := NewRegister()

	// Load stock from save data.
	game.Stock.Limit = save.Stock.Limit
	game.Stock.Loop = save.Stock.Loop
	game.Stock.Pos = save.Stock.Pos
	if stack, err := r.AddCards(save.Stock.Stack); err != nil {
		return err
	} else {
		game.Stock.Stack = stack
	}

	// Load tableau.
	var fdTotal int // Count total facedown cards
	tbSize := len(save.Tableau.Stacks)
	if tbSize > 7 {
		return fmt.Errorf("Number of stacks in tableau exceed max of 7 with %d stacks.", tbSize)
	}
	if len(save.Tableau.Facedown) != tbSize {
		return errors.New("tableau.stacks and tableau.facedown lengths do not match.")
	}
	for i, codes := range save.Tableau.Stacks {
		facedown := save.Tableau.Facedown[i]
		fdTotal += facedown
		if facedown >= len(codes) {
			return fmt.Errorf("Tableau %d is invalid: Top card must not be facedown: %d cards; %d facedown.", i, len(codes), facedown)
		}
		if stack, err := r.AddCards(codes); err != nil {
			return err
		} else {
			game.Tableau.Stacks[i] = stack
			game.Tableau.Facedown[i] = facedown
		}
	}
	if fdTotal > 21 {
		return fmt.Errorf("Facedown cards exceed max of 21 with %d cards.", fdTotal)
	}

	// Load foundations.
	for key, codes := range save.Foundations {
		var suit CardSuit
		switch {
		case key == "spades":
			suit = SPADES
		case key == "clubs":
			suit = CLUBS
		case key == "hearts":
			suit = HEARTS
		case key == "diamonds":
			suit = DIAMONDS
		default:
			return errors.New("Unrecognized foundation name: " + key)
		}

		size := len(codes)
		stack := make([]*Card, size, size)
		for i, code := range codes {
			card, err := r.AddCard(code)
			if err != nil {
				return err
			}
			if card.Suit == suit {
				stack[i] = card
			} else {
				return fmt.Errorf("Suit mismatch in %s foundation: %s at index %d", key, code, i)
			}
		}
		game.Foundations[suit] = stack
	}
	if r.Total != 52 {
		return fmt.Errorf("Found %d cards. Game requires 52 total cards.", r.Total)
	}

	return nil
}

type Register struct {
	Cards map[string]struct{}
	Suits map[CardSuit]int
	Ranks map[CardRank]int
	Total int
}

func NewRegister() *Register {
	var r Register
	r.Cards = make(map[string]struct{})
	r.Suits = make(map[CardSuit]int)
	r.Ranks = make(map[CardRank]int)
	return &r
}

func (r *Register) AddCard(code string) (card *Card, err error) {
	card, err = ParseCard(code)
	if err != nil {
		return nil, err
	}

	// Prevent duplicate cards.
	id := card.Id()
	if !strings.Contains(id, "?") {
		if _, set := r.Cards[id]; set {
			return nil, errors.New("Found duplicate card.")
		} else {
			r.Cards[id] = struct{}{}
		}
	}

	// Prevent invalid deck.
	var (
		suitTotal, rankTotal int
		ok                   bool
	)
	invalids := make([]string, 0, 3)

	// Count all cards.
	r.Total++
	if r.Total > 52 {
		r.Total--
		invalids = append(invalids, "too many cards.")
	}

	// Count cards by suit.
	suitTotal, ok = r.Suits[card.Suit]
	if ok {
		suitTotal++
		r.Suits[card.Suit] = suitTotal
	} else {
		r.Suits[card.Suit] = 1
	}
	if suitTotal > 13 {
		suitTotal--
		invalids = append(invalids,
			fmt.Sprint("too many", SuitName(card.Suit), "cards"),
		)
	}

	// Count cards by rank.
	rankTotal, ok = r.Ranks[card.Rank]
	if ok {
		rankTotal++
		r.Ranks[card.Rank] = rankTotal
	} else {
		r.Ranks[card.Rank] = 1
	}
	if rankTotal > 4 {
		rankTotal--
		invalids = append(invalids,
			fmt.Sprint("too many", RankName(card.Rank), "cards"),
		)
	}

	// check for errors.
	if len(invalids) > 0 {
		return nil, errors.New(strings.Join(invalids, ", ") + ".")
	}

	return
}

func (r *Register) AddCards(codes []string) (card []*Card, err error) {
	size := len(codes)
	stack := make([]*Card, size, size)
	for i, code := range codes {
		card, err := r.AddCard(code)
		if err != nil {
			return nil, err
		}
		stack[i] = card
	}
	return stack, nil
}
