package main

import (
	"fmt"
)

type coords struct {
	x, y int
}

func (c coords) String() string {
	if c.x < 0 || 7 < c.x || c.y < 0 || 7 < c.y {
		panic(fmt.Sprint("Illegal coordinates (%d, %d)", c.x, c.y))
	}
	return fmt.Sprintf("%c%d", "ABCDEFGH"[c.x], c.y+1)
}

type pieceType int

const (
	PAWN pieceType = iota
	KNIGHT
	BISHOP
	ROOK
	QUEEN
	KING
)

func (pt pieceType) String() string {
	switch pt {
	case PAWN:
		return "pawn"
	case KNIGHT:
		return "knight"
	case BISHOP:
		return "bishop"
	case ROOK:
		return "rook"
	case QUEEN:
		return "queen"
	case KING:
		return "king"
	}
	panic("Illegal piece type")
}

// Operations on pieces
type pop interface{}

type popGetCoords chan<- coords

type popSetCoords coords

// Die. Close this channel when operation acknowledged (for sync)
type popKill chan<- bool

// Subscribe to moves by request all coordinates updates be sent down here.
// Send nil channel to cancel.
type popMoveCallback chan<- coords

type popSetType pieceType

type popGetType chan<- pieceType

// Operations on a chess board
type bop interface{}

type bopSetPiece struct {
	coords
	ctrl chan<- pop
}

// Control operations are read from the control channel.
func spawnPiece(c <-chan pop) {
	var x, y int
	var movechan chan<- coords
	defer func() {
		if movechan != nil {
			close(movechan)
		}
	}()
	var pt pieceType
	for op := range c {
		switch t := op.(type) {
		case popSetCoords:
			x = t.x
			y = t.y
			if movechan != nil {
				movechan <- coords(t)
			}
		case popGetCoords:
			t <- coords{x, y}
			close(t)
		case popKill:
			close(t)
			return
		case popMoveCallback:
			movechan = t
		case popSetType:
			pt = pieceType(t)
		case popGetType:
			t <- pt
			close(t)
		default:
			panic(fmt.Sprintf("Illegal operation: %v", op))
		}
	}
}

func addPawn(x, y int, mu chan<- bop) chan<- pop {
	c := make(chan pop)
	// Start a block
	go spawnPiece(c)
	// piece will push updates to coordinates down this channel
	coordUpdates := make(chan coords)
	c <- popMoveCallback(coordUpdates)
	// Translate those updates to a message that includes the control channel
	go func() {
		for xy := range coordUpdates {
			mu <- bopSetPiece{xy, c}
		}
	}()
	// Make it a pawn
	c <- popSetType(PAWN)
	// Move it to the desired coordinates
	c <- popSetCoords{x, y}
	return c
}

// Run a board management unit. Push all location changes down this channel.
// Closes the done channel when all updates have been consumed and the input
// channel is closed (for sync).
func runBoard(c <-chan bop, done chan<- bool) {
	for o := range c {
		switch t := o.(type) {
		case bopSetPiece:
			pc := make(chan pieceType)
			t.ctrl <- popGetType(pc)
			fmt.Printf("Moved %s to %s\n", <-pc, t.coords)
			break
		default:
			panic(fmt.Sprintf("Illegal board operation: %v", o))
		}
	}
	close(done)
}

// Initialize an empty chess board by putting pieces in the right places
func initBoard(c chan<- bop) {
	addPawn(0, 1, c)
	addPawn(1, 1, c)
	addPawn(2, 1, c)
	addPawn(3, 1, c)
	addPawn(4, 1, c)
	addPawn(5, 1, c)
	addPawn(6, 1, c)
	addPawn(7, 1, c)
	// TODO: Other pieces
	// TODO: adversary
}

func main() {
	boardc := make(chan bop)
	boarddone := make(chan bool)
	go runBoard(boardc, boarddone)
	initBoard(boardc)
	// TODO: clearBoard()...
}
